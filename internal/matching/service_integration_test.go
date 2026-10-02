package matching_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/matching"
	"github.com/kmradished/GOsapiens/internal/memory"
)

func testProfiles() []domain.Profile {
	return []domain.Profile{
		{
			ID:        "u1",
			Name:      "Арина",
			PhotoURL:  "arina.jpg",
			Bio:       "Люблю книги",
			Active:    true,
			Interests: domain.Interests{Books: []string{"Дюна"}},
		},
		{
			ID:        "u10",
			Name:      "Даша",
			PhotoURL:  "dasha.jpg",
			Bio:       "Люблю кино",
			Active:    true,
			Interests: domain.Interests{Movies: []string{"Матрица"}},
		},
	}
}

func TestServiceCreatesMatchAndOpensProfiles(t *testing.T) {
	profiles := testProfiles()

	reactions := []domain.Reaction{
		{
			SenderID:   "u1",
			ReceiverID: "u10",
			Kind:       domain.ReactionLike,
		},
		{
			SenderID:   "u10",
			ReceiverID: "u1",
			Kind:       domain.ReactionLike,
		},
	}

	source := memory.NewInteractions(profiles, reactions, nil)
	service := matching.NewService(source)

	got, err := service.Resolve("u1", "u10")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	want := matching.Result{
		Status: matching.StatusCreated,
		Match: domain.Match{
			FirstUserID:  "u1",
			SecondUserID: "u10",
		},
		FullProfilesAvailable: true,
	}

	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	dasha, err := service.FullProfile("u1", "u10")
	if err != nil || !reflect.DeepEqual(dasha, profiles[1]) {
		t.Fatalf("Dasha's profile: got %+v, error %v", dasha, err)
	}

	arina, err := service.FullProfile("u10", "u1")
	if err != nil || !reflect.DeepEqual(arina, profiles[0]) {
		t.Fatalf("Arina's profile: got %+v, error %v", arina, err)
	}

	again, err := service.Resolve("u10", "u1")
	if err != nil ||
		again.Status != matching.StatusAlreadyExists ||
		!again.FullProfilesAvailable {
		t.Fatalf("repeated resolve: got %+v, error %v", again, err)
	}
}
func TestServiceWaitsAndKeepsProfilesClosed(t *testing.T) {
	reactions := []domain.Reaction{
		{
			SenderID:   "u1",
			ReceiverID: "u10",
			Kind:       domain.ReactionLike,
		},
	}

	source := memory.NewInteractions(testProfiles(), reactions, nil)
	service := matching.NewService(source)

	got, err := service.Resolve("u1", "u10")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	want := matching.Result{
		Status: matching.StatusWaiting,
	}

	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	exists, err := source.HasMatch("u1", "u10")
	if err != nil || exists {
		t.Fatalf("unexpected match: exists %v, error %v", exists, err)
	}

	pairs := [][2]domain.UserID{
		{"u1", "u10"},
		{"u10", "u1"},
	}

	for _, pair := range pairs {
		profile, err := service.FullProfile(pair[0], pair[1])

		if !errors.Is(err, matching.ErrProfileAccessDenied) {
			t.Fatalf("expected access denied for %v, got %v", pair, err)
		}

		if !reflect.DeepEqual(profile, domain.Profile{}) {
			t.Fatalf("profile leaked for %v: %+v", pair, profile)
		}
	}
}
