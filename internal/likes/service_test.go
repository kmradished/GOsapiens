package likes

import (
	"errors"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/memory"
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
		return domain.Profile{}, memory.ErrProfileNotFound
	}

	return profile, nil
}

// тестовое хранилище блокировок и реакций
type interactionStub struct {
	blocked        bool
	alreadyReacted bool
	blockErr       error
	reactionErr    error
}

func (s *interactionStub) IsBlocked(
	userID, candidateID domain.UserID,
) (bool, error) {
	return s.blocked, s.blockErr
}

func (s *interactionStub) HasReaction(
	userID, candidateID domain.UserID,
) (bool, error) {
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

// ошибка проверки реакции
func TestServiceReactionSourceError(t *testing.T) {
	sourceErr := errors.New("реакции недоступны")
	interactions := &interactionStub{
		reactionErr: sourceErr,
	}
	service := NewService(activeProfiles(), interactions)

	_, err := service.Check("u1", "u2")

	if !errors.Is(err, sourceErr) {
		t.Fatalf("ожидалась ошибка хранилища, получено: %v", err)
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

// повторная реакция запрещена
func TestServiceAlreadyReacted(t *testing.T) {
	interactions := &interactionStub{
		alreadyReacted: true,
	}
	service := NewService(activeProfiles(), interactions)

	result, err := service.Check("u1", "u2")

	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	if result.Allowed || result.Reason != ReasonAlreadyReacted {
		t.Fatalf("ожидался отказ за повторную реакцию, получено: %+v", result)
	}
}