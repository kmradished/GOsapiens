package interests_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kmradished/GOsapiens/internal/discovery/interests"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestServiceRecommend(t *testing.T) {
	current := domain.Profile{
		ID: "u1",
		Interests: domain.Interests{
			Movies: []string{"Dune"},
		},
	}
	candidates := []domain.Profile{
		{
			ID:     "u2",
			Active: true,
			Interests: domain.Interests{
				Movies: []string{"Dune"},
			},
		},
	}
	source := &profileSourceStub{
		getProfile: current,
		list:       candidates,
	}
	service := interests.NewService(source)

	got, err := service.Recommend("u1")
	if err != nil {
		t.Fatalf("Recommend() returned an unexpected error: %v", err)
	}

	want := []interests.AnonymousCard{
		{
			CandidateID:  "u2",
			CommonMovies: []string{"Dune"},
			CommonMusic:  []string{},
			CommonBooks:  []string{},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Recommend() = %#v, want %#v", got, want)
	}

	if source.getID != "u1" {
		t.Fatalf("Get() received ID %q, want %q", source.getID, domain.UserID("u1"))
	}
}

func TestServiceRecommendReturnsGetError(t *testing.T) {
	errUnavailable := errors.New("profile source unavailable")
	source := &profileSourceStub{getErr: errUnavailable}
	service := interests.NewService(source)

	got, err := service.Recommend("u1")

	if got != nil {
		t.Fatalf("Recommend() = %#v, want nil result after Get error", got)
	}

	if !errors.Is(err, errUnavailable) {
		t.Fatalf("Recommend() error = %v, want wrapped error %v", err, errUnavailable)
	}

	if source.listCalls != 0 {
		t.Fatalf("List() called %d times after Get error, want 0", source.listCalls)
	}
}

func TestServiceRecommendReturnsListError(t *testing.T) {
	errUnavailable := errors.New("profile source unavailable")
	source := &profileSourceStub{
		getProfile: domain.Profile{ID: "u1"},
		listErr:    errUnavailable,
	}
	service := interests.NewService(source)

	got, err := service.Recommend("u1")

	if got != nil {
		t.Fatalf("Recommend() = %#v, want nil result after List error", got)
	}

	if !errors.Is(err, errUnavailable) {
		t.Fatalf("Recommend() error = %v, want wrapped error %v", err, errUnavailable)
	}

	if source.listCalls != 1 {
		t.Fatalf("List() called %d times, want 1", source.listCalls)
	}
}

type profileSourceStub struct {
	getID      domain.UserID
	getProfile domain.Profile
	getErr     error
	list       []domain.Profile
	listErr    error
	listCalls  int
}

var _ interests.ProfileSource = (*profileSourceStub)(nil)

func (s *profileSourceStub) Get(id domain.UserID) (domain.Profile, error) {
	s.getID = id
	return s.getProfile, s.getErr
}

func (s *profileSourceStub) List() ([]domain.Profile, error) {
	s.listCalls++
	return s.list, s.listErr
}
