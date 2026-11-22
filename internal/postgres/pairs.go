package postgres

import (
	"context"
	"fmt"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func (s *Store) PairMatched(
	ctx context.Context,
	actorID, targetID domain.UserID,
) (bool, error) {
	key, err := domain.NewPairKey(actorID, targetID)
	if err != nil {
		return false, err
	}

	var matched bool
	err = s.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM matches
			WHERE left_id = $1
			  AND right_id = $2
			  AND active = true
		)`, key.LeftID, key.RightID).Scan(&matched)
	if err != nil {
		return false, fmt.Errorf("read pair match: %w", classify(err))
	}

	return matched, nil
}
