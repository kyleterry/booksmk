package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kyleterry/booksmk/internal/store/sqlstore"
)

const maxAPIKeys = 5

var ErrAPIKeyLimitReached = errors.New("api key limit reached (max 5)")

type APIKey struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Name        string
	TokenPrefix string
	ExpiresAt   *time.Time
	CreatedAt   time.Time
}

// NewAPIKeyResult holds the created APIKey and the plaintext token (shown once).
type NewAPIKeyResult struct {
	APIKey
	Token string
}

func apiKeyFromSQL(k sqlstore.APIKey) APIKey {
	var expiresAt *time.Time
	if k.ExpiresAt.Valid {
		t := k.ExpiresAt.Time
		expiresAt = &t
	}
	return APIKey{
		ID:          k.ID,
		UserID:      k.UserID,
		Name:        k.Name,
		TokenPrefix: k.TokenPrefix,
		ExpiresAt:   expiresAt,
		CreatedAt:   k.CreatedAt.Time,
	}
}

func (s *Store) CreateAPIKey(ctx context.Context, userID uuid.UUID, name string, expiresAt *time.Time) (NewAPIKeyResult, error) {
	count, err := s.queries.CountAPIKeys(ctx, userID)
	if err != nil {
		return NewAPIKeyResult{}, err
	}
	if count >= maxAPIKeys {
		return NewAPIKeyResult{}, ErrAPIKeyLimitReached
	}

	token, err := generateAPIToken()
	if err != nil {
		return NewAPIKeyResult{}, err
	}

	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])
	tokenPrefix := token[:12]

	var pgExpiresAt pgtype.Timestamptz
	if expiresAt != nil {
		pgExpiresAt = pgtype.Timestamptz{Time: *expiresAt, Valid: true}
	}
	k, err := s.queries.CreateAPIKey(ctx, sqlstore.CreateAPIKeyParams{
		UserID:      userID,
		Name:        name,
		TokenHash:   tokenHash,
		TokenPrefix: tokenPrefix,
		ExpiresAt:   pgExpiresAt,
	})
	if err != nil {
		return NewAPIKeyResult{}, err
	}

	return NewAPIKeyResult{APIKey: apiKeyFromSQL(k), Token: token}, nil
}

func (s *Store) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]APIKey, error) {
	rows, err := s.queries.ListAPIKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	keys := make([]APIKey, len(rows))
	for i, k := range rows {
		keys[i] = apiKeyFromSQL(k)
	}
	return keys, nil
}

func (s *Store) DeleteAPIKey(ctx context.Context, id, userID uuid.UUID) error {
	return s.queries.DeleteAPIKey(ctx, sqlstore.DeleteAPIKeyParams{ID: id, UserID: userID})
}

func (s *Store) GetAPIKeyByToken(ctx context.Context, token string) (APIKey, error) {
	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])
	k, err := s.queries.GetAPIKeyByTokenHash(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return APIKey{}, ErrNotFound
	}
	if err != nil {
		return APIKey{}, err
	}
	return apiKeyFromSQL(k), nil
}

func generateAPIToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "bsmk_" + base64.RawURLEncoding.EncodeToString(b), nil
}
