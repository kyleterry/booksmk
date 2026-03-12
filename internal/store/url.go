package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kyleterry/booksmk/internal/store/sqlstore"
)

type URL struct {
	ID          uuid.UUID
	URL         string
	Title       string
	Description string
	Tags        []string
	CreatedAt   pgtype.Timestamptz
	UpdatedAt   pgtype.Timestamptz
}

func newURL(id uuid.UUID, rawURL, title, description string, tags []string, createdAt, updatedAt pgtype.Timestamptz) URL {
	if tags == nil {
		tags = []string{}
	}
	return URL{
		ID:          id,
		URL:         rawURL,
		Title:       title,
		Description: description,
		Tags:        tags,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
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
	tags, err := s.queries.ListTagNamesForURL(ctx, sqlstore.ListTagNamesForURLParams{UserID: userID, URLID: id})
	if err != nil {
		return URL{}, err
	}
	return newURL(u.ID, u.Url, u.Title, u.Description, tags, u.CreatedAt, u.UpdatedAt), nil
}

func (s *Store) ListURLs(ctx context.Context, userID uuid.UUID) ([]URL, error) {
	rows, err := s.queries.ListURLs(ctx, userID)
	if err != nil {
		return nil, err
	}
	urls := make([]URL, len(rows))
	for i, u := range rows {
		tags, err := s.queries.ListTagNamesForURL(ctx, sqlstore.ListTagNamesForURLParams{UserID: userID, URLID: u.ID})
		if err != nil {
			return nil, err
		}
		urls[i] = newURL(u.ID, u.Url, u.Title, u.Description, tags, u.CreatedAt, u.UpdatedAt)
	}
	return urls, nil
}

func (s *Store) ListURLsByTag(ctx context.Context, userID uuid.UUID, tag string) ([]URL, error) {
	rows, err := s.queries.ListURLsByTag(ctx, sqlstore.ListURLsByTagParams{UserID: userID, Name: tag})
	if err != nil {
		return nil, err
	}
	urls := make([]URL, len(rows))
	for i, u := range rows {
		tags, err := s.queries.ListTagNamesForURL(ctx, sqlstore.ListTagNamesForURLParams{UserID: userID, URLID: u.ID})
		if err != nil {
			return nil, err
		}
		urls[i] = newURL(u.ID, u.Url, u.Title, u.Description, tags, u.CreatedAt, u.UpdatedAt)
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
	return s.queries.RemoveURLFromUser(ctx, sqlstore.RemoveURLFromUserParams{UserID: userID, URLID: id})
}
