// Package testdb provides per-test PostgreSQL databases for integration tests.
//
// When BOOKSMK_DATABASE_URL is set, testdb uses that server. Otherwise it
// starts an embedded postgres once per test process on a free local port.
// Call Stop from TestMain to shut the embedded server down.
package testdb

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	embeddedOnce sync.Once
	embeddedDB   *embeddedpostgres.EmbeddedPostgres
	embeddedURL  string
	embeddedErr  error
)

// Stop shuts down the embedded postgres if it was started. Safe to call when
// no embedded server is running. Intended for TestMain.
func Stop() {
	if embeddedDB != nil {
		_ = embeddedDB.Stop()
	}
}

// New creates a uniquely-named database for the test and returns a pool
// connected to it. The database is forcibly dropped when the test ends.
//
// No migrations are applied — callers that need a fully-migrated schema should
// call migrate.Run on the returned pool themselves.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL, err := resolveURL()
	if err != nil {
		t.Fatalf("testdb: %v", err)
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

func resolveURL() (string, error) {
	if url := os.Getenv("BOOKSMK_DATABASE_URL"); url != "" {
		return url, nil
	}
	return ensureEmbedded()
}

func ensureEmbedded() (string, error) {
	embeddedOnce.Do(func() {
		port, err := freePort()
		if err != nil {
			embeddedErr = fmt.Errorf("find free port: %w", err)
			return
		}

		runtimePath, err := os.MkdirTemp("", "booksmk-embedded-pg-*")
		if err != nil {
			embeddedErr = fmt.Errorf("runtime tempdir: %w", err)
			return
		}
		dataPath, err := os.MkdirTemp("", "booksmk-embedded-pg-data-*")
		if err != nil {
			embeddedErr = fmt.Errorf("data tempdir: %w", err)
			return
		}

		cfg := embeddedpostgres.DefaultConfig().
			Username("postgres").
			Password("postgres").
			Database("postgres").
			Port(port).
			RuntimePath(runtimePath).
			DataPath(dataPath).
			Logger(nil)

		db := embeddedpostgres.NewDatabase(cfg)
		if err := db.Start(); err != nil {
			embeddedErr = fmt.Errorf("start embedded postgres: %w", err)
			return
		}

		embeddedDB = db
		embeddedURL = fmt.Sprintf(
			"postgres://postgres:postgres@localhost:%d/postgres?sslmode=disable",
			port,
		)
	})
	return embeddedURL, embeddedErr
}

func freePort() (uint32, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return uint32(l.Addr().(*net.TCPAddr).Port), nil
}
