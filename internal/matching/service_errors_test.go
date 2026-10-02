package matching_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/matching"
)

type sourceStub struct {
	failAt   string
	cause    error
	hasMatch bool
	calls    []string
}

func (s *sourceStub) record(step string) error {
	s.calls = append(s.calls, step)

	if s.failAt == step {
		return s.cause
	}

	return nil
}

func (s *sourceStub) HasMatch(
	firstID, secondID domain.UserID,
) (bool, error) {
	return s.hasMatch, s.record("match")
}

func (s *sourceStub) GetReaction(
	senderID, receiverID domain.UserID,
) (domain.ReactionKind, error) {
	return domain.ReactionLike, s.record("reaction:" + string(senderID))
}

func (s *sourceStub) SaveMatch(match domain.Match) error {
	return s.record("save")
}

func (s *sourceStub) GetProfile(
	id domain.UserID,
) (domain.Profile, error) {
	return domain.Profile{ID: id}, s.record("profile")
}

func TestServicePreservesSourceErrors(t *testing.T) {
	cause := errors.New("source unavailable")

	tests := []struct {
		step      string
		wantCalls []string
	}{
		{"match", []string{"match"}},
		{"reaction:u1", []string{"match", "reaction:u1"}},
		{"reaction:u10", []string{"match", "reaction:u1", "reaction:u10"}},
		{"save", []string{"match", "reaction:u1", "reaction:u10", "save"}},
	}

	for _, tt := range tests {
		t.Run(tt.step, func(t *testing.T) {
			source := &sourceStub{
				failAt: tt.step,
				cause:  cause,
			}

			service := matching.NewService(source)
			got, err := service.Resolve("u1", "u10")

			if !errors.Is(err, cause) {
				t.Fatalf("expected source error, got %v", err)
			}

			if got != (matching.Result{}) {
				t.Fatalf("unexpected result on failure: %+v", got)
			}

			if !reflect.DeepEqual(source.calls, tt.wantCalls) {
				t.Fatalf("calls %v, want %v", source.calls, tt.wantCalls)
			}
		})
	}
}
func TestFullProfilePreservesSourceErrors(t *testing.T) {
	cause := errors.New("source unavailable")

	tests := []struct {
		step      string
		wantCalls []string
	}{
		{"match", []string{"match"}},
		{"profile", []string{"match", "profile"}},
	}

	for _, tt := range tests {
		t.Run(tt.step, func(t *testing.T) {
			source := &sourceStub{
				failAt:   tt.step,
				cause:    cause,
				hasMatch: true,
			}

			profile, err := matching.NewService(source).FullProfile("u1", "u10")

			if !errors.Is(err, cause) {
				t.Fatalf("expected source error, got %v", err)
			}

			if !reflect.DeepEqual(profile, domain.Profile{}) {
				t.Fatalf("unexpected profile on failure: %+v", profile)
			}

			if !reflect.DeepEqual(source.calls, tt.wantCalls) {
				t.Fatalf("calls %v, want %v", source.calls, tt.wantCalls)
			}
		})
	}
}

func TestServiceDoesNotSaveExistingMatch(t *testing.T) {
	source := &sourceStub{hasMatch: true}

	got, err := matching.NewService(source).Resolve("u1", "u10")

	want := matching.Result{
		Status: matching.StatusAlreadyExists,
		Match: domain.Match{
			FirstUserID:  "u1",
			SecondUserID: "u10",
		},
		FullProfilesAvailable: true,
	}

	if err != nil || got != want {
		t.Fatalf("got %+v, error %v, want %+v", got, err, want)
	}

	if !reflect.DeepEqual(source.calls, []string{"match"}) {
		t.Fatalf("unexpected calls: %v", source.calls)
	}
}

func TestFullProfileDoesNotLoadProfileWithoutMatch(t *testing.T) {
	source := &sourceStub{}

	profile, err := matching.NewService(source).FullProfile("u1", "u10")

	if !errors.Is(err, matching.ErrProfileAccessDenied) {
		t.Fatalf("expected access denied, got %v", err)
	}

	if !reflect.DeepEqual(profile, domain.Profile{}) {
		t.Fatalf("profile leaked: %+v", profile)
	}

	if !reflect.DeepEqual(source.calls, []string{"match"}) {
		t.Fatalf("unexpected calls: %v", source.calls)
	}
}
func TestServiceRejectsInvalidPairs(t *testing.T) {
	pairs := [][2]domain.UserID{
		{"", "u10"},
		{"u1", ""},
		{"u1", "u1"},
	}

	for _, pair := range pairs {
		source := &sourceStub{}
		service := matching.NewService(source)

		result, err := service.Resolve(pair[0], pair[1])
		if !errors.Is(err, matching.ErrInvalidPair) ||
			result != (matching.Result{}) {
			t.Fatalf("resolve %v: result %+v, error %v", pair, result, err)
		}

		profile, err := service.FullProfile(pair[0], pair[1])
		if !errors.Is(err, matching.ErrInvalidPair) ||
			!reflect.DeepEqual(profile, domain.Profile{}) {
			t.Fatalf("profile %v: result %+v, error %v", pair, profile, err)
		}

		if len(source.calls) != 0 {
			t.Fatalf("invalid pair reached source: %v", source.calls)
		}
	}
}
