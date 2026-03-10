package store_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/kyleterry/booksmk/internal/migrate"
	"github.com/kyleterry/booksmk/internal/store"
	"github.com/kyleterry/booksmk/sql/migrations"
)

var silentLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

// testStore connects to the DB, runs migrations, truncates all app tables,
// and returns a ready-to-use *store.Store. It skips the test if
// BOOKSMK_DATABASE_URL is not set.
func testStore(t *testing.T) *store.Store {
	t.Helper()
	dbURL := os.Getenv("BOOKSMK_DATABASE_URL")
	if dbURL == "" {
		t.Skip("BOOKSMK_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := migrate.Run(context.Background(), pool, migrations.FS, silentLogger); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	_, err = pool.Exec(context.Background(),
		"truncate users, urls, tags cascade")
	if err != nil {
		t.Fatalf("truncate: %v", err)
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
