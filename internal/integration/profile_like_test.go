package integration_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
	"github.com/kmradished/GOsapiens/internal/matching"
	"github.com/kmradished/GOsapiens/internal/memory"
	"github.com/kmradished/GOsapiens/internal/profileapi"
)

func TestLikeWithHTTPProfiles(t *testing.T) {
	profiles := []domain.Profile{
		{ID: "u1", Active: true},
		{ID: "u2", Active: true},
		{ID: "u3", Active: false},
		{ID: "u4", Active: true},
	}
	profilesAPI := profileapi.NewServer(profiles, 2*time.Second, 8192)
	server := httptest.NewServer(profilesAPI.Handler())
	defer server.Close()
	httpClient := server.Client()
	httpClient.Timeout = time.Second
	client, err := profileapi.NewClient(server.URL, httpClient, 4096)
	if err != nil {
		t.Fatal(err)
	}
	interactions := memory.NewLikeState([]domain.Block{{BlockerID: "u4", BlockedID: "u1"}}, nil)
	checker := likes.NewService(client, interactions)
	store := memory.NewPairStore()
	service := application.NewService(checker, store, matching.DecideLike)

	for _, tc := range []struct {
		name   string
		mode   string
		target domain.UserID
		reason likes.Reason
		cause  error
	}{
		{name: "missing mode", mode: "missing", target: "u2", reason: likes.ReasonProfileNotFound},
		{name: "missing target", mode: "ok", target: "unknown", reason: likes.ReasonProfileNotFound},
		{name: "error", mode: "error", target: "u2", cause: profileapi.ErrDependency},
		{name: "slow", mode: "slow", target: "u2", cause: context.DeadlineExceeded},
		{name: "large", mode: "large", target: "u2", cause: profileapi.ErrResponseTooLarge},
		{name: "inactive", mode: "ok", target: "u3", reason: likes.ReasonInactiveProfile},
		{name: "blocked", mode: "ok", target: "u4", reason: likes.ReasonBlocked},
		{name: "recovered", mode: "ok", target: "u2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := profilesAPI.SetMode(tc.mode); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
			defer cancel()
			decision, err := service.LikeContext(ctx, "u1", tc.target)

			switch {
			case tc.reason != likes.ReasonNone:
				var denied *application.LikeDeniedError
				if !errors.As(err, &denied) || denied.Reason != tc.reason {
					t.Fatalf("error = %v, want denial %s", err, tc.reason)
				}
			case tc.cause != nil:
				if !errors.Is(err, tc.cause) || !errors.Is(err, profileapi.ErrDependency) {
					t.Fatalf("error = %v, want dependency error wrapping %v", err, tc.cause)
				}
			default:
				if err != nil || !decision.Changed || decision.Matched {
					t.Fatalf("LikeContext = %+v, %v", decision, err)
				}
			}

			key, err := domain.NewPairKey("u1", tc.target)
			if err != nil {
				t.Fatal(err)
			}
			state, found, err := store.Load(key)
			wantSaved := tc.reason == likes.ReasonNone && tc.cause == nil
			if err != nil || found != wantSaved || state.LeftLiked != wantSaved || state.RightLiked {
				t.Fatalf("pair = %+v, found = %t, error = %v; want saved = %t", state, found, err, wantSaved)
			}
		})
	}
}
