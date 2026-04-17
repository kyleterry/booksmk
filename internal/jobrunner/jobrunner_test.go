package jobrunner

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type stubStore struct{}

func (s *stubStore) ClaimJobRun(ctx context.Context, jobName string, lockDuration time.Duration, nextRunAt time.Time) (bool, error) {
	return true, nil
}
func (s *stubStore) CreateJobRun(ctx context.Context, jobName string, startedAt time.Time) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (s *stubStore) CompleteJobRun(ctx context.Context, id uuid.UUID, completedAt time.Time, err error, metadata any) error {
	return nil
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

type stubJob struct {
	ticks atomic.Int32
}

func (s *stubJob) Name() string            { return "stub" }
func (s *stubJob) Interval() time.Duration { return 10 * time.Millisecond }
func (s *stubJob) Run(_ context.Context) (any, error) {
	s.ticks.Add(1)
	return nil, nil
}

func TestRunnerStopsOnContextCancel(t *testing.T) {
	j := &stubJob{}
	ctx, cancel := context.WithCancel(context.Background())
	st := &stubStore{}
	r := New(st, discardLogger())

	done := make(chan struct{})
	go func() {
		r.Run(ctx, j)
		close(done)
	}()

	time.Sleep(35 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancel")
	}

	if j.ticks.Load() == 0 {
		t.Errorf("expected at least one tick, got 0")
	}
}
