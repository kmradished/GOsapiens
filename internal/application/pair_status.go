package application

import (
	"fmt"

	"github.com/kmradished/GOsapiens/internal/domain"
)

// PairMatched сообщает, есть ли взаимный лайк у пары.
func (s *Service) PairMatched(actorID, targetID domain.UserID) (bool, error) {
	key, err := domain.NewPairKey(actorID, targetID)
	if err != nil {
		return false, err
	}

	state, found, err := s.store.Load(key)
	if err != nil {
		return false, fmt.Errorf("load pair: %w", err)
	}
	if !found {
		return false, nil
	}

	return state.LeftLiked && state.RightLiked, nil
}
