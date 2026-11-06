package application

import (
	"errors"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func TestPairMatchedReadsWithoutSaving(t *testing.T) {
	tests := []struct {
		name  string
		state domain.PairState
		found bool
		want  bool
	}{
		{
			name: "missing pair",
		},
		{
			name:  "one like",
			state: domain.PairState{LeftLiked: true},
			found: true,
		},
		{
			name:  "mutual likes",
			state: domain.PairState{LeftLiked: true, RightLiked: true},
			found: true,
			want:  true,
		},
	}

	wantKey := domain.PairKey{LeftID: "u1", RightID: "u2"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakePairStore{state: tt.state, found: tt.found}
			service := &Service{store: store}

			got, err := service.PairMatched("u2", "u1")
			if err != nil {
				t.Fatalf("PairMatched() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("PairMatched() = %t, want %t", got, tt.want)
			}
			if store.loadCalls != 1 || store.loadedKey != wantKey {
				t.Errorf("Load called %d times with key %+v, want once with %+v",
					store.loadCalls, store.loadedKey, wantKey)
			}
			if store.saveCalls != 0 {
				t.Errorf("Save called %d times, want 0", store.saveCalls)
			}
		})
	}
}

func TestPairMatchedPreservesLoadError(t *testing.T) {
	cause := errors.New("store unavailable")
	store := &fakePairStore{loadErr: cause}
	service := &Service{store: store}

	_, err := service.PairMatched("u1", "u2")
	if !errors.Is(err, cause) {
		t.Fatalf("PairMatched() error = %v, want original cause %v", err, cause)
	}
}
