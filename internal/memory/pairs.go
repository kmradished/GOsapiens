package memory

import "github.com/kmradished/GOsapiens/internal/domain"

// PairStore хранит состояния канонических пар в памяти.
type PairStore struct {
	pairs map[domain.PairKey]domain.PairState
}

// NewPairStore создаёт пустое хранилище пар.
func NewPairStore() *PairStore {
	return &PairStore{
		pairs: make(map[domain.PairKey]domain.PairState),
	}
}

// Load возвращает сохранённое состояние пары.
func (s *PairStore) Load(
	key domain.PairKey,
) (domain.PairState, bool, error) {
	if !key.Valid() {
		return domain.PairState{}, false, domain.ErrInvalidPair
	}

	state, found := s.pairs[key]
	return state, found, nil
}

// Save сохраняет или полностью заменяет состояние пары.
func (s *PairStore) Save(
	key domain.PairKey,
	state domain.PairState,
) error {
	if !key.Valid() {
		return domain.ErrInvalidPair
	}

	s.pairs[key] = state
	return nil
}
