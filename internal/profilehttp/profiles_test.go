package profilehttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/webserver"
)

const profilePath = "/v1/profiles/u1"

type profileStoreStub struct {
	read   func(context.Context, domain.UserID, domain.UserID) (domain.ProfileRevision, error)
	update func(context.Context, domain.UserID, domain.UserID, domain.ProfileEdit) (domain.ProfileRevision, error)
}

func (s profileStoreStub) ReadProfile(ctx context.Context, actorID, profileID domain.UserID) (domain.ProfileRevision, error) {
	return s.read(ctx, actorID, profileID)
}

func (s profileStoreStub) UpdateProfile(ctx context.Context, actorID, profileID domain.UserID, edit domain.ProfileEdit) (domain.ProfileRevision, error) {
	return s.update(ctx, actorID, profileID, edit)
}

func testHandler(store application.ProfileStore) http.Handler {
	return NewHandler(store, func(r *http.Request) (domain.UserID, bool) {
		actorID := domain.UserID(r.Header.Get("X-Demo-User"))
		return actorID, actorID != ""
	}, webserver.ValidID)
}

func request(handler http.Handler, method, path, actor, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("X-Demo-User", actor)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestProfileRoutes(t *testing.T) {
	initial := domain.ProfileRevision{ID: "u1", Name: "u1", Version: 1}
	wantEdit := domain.ProfileEdit{Name: "Полина", Bio: " люблю кино ", ExpectedVersion: 1}
	handler := testHandler(profileStoreStub{
		read: func(_ context.Context, actorID, profileID domain.UserID) (domain.ProfileRevision, error) {
			if actorID != "u1" || profileID != "u1" {
				t.Fatalf("read IDs = %q, %q", actorID, profileID)
			}
			return initial, nil
		},
		update: func(_ context.Context, actorID, profileID domain.UserID, edit domain.ProfileEdit) (domain.ProfileRevision, error) {
			if actorID != "u1" || profileID != "u1" || edit != wantEdit {
				t.Fatalf("update IDs = %q, %q, edit = %+v", actorID, profileID, edit)
			}
			return domain.ProfileRevision{ID: profileID, Name: edit.Name, Bio: edit.Bio, Version: 2}, nil
		},
	})
	for _, tc := range []struct{ method, body, want string }{
		{"GET", "", `{"id":"u1","name":"u1","bio":"","version":1}`},
		{"PUT", `{"name":"  Полина ","bio":" люблю кино ","version":1}`, `{"id":"u1","name":"Полина","bio":" люблю кино ","version":2}`},
	} {
		body := tc.body
		if tc.method == http.MethodPut {
			body += strings.Repeat(" ", 4096-len(body))
		}
		response := request(handler, tc.method, profilePath, "u1", body)
		if response.Code != 200 || response.Header().Get("Content-Type") != "application/json" ||
			strings.TrimSpace(response.Body.String()) != tc.want {
			t.Fatalf("%s: got %d %s, want 200 %s", tc.method, response.Code, response.Body, tc.want)
		}
	}
}

func TestProfileInvalidInput(t *testing.T) {
	// Некорректный запрос должен завершиться до вызова хранилища.
	handler := testHandler(nil)
	const valid = `{"name":"A","bio":"","version":1}`
	for _, tc := range []struct{ name, method, path, actor, body, code string }{
		{"missing actor", "GET", profilePath, "", "", "invalid_demo_user"},
		{"invalid ID", "GET", "/v1/profiles/u%2F1", "u1", "", "invalid_id"},
		{"GET query", "GET", profilePath + "?version=1", "u1", "", "invalid_query"},
		{"PUT query", "PUT", profilePath + "?version=1", "u1", valid, "invalid_query"},
		{"GET body", "GET", profilePath, "u1", "{}", "invalid_body"},
		{"malformed", "PUT", profilePath, "u1", "{", "invalid_body"},
		{"extra JSON", "PUT", profilePath, "u1", valid + "{}", "invalid_body"},
		{"unknown field", "PUT", profilePath, "u1", `{"name":"A","bio":"","version":1,"active":true}`, "invalid_body"},
		{"incorrect field case", "PUT", profilePath, "u1", `{"Name":"A","bio":"","version":1}`, "invalid_body"},
		{"duplicate field", "PUT", profilePath, "u1", `{"name":"A","bio":"","version":1,"version":2}`, "invalid_body"},
		{"missing name", "PUT", profilePath, "u1", `{"bio":"","version":1}`, "invalid_body"},
		{"blank name", "PUT", profilePath, "u1", `{"name":" \t","bio":"","version":1}`, "invalid_body"},
		{"long name", "PUT", profilePath, "u1", `{"name":"` + strings.Repeat("я", 101) + `","bio":"","version":1}`, "invalid_body"},
		{"missing bio", "PUT", profilePath, "u1", `{"name":"A","version":1}`, "invalid_body"},
		{"null bio", "PUT", profilePath, "u1", `{"name":"A","bio":null,"version":1}`, "invalid_body"},
		{"long bio", "PUT", profilePath, "u1", `{"name":"A","bio":"` + strings.Repeat("a", 2001) + `","version":1}`, "invalid_body"},
		{"missing version", "PUT", profilePath, "u1", `{"name":"A","bio":""}`, "invalid_body"},
		{"zero version", "PUT", profilePath, "u1", `{"name":"A","bio":"","version":0}`, "invalid_body"},
		{"string version", "PUT", profilePath, "u1", `{"name":"A","bio":"","version":"1"}`, "invalid_body"},
		{"fractional version", "PUT", profilePath, "u1", `{"name":"A","bio":"","version":1.5}`, "invalid_body"},
		{"large body", "PUT", profilePath, "u1", valid + strings.Repeat(" ", 4097-len(valid)), "invalid_body"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := request(handler, tc.method, tc.path, tc.actor, tc.body)
			assertProfileError(t, response, 400, tc.code)
		})
	}
}

func TestProfileStoreErrors(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		err        error
		status     int
	}{
		{"missing", "profile_not_found", domain.ErrProfileNotFound, 404},
		{"foreign", "forbidden", domain.ErrForbidden, 403},
		{"stale", "version_conflict", domain.ErrVersionConflict, 409},
		{"timeout", "database_timeout", context.DeadlineExceeded, 504},
		{"retry", "transaction_retryable", domain.ErrTransactionRetryable, 503},
		{"unconfirmed", "commit_unconfirmed", errors.Join(domain.ErrCommitUnconfirmed, context.DeadlineExceeded), 503},
		{"technical", "internal_error", errors.New("private database error"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := testHandler(profileStoreStub{
				read: func(context.Context, domain.UserID, domain.UserID) (domain.ProfileRevision, error) {
					return domain.ProfileRevision{}, tc.err
				},
				update: func(context.Context, domain.UserID, domain.UserID, domain.ProfileEdit) (domain.ProfileRevision, error) {
					return domain.ProfileRevision{}, tc.err
				},
			})
			assertProfileError(t, request(handler, "PUT", profilePath, "u1", `{"name":"A","bio":"","version":1}`), tc.status, tc.code)
			if tc.status == 403 || tc.status == 404 {
				assertProfileError(t, request(handler, "GET", profilePath, "u1", ""), tc.status, tc.code)
			}
		})
	}
}

func assertProfileError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var reply struct{ Code, Message string }
	if err := json.Unmarshal(response.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	if response.Code != status || reply.Code != code || reply.Message == "" ||
		strings.Contains(reply.Message, "private database error") {
		t.Fatalf("got %d %+v, want %d %s", response.Code, reply, status, code)
	}
}
