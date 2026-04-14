package store

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.e64ec.com/booksmk/internal/store/sqlstore"
)

type BlocklistEntry struct {
	ID        uuid.UUID
	Pattern   string
	Kind      string
	CreatedAt time.Time
}

func (s *Store) CreateBlocklistEntry(ctx context.Context, pattern, kind string) (sqlstore.Blocklist, error) {
	return s.queries.CreateBlocklistEntry(ctx, sqlstore.CreateBlocklistEntryParams{
		Pattern: pattern,
		Kind:    kind,
	})
}

func (s *Store) DeleteBlocklistEntry(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteBlocklistEntry(ctx, id)
}

func (s *Store) ListBlocklistEntries(ctx context.Context) ([]sqlstore.Blocklist, error) {
	return s.queries.ListBlocklistEntries(ctx)
}

func (s *Store) IsBlocked(ctx context.Context, rawURL string) (bool, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false, err
	}

	hostname := strings.ToLower(u.Hostname())
	blocked, err := s.queries.IsBlocked(ctx, sqlstore.IsBlockedParams{
		Pattern: rawURL,
		Column2: hostname,
	})
	if err != nil {
		return false, err
	}
	return blocked, nil
}
