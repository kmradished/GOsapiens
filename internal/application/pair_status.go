package application

import (
	"context"
	"fmt"

	"github.com/kmradished/GOsapiens/internal/domain"
)

// PairMatched сохраняет прежний способ чтения для ЛР № 3.
func (s *Service) PairMatched(actorID, targetID domain.UserID) (bool, error) {
	return s.PairMatchedContext(context.Background(), actorID, targetID)
}

// PairMatchedContext читает результат с контекстом текущего запроса.
func (s *Service) PairMatchedContext(
	ctx context.Context,
	actorID, targetID domain.UserID,
) (bool, error) {
	key, err := domain.NewPairKey(actorID, targetID)
	if err != nil {
		return false, err
	}

	if s.likeStore != nil {
		matched, err := s.likeStore.PairMatched(ctx, actorID, targetID)
		if err != nil {
			return false, fmt.Errorf("read stored pair: %w", err)
		}
		return matched, nil
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
