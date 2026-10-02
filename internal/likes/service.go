package likes

import (
	"errors"
	"fmt"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/memory"
)

// получение профиля
type ProfileSource interface {
	Get(id domain.UserID) (domain.Profile, error)
}

// проверка блокировок и реакций
type InteractionSource interface {
	IsBlocked(userID, candidateID domain.UserID) (bool, error)
	HasReaction(userID, candidateID domain.UserID) (bool, error)
}

// сервис проверки лайка
type Service struct {
	profiles     ProfileSource
	interactions InteractionSource
}

// создание сервиса с готовыми хранилищами
func NewService(
	profiles ProfileSource,
	interactions InteractionSource,
) *Service {
	return &Service{
		profiles:     profiles,
		interactions: interactions,
	}
}

// получение данных и проверка лайка
func (s *Service) Check(userID, candidateID domain.UserID) (Result, error) {
	if userID == candidateID {
		return Result{Reason: ReasonSelfLike}, nil
	}

	user, err := s.profiles.Get(userID)
	if err != nil {
		if errors.Is(err, memory.ErrProfileNotFound) {
			return Result{Reason: ReasonProfileNotFound}, nil
		}

		return Result{}, fmt.Errorf("получение пользователя: %w", err)
	}

	candidate, err := s.profiles.Get(candidateID)
	if err != nil {
		if errors.Is(err, memory.ErrProfileNotFound) {
			return Result{Reason: ReasonProfileNotFound}, nil
		}

		return Result{}, fmt.Errorf("получение кандидата: %w", err)
	}

	result := Check(user, candidate, false, false)
	if !result.Allowed {
		return result, nil
	}

	blocked, err := s.interactions.IsBlocked(userID, candidateID)
	if err != nil {
		return Result{}, fmt.Errorf("проверка блокировки: %w", err)
	}

	if blocked {
		return Check(user, candidate, true, false), nil
	}

	alreadyReacted, err := s.interactions.HasReaction(userID, candidateID)
	if err != nil {
		return Result{}, fmt.Errorf("проверка прошлой реакции: %w", err)
	}

	return Check(user, candidate, blocked, alreadyReacted), nil
}