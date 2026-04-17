package discuss

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"go.e64ec.com/booksmk/internal/jobrunner"
	"go.e64ec.com/booksmk/internal/store"
)

const (
	discussInterval    = 30 * time.Minute
	discussConcurrency = 10
)

// Discussion is a single discussion thread found for a URL.
type Discussion struct {
	Title         string
	DiscussionURL string
	Score         int
	CommentCount  int
}

// Fetcher retrieves discussions for a given URL from a single source.
type Fetcher interface {
	Name() string
	Fetch(ctx context.Context, rawURL string) ([]Discussion, error)
}

type discussStore interface {
	ListDueURLs(ctx context.Context) ([]store.DiscussionURLJob, error)
	SaveDiscussion(ctx context.Context, p store.SaveDiscussionParams) error
	CompleteDiscussionJob(ctx context.Context, id uuid.UUID, nextAt time.Time, checkCount, emptyCount int32) error
}

// Worker polls for pending discussion jobs and processes them concurrently.
type Worker struct {
	store    discussStore
	fetchers []Fetcher
	logger   *slog.Logger
}

func New(st discussStore, logger *slog.Logger) *Worker {
	fetchers := []Fetcher{
		NewHackerNewsFetcher(),
	}

	redditID := os.Getenv("REDDIT_CLIENT_ID")
	redditSecret := os.Getenv("REDDIT_CLIENT_SECRET")
	if redditID != "" && redditSecret != "" {
		fetchers = append(fetchers, NewRedditFetcher(redditID, redditSecret))
		logger.Info("reddit fetcher enabled")
	}

	return &Worker{
		store:    st,
		logger:   logger,
		fetchers: fetchers,
	}
}

// Name implements jobrunner.Job.
func (w *Worker) Name() string { return "discuss" }

// Interval implements jobrunner.Job.
func (w *Worker) Interval() time.Duration { return discussInterval }

// Run implements jobrunner.Job.
func (w *Worker) Run(ctx context.Context) (any, error) {
	urls, err := w.store.ListDueURLs(ctx)
	if err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return map[string]int32{"url_count": 0, "found_count": 0}, nil
	}

	var (
		mu         sync.Mutex
		totalFound int32
	)

	jobrunner.Pool(ctx, discussConcurrency, urls, func(ctx context.Context, u store.DiscussionURLJob) {
		found := w.processURL(ctx, u)
		mu.Lock()
		totalFound += found
		mu.Unlock()
	})

	return map[string]int32{
		"url_count":   int32(len(urls)),
		"found_count": totalFound,
	}, nil
}

func (w *Worker) processURL(ctx context.Context, job store.DiscussionURLJob) int32 {
	totalFound := 0

	for _, f := range w.fetchers {
		fctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		results, err := f.Fetch(fctx, job.URL)
		cancel()

		if err != nil {
			w.logger.Warn("discussion fetch error", "source", f.Name(), "url", job.URL, "error", err)
			continue
		}

		for _, d := range results {
			if err := w.store.SaveDiscussion(ctx, store.SaveDiscussionParams{
				URLID:         job.ID,
				Source:        f.Name(),
				Title:         d.Title,
				DiscussionURL: d.DiscussionURL,
				Score:         int32(d.Score),
				CommentCount:  int32(d.CommentCount),
			}); err != nil {
				w.logger.Warn("save discussion", "source", f.Name(), "url", job.URL, "error", err)
			}
		}

		totalFound += len(results)
	}

	newEmptyCount := job.EmptyCount
	if totalFound == 0 {
		newEmptyCount++
	} else {
		newEmptyCount = 0
	}

	nextAt := nextScheduledAt(newEmptyCount)
	if err := w.store.CompleteDiscussionJob(ctx, job.ID, nextAt, job.CheckCount+1, newEmptyCount); err != nil {
		w.logger.Error("complete discussion job", "job_id", job.ID, "error", err)
		return 0
	}
	return int32(totalFound)
}

// nextScheduledAt returns the time of the next check using exponential backoff
// based on the number of consecutive empty results. Starts at 1 day, caps at 30.
// Adds +/- 5 minutes of jitter.
func nextScheduledAt(emptyCount int32) time.Time {
	days := min(int32(1)<<emptyCount, 30)
	jitter := jobrunner.Jitter(5 * time.Minute)
	return time.Now().Add(time.Duration(days)*24*time.Hour + jitter)
}
