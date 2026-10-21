package application

import (
	"fmt"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
)

// LikeDeniedError сообщает причину бизнес-отказа без обращения к хранилищу пары
type LikeDeniedError struct {
	Reason likes.Reason
}

func (e *LikeDeniedError) Error() string {
	return fmt.Sprintf("like denied: %s", e.Reason)
}

type Service struct {
	checker LikeChecker
	store   PairStore
	decide  DecideLikeFunc
}

func NewService(checker LikeChecker, store PairStore, decide DecideLikeFunc) *Service {
	return &Service{
		checker: checker,
		store:   store,
		decide:  decide,
	}
}

// Like проверяет допуск, вычисляет переход состояния пары и сохраняет изменение
func (s *Service) Like(actorID, targetID domain.UserID) (domain.LikeDecision, error) {
	check, err := s.checker.Check(actorID, targetID)
	if err != nil {
		return domain.LikeDecision{}, fmt.Errorf("check like: %w", err)
	}
	if !check.Allowed {
		return domain.LikeDecision{}, &LikeDeniedError{Reason: check.Reason}
	}

	key, err := domain.NewPairKey(actorID, targetID)
	if err != nil {
		return domain.LikeDecision{}, fmt.Errorf("create pair key: %w", err)
	}

	state, found, err := s.store.Load(key)
	if err != nil {
		return domain.LikeDecision{}, fmt.Errorf("load pair: %w", err)
	}
	if !found {
		state = domain.PairState{}
	}

	decision, err := s.decide(key, state, actorID)
	if err != nil {
		return domain.LikeDecision{}, fmt.Errorf("decide like: %w", err)
	}
	if !decision.Changed {
		return decision, nil
	}

	if err := s.store.Save(key, decision.State); err != nil {
		return domain.LikeDecision{}, fmt.Errorf("save pair: %w", err)
	}

	return decision, nil
}
