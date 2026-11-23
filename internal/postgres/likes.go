package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/matching"
)

// Like сохраняет реакцию и создание или активацию мэтча одной транзакцией.
func (s *Store) Like(
	ctx context.Context,
	actorID, targetID domain.UserID,
) (domain.LikeDecision, error) {
	key, err := domain.NewPairKey(actorID, targetID)
	if err != nil {
		return domain.LikeDecision{}, err
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return domain.LikeDecision{}, fmt.Errorf("begin like: %w", classify(err))
	}
	defer rollback(tx)

	row, err := lockPair(ctx, tx, key)
	if err != nil {
		return domain.LikeDecision{}, fmt.Errorf("lock like pair: %w", err)
	}
	if row.LeftBlocked || row.RightBlocked {
		return domain.LikeDecision{}, domain.ErrBlocked
	}

	decision, err := matching.DecideLike(key, row.State, actorID)
	if err != nil {
		return domain.LikeDecision{}, fmt.Errorf("decide like: %w", err)
	}

	if decision.Changed {
		_, err = tx.Exec(ctx, `
			UPDATE pairs
			SET left_liked = $3, right_liked = $4
			WHERE left_id = $1 AND right_id = $2`,
			key.LeftID, key.RightID,
			decision.State.LeftLiked, decision.State.RightLiked,
		)
		if err != nil {
			return domain.LikeDecision{}, fmt.Errorf("save reaction: %w", classify(err))
		}

		if decision.Matched {
			_, err = tx.Exec(ctx, `
				INSERT INTO matches (left_id, right_id, active)
				VALUES ($1, $2, true)
				ON CONFLICT (left_id, right_id)
				DO UPDATE SET active = true
				WHERE matches.active = false`,
				key.LeftID, key.RightID,
			)
			if err != nil {
				return domain.LikeDecision{}, fmt.Errorf("save match: %w", classify(err))
			}
		}
	}

	if err := commit(ctx, tx); err != nil {
		return domain.LikeDecision{}, fmt.Errorf("commit like: %w", err)
	}

	return decision, nil
}
