package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"go.e64ec.com/booksmk/internal/store/sqlstore"
)

type JobConfig struct {
	JobName     string
	NextRunAt   time.Time
	LockedUntil time.Time
}

type JobRun struct {
	ID          uuid.UUID
	JobName     string
	StartedAt   time.Time
	CompletedAt *time.Time
	Error       *string
	Metadata    map[string]any
}

func (s *Store) CreateJobRun(ctx context.Context, jobName string, startedAt time.Time) (uuid.UUID, error) {
	return s.queries.CreateJobRun(ctx, sqlstore.CreateJobRunParams{
		JobName:   jobName,
		StartedAt: pgtype.Timestamptz{Time: startedAt.UTC(), Valid: true},
	})
}

func (s *Store) CompleteJobRun(ctx context.Context, id uuid.UUID, completedAt time.Time, jobErr error, metadata any) error {
	var errStr *string
	if jobErr != nil {
		s := jobErr.Error()
		errStr = &s
	}

	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	return s.queries.CompleteJobRun(ctx, sqlstore.CompleteJobRunParams{
		ID:          id,
		CompletedAt: pgtype.Timestamptz{Time: completedAt.UTC(), Valid: true},
		Error:       errStr,
		Metadata:    metaJSON,
	})
}

func (s *Store) ListJobRuns(ctx context.Context, jobName string, limit int32) ([]JobRun, error) {
	rows, err := s.queries.ListJobRuns(ctx, sqlstore.ListJobRunsParams{
		JobName: jobName,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}

	runs := make([]JobRun, len(rows))
	for i, r := range rows {
		var metadata map[string]any
		if err := json.Unmarshal(r.Metadata, &metadata); err != nil {
			return nil, err
		}

		run := JobRun{
			ID:        r.ID,
			JobName:   r.JobName,
			StartedAt: r.StartedAt.Time,
			Metadata:  metadata,
			Error:     r.Error,
		}
		if r.CompletedAt.Valid {
			run.CompletedAt = &r.CompletedAt.Time
		}
		runs[i] = run
	}
	return runs, nil
}

func (s *Store) GetJobConfig(ctx context.Context, jobName string) (JobConfig, error) {
	r, err := s.queries.GetJobConfig(ctx, jobName)
	if err != nil {
		return JobConfig{}, err
	}
	return JobConfig{
		JobName:     r.JobName,
		NextRunAt:   r.NextRunAt.Time,
		LockedUntil: r.LockedUntil.Time,
	}, nil
}

func (s *Store) ClaimJobRun(ctx context.Context, jobName string, lockDuration time.Duration, nextRunAt time.Time) (bool, error) {
	lockedUntil := time.Now().Add(lockDuration)
	_, err := s.queries.ClaimJobRun(ctx, sqlstore.ClaimJobRunParams{
		JobName:     jobName,
		LockedUntil: pgtype.Timestamptz{Time: lockedUntil.UTC(), Valid: true},
		NextRunAt:   pgtype.Timestamptz{Time: nextRunAt.UTC(), Valid: true},
	})
	if err != nil {
		return false, nil // Assume it means it couldn't be claimed (e.g. no rows updated)
	}
	return true, nil
}

func (s *Store) UpdateJobNextRun(ctx context.Context, jobName string, nextRunAt time.Time) error {
	return s.queries.UpdateJobNextRun(ctx, sqlstore.UpdateJobNextRunParams{
		JobName:   jobName,
		NextRunAt: pgtype.Timestamptz{Time: nextRunAt.UTC(), Valid: true},
	})
}
