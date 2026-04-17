package jobrunner

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestPoolBoundedConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		items       int
		wantMax     int32
	}{
		{"single", 1, 5, 1},
		{"bounded", 3, 20, 3},
		{"items below limit", 5, 2, 2},
		{"zero treated as one", 0, 3, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := make([]int, tt.items)
			var (
				active  atomic.Int32
				maxSeen atomic.Int32
				mu      sync.Mutex
				done    []int
			)

			Pool(context.Background(), tt.concurrency, items, func(_ context.Context, _ int) {
				n := active.Add(1)
				for {
					cur := maxSeen.Load()
					if n <= cur || maxSeen.CompareAndSwap(cur, n) {
						break
					}
				}
				time.Sleep(5 * time.Millisecond)
				active.Add(-1)
				mu.Lock()
				done = append(done, 1)
				mu.Unlock()
			})

			if got := maxSeen.Load(); got > tt.wantMax {
				t.Errorf("max concurrency = %d; want <= %d", got, tt.wantMax)
			}
			if len(done) != tt.items {
				t.Errorf("completed = %d; want %d", len(done), tt.items)
			}
		})
	}
}

type stubHandler struct {
	ticks atomic.Int32
}

func (s *stubHandler) Name() string            { return "stub" }
func (s *stubHandler) Interval() time.Duration { return 10 * time.Millisecond }
func (s *stubHandler) Tick(_ context.Context)  { s.ticks.Add(1) }

func TestRunStopsOnContextCancel(t *testing.T) {
	h := &stubHandler{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		Run(ctx, h, discardLogger())
		close(done)
	}()

	time.Sleep(35 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancel")
	}

	if h.ticks.Load() == 0 {
		t.Errorf("expected at least one tick, got 0")
	}
}
