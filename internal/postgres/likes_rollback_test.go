//go:build dbintegration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestLikeRollbackPreservesReciprocalLike(t *testing.T) {
	tests := []struct {
		name             string
		hasInactiveMatch bool
	}{
		{name: "creating match fails"},
		{name: "reactivating match fails", hasInactiveMatch: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := testStore(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Этот встречный лайк уже подтверждён отдельной транзакцией.
			if _, err := store.Like(ctx, "u2", "u1"); err != nil {
				t.Fatal(err)
			}

			wantMatches := 0
			if tt.hasInactiveMatch {
				_, err := store.Pool.Exec(ctx, `
					INSERT INTO matches (left_id, right_id, active)
					VALUES ('u1', 'u2', false)
				`)
				if err != nil {
					t.Fatal(err)
				}
				wantMatches = 1
			}

			// Тестовый триггер ломает запись мэтча.
			// Сначала он убеждается, что обе реакции уже записаны внутри tx.
			_, err := store.Pool.Exec(ctx, `
				CREATE FUNCTION reject_match_write()
				RETURNS trigger
				LANGUAGE plpgsql
				AS $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1 FROM pairs
						WHERE left_id = NEW.left_id
						  AND right_id = NEW.right_id
						  AND left_liked
						  AND right_liked
					) THEN
						RAISE EXCEPTION 'reaction_missing_before_match'
							USING ERRCODE = 'P0002';
					END IF;

					RAISE EXCEPTION 'forced_match_failure'
						USING ERRCODE = 'P0001';
				END;
				$$;

				CREATE TRIGGER reject_match_write
				BEFORE INSERT OR UPDATE ON matches
				FOR EACH ROW
				EXECUTE FUNCTION reject_match_write();
			`)
			if err != nil {
				t.Fatal(err)
			}

			// Держим отдельное соединение для наблюдения.
			// Операция Like вынуждена использовать другое соединение пула.
			observer, err := store.Pool.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer observer.Release()

			got, err := store.Like(ctx, "u1", "u2")

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) ||
				pgErr.Code != "P0001" ||
				pgErr.Message != "forced_match_failure" {
				t.Fatalf("Like() error = %v, want forced match failure", err)
			}
			if got != (domain.LikeDecision{}) {
				t.Errorf("failed Like returned %+v, want empty decision", got)
			}

			var saved domain.PairState
			err = observer.QueryRow(ctx, `
				SELECT left_liked, right_liked
				FROM pairs
				WHERE left_id = 'u1' AND right_id = 'u2'
			`).Scan(&saved.LeftLiked, &saved.RightLiked)
			if err != nil {
				t.Fatal(err)
			}

			wantState := domain.PairState{RightLiked: true}
			if saved != wantState {
				t.Fatalf("rollback left state %+v, want %+v", saved, wantState)
			}

			var total, active int
			err = observer.QueryRow(ctx, `
				SELECT count(*), count(*) FILTER (WHERE active)
				FROM matches
				WHERE left_id = 'u1' AND right_id = 'u2'
			`).Scan(&total, &active)
			if err != nil {
				t.Fatal(err)
			}
			if total != wantMatches || active != 0 {
				t.Fatalf("rollback left matches: total=%d, active=%d; want %d and 0",
					total, active, wantMatches)
			}

			// Убираем искусственную ошибку и проверяем восстановление работы.
			_, err = store.Pool.Exec(ctx, `DROP TRIGGER reject_match_write ON matches`)
			if err != nil {
				t.Fatal(err)
			}

			got, err = store.Like(ctx, "u1", "u2")
			want := domain.LikeDecision{
				State: domain.PairState{
					LeftLiked:  true,
					RightLiked: true,
				},
				Changed:    true,
				Matched:    true,
				MatchedNow: true,
			}
			if err != nil || got != want {
				t.Fatalf("retry Like() = %+v, %v; want %+v, nil", got, err, want)
			}

			err = observer.QueryRow(ctx, `
				SELECT count(*), count(*) FILTER (WHERE active)
				FROM matches
				WHERE left_id = 'u1' AND right_id = 'u2'
			`).Scan(&total, &active)
			if err != nil {
				t.Fatal(err)
			}
			if total != 1 || active != 1 {
				t.Errorf("after recovery: total=%d, active=%d; want 1 and 1", total, active)
			}
		})
	}
}
