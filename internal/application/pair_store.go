package application

import "github.com/kmradished/GOsapiens/internal/domain"

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
