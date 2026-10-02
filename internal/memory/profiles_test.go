package memory_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kmradished/GOsapiens/internal/discovery/interests"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/memory"
)

var _ interests.ProfileSource = (*memory.Profiles)(nil)

func TestProfilesGet(t *testing.T) {
	want := domain.Profile{
		ID:     "u1",
		Name:   "Anna",
		Active: true,
		Interests: domain.Interests{
			Movies: []string{"Dune"},
		},
	}
	profiles := memory.NewProfiles([]domain.Profile{want})

	got, err := profiles.Get("u1")
	if err != nil {
		t.Fatalf("Get() returned an unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Get() = %#v, want %#v", got, want)
	}
}

func TestProfilesGetReturnsNotFound(t *testing.T) {
	profiles := memory.NewProfiles(nil)

	got, err := profiles.Get("missing")

	if !reflect.DeepEqual(got, domain.Profile{}) {
		t.Fatalf("Get() = %#v, want zero profile", got)
	}

	if !errors.Is(err, memory.ErrProfileNotFound) {
		t.Fatalf("Get() error = %v, want %v", err, memory.ErrProfileNotFound)
	}
}

func TestProfilesListPreservesSeedOrder(t *testing.T) {
	seed := []domain.Profile{
		{ID: "u3", Name: "Third"},
		{ID: "u1", Name: "First"},
		{ID: "u2", Name: "Second"},
	}
	profiles := memory.NewProfiles(seed)

	got, err := profiles.List()
	if err != nil {
		t.Fatalf("List() returned an unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got, seed) {
		t.Fatalf("List() = %#v, want %#v", got, seed)
	}
}

func TestProfilesIsolatesSeedData(t *testing.T) {
	seed := []domain.Profile{
		{
			ID: "u1",
			Interests: domain.Interests{
				Movies: []string{"Dune"},
			},
		},
	}
	profiles := memory.NewProfiles(seed)

	seed[0].Interests.Movies[0] = "changed outside store"

	got, err := profiles.Get("u1")
	if err != nil {
		t.Fatalf("Get() returned an unexpected error: %v", err)
	}

	if got.Interests.Movies[0] != "Dune" {
		t.Fatalf("stored movie = %q, want %q", got.Interests.Movies[0], "Dune")
	}
}

func TestProfilesIsolatesReturnedData(t *testing.T) {
	profiles := memory.NewProfiles([]domain.Profile{
		{
			ID: "u1",
			Interests: domain.Interests{
				Movies: []string{"Dune"},
				Music:  []string{"Queen"},
				Books:  []string{"1984"},
			},
		},
	})

	fromGet, err := profiles.Get("u1")
	if err != nil {
		t.Fatalf("Get() returned an unexpected error: %v", err)
	}
	fromGet.Interests.Movies[0] = "changed Get result"

	fromList, err := profiles.List()
	if err != nil {
		t.Fatalf("List() returned an unexpected error: %v", err)
	}
	fromList[0].Interests.Music[0] = "changed List result"
	fromList[0].Interests.Books[0] = "changed List result"

	stored, err := profiles.Get("u1")
	if err != nil {
		t.Fatalf("Get() returned an unexpected error: %v", err)
	}

	want := domain.Interests{
		Movies: []string{"Dune"},
		Music:  []string{"Queen"},
		Books:  []string{"1984"},
	}
	if !reflect.DeepEqual(stored.Interests, want) {
		t.Fatalf("stored interests = %#v, want %#v", stored.Interests, want)
	}
}
