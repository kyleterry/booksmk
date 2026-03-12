package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kyleterry/booksmk/internal/store/sqlstore"
)

var ErrNotFound = errors.New("not found")

type User struct {
	ID             uuid.UUID
	Email          string
	PasswordDigest string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func userFromSQL(u sqlstore.User) User {
	return User{
		ID:             u.ID,
		Email:          u.Email,
		PasswordDigest: u.PasswordDigest,
		CreatedAt:      u.CreatedAt.Time,
		UpdatedAt:      u.UpdatedAt.Time,
	}
}

func (s *Store) GetUser(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := s.queries.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return userFromSQL(u), nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	u, err := s.queries.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return userFromSQL(u), nil
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.queries.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]User, len(rows))
	for i, u := range rows {
		users[i] = userFromSQL(u)
	}
	return users, nil
}

func (s *Store) CreateUser(ctx context.Context, email, passwordDigest string) (User, error) {
	u, err := s.queries.CreateUser(ctx, sqlstore.CreateUserParams{
		Email:          email,
		PasswordDigest: passwordDigest,
	})
	if err != nil {
		return User{}, err
	}
	return userFromSQL(u), nil
}

func (s *Store) UpdateUser(ctx context.Context, id uuid.UUID, email string) (User, error) {
	u, err := s.queries.UpdateUser(ctx, sqlstore.UpdateUserParams{
		ID:    id,
		Email: email,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return userFromSQL(u), nil
}

func (s *Store) UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordDigest string) (User, error) {
	u, err := s.queries.UpdateUserPassword(ctx, sqlstore.UpdateUserPasswordParams{
		ID:             id,
		PasswordDigest: passwordDigest,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return userFromSQL(u), nil
}

func (s *Store) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteUser(ctx, id)
}
