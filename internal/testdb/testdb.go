package testdb

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// New creates a uniquely-named PostgreSQL database for the test and returns a
// pool connected to it. The database is forcibly dropped when the test ends.
// The test is skipped if BOOKSMK_DATABASE_URL is not set.
//
// No migrations are applied — callers that need a fully-migrated schema should
// call migrate.Run on the returned pool themselves.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("BOOKSMK_DATABASE_URL")
	if dbURL == "" {
		t.Skip("BOOKSMK_DATABASE_URL not set")
	}

	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Fatalf("testdb: parse config: %v", err)
	}

	dbName := fmt.Sprintf("booksmk_test_%d", time.Now().UnixNano())

	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(context.Background(), adminCfg)
	if err != nil {
		t.Fatalf("testdb: admin connect: %v", err)
	}

	if _, err := adminPool.Exec(context.Background(), "create database "+dbName); err != nil {
		adminPool.Close()
		t.Fatalf("testdb: create database %q: %v", dbName, err)
	}

	t.Cleanup(func() {
		_, _ = adminPool.Exec(context.Background(), "drop database "+dbName+" with (force)")
		adminPool.Close()
	})

	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	pool, err := pgxpool.NewWithConfig(context.Background(), testCfg)
	if err != nil {
		t.Fatalf("testdb: connect to %q: %v", dbName, err)
	}
	t.Cleanup(pool.Close)

	return pool
}
