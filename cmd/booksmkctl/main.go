package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"go.e64ec.com/booksmk/internal/migrate"
	"go.e64ec.com/booksmk/internal/store"
	"go.e64ec.com/booksmk/internal/urlfetch"
	"go.e64ec.com/booksmk/sql/migrations"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: booksmkctl <command>")
		fmt.Fprintln(os.Stderr, "commands:")
		fmt.Fprintln(os.Stderr, "  backfill-feed-urls    scan existing bookmarks for feed URLs")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "backfill-feed-urls":
		if err := runBackfillFeedURLs(logger); err != nil {
			logger.Error("backfill failed", "error", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runBackfillFeedURLs(logger *slog.Logger) error {
	dbURL := mustEnv("BOOKSMK_DATABASE_URL")

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	if err := migrate.Run(context.Background(), pool, migrations.FS, logger); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	st := store.New(pool)

	urls, err := st.ListURLsForFeedBackfill(context.Background())
	if err != nil {
		return fmt.Errorf("list urls: %w", err)
	}

	logger.Info("scanning urls for feed links", "count", len(urls))

	var (
		updated atomic.Int64
		skipped atomic.Int64
	)

	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for _, u := range urls {
		wg.Add(1)
		sem <- struct{}{}
		go func(id uuid.UUID, rawURL string) {
			defer wg.Done()
			defer func() { <-sem }()

			meta := urlfetch.Fetch(rawURL)
			if meta.FeedURL == "" {
				skipped.Add(1)
				return
			}

			if err := st.SetURLFeedURL(context.Background(), id, meta.FeedURL); err != nil {
				logger.Warn("set feed url", "url", rawURL, "error", err)
				return
			}

			logger.Info("found feed", "url", rawURL, "feed_url", meta.FeedURL)
			updated.Add(1)
		}(u.ID, u.URL)
	}

	wg.Wait()

	logger.Info("backfill complete", "updated", updated.Load(), "skipped", skipped.Load())
	return nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "required environment variable not set: %s\n", key)
		os.Exit(1)
	}
	return v
}
