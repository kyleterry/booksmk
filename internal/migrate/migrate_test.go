package migrate_test

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kyleterry/booksmk/internal/migrate"
	"github.com/kyleterry/booksmk/internal/testdb"
	"github.com/kyleterry/booksmk/sql/migrations"
)

var silentLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(context.Background(),
		"select exists(select 1 from information_schema.tables where table_schema=current_schema() and table_name=$1)",
		name,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("check table %q: %v", name, err)
	}
	return exists
}

func appliedVersions(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "select version from schema_migrations order by version")
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	defer rows.Close()
	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		versions = append(versions, v)
	}
	return versions
}

func TestRun_AppliesMigrations(t *testing.T) {
	pool := testdb.New(t)

	if err := migrate.Run(context.Background(), pool, migrations.FS, silentLogger); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}

	tables := []string{"users", "sessions", "urls", "user_urls", "tags", "url_tags", "schema_migrations"}
	for _, tbl := range tables {
		if !tableExists(t, pool, tbl) {
			t.Errorf("table %q does not exist after migration", tbl)
		}
	}

	versions := appliedVersions(t, pool)
	if len(versions) == 0 {
		t.Fatal("no versions recorded in schema_migrations")
	}
	if versions[0] != "001-initial-schema" {
		t.Errorf("versions[0] = %q, want %q", versions[0], "001-initial-schema")
	}
}

func TestRun_Idempotent(t *testing.T) {
	pool := testdb.New(t)

	for i := range 3 {
		if err := migrate.Run(context.Background(), pool, migrations.FS, silentLogger); err != nil {
			t.Fatalf("run %d: migrate.Run: %v", i+1, err)
		}
	}

	versions := appliedVersions(t, pool)
	seen := make(map[string]int)
	for _, v := range versions {
		seen[v]++
	}
	for v, count := range seen {
		if count != 1 {
			t.Errorf("version %q recorded %d times, want 1", v, count)
		}
	}
}

func TestRun_AppliesInOrder(t *testing.T) {
	pool := testdb.New(t)

	fakeFS := fstest.MapFS{
		"002-second.sql": {Data: []byte("create table second_table (id int primary key);")},
		"001-first.sql":  {Data: []byte("create table first_table (id int primary key);")},
	}

	if err := migrate.Run(context.Background(), pool, fakeFS, silentLogger); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}

	versions := appliedVersions(t, pool)
	if len(versions) != 2 {
		t.Fatalf("want 2 versions, got %d: %v", len(versions), versions)
	}
	if versions[0] != "001-first" {
		t.Errorf("versions[0] = %q, want %q", versions[0], "001-first")
	}
	if versions[1] != "002-second" {
		t.Errorf("versions[1] = %q, want %q", versions[1], "002-second")
	}
}

func TestRun_SkipsAlreadyApplied(t *testing.T) {
	pool := testdb.New(t)

	first := fstest.MapFS{
		"001-first.sql": {Data: []byte("create table first_table (id int primary key);")},
	}
	if err := migrate.Run(context.Background(), pool, first, silentLogger); err != nil {
		t.Fatalf("first run: %v", err)
	}

	both := fstest.MapFS{
		"001-first.sql":  {Data: []byte("create table first_table (id int primary key);")},
		"002-second.sql": {Data: []byte("create table second_table (id int primary key);")},
	}
	if err := migrate.Run(context.Background(), pool, both, silentLogger); err != nil {
		t.Fatalf("second run: %v", err)
	}

	versions := appliedVersions(t, pool)
	if len(versions) != 2 {
		t.Fatalf("want 2 versions, got %d: %v", len(versions), versions)
	}
}

func TestRun_RollsBackFailedMigration(t *testing.T) {
	pool := testdb.New(t)

	bad := fstest.MapFS{
		"001-bad.sql": {Data: []byte("this is not valid sql !!!;")},
	}
	err := migrate.Run(context.Background(), pool, bad, silentLogger)
	if err == nil {
		t.Fatal("expected error from bad SQL, got nil")
	}

	versions := appliedVersions(t, pool)
	for _, v := range versions {
		if v == "001-bad" {
			t.Error("failed migration version was recorded in schema_migrations")
		}
	}
}

func TestRun_IgnoresNonSQLFiles(t *testing.T) {
	pool := testdb.New(t)

	mixed := fstest.MapFS{
		"001-real.sql": {Data: []byte("create table real_table (id int primary key);")},
		"README.md":    {Data: []byte("# migrations")},
		"embed.go":     {Data: []byte("package migrations")},
	}
	if err := migrate.Run(context.Background(), pool, mixed, silentLogger); err != nil {
		t.Fatalf("migrate.Run: %v", err)
	}

	versions := appliedVersions(t, pool)
	if len(versions) != 1 || versions[0] != "001-real" {
		t.Errorf("versions = %v, want [001-real]", versions)
	}
}

// Ensure any fs.FS satisfies the Run parameter — not just embed.FS.
var _ fs.FS = fstest.MapFS{}
