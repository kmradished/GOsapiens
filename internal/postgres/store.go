// Package postgres содержит общую основу хранилища ЛР4.
package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmradished/GOsapiens/internal/domain"
)

type Store struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{Pool: pool}
}

// Open проверяет подключение. Вызывающий код ограничивает ctx по времени
// и вызывает pool.Close при остановке приложения.
func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	cfg.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

type pairRow struct {
	State        domain.PairState
	LeftBlocked  bool
	RightBlocked bool
}

// lockPair вызывается после BeginTx и до чтения/изменения реакции, запрета
// или мэтча. Все писатели пары используют этот протокол в одном tx.
func lockPair(ctx context.Context, tx pgx.Tx, key domain.PairKey) (pairRow, error) {
	if !key.Valid() {
		return pairRow{}, domain.ErrInvalidPair
	}

	// Создаём защищаемый ресурс даже для первой реакции. При конкуренции
	// INSERT ждёт незавершённую вставку той же пары; пустой SELECT не нужен.
	_, err := tx.Exec(ctx, `
		INSERT INTO pairs (left_id, right_id) VALUES ($1, $2)
		ON CONFLICT (left_id, right_id) DO NOTHING`, key.LeftID, key.RightID)
	if err != nil {
		return pairRow{}, classify(err)
	}

	var row pairRow
	err = tx.QueryRow(ctx, `
		SELECT left_liked, right_liked, left_blocked, right_blocked
		FROM pairs WHERE left_id = $1 AND right_id = $2
		FOR UPDATE`, key.LeftID, key.RightID).Scan(
		&row.State.LeftLiked, &row.State.RightLiked,
		&row.LeftBlocked, &row.RightBlocked,
	)
	if err != nil {
		return pairRow{}, classify(err)
	}
	return row, nil
}
