package application

import (
	"context"
	"fmt"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
)

// LikeDeniedError сообщает причину предметного отказа.
type LikeDeniedError struct {
	Reason likes.Reason
}

func (e *LikeDeniedError) Error() string {
	return fmt.Sprintf("like denied: %s", e.Reason)
}

type pairReader interface {
	Load(key domain.PairKey) (domain.PairState, bool, error)
}

type atomicPairStore interface {
	pairReader
	Update(
		key domain.PairKey,
		change func(domain.PairState) (domain.LikeDecision, error),
	) (domain.LikeDecision, error)
}

type checkLikeFunc func(context.Context, domain.UserID, domain.UserID) (likes.Result, error)

type updatePairFunc func(
	domain.PairKey,
	func(domain.PairState) (domain.LikeDecision, error),
) (domain.LikeDecision, error)

type Service struct {
	check  checkLikeFunc
	store  pairReader
	update updatePairFunc
	decide DecideLikeFunc
}

// NewService сохраняет поддержку хранилищ ЛР № 2 с отдельными Load и Save.
func NewService(checker LikeChecker, store PairStore, decide DecideLikeFunc) *Service {
	return &Service{
		check: func(ctx context.Context, actorID, targetID domain.UserID) (likes.Result, error) {
			return checker.Check(ctx, actorID, targetID)
		},
		store: store,
		update: func(
			key domain.PairKey,
			change func(domain.PairState) (domain.LikeDecision, error),
		) (domain.LikeDecision, error) {
			state, found, err := store.Load(key)
			if err != nil {
				return domain.LikeDecision{}, fmt.Errorf("load pair: %w", err)
			}
			if !found {
				state = domain.PairState{}
			}

			decision, err := change(state)
			if err != nil {
				return domain.LikeDecision{}, err
			}
			if !decision.Changed {
				return decision, nil
			}
			if err := store.Save(key, decision.State); err != nil {
				return domain.LikeDecision{}, fmt.Errorf("save pair: %w", err)
			}
			return decision, nil
		},
		decide: decide,
	}
}

// NewServiceWithUpdate подключает проверку с контекстом и атомарное обновление.
func NewServiceWithUpdate(check checkLikeFunc, store atomicPairStore, decide DecideLikeFunc) *Service {
	return &Service{
		check:  check,
		store:  store,
		update: store.Update,
		decide: decide,
	}
}

// Like сохраняет прежний способ вызова для ЛР № 2.
func (s *Service) Like(actorID, targetID domain.UserID) (domain.LikeDecision, error) {
	return s.LikeContext(context.Background(), actorID, targetID)
}

// LikeContext проверяет допуск и применяет лайк через подключённый Update.
func (s *Service) LikeContext(
	ctx context.Context,
	actorID, targetID domain.UserID,
) (domain.LikeDecision, error) {
	check, err := s.check(ctx, actorID, targetID)
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

	decision, err := s.update(key, func(state domain.PairState) (domain.LikeDecision, error) {
		return s.decide(key, state, actorID)
	})
	if err != nil {
		return domain.LikeDecision{}, fmt.Errorf("update pair: %w", err)
	}

	return decision, nil
}
