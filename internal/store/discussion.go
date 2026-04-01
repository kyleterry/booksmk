package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kyleterry/booksmk/internal/store/sqlstore"
)

type Discussion struct {
	ID            uuid.UUID
	URLID         uuid.UUID
	Source        string
	Title         string
	DiscussionURL string
	Score         int32
	CommentCount  int32
	FoundAt       time.Time
}

// DiscussionURLJob is a URL that is due for a discussion check within the current batch.
type DiscussionURLJob struct {
	ID         uuid.UUID
	URLID      uuid.UUID
	URL        string
	CheckCount int32
	EmptyCount int32
}

type BatchRunSummary struct {
	ID          uuid.UUID
	StartedAt   time.Time
	CompletedAt time.Time
	URLCount    int32
	FoundCount  int32
}

type SaveDiscussionParams struct {
	URLID         uuid.UUID
	Source        string
	Title         string
	DiscussionURL string
	Score         int32
	CommentCount  int32
}

func (s *Store) EnqueueDiscussionJob(ctx context.Context, urlID uuid.UUID) error {
	return s.queries.EnqueueDiscussionJob(ctx, urlID)
}

// ClaimBatchRun attempts to claim the next batch run. Returns true if this
// caller claimed it; false means another server already claimed it or it is
// not yet due.
func (s *Store) ClaimBatchRun(ctx context.Context) (bool, error) {
	_, err := s.queries.ClaimBatchRun(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListDueURLs returns all URLs whose discussion check is due.
func (s *Store) ListDueURLs(ctx context.Context) ([]DiscussionURLJob, error) {
	rows, err := s.queries.ListDueURLs(ctx)
	if err != nil {
		return nil, err
	}
	jobs := make([]DiscussionURLJob, len(rows))
	for i, r := range rows {
		jobs[i] = DiscussionURLJob{
			ID:         r.ID,
			URLID:      r.URLID,
			URL:        r.Url,
			CheckCount: r.CheckCount,
			EmptyCount: r.EmptyCount,
		}
	}
	return jobs, nil
}

func (s *Store) CompleteDiscussionJob(ctx context.Context, id uuid.UUID, nextAt time.Time, checkCount, emptyCount int32) error {
	return s.queries.CompleteDiscussionJob(ctx, sqlstore.CompleteDiscussionJobParams{
		ID:          id,
		ScheduledAt: pgtype.Timestamptz{Time: nextAt, Valid: true},
		CheckCount:  checkCount,
		EmptyCount:  emptyCount,
	})
}

func (s *Store) SaveDiscussion(ctx context.Context, p SaveDiscussionParams) error {
	return s.queries.UpsertDiscussion(ctx, sqlstore.UpsertDiscussionParams{
		URLID:         p.URLID,
		Source:        p.Source,
		Title:         p.Title,
		DiscussionUrl: p.DiscussionURL,
		Score:         p.Score,
		CommentCount:  p.CommentCount,
	})
}

func (s *Store) RecordBatchRun(ctx context.Context, startedAt time.Time, urlCount, foundCount int32) error {
	return s.queries.RecordBatchRun(ctx, sqlstore.RecordBatchRunParams{
		StartedAt:  pgtype.Timestamptz{Time: startedAt, Valid: true},
		UrlCount:   urlCount,
		FoundCount: foundCount,
	})
}

func (s *Store) ListBatchRuns(ctx context.Context) ([]BatchRunSummary, error) {
	rows, err := s.queries.ListBatchRuns(ctx)
	if err != nil {
		return nil, err
	}
	runs := make([]BatchRunSummary, len(rows))
	for i, r := range rows {
		runs[i] = BatchRunSummary{
			ID:          r.ID,
			StartedAt:   r.StartedAt.Time,
			CompletedAt: r.CompletedAt.Time,
			URLCount:    r.UrlCount,
			FoundCount:  r.FoundCount,
		}
	}
	return runs, nil
}

func (s *Store) ScheduleBatchRunNow(ctx context.Context) error {
	return s.queries.ScheduleBatchRunNow(ctx)
}

func (s *Store) GetNextBatchRunAt(ctx context.Context) (time.Time, error) {
	ts, err := s.queries.GetNextBatchRunAt(ctx)
	if err != nil {
		return time.Time{}, err
	}
	return ts.Time, nil
}

func (s *Store) ListDiscussionsForURL(ctx context.Context, urlID uuid.UUID) ([]Discussion, error) {
	rows, err := s.queries.ListDiscussionsForURL(ctx, urlID)
	if err != nil {
		return nil, err
	}
	discussions := make([]Discussion, len(rows))
	for i, r := range rows {
		discussions[i] = Discussion{
			ID:            r.ID,
			URLID:         r.URLID,
			Source:        r.Source,
			Title:         r.Title,
			DiscussionURL: r.DiscussionUrl,
			Score:         r.Score,
			CommentCount:  r.CommentCount,
			FoundAt:       r.FoundAt.Time,
		}
	}
	return discussions, nil
}
