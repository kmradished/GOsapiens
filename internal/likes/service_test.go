package likes

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

// тестовое хранилище профилей
type profileStub struct {
	profiles map[domain.UserID]domain.Profile
	getErr   error
	errID    domain.UserID
}

func (s *profileStub) Get(id domain.UserID) (domain.Profile, error) {
	if s.getErr != nil && (s.errID == "" || s.errID == id) {
		return domain.Profile{}, s.getErr
	}

	profile, found := s.profiles[id]
	if !found {
		return domain.Profile{}, domain.ErrProfileNotFound
	}

	return profile, nil
}

// тестовое хранилище блокировок и реакций
type interactionStub struct {
	blocked        bool
	alreadyReacted bool
	blockErr       error
	reactionErr    error
	reactionCalls  int
}

func (s *interactionStub) IsBlocked(
	userID, candidateID domain.UserID,
) (bool, error) {
	return s.blocked, s.blockErr
}

func (s *interactionStub) HasReaction(
	userID, candidateID domain.UserID,
) (bool, error) {
	s.reactionCalls++
	return s.alreadyReacted, s.reactionErr
}

// два активных профиля для тестов
func activeProfiles() *profileStub {
	return &profileStub{
		profiles: map[domain.UserID]domain.Profile{
			"u1": {ID: "u1", Active: true},
			"u2": {ID: "u2", Active: true},
		},
	}
}

// лайк разрешён
func TestServiceAllowed(t *testing.T) {
	service := NewService(activeProfiles(), &interactionStub{})

	result, err := service.Check("u1", "u2")

	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	if !result.Allowed || result.Reason != ReasonNone {
		t.Fatalf("ожидалось разрешение, получено: %+v", result)
	}
}

// пользователь не найден
func TestServiceUserNotFound(t *testing.T) {
	profiles := &profileStub{
		profiles: map[domain.UserID]domain.Profile{
			"u2": {ID: "u2", Active: true},
		},
	}
	service := NewService(profiles, &interactionStub{})

	result, err := service.Check("u1", "u2")

	if err != nil {
		t.Fatalf("ожидался обычный отказ, получена ошибка: %v", err)
	}

	if result.Allowed || result.Reason != ReasonProfileNotFound {
		t.Fatalf("ожидался отказ за отсутствующий профиль, получено: %+v", result)
	}
}

// кандидат не найден
func TestServiceCandidateNotFound(t *testing.T) {
	profiles := &profileStub{
		profiles: map[domain.UserID]domain.Profile{
			"u1": {ID: "u1", Active: true},
		},
	}
	service := NewService(profiles, &interactionStub{})

	result, err := service.Check("u1", "u2")

	if err != nil {
		t.Fatalf("ожидался обычный отказ, получена ошибка: %v", err)
	}

	if result.Allowed || result.Reason != ReasonProfileNotFound {
		t.Fatalf("ожидался отказ за отсутствующий профиль, получено: %+v", result)
	}
}

// ошибка загрузки пользователя
func TestServiceUserSourceError(t *testing.T) {
	sourceErr := errors.New("хранилище недоступно")
	profiles := activeProfiles()
	profiles.getErr = sourceErr
	profiles.errID = "u1"

	service := NewService(profiles, &interactionStub{})

	_, err := service.Check("u1", "u2")

	if !errors.Is(err, sourceErr) {
		t.Fatalf("ожидалась ошибка хранилища, получено: %v", err)
	}
}

// ошибка загрузки кандидата
func TestServiceCandidateSourceError(t *testing.T) {
	sourceErr := errors.New("хранилище недоступно")
	profiles := activeProfiles()
	profiles.getErr = sourceErr
	profiles.errID = "u2"

	service := NewService(profiles, &interactionStub{})

	_, err := service.Check("u1", "u2")

	if !errors.Is(err, sourceErr) {
		t.Fatalf("ожидалась ошибка хранилища, получено: %v", err)
	}
}

// ошибка проверки блокировки
func TestServiceBlockSourceError(t *testing.T) {
	sourceErr := errors.New("блокировки недоступны")
	interactions := &interactionStub{
		blockErr: sourceErr,
	}
	service := NewService(activeProfiles(), interactions)

	_, err := service.Check("u1", "u2")

	if !errors.Is(err, sourceErr) {
		t.Fatalf("ожидалась ошибка хранилища, получено: %v", err)
	}
}

// прошлые реакции не проверяются
func TestServiceIgnoresReactionSourceError(t *testing.T) {
	sourceErr := errors.New("реакции недоступны")
	interactions := &interactionStub{
		reactionErr: sourceErr,
	}
	service := NewService(activeProfiles(), interactions)

	result, err := service.Check("u1", "u2")

	if err != nil || result != (Result{Allowed: true}) {
		t.Fatalf("ожидалось разрешение, получено: %+v, %v", result, err)
	}
	if interactions.reactionCalls != 0 {
		t.Fatal("HasReaction не должен вызываться")
	}
}

