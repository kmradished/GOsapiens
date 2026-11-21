//go:build dbintegration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmradished/GOsapiens/internal/domain"
)

// Каждому тесту — отдельная schema. Отсутствие БД является ошибкой,
// а обычный go test без build tag не включает интеграционные проверки.
func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL is required for -tags=dbintegration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("lr4_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, file := range []string{"../../sql/001.sql", "../../sql/reset.sql"} {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}
	return New(pool)
}

func TestSchemaConstraints(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cases := []struct {
		name, query, code, constraint string
	}{
		{"unique match", `INSERT INTO matches VALUES ('u4', 'u6', true)`, "23505", "matches_pkey"},
		{"missing pair", `INSERT INTO matches VALUES ('u1', 'u2', true)`, "23503", "matches_pair_fk"},
		{"missing profile", `INSERT INTO pairs (left_id, right_id) VALUES ('u1', 'unknown')`, "23503", "pairs_right_profile_fk"},
		{"canonical pair", `INSERT INTO pairs (left_id, right_id) VALUES ('u2', 'u1')`, "23514", "pairs_order_check"},
		{"positive version", `UPDATE profiles SET version = 0 WHERE id = 'u1'`, "23514", "profiles_version_check"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.Pool.Exec(ctx, tc.query)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code || pgErr.ConstraintName != tc.constraint {
				t.Fatalf("want %s / %s, got %v", tc.code, tc.constraint, err)
			}
		})
	}
}

func TestPairLockAndCancellation(t *testing.T) {
	s := testStore(t)
	for _, ids := range [][2]domain.UserID{{"u1", "u2"}, {"u2", "u5"}} {
		t.Run(string(ids[0])+"_"+string(ids[1]), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			key, err := domain.NewPairKey(ids[0], ids[1])
			if err != nil {
				t.Fatal(err)
			}
			var initialRows int
			if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM pairs WHERE left_id=$1 AND right_id=$2`, key.LeftID, key.RightID).Scan(&initialRows); err != nil {
				t.Fatal(err)
			}
			first, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
			if err != nil {
				t.Fatal(err)
			}
			defer rollback(first)
			before, err := lockPair(ctx, first, key)
			if err != nil {
				t.Fatal(err)
			}
			// Промежуточный SQL должен откатиться, включая новую строку пары.
			if _, err := first.Exec(ctx, `UPDATE pairs SET right_liked = true WHERE left_id=$1 AND right_id=$2`, key.LeftID, key.RightID); err != nil {
				t.Fatal(err)
			}
			waiting, stopWaiting := context.WithCancel(ctx)
			defer stopWaiting()
			pid := make(chan uint32, 1)
			result := make(chan error, 1)
			done := make(chan struct{})
			go func() {
				defer close(done)
				tx, err := s.Pool.BeginTx(waiting, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
				if err != nil {
					result <- err
					return
				}
				defer rollback(tx)
				pid <- tx.Conn().PgConn().PID()
				_, err = lockPair(waiting, tx, key)
				result <- err
			}()
			// При раннем отказе теста завершаем ожидающую транзакцию и goroutine.
			defer func() {
				stopWaiting()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("waiting transaction did not finish")
				}
			}()
			var backendPID uint32
			select {
			case backendPID = <-pid:
			case err := <-result:
				t.Fatalf("second transaction: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			// Подтверждаем ожидание в PostgreSQL, а не только запуск goroutine.
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				var blocked bool
				if err := s.Pool.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1)) > 0`, backendPID).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				select {
				case <-ticker.C:
				case <-ctx.Done():
					t.Fatal("second connection did not wait for the pair")
				}
			}
			stopWaiting()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("want canceled wait, got %v", err)
				}
			case <-ctx.Done():
				t.Fatal("canceled wait did not finish")
			}
			rollback(first)
			var remainingRows int
			if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM pairs WHERE left_id=$1 AND right_id=$2`, key.LeftID, key.RightID).Scan(&remainingRows); err != nil {
				t.Fatal(err)
			}
			if remainingRows != initialRows {
				t.Fatalf("rollback left a partial pair: before=%d, after=%d", initialRows, remainingRows)
			}
			next, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
			if err != nil {
				t.Fatal(err)
			}
			defer rollback(next)
			after, err := lockPair(ctx, next, key)
			if err != nil {
				t.Fatal(err)
			}
			if after != before {
				t.Fatalf("rollback did not restore pair: before=%+v, after=%+v", before, after)
			}
		})
	}
}
