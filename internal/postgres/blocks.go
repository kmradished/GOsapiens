package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func (s *Store) Block(ctx context.Context, actorID, targetID domain.UserID) (domain.PairChange, error) {
	key, err := domain.NewPairKey(actorID, targetID)
	if err != nil {
		return domain.PairChange{}, err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.PairChange{}, fmt.Errorf("begin block: %w", classify(err))
	}
	defer rollback(tx)

	row, err := lockPair(ctx, tx, key)
	if err != nil {
		return domain.PairChange{}, fmt.Errorf("lock block pair: %w", err)
	}
	blocked := row.LeftBlocked
	if actorID == key.RightID {
		blocked = row.RightBlocked
	}
	if !blocked {
		_, err = tx.Exec(ctx, `
			UPDATE pairs
			SET left_blocked = left_blocked OR left_id = $3,
			    right_blocked = right_blocked OR right_id = $3
			WHERE left_id = $1 AND right_id = $2`, key.LeftID, key.RightID, actorID)
		if err != nil {
			return domain.PairChange{}, fmt.Errorf("save block: %w", classify(err))
		}
	}
	result, err := tx.Exec(ctx, `
		UPDATE matches SET active = false
		WHERE left_id = $1 AND right_id = $2 AND active`, key.LeftID, key.RightID)
	if err != nil {
		return domain.PairChange{}, fmt.Errorf("deactivate blocked match: %w", classify(err))
	}
	if err := commit(ctx, tx); err != nil {
		return domain.PairChange{}, fmt.Errorf("commit block: %w", err)
	}
	return domain.PairChange{Changed: !blocked || result.RowsAffected() != 0}, nil
}
