package memory

import (
	"slices"
	"sync"

	"github.com/kmradished/GOsapiens/internal/domain"
)

// PairStore хранит состояния канонических пар в памяти.
type PairStore struct {
	mu     sync.RWMutex
	pairs  map[domain.PairKey]domain.PairState
	writes int
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

	s.mu.RLock()
	defer s.mu.RUnlock()

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

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pairs[key] = state
	s.writes++
	return nil
}

// Update выполняет чтение, чистое правило и запись под одной блокировкой.
// Внутри change нельзя обращаться к сети или повторно вызывать хранилище.
func (s *PairStore) Update(
	key domain.PairKey,
	change func(domain.PairState) (domain.LikeDecision, error),
) (domain.LikeDecision, error) {
	if !key.Valid() {
		return domain.LikeDecision{}, domain.ErrInvalidPair
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	decision, err := change(s.pairs[key])
	if err != nil {
		return domain.LikeDecision{}, err
	}
	if decision.Changed {
		s.pairs[key] = decision.State
		s.writes++
	}
	return decision, nil
}

// LikedIDs возвращает пользователей, которым actor уже поставил Like.
func (s *PairStore) LikedIDs(actorID domain.UserID) ([]domain.UserID, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]domain.UserID, 0)
	for key, state := range s.pairs {
		if key.LeftID == actorID && state.LeftLiked {
			ids = append(ids, key.RightID)
		}
		if key.RightID == actorID && state.RightLiked {
			ids = append(ids, key.LeftID)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

// Writes позволяет проверить, что повтор команды не создаёт лишнюю запись.
func (s *PairStore) Writes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.writes
}
