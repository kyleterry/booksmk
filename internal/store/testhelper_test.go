package store_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"go.e64ec.com/booksmk/internal/migrate"
	"go.e64ec.com/booksmk/internal/store"
	"go.e64ec.com/booksmk/internal/testdb"
	"go.e64ec.com/booksmk/sql/migrations"
)

var silentLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

// testStore creates an isolated test database, runs all migrations, and returns
// a ready-to-use *store.Store. The database is dropped when the test ends.
func testStore(t *testing.T) *store.Store {
	t.Helper()
	pool := testdb.New(t)
	if err := migrate.Run(context.Background(), pool, migrations.FS, silentLogger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.New(pool)
}

// mustHashPassword returns a bcrypt digest for password using MinCost (fast for tests).
func mustHashPassword(t *testing.T, password string) string {
	t.Helper()
	digest, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return string(digest)
}
