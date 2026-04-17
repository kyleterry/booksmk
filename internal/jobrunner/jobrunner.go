// Package jobrunner drives periodic background jobs with bounded concurrency.
package jobrunner

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Handler is a periodic batch job.
type Handler interface {
	Name() string
	Interval() time.Duration
	Tick(ctx context.Context)
}

// Run invokes h.Tick on h.Interval until ctx is cancelled.
func Run(ctx context.Context, h Handler, logger *slog.Logger) {
	logger = logger.With("job", h.Name())
	logger.Info("starting job runner", "interval", h.Interval())
	ticker := time.NewTicker(h.Interval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping job runner")
			return
		case <-ticker.C:
			h.Tick(ctx)
		}
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
