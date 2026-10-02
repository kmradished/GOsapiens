package available_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kmradished/GOsapiens/internal/discovery/available"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/memory"
)

func TestServiceRecommend(t *testing.T) {
	profiles := memory.NewProfiles([]domain.Profile{
		{ID: "u1", Active: true},
		{ID: "u2", Active: true},
		{ID: "u3", Active: true},
	})
	exclusions := available.NewMemoryExclusions(
		[]domain.Block{{BlockerID: "u3", BlockedID: "u1"}},
		nil,
		nil,
	)
	service := available.NewService(profiles, exclusions)

	got, err := service.Recommend("u1")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	want := []available.Candidate{{ID: "u2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Recommend() = %#v, want %#v", got, want)
	}
}

func TestServiceReturnsSourceErrors(t *testing.T) {
	errSource := errors.New("ошибка источника")
	tests := []struct {
		name       string
		profiles   profileSourceStub
		exclusions exclusionSourceStub
	}{
		{name: "ошибка профилей", profiles: profileSourceStub{err: errSource}},
		{name: "ошибка блокировок", exclusions: exclusionSourceStub{blockedErr: errSource}},
		{name: "ошибка просмотров", exclusions: exclusionSourceStub{viewedErr: errSource}},
		{name: "ошибка реакций", exclusions: exclusionSourceStub{reactedErr: errSource}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := available.NewService(tt.profiles, tt.exclusions)

			got, err := service.Recommend("u1")

			if got != nil {
				t.Fatalf("при ошибке ожидался nil, получено %#v", got)
			}
			if !errors.Is(err, errSource) {
				t.Fatalf("исходная ошибка не сохранилась: %v", err)
			}
		})
	}
}

type profileSourceStub struct {
	profiles []domain.Profile
	err      error
}

func (s profileSourceStub) List() ([]domain.Profile, error) {
	return s.profiles, s.err
}

type exclusionSourceStub struct {
	blockedIDs []domain.UserID
	viewedIDs  []domain.UserID
	reactedIDs []domain.UserID
	blockedErr error
	viewedErr  error
	reactedErr error
}

func (s exclusionSourceStub) BlockedIDs(domain.UserID) ([]domain.UserID, error) {
	return s.blockedIDs, s.blockedErr
}

func (s exclusionSourceStub) ViewedIDs(domain.UserID) ([]domain.UserID, error) {
	return s.viewedIDs, s.viewedErr
}

func (s exclusionSourceStub) ReactedIDs(domain.UserID) ([]domain.UserID, error) {
	return s.reactedIDs, s.reactedErr
}
