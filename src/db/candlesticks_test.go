package db

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	shared_models "github.com/raian621/pricestore/src/shared/models"
)

var ctx = context.Background()

func migrationsDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "scripts", "migrations")
}

func setupFromMigrations(t *testing.T, p *pgxpool.Pool) {
	t.Helper()

	cleanupCandlestickTables(t, p)
	cleanupMigrationsTable(t, p)

	require.NoError(t, ApplyMigrations(p, migrationsDir(), true),
		"ApplyMigrations failed")
}

func cleanupCandlestickTables(t *testing.T, p *pgxpool.Pool) {
	t.Helper()
	p.Exec(ctx, "DROP TABLE IF EXISTS candlesticks")
	p.Exec(ctx, "DROP TABLE IF EXISTS asset_metadata")
}

func insertTestAsset(t *testing.T, p *pgxpool.Pool, name string) int64 {
	t.Helper()
	var id int64
	err := p.QueryRow(ctx,
		"INSERT INTO asset_metadata (asset_name) VALUES ($1) RETURNING id", name,
	).Scan(&id)
	require.NoError(t, err, "failed to insert test asset")
	return id
}

func TestInsertCandlesticks(t *testing.T) {
	p := testPool(t)
	setupFromMigrations(t, p)
	t.Cleanup(func() {
		cleanupCandlestickTables(t, p)
		cleanupMigrationsTable(t, p)
	})

	assetName := "test_insert"
	insertTestAsset(t, p, assetName)

	now := time.Now().UnixMicro()
	interval := int64(60_000_000)
	candlesticks := []shared_models.Candlestick{
		{StartUsec: now, EndUsec: now + interval, Open: 100.0, High: 110.0, Low: 95.0, Close: 105.0, Volume: 1000.0},
		{StartUsec: now + interval, EndUsec: now + 2*interval, Open: 105.0, High: 115.0, Low: 102.0, Close: 112.0, Volume: 1500.0},
	}

	require.NoError(t, InsertCandlesticks(p, assetName, candlesticks))

	var count int
	err := p.QueryRow(ctx,
		`SELECT COUNT(*) FROM candlesticks c
		 INNER JOIN asset_metadata a ON c.asset_id = a.id
		 WHERE a.asset_name = $1`, assetName,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestInsertCandlesticks_DuplicateReplacement(t *testing.T) {
	p := testPool(t)
	setupFromMigrations(t, p)
	t.Cleanup(func() {
		cleanupCandlestickTables(t, p)
		cleanupMigrationsTable(t, p)
	})

	assetName := "test_dup"
	insertTestAsset(t, p, assetName)

	now := time.Now().UnixMicro()
	interval := int64(60_000_000)
	cs := []shared_models.Candlestick{
		{StartUsec: now, EndUsec: now + interval, Open: 100.0, High: 110.0, Low: 95.0, Close: 105.0, Volume: 1000.0},
	}

	require.NoError(t, InsertCandlesticks(p, assetName, cs))

	cs[0].Close = 200.0
	require.NoError(t, InsertCandlesticks(p, assetName, cs))

	var closePrice float64
	err := p.QueryRow(ctx,
		`SELECT close_price FROM candlesticks c
		 INNER JOIN asset_metadata a ON c.asset_id = a.id
		 WHERE a.asset_name = $1 AND c.start_usec = $2`,
		assetName, now,
	).Scan(&closePrice)
	require.NoError(t, err)
	assert.Equal(t, 200.0, closePrice)
}

func TestInsertCandlesticks_NonexistentAsset(t *testing.T) {
	p := testPool(t)
	setupFromMigrations(t, p)
	t.Cleanup(func() {
		cleanupCandlestickTables(t, p)
		cleanupMigrationsTable(t, p)
	})

	now := time.Now().UnixMicro()
	interval := int64(60_000_000)
	cs := []shared_models.Candlestick{
		{StartUsec: now, EndUsec: now + interval, Open: 100.0, High: 110.0, Low: 95.0, Close: 105.0, Volume: 1000.0},
	}

	assert.Error(t, InsertCandlesticks(p, "nonexistent", cs))
}

func TestGetCandlesticks(t *testing.T) {
	p := testPool(t)
	setupFromMigrations(t, p)
	t.Cleanup(func() {
		cleanupCandlestickTables(t, p)
		cleanupMigrationsTable(t, p)
	})

	assetName := "test_get"
	insertTestAsset(t, p, assetName)

	now := time.Now().UnixMicro()
	interval := int64(60_000_000)
	candlesticks := []shared_models.Candlestick{
		{StartUsec: now, EndUsec: now + interval, Open: 100.0, High: 110.0, Low: 95.0, Close: 105.0, Volume: 1000.0},
		{StartUsec: now + interval, EndUsec: now + 2*interval, Open: 105.0, High: 115.0, Low: 102.0, Close: 112.0, Volume: 1500.0},
		{StartUsec: now + 2*interval, EndUsec: now + 3*interval, Open: 112.0, High: 120.0, Low: 108.0, Close: 118.0, Volume: 2000.0},
	}

	require.NoError(t, InsertCandlesticks(p, assetName, candlesticks))

	got, lastEndUsec, err := GetCandlesticks(p, assetName, 0, now+3*interval, 10)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, now+3*interval, lastEndUsec)
	assert.Equal(t, 100.0, got[0].Open)
	assert.Equal(t, 105.0, got[0].Close)
}

func TestGetCandlesticks_WithTimeRange(t *testing.T) {
	p := testPool(t)
	setupFromMigrations(t, p)
	t.Cleanup(func() {
		cleanupCandlestickTables(t, p)
		cleanupMigrationsTable(t, p)
	})

	assetName := "test_range"
	insertTestAsset(t, p, assetName)

	now := time.Now().UnixMicro()
	interval := int64(60_000_000)
	candlesticks := []shared_models.Candlestick{
		{StartUsec: now, EndUsec: now + interval, Open: 100.0, High: 110.0, Low: 95.0, Close: 105.0, Volume: 1000.0},
		{StartUsec: now + interval, EndUsec: now + 2*interval, Open: 105.0, High: 115.0, Low: 102.0, Close: 112.0, Volume: 1500.0},
		{StartUsec: now + 2*interval, EndUsec: now + 3*interval, Open: 112.0, High: 120.0, Low: 108.0, Close: 118.0, Volume: 2000.0},
	}

	require.NoError(t, InsertCandlesticks(p, assetName, candlesticks))

	got, _, err := GetCandlesticks(p, assetName, now+interval, now+3*interval, 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
}

func TestGetCandlesticks_WithLimit(t *testing.T) {
	p := testPool(t)
	setupFromMigrations(t, p)
	t.Cleanup(func() {
		cleanupCandlestickTables(t, p)
		cleanupMigrationsTable(t, p)
	})

	assetName := "test_limit"
	insertTestAsset(t, p, assetName)

	now := time.Now().UnixMicro()
	interval := int64(60_000_000)
	candlesticks := []shared_models.Candlestick{
		{StartUsec: now, EndUsec: now + interval, Open: 100.0, High: 110.0, Low: 95.0, Close: 105.0, Volume: 1000.0},
		{StartUsec: now + interval, EndUsec: now + 2*interval, Open: 105.0, High: 115.0, Low: 102.0, Close: 112.0, Volume: 1500.0},
	}

	require.NoError(t, InsertCandlesticks(p, assetName, candlesticks))

	got, _, err := GetCandlesticks(p, assetName, 0, now+2*interval, 1)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, now, got[0].StartUsec)
}

func TestGetCandlesticks_NoResults(t *testing.T) {
	p := testPool(t)
	setupFromMigrations(t, p)
	t.Cleanup(func() {
		cleanupCandlestickTables(t, p)
		cleanupMigrationsTable(t, p)
	})

	assetName := "test_empty"
	insertTestAsset(t, p, assetName)

	got, lastEndUsec, err := GetCandlesticks(p, assetName, 0, 1000, 10)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Equal(t, int64(0), lastEndUsec)
}
