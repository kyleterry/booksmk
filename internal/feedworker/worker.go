package feedworker

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"

	"go.e64ec.com/booksmk/internal/store"
)

// feedStore is the subset of store.Store the worker needs.
type feedStore interface {
	ListDueFeedPollJobs(ctx context.Context) ([]store.FeedPollJob, error)
	UpdateFeedMeta(ctx context.Context, feedID uuid.UUID, siteURL, title, description, imageURL string) error
	UpsertFeedItem(ctx context.Context, p store.UpsertFeedItemParams) (uuid.UUID, error)
	CompleteFeedPollJob(ctx context.Context, jobID uuid.UUID, nextAt time.Time, fetchCount, errorCount int32, lastError string) error
}

// Worker polls feeds that are due for a refresh.
type Worker struct {
	store  feedStore
	parser *gofeed.Parser
	logger *slog.Logger
}

// New creates a Worker.
func New(st feedStore, logger *slog.Logger) *Worker {
	return &Worker{
		store:  st,
		parser: gofeed.NewParser(),
		logger: logger,
	}
}

// Run starts the polling loop and blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	jobs, err := w.store.ListDueFeedPollJobs(ctx)
	if err != nil {
		w.logger.Error("list due feed poll jobs", "error", err)
		return
	}
	if len(jobs) == 0 {
		return
	}

	w.logger.Info("starting feed poll batch", "feeds", len(jobs))

	sem := make(chan struct{}, 5)
	for _, job := range jobs {
		sem <- struct{}{}
		go func(j store.FeedPollJob) {
			defer func() { <-sem }()
			w.processJob(ctx, j)
		}(job)
	}
	// drain semaphore to wait for all goroutines
	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}

	w.logger.Info("feed poll batch complete", "feeds", len(jobs))
}

func (w *Worker) processJob(ctx context.Context, job store.FeedPollJob) {
	fctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	feed, err := w.parser.ParseURLWithContext(job.FeedURL, fctx)
	if err != nil {
		w.logger.Warn("feed fetch failed", "url", job.FeedURL, "error", err)
		nextAt := time.Now().Add(time.Hour)
		if completeErr := w.store.CompleteFeedPollJob(ctx, job.ID, nextAt, job.FetchCount, job.ErrorCount+1, err.Error()); completeErr != nil {
			w.logger.Error("complete feed poll job", "job_id", job.ID, "error", completeErr)
		}
		return
	}

	// feed.Link (channel <link>) can be a relative URL (e.g. pjrc.com uses "/").
	// Resolve it against the feed URL so we always have an absolute site URL.
	siteURL := resolveURL(job.FeedURL, feed.Link)
	title := feed.Title
	description := feed.Description
	imageURL := ""
	if feed.Image != nil && feed.Image.URL != "" {
		imageURL = feed.Image.URL
	} else if feed.ITunesExt != nil && feed.ITunesExt.Image != "" {
		imageURL = feed.ITunesExt.Image
	}
	if err := w.store.UpdateFeedMeta(ctx, job.FeedID, siteURL, title, description, imageURL); err != nil {
		w.logger.Warn("update feed meta", "feed_id", job.FeedID, "error", err)
	}

	// base URL for resolving relative item links: prefer resolved site URL, fall back to feed URL.
	baseURL := siteURL
	if baseURL == "" {
		baseURL = job.FeedURL
	}

	for _, item := range feed.Items {
		guid := item.GUID
		if guid == "" {
			guid = item.Link
		}
		if guid == "" {
			continue
		}

		author := ""
		if item.Author != nil {
			author = item.Author.Name
		}

		summary := item.Description
		if summary == "" {
			summary = item.Content
		}

		var publishedAt *time.Time
		if item.PublishedParsed != nil {
			publishedAt = item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			publishedAt = item.UpdatedParsed
		}

		_, upsertErr := w.store.UpsertFeedItem(ctx, store.UpsertFeedItemParams{
			FeedID:      job.FeedID,
			GUID:        guid,
			URL:         resolveURL(baseURL, item.Link),
			Title:       item.Title,
			Summary:     truncateSummary(summary),
			Author:      author,
			PublishedAt: publishedAt,
		})
		if upsertErr != nil {
			w.logger.Warn("upsert feed item", "feed_id", job.FeedID, "guid", guid, "error", upsertErr)
		}
	}

	nextAt := time.Now().Add(time.Hour)
	if err := w.store.CompleteFeedPollJob(ctx, job.ID, nextAt, job.FetchCount+1, 0, ""); err != nil {
		w.logger.Error("complete feed poll job", "job_id", job.ID, "error", err)
	}

	w.logger.Info("feed poll complete", "url", job.FeedURL, "items", len(feed.Items))
}

// resolveURL resolves ref against base. If ref is already absolute, it is
// returned unchanged. If base is empty or unparseable, ref is returned as-is.
func resolveURL(base, ref string) string {
	if ref == "" {
		return ref
	}
	refURL, err := url.Parse(ref)
	if err != nil || refURL.IsAbs() {
		return ref
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ref
	}
	return baseURL.ResolveReference(refURL).String()
}

// truncateSummary strips HTML tags and truncates the summary to 500 characters.
func truncateSummary(s string) string {
	// simple tag strip: remove anything between < and >
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	result := strings.TrimSpace(b.String())
	runes := []rune(result)
	if len(runes) > 500 {
		result = string(runes[:500])
	}
	return result
}