// лайк себе без загрузки данных
func TestServiceSelfLike(t *testing.T) {
	profiles := &profileStub{
		getErr: errors.New("профили недоступны"),
	}
	service := NewService(profiles, &interactionStub{})

	result, err := service.Check("u1", "u1")

	if err != nil {
		t.Fatalf("хранилище не должно вызываться: %v", err)
	}

	if result.Allowed || result.Reason != ReasonSelfLike {
		t.Fatalf("ожидался отказ за лайк себе, получено: %+v", result)
	}
}

// неактивный пользователь
func TestServiceInactiveUser(t *testing.T) {
	profiles := activeProfiles()
	profiles.profiles["u1"] = domain.Profile{ID: "u1", Active: false}
	interactions := &interactionStub{
		blockErr: errors.New("блокировки недоступны"),
	}
	service := NewService(profiles, interactions)

	result, err := service.Check("u1", "u2")

	if err != nil {
		t.Fatalf("блокировки не должны проверяться: %v", err)
	}

	if result.Allowed || result.Reason != ReasonInactiveProfile {
		t.Fatalf("ожидался отказ за неактивный профиль, получено: %+v", result)
	}
}

// неактивный кандидат
func TestServiceInactiveCandidate(t *testing.T) {
	profiles := activeProfiles()
	profiles.profiles["u2"] = domain.Profile{ID: "u2", Active: false}
	interactions := &interactionStub{
		blockErr: errors.New("блокировки недоступны"),
	}
	service := NewService(profiles, interactions)

	result, err := service.Check("u1", "u2")

	if err != nil {
		t.Fatalf("блокировки не должны проверяться: %v", err)
	}

	if result.Allowed || result.Reason != ReasonInactiveProfile {
		t.Fatalf("ожидался отказ за неактивный профиль, получено: %+v", result)
	}
}

// при блокировке реакции не проверяются
func TestServiceBlocked(t *testing.T) {
	interactions := &interactionStub{
		blocked:     true,
		reactionErr: errors.New("реакции недоступны"),
	}
	service := NewService(activeProfiles(), interactions)

	result, err := service.Check("u1", "u2")

	if err != nil {
		t.Fatalf("реакции не должны проверяться: %v", err)
	}

	if result.Allowed || result.Reason != ReasonBlocked {
		t.Fatalf("ожидался отказ за блокировку, получено: %+v", result)
	}
}

// повторный лайк разрешён
func TestServiceAlreadyReacted(t *testing.T) {
	interactions := &interactionStub{
		alreadyReacted: true,
	}
	service := NewService(activeProfiles(), interactions)

	result, err := service.Check("u1", "u2")

	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	if !result.Allowed || result.Reason != ReasonNone {
		t.Fatalf("ожидалось разрешение повторного Like, получено: %+v", result)
	}
	if interactions.reactionCalls != 0 {
		t.Fatal("HasReaction не должен вызываться")
	}
}

func TestServiceWrappedProfileNotFound(t *testing.T) {
	for _, id := range []domain.UserID{"u1", "u2"} {
		t.Run(string(id), func(t *testing.T) {
			profiles := activeProfiles()
			profiles.errID = id
			profiles.getErr = fmt.Errorf("profile source: %w", domain.ErrProfileNotFound)
			result, err := NewService(profiles, &interactionStub{}).Check("u1", "u2")
			if err != nil || result != (Result{Reason: ReasonProfileNotFound}) {
				t.Fatalf("ожидался отказ за отсутствующий профиль, получено: %+v, %v", result, err)
			}
		})
	}
}

func TestServiceWrappedTechnicalErrors(t *testing.T) {
	for _, stage := range []string{"actor", "target", "blocks"} {
		t.Run(stage, func(t *testing.T) {
			cause := errors.New("source unavailable")
			wrapped := fmt.Errorf("adapter: %w", cause)
			profiles := activeProfiles()
			interactions := &interactionStub{}
			switch stage {
			case "actor":
				profiles.errID, profiles.getErr = "u1", wrapped
			case "target":
				profiles.errID, profiles.getErr = "u2", wrapped
			case "blocks":
				interactions.blockErr = wrapped
			}
			result, err := NewService(profiles, interactions).Check("u1", "u2")
			if !errors.Is(err, cause) || !errors.Is(err, wrapped) {
				t.Fatalf("причина ошибки потеряна: %v", err)
			}
			if result != (Result{}) {
				t.Fatalf("при технической ошибке ожидался нулевой результат: %+v", result)
			}
		})
	}
}
