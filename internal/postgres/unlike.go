package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func (s *Store) Unlike(ctx context.Context, actorID, targetID domain.UserID) (domain.PairChange, error) {
	key, err := domain.NewPairKey(actorID, targetID)
	if err != nil {
		return domain.PairChange{}, err
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.PairChange{}, fmt.Errorf("begin unlike: %w", classify(err))
	}
	defer rollback(tx)

	if _, err := lockPair(ctx, tx, key); err != nil {
		return domain.PairChange{}, fmt.Errorf("lock pair: %w", err)
	}

	query := `UPDATE pairs SET left_liked = false
		WHERE left_id = $1 AND right_id = $2 AND left_liked`
	if actorID == key.RightID {
		query = `UPDATE pairs SET right_liked = false
			WHERE left_id = $1 AND right_id = $2 AND right_liked`
	}
	like, err := tx.Exec(ctx, query, key.LeftID, key.RightID)
	if err != nil {
		return domain.PairChange{}, fmt.Errorf("remove like: %w", classify(err))
	}

	match, err := tx.Exec(ctx, `UPDATE matches SET active = false
		WHERE left_id = $1 AND right_id = $2 AND active`, key.LeftID, key.RightID)
	if err != nil {
		return domain.PairChange{}, fmt.Errorf("deactivate match: %w", classify(err))
	}

	if err := commit(ctx, tx); err != nil {
		return domain.PairChange{}, fmt.Errorf("commit unlike: %w", err)
	}
	return domain.PairChange{Changed: like.RowsAffected() > 0 || match.RowsAffected() > 0}, nil
}
