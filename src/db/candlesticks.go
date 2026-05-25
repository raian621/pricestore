package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	shared_models "github.com/raian621/pricestore/src/shared/models"
)

func InsertCandlesticks(
	p *pgxpool.Pool, assetName string, candlesticks []shared_models.Candlestick,
) error {
	row := p.QueryRow(context.Background(),
		"SELECT id FROM asset_metadata WHERE asset_name = $1", assetName)
	var assetId int
	if err := row.Scan(&assetId); err != nil {
		return err
	}

	// delete duplicate entries
	// this is definitely not optimal
	batch := &pgx.Batch{}
	for _, c := range candlesticks {
		batch.Queue(
			`DELETE FROM candlesticks
			WHERE asset_id = $1 AND start_usec = $2 AND end_usec = $3`,
			assetId, c.StartUsec, c.EndUsec)
	}
	tx, err := p.Begin(context.Background())
	if err != nil {
		return err
	}
	res := tx.SendBatch(context.Background(), batch)
	if err := res.Close(); err != nil {
		return err
	}

	// finally safe to insert candlestick data
	_, err = tx.CopyFrom(context.Background(), pgx.Identifier{"candlesticks"},
		[]string{
			"asset_id", "start_usec", "end_usec", "open_price", "high_price",
			"low_price", "close_price", "volume",
		},
		pgx.CopyFromSlice(len(candlesticks), func(i int) ([]any, error) {
			c := candlesticks[i]
			return []any{
				assetId, c.StartUsec, c.EndUsec, c.Open, c.High, c.Low, c.Close,
				c.Volume,
			}, nil
		}))
	if err != nil {
		return err
	}

	return tx.Commit(context.Background())
}

func GetCandlesticks(
	p *pgxpool.Pool, assetName string, startUsec int64, endUsec int64, limit int,
) ([]shared_models.Candlestick, int64, error) {
	rows, err := p.Query(context.Background(),
		`SELECT 
			start_usec, end_usec, open_price, high_price, low_price, close_price,
			volume 
		FROM candlesticks INNER JOIN asset_metadata 
		ON candlesticks.asset_id = asset_metadata.id
		WHERE asset_name = $1 AND start_usec BETWEEN $2 AND $3
		ORDER BY start_usec
		LIMIT $4`,
		assetName, startUsec, endUsec, limit)
	if err != nil {
		return nil, 0, err
	}
	candlesticks := make([]shared_models.Candlestick, 0)
	for rows.Next() {
		var c shared_models.Candlestick
		if err := rows.Scan(
			&c.StartUsec, &c.EndUsec, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume,
		); err != nil {
			return nil, 0, err
		}
		candlesticks = append(candlesticks, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	lastEndUsec := int64(0)
	if len(candlesticks) > 0 {
		lastEndUsec = candlesticks[len(candlesticks)-1].EndUsec
	}
	return candlesticks, lastEndUsec, nil
}
