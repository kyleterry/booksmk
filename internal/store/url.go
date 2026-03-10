package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kyleterry/booksmk/internal/store/sqlstore"
)

type URL struct {
	ID          uuid.UUID
	URL         string
	Title       string
	Description string
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func urlFromSQL(u sqlstore.UserUrlRow) URL {
	tags := u.Tags
	if tags == nil {
		tags = []string{}
	}
	return URL{
		ID:          u.ID,
		URL:         u.Url,
		Title:       u.Title,
		Description: u.Description,
		Tags:        tags,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

func (s *Store) GetURL(ctx context.Context, id, userID uuid.UUID) (URL, error) {
	u, err := s.queries.GetURL(ctx, sqlstore.GetURLParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return URL{}, ErrNotFound
	}
	if err != nil {
		return URL{}, err
	}
	tags, err := s.queries.ListTagNamesForURL(ctx, userID, id)
	if err != nil {
		return URL{}, err
	}
	u.Tags = tags
	return urlFromSQL(u), nil
}

func (s *Store) ListURLs(ctx context.Context, userID uuid.UUID) ([]URL, error) {
	rows, err := s.queries.ListURLs(ctx, userID)
	if err != nil {
		return nil, err
	}
	urls := make([]URL, len(rows))
	for i, u := range rows {
		tags, err := s.queries.ListTagNamesForURL(ctx, userID, u.ID)
		if err != nil {
			return nil, err
		}
		u.Tags = tags
		urls[i] = urlFromSQL(u)
	}
	return urls, nil
}

// CreateURL upserts the URL string (deduplicating across users), links it to the
// user with per-user title, description, and tags, then returns the full URL record.
func (s *Store) CreateURL(ctx context.Context, userID uuid.UUID, rawURL, title, description string, tags []string) (URL, error) {
	urlID, err := s.queries.UpsertURL(ctx, rawURL)
	if err != nil {
		return URL{}, err
	}
	if err := s.queries.AddURLToUser(ctx, sqlstore.AddURLToUserParams{
		UserID:      userID,
		URLID:       urlID,
		Title:       title,
		Description: description,
	}); err != nil {
		return URL{}, err
	}
	if err := s.setURLTags(ctx, userID, urlID, tags); err != nil {
		return URL{}, err
	}
	return s.GetURL(ctx, urlID, userID)
}

// UpdateURL updates the per-user title, description, and tags. The URL string
// itself is immutable once created.
func (s *Store) UpdateURL(ctx context.Context, id, userID uuid.UUID, title, description string, tags []string) (URL, error) {
	_, err := s.queries.UpdateUserURL(ctx, sqlstore.UpdateUserURLParams{
		URLID:       id,
		UserID:      userID,
		Title:       title,
		Description: description,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return URL{}, ErrNotFound
	}
	if err != nil {
		return URL{}, err
	}
	if err := s.setURLTags(ctx, userID, id, tags); err != nil {
		return URL{}, err
	}
	return s.GetURL(ctx, id, userID)
}

// DeleteURL removes the user's association with the URL. The URL row itself is
// retained for other users who may share it.
func (s *Store) DeleteURL(ctx context.Context, id, userID uuid.UUID) error {
	return s.queries.RemoveURLFromUser(ctx, userID, id)
}
