package discuss

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/kyleterry/booksmk/internal/store"
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
	ClaimBatchRun(ctx context.Context) (bool, error)
	ListDueURLs(ctx context.Context) ([]store.DiscussionURLJob, error)
	SaveDiscussion(ctx context.Context, p store.SaveDiscussionParams) error
	CompleteDiscussionJob(ctx context.Context, id uuid.UUID, nextAt time.Time, checkCount, emptyCount int32) error
	RecordBatchRun(ctx context.Context, startedAt time.Time, urlCount, foundCount int32) error
}

// Worker polls for pending discussion jobs and processes them concurrently.
type Worker struct {
	store    discussStore
	fetchers []Fetcher
	logger   *slog.Logger
}

func New(st discussStore, logger *slog.Logger) *Worker {
	return &Worker{
		store:  st,
		logger: logger,
		fetchers: []Fetcher{
			&HackerNewsFetcher{},
			&RedditFetcher{},
		},
	}
}

// Run starts the polling loop. It returns when ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
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
	claimed, err := w.store.ClaimBatchRun(ctx)
	if err != nil {
		w.logger.Error("claim discussion batch run", "error", err)
		return
	}
	if !claimed {
		return
	}

	urls, err := w.store.ListDueURLs(ctx)
	if err != nil {
		w.logger.Error("list due discussion urls", "error", err)
		return
	}
	if len(urls) == 0 {
		return
	}

	w.logger.Info("starting discussion batch", "urls", len(urls))

	startedAt := time.Now()
	var (
		mu         sync.Mutex
		totalFound int32
	)

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	for _, u := range urls {
		wg.Add(1)
		sem <- struct{}{}
		go func(u store.DiscussionURLJob) {
			defer wg.Done()
			defer func() { <-sem }()
			found := w.processURL(ctx, u)
			mu.Lock()
			totalFound += found
			mu.Unlock()
		}(u)
	}
	wg.Wait()

	if err := w.store.RecordBatchRun(ctx, startedAt, int32(len(urls)), totalFound); err != nil {
		w.logger.Error("record batch run", "error", err)
	}
	w.logger.Info("discussion batch complete", "urls", len(urls), "found", totalFound)
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
				URLID:         job.URLID,
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
	w.logger.Info("discussion url complete", "url", job.URL, "found", totalFound, "next_check", nextAt.Format("2006-01-02 15:04"))
	return int32(totalFound)
}

// nextScheduledAt returns the time of the next check using exponential backoff
// based on the number of consecutive empty results. Starts at 1 day, caps at 30.
func nextScheduledAt(emptyCount int32) time.Time {
	days := min(int32(1)<<emptyCount, 30)
	return time.Now().Add(time.Duration(days) * 24 * time.Hour)
}
