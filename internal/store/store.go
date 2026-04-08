//go:generate sqlc generate

package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"go.e64ec.com/booksmk/internal/store/sqlstore"
)

// Store wraps the sqlc-generated sqlstore package with domain-level operations.
// Run `go generate ./internal/store/...` to regenerate internal/store/sqlstore from sql/.
type Store struct {
	pool    *pgxpool.Pool
	queries *sqlstore.Queries
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{
		pool:    pool,
		queries: sqlstore.New(pool),
	}
}

// Ping checks the database connection.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
