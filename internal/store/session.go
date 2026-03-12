package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kyleterry/booksmk/internal/store/sqlstore"
)

const sessionDuration = 30 * 24 * time.Hour

type Session struct {
	Token     string
	UserID    uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
}

func sessionFromSQL(s sqlstore.Session) Session {
	return Session{
		Token:     s.Token,
		UserID:    s.UserID,
		CreatedAt: s.CreatedAt.Time,
		ExpiresAt: s.ExpiresAt.Time,
	}
}

func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID) (Session, error) {
	token, err := generateToken()
	if err != nil {
		return Session{}, err
	}
	sess, err := s.queries.CreateSession(ctx, sqlstore.CreateSessionParams{
		Token:     token,
		UserID:    userID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(sessionDuration), Valid: true},
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromSQL(sess), nil
}

func (s *Store) GetSessionUser(ctx context.Context, token string) (User, error) {
	u, err := s.queries.GetSessionUser(ctx, token)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return userFromSQL(u), nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	return s.queries.DeleteSession(ctx, token)
}

func (s *Store) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	return s.queries.DeleteUserSessions(ctx, userID)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
