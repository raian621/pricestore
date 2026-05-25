package db

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	connStr := CreateDbConnString("postgres", "postgres", "localhost", 5432, "postgres")
	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func cleanupMigrationsTable(t *testing.T, p *pgxpool.Pool) {
	t.Helper()
	p.Exec(context.Background(), "DROP TABLE IF EXISTS migrations")
}

func TestReadMigrations(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"001-create-foo.sql": "CREATE TABLE foo (id INT);",
		"002-create-bar.sql": "CREATE TABLE bar (id INT);",
		"not-a-migration.txt": "ignore me",
	}
	for name, content := range files {
		if err := os.WriteFile(path.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	migrations, err := ReadMigrations(dir)
	if err != nil {
		t.Fatalf("ReadMigrations failed: %v", err)
	}

	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}

	seen := make(map[string]bool)
	for _, m := range migrations {
		seen[m.Name] = true
		if m.Name == "001-create-foo.sql" && m.Script != "CREATE TABLE foo (id INT);" {
			t.Errorf("unexpected script for foo: %s", m.Script)
		}
		if m.Name == "002-create-bar.sql" && m.Script != "CREATE TABLE bar (id INT);" {
			t.Errorf("unexpected script for bar: %s", m.Script)
		}
	}
	if !seen["001-create-foo.sql"] || !seen["002-create-bar.sql"] {
		t.Error("not all migrations were read")
	}
}

func TestReadMigrations_NonExistentDir(t *testing.T) {
	_, err := ReadMigrations("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestBootstrapMigrationTable(t *testing.T) {
	p := testPool(t)
	cleanupMigrationsTable(t, p)

	BootstrapMigrationTable(p)

	var exists bool
	err := p.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'migrations')",
	).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to check migrations table: %v", err)
	}
	if !exists {
		t.Fatal("migrations table was not created")
	}
}

func TestApplyMigration(t *testing.T) {
	p := testPool(t)
	cleanupMigrationsTable(t, p)
	BootstrapMigrationTable(p)
	t.Cleanup(func() {
		p.Exec(context.Background(), "DROP TABLE IF EXISTS test_migration_apply")
	})

	m := Migration{
		Name:   "test-apply.sql",
		Script: "CREATE TABLE test_migration_apply (id INT);",
	}

	if err := m.Apply(p); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	var count int
	err := p.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM migrations WHERE migration_name = $1", m.Name,
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query migrations table: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 migration record, got %d", count)
	}
}

func TestApplyMigration_SkipsExisting(t *testing.T) {
	p := testPool(t)
	cleanupMigrationsTable(t, p)
	BootstrapMigrationTable(p)
	t.Cleanup(func() {
		p.Exec(context.Background(), "DROP TABLE IF EXISTS test_migration_skip")
	})

	p.Exec(context.Background(),
		"INSERT INTO migrations (migration_name) VALUES ($1)", "test-skip.sql",
	)

	m := Migration{
		Name:   "test-skip.sql",
		Script: "CREATE TABLE test_migration_skip (id INT);",
	}

	p.Exec(context.Background(), "DROP TABLE IF EXISTS test_migration_skip")

	if err := m.Apply(p); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// table should not exist since migration was skipped
	var exists bool
	err := p.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'test_migration_skip')",
	).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to check table existence: %v", err)
	}
	if exists {
		t.Error("table was created even though migration was already applied")
	}
}

func TestApplyMigrations(t *testing.T) {
	p := testPool(t)
	cleanupMigrationsTable(t, p)

	dir := t.TempDir()
	migrationContent := "CREATE TABLE test_apply_all (id INT);"
	if err := os.WriteFile(
		path.Join(dir, "001-test-apply-all.sql"),
		[]byte(migrationContent),
		0644,
	); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DROP TABLE IF EXISTS test_apply_all")
	})

	if err := ApplyMigrations(p, dir, true); err != nil {
		t.Fatalf("ApplyMigrations failed: %v", err)
	}

	var count int
	err := p.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM migrations WHERE migration_name = $1",
		"001-test-apply-all.sql",
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 migration record, got %d", count)
	}
}

func TestApplyMigrations_SubsequentRunsAreIdempotent(t *testing.T) {
	p := testPool(t)
	cleanupMigrationsTable(t, p)

	dir := t.TempDir()
	migrationContent := "CREATE TABLE test_idempotent (id INT);"
	if err := os.WriteFile(
		path.Join(dir, "001-test-idempotent.sql"),
		[]byte(migrationContent),
		0644,
	); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DROP TABLE IF EXISTS test_idempotent")
	})

	if err := ApplyMigrations(p, dir, true); err != nil {
		t.Fatalf("first ApplyMigrations failed: %v", err)
	}

	if err := ApplyMigrations(p, dir, false); err != nil {
		t.Fatalf("second ApplyMigrations failed: %v", err)
	}

	var count int
	err := p.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM migrations",
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 migration record after second run, got %d", count)
	}
}
