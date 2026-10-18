package application

import (
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
)

// PairStore хранит состояние Like одной канонической пары.
type PairStore interface {
	Load(key domain.PairKey) (
		state domain.PairState,
		found bool,
		err error,
	)

	Save(
		key domain.PairKey,
		state domain.PairState,
	) error
}

type LikeChecker interface {
	Check(
		actorID domain.UserID,
		targetID domain.UserID,
	) (likes.Result, error)
}

type DecideLikeFunc func(
	key domain.PairKey,
	state domain.PairState,
	actorID domain.UserID,
) (domain.LikeDecision, error)
