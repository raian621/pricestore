package db

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	connStr := CreateDbConnString("postgres", "postgres", "localhost", 5432,
		"postgres")
	pool, err := pgxpool.New(context.Background(), connStr)
	require.NoError(t, err, "failed to connect to test db")
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
		"001-create-foo.sql":  "CREATE TABLE foo (id INT);",
		"002-create-bar.sql":  "CREATE TABLE bar (id INT);",
		"not-a-migration.txt": "ignore me",
	}
	for name, content := range files {
		err := os.WriteFile(path.Join(dir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	migrations, err := ReadMigrations(dir)
	require.NoError(t, err)
	require.Len(t, migrations, 2)

	for _, m := range migrations {
		switch m.Name {
		case "001-create-foo.sql":
			assert.Equal(t, "CREATE TABLE foo (id INT);", m.Script)
		case "002-create-bar.sql":
			assert.Equal(t, "CREATE TABLE bar (id INT);", m.Script)
		default:
			t.Errorf("unexpected migration name: %s", m.Name)
		}
	}
}

func TestReadMigrations_NonExistentDir(t *testing.T) {
	_, err := ReadMigrations("/nonexistent/path")
	assert.Error(t, err)
}

func TestBootstrapMigrationTable(t *testing.T) {
	p := testPool(t)
	cleanupMigrationsTable(t, p)

	BootstrapMigrationTable(p)

	var exists bool
	err := p.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT FROM information_schema.tables
		WHERE table_name = 'migrations')`,
	).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "migrations table was not created")
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

	require.NoError(t, m.Apply(p))

	var count int
	err := p.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM migrations WHERE migration_name = $1", m.Name,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
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

	require.NoError(t, m.Apply(p))

	var exists bool
	err := p.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT FROM information_schema.tables
		WHERE table_name = 'test_migration_skip')`,
	).Scan(&exists)
	require.NoError(t, err)
	assert.False(t, exists,
		"table was created even though migration was already applied")
}

func TestApplyMigrations(t *testing.T) {
	p := testPool(t)
	cleanupMigrationsTable(t, p)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		path.Join(dir, "001-test-apply-all.sql"),
		[]byte("CREATE TABLE test_apply_all (id INT);"),
		0644,
	))
	t.Cleanup(func() {
		p.Exec(context.Background(), "DROP TABLE IF EXISTS test_apply_all")
	})

	require.NoError(t, ApplyMigrations(p, dir, true))

	var count int
	err := p.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM migrations WHERE migration_name = $1",
		"001-test-apply-all.sql",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestApplyMigrations_SubsequentRunsAreIdempotent(t *testing.T) {
	p := testPool(t)
	cleanupMigrationsTable(t, p)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		path.Join(dir, "001-test-idempotent.sql"),
		[]byte("CREATE TABLE test_idempotent (id INT);"),
		0644,
	))
	t.Cleanup(func() {
		p.Exec(context.Background(), "DROP TABLE IF EXISTS test_idempotent")
	})

	require.NoError(t, ApplyMigrations(p, dir, true))
	require.NoError(t, ApplyMigrations(p, dir, false))

	var count int
	err := p.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM migrations",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "expected 1 migration record after second run")
}
