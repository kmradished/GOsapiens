package available_test

import (
	"reflect"
	"testing"

	"github.com/kmradished/GOsapiens/internal/discovery/available"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestFilterCandidates(t *testing.T) {
	profiles := []domain.Profile{
		{ID: "u1", Active: true},
		{ID: "u2", Active: true},
		{ID: "u3", Active: false},
		{ID: "u4", Active: true},
		{ID: "u5", Active: true},
		{ID: "u6", Active: true},
	}
	profilesBefore := append([]domain.Profile(nil), profiles...)

	got := available.FilterCandidates(
		"u1",
		profiles,
		[]domain.UserID{"u4"},
		[]domain.UserID{"u5"},
		[]domain.UserID{"u6"},
	)

	want := []available.Candidate{{ID: "u2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterCandidates() = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(profiles, profilesBefore) {
		t.Fatal("FilterCandidates() изменил входные данные")
	}
}

func TestFilterCandidatesReturnsEmptyList(t *testing.T) {
	profiles := []domain.Profile{
		{ID: "u1", Active: true},
		{ID: "u2", Active: false},
	}

	got := available.FilterCandidates("u1", profiles, nil, nil, nil)

	if got == nil {
		t.Fatal("ожидался пустой, но не nil, список")
	}
	if len(got) != 0 {
		t.Fatalf("ожидался пустой список, получено %#v", got)
	}
}

func TestFilterCandidatesExclusions(t *testing.T) {
	tests := []struct {
		name       string
		profile    domain.Profile
		blockedIDs []domain.UserID
		viewedIDs  []domain.UserID
		reactedIDs []domain.UserID
	}{
		{name: "сам пользователь", profile: domain.Profile{ID: "u1", Active: true}},
		{name: "неактивная анкета", profile: domain.Profile{ID: "u2", Active: false}},
		{name: "блокировка", profile: domain.Profile{ID: "u2", Active: true}, blockedIDs: []domain.UserID{"u2"}},
		{name: "просмотренная анкета", profile: domain.Profile{ID: "u2", Active: true}, viewedIDs: []domain.UserID{"u2"}},
		{name: "отправленная реакция", profile: domain.Profile{ID: "u2", Active: true}, reactedIDs: []domain.UserID{"u2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := available.FilterCandidates(
				"u1",
				[]domain.Profile{tt.profile},
				tt.blockedIDs,
				tt.viewedIDs,
				tt.reactedIDs,
			)

			if len(got) != 0 {
				t.Fatalf("ожидался пустой список, получено %#v", got)
			}
		})
	}
}
