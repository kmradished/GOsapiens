package matching

import (
	"errors"

	"github.com/kmradished/GOsapiens/internal/domain"
)

var ErrActorNotInPair = errors.New("actor does not belong to pair")

func DecideLike(
	key domain.PairKey,
	state domain.PairState,
	actorID domain.UserID,
) (domain.LikeDecision, error) {
	if !key.Valid() {
		return domain.LikeDecision{}, domain.ErrInvalidPair
	}

	next := state

	switch actorID {
	case key.LeftID:
		next.LeftLiked = true
	case key.RightID:
		next.RightLiked = true
	default:
		return domain.LikeDecision{}, ErrActorNotInPair
	}

	wasMatched := state.LeftLiked && state.RightLiked
	matched := next.LeftLiked && next.RightLiked

	return domain.LikeDecision{
		State:      next,
		Changed:    next != state,
		Matched:    matched,
		MatchedNow: matched && !wasMatched,
	}, nil
}
