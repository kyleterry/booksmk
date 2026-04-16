package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
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
		fmt.Fprintln(os.Stderr, "  import-blocklist      import a list of domains from a URL or file")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "backfill-feed-urls":
		if err := runBackfillFeedURLs(logger); err != nil {
			logger.Error("backfill failed", "error", err)
			os.Exit(1)
		}
	case "import-blocklist":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: booksmkctl import-blocklist <url_or_path>")
			os.Exit(1)
		}
		if err := runImportBlocklist(logger, os.Args[2]); err != nil {
			logger.Error("import failed", "error", err)
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

func runImportBlocklist(logger *slog.Logger, source string) error {
	dbURL := mustEnv("BOOKSMK_DATABASE_URL")

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	st := store.New(pool)

	var r io.Reader
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		resp, err := http.Get(source)
		if err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		r = resp.Body
	} else {
		f, err := os.Open(source)
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		defer func() { _ = f.Close() }()
		r = f
	}

	scanner := bufio.NewScanner(r)
	var (
		added   atomic.Int64
		skipped atomic.Int64
	)

	logger.Info("importing blocklist domains", "source", source)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle hosts file format (e.g. "0.0.0.0 domain.com")
		fields := strings.Fields(line)
		if len(fields) > 1 {
			if net.ParseIP(fields[0]) != nil {
				line = fields[1]
			}
		}

		// Basic validation
		if strings.Contains(line, "/") || strings.Contains(line, ":") {
			skipped.Add(1)
			continue
		}

		_, err := st.CreateBlocklistEntry(context.Background(), line, "domain")
		if err != nil {
			skipped.Add(1)
			continue
		}

		added.Add(1)
		if added.Load()%1000 == 0 {
			logger.Info("progress", "added", added.Load())
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	logger.Info("import complete", "added", added.Load(), "skipped", skipped.Load())
	return nil
}
