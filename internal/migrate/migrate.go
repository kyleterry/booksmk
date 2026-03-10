package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// advisoryLockKey is a fixed application-specific key used to serialise
// migration runs across multiple processes.
const advisoryLockKey = int64(0x626f6f6b736d6b) // "booksmk" as hex

// Run applies any pending migrations in order. It acquires a PostgreSQL
// advisory lock so that only one process can run migrations at a time; other
// callers block until the lock is released.
func Run(ctx context.Context, pool *pgxpool.Pool, migrations fs.FS, logger *slog.Logger) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrate: acquire connection: %w", err)
	}
	defer conn.Release()

	// Block until we hold the advisory lock. Released automatically when the
	// connection is returned to the pool.
	if _, err := conn.Exec(ctx, "select pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return fmt.Errorf("migrate: acquire lock: %w", err)
	}
	defer conn.Exec(context.Background(), "select pg_advisory_unlock($1)", advisoryLockKey)

	// Ensure the versions table exists.
	_, err = conn.Exec(ctx, `
		create table if not exists schema_migrations (
			version    text        primary key,
			applied_at timestamptz not null default now()
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate: create schema_migrations: %w", err)
	}

	// Fetch already-applied versions.
	rows, err := conn.Query(ctx, "select version from schema_migrations")
	if err != nil {
		return fmt.Errorf("migrate: query applied versions: %w", err)
	}
	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("migrate: scan version: %w", err)
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("migrate: read versions: %w", err)
	}

	// Collect and sort migration files.
	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return fmt.Errorf("migrate: read dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	// Apply pending migrations.
	for _, name := range files {
		version := strings.TrimSuffix(name, ".sql")
		if applied[version] {
			continue
		}

		sql, err := fs.ReadFile(migrations, name)
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", name, err)
		}

		logger.Info("applying migration", "version", version)

		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("migrate: begin tx for %s: %w", version, err)
		}

		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migrate: apply %s: %w", version, err)
		}

		if _, err := tx.Exec(ctx, "insert into schema_migrations (version) values ($1)", version); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migrate: record %s: %w", version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("migrate: commit %s: %w", version, err)
		}

		logger.Info("applied migration", "version", version)
	}

	return nil
}
