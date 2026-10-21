package jsonpairs

import (
	"encoding/json"

	"github.com/kmradished/GOsapiens/internal/domain"
)

// PairStore хранит JSON-представление состояния каждой пары.
type PairStore struct {
	pairs map[domain.PairKey][]byte
}

// NewPairStore создаёт пустое JSON-хранилище пар.
func NewPairStore() *PairStore {
	return &PairStore{
		pairs: make(map[domain.PairKey][]byte),
	}
}

// Load десериализует и возвращает сохранённое состояние пары.
func (s *PairStore) Load(
	key domain.PairKey,
) (domain.PairState, bool, error) {
	if !key.Valid() {
		return domain.PairState{}, false, domain.ErrInvalidPair
	}

	data, found := s.pairs[key]
	if !found {
		return domain.PairState{}, false, nil
	}

	var state domain.PairState
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.PairState{}, false, err
	}

	return state, true, nil
}

// Save сериализует и сохраняет полное состояние пары.
func (s *PairStore) Save(
	key domain.PairKey,
	state domain.PairState,
) error {
	if !key.Valid() {
		return domain.ErrInvalidPair
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	s.pairs[key] = data
	return nil
}
