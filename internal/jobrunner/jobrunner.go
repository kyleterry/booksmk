// Package jobrunner drives periodic background jobs with bounded concurrency.
package jobrunner

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Job is a periodic batch job.
type Job interface {
	Name() string
	Interval() time.Duration
	Run(ctx context.Context) (metadata any, err error)
}

// Store is the subset of store.Store needed by the Runner.
type Store interface {
	ClaimJobRun(ctx context.Context, jobName string, lockDuration time.Duration, nextRunAt time.Time) (bool, error)
	CreateJobRun(ctx context.Context, jobName string, startedAt time.Time) (uuid.UUID, error)
	CompleteJobRun(ctx context.Context, id uuid.UUID, completedAt time.Time, err error, metadata any) error
}

// Runner manages and executes Jobs.
type Runner struct {
	store  Store
	logger *slog.Logger
}

// New creates a Runner.
func New(st Store, logger *slog.Logger) *Runner {
	return &Runner{
		store:  st,
		logger: logger,
	}
}

// Run executes the job on its interval.
func (r *Runner) Run(ctx context.Context, j Job) {
	logger := r.logger.With("job", j.Name())
	logger.Info("starting job runner", "interval", j.Interval())

	r.tick(ctx, j, logger)

	ticker := time.NewTicker(time.Second * 10) // Check if due every 10s
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping job runner")
			return
		case <-ticker.C:
			r.tick(ctx, j, logger)
		}
	}
}

func (r *Runner) tick(ctx context.Context, j Job, logger *slog.Logger) {
	nextRunAt := time.Now().Add(j.Interval() + Jitter(5*time.Minute))
	claimed, err := r.store.ClaimJobRun(ctx, j.Name(), j.Interval(), nextRunAt)
	if err != nil {
		logger.Error("claim job run", "error", err)
		return
	}
	if !claimed {
		return
	}

	startedAt := time.Now()
	runID, err := r.store.CreateJobRun(ctx, j.Name(), startedAt)
	if err != nil {
		logger.Error("create job run", "error", err)
		return
	}

	logger.Info("starting job run", "run_id", runID)
	metadata, jobErr := j.Run(ctx)
	completedAt := time.Now()

	if jobErr != nil {
		logger.Error("job run failed", "run_id", runID, "error", jobErr)
	} else {
		logger.Info("job run complete", "run_id", runID, "duration", completedAt.Sub(startedAt))
	}

	if err := r.store.CompleteJobRun(ctx, runID, completedAt, jobErr, metadata); err != nil {
		logger.Error("complete job run", "run_id", runID, "error", err)
	}
}

// Pool runs fn over items with bounded concurrency and blocks until all complete.
func Pool[T any](ctx context.Context, concurrency int, items []T, fn func(context.Context, T)) {
	if concurrency < 1 {
		concurrency = 1
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for _, it := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(item T) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(ctx, item)
		}(it)
	}
	wg.Wait()
}
