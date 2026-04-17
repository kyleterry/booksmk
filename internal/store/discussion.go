package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"go.e64ec.com/booksmk/internal/store/sqlstore"
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

// DiscussionURLJob is a URL that is due for a discussion check.
type DiscussionURLJob struct {
	ID         uuid.UUID
	URL        string
	CheckCount int32
	EmptyCount int32
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
			URL:        r.Url,
			CheckCount: r.CheckCount,
			EmptyCount: r.EmptyCount,
		}
	}
	return jobs, nil
}

func (s *Store) CompleteDiscussionJob(ctx context.Context, id uuid.UUID, nextAt time.Time, checkCount, emptyCount int32) error {
	return s.queries.CompleteDiscussionJob(ctx, sqlstore.CompleteDiscussionJobParams{
		ID:            id,
		NextCheckAt:   pgtype.Timestamptz{Time: nextAt.UTC(), Valid: true},
		CheckCount:    checkCount,
		EmptyCount:    emptyCount,
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
