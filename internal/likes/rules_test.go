package likes

import (
	"reflect"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

// лайк разрешён
func TestCheckAllowed(t *testing.T) {
	user := domain.Profile{ID: "u1", Active: true}
	candidate := domain.Profile{ID: "u2", Active: true}

	result := Check(user, candidate, false, false)

	if !result.Allowed || result.Reason != ReasonNone {
		t.Fatalf("ожидалось разрешение, получено: %+v", result)
	}
}

// лайк себе запрещён
func TestCheckSelfLike(t *testing.T) {
	user := domain.Profile{ID: "u1", Active: true}

	result := Check(user, user, false, false)

	if result.Allowed || result.Reason != ReasonSelfLike {
		t.Fatalf("ожидался отказ за лайк себе, получено: %+v", result)
	}
}

// неактивный пользователь
func TestCheckInactiveUser(t *testing.T) {
	user := domain.Profile{ID: "u1", Active: false}
	candidate := domain.Profile{ID: "u2", Active: true}

	result := Check(user, candidate, false, false)

	if result.Allowed || result.Reason != ReasonInactiveProfile {
		t.Fatalf("ожидался отказ за неактивный профиль, получено: %+v", result)
	}
}

// неактивный кандидат
func TestCheckInactiveCandidate(t *testing.T) {
	user := domain.Profile{ID: "u1", Active: true}
	candidate := domain.Profile{ID: "u2", Active: false}

	result := Check(user, candidate, false, false)

	if result.Allowed || result.Reason != ReasonInactiveProfile {
		t.Fatalf("ожидался отказ за неактивный профиль, получено: %+v", result)
	}
}

// блокировка запрещает лайк
func TestCheckBlocked(t *testing.T) {
	user := domain.Profile{ID: "u1", Active: true}
	candidate := domain.Profile{ID: "u2", Active: true}

	result := Check(user, candidate, true, false)

	if result.Allowed || result.Reason != ReasonBlocked {
		t.Fatalf("ожидался отказ за блокировку, получено: %+v", result)
	}
}

// повторная реакция запрещена
func TestCheckAlreadyReacted(t *testing.T) {
	user := domain.Profile{ID: "u1", Active: true}
	candidate := domain.Profile{ID: "u2", Active: true}

	result := Check(user, candidate, false, true)

	if result.Allowed || result.Reason != ReasonAlreadyReacted {
		t.Fatalf("ожидался отказ за повторную реакцию, получено: %+v", result)
	}
}

// входные профили не меняются
func TestCheckDoesNotChangeProfiles(t *testing.T) {
	user := domain.Profile{
		ID:     "u1",
		Active: true,
		Interests: domain.Interests{
			Movies: []string{"Dune"},
		},
	}
	candidate := domain.Profile{
		ID:     "u2",
		Active: true,
		Interests: domain.Interests{
			Music: []string{"Queen"},
		},
	}

	expectedUser := domain.Profile{
		ID:     "u1",
		Active: true,
		Interests: domain.Interests{
			Movies: []string{"Dune"},
		},
	}
	expectedCandidate := domain.Profile{
		ID:     "u2",
		Active: true,
		Interests: domain.Interests{
			Music: []string{"Queen"},
		},
	}

	Check(user, candidate, false, false)

	if !reflect.DeepEqual(user, expectedUser) {
		t.Fatal("профиль пользователя изменился")
	}

	if !reflect.DeepEqual(candidate, expectedCandidate) {
		t.Fatal("профиль кандидата изменился")
	}
}
