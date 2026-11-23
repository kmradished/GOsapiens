package profilehttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/domain"
)

type profileResponse struct {
	ID      domain.UserID `json:"id"`
	Name    string        `json:"name"`
	Bio     string        `json:"bio"`
	Version int64         `json:"version"`
}

// NewHandler регистрирует маршруты профиля. Сборка сервера передаёт
// CurrentUser и ValidID и оборачивает обработчик общим middleware.
func NewHandler(
	store application.ProfileStore,
	currentUser func(*http.Request) (domain.UserID, bool),
	validID func(domain.UserID) bool,
) http.Handler {
	profileHandler := func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := currentUser(r)
		if !ok || actorID == "" {
			writeError(w, http.StatusBadRequest, "invalid_demo_user", "X-Demo-User is missing or invalid")
			return
		}
		profileID := domain.UserID(r.PathValue("profileID"))
		if !validID(profileID) {
			writeError(w, http.StatusBadRequest, "invalid_id", "profileID is invalid")
			return
		}
		if r.URL.RawQuery != "" {
			writeError(w, http.StatusBadRequest, "invalid_query", "This route does not accept query parameters")
			return
		}

		var profile domain.ProfileRevision
		var err error
		if r.Method == http.MethodPut {
			edit, valid := decodeEdit(w, r)
			if !valid {
				writeError(w, http.StatusBadRequest, "invalid_body", "Expected name, bio and a positive integer version")
				return
			}
			profile, err = store.UpdateProfile(r.Context(), actorID, profileID, edit)
		} else {
			if r.Body != nil {
				body, readErr := io.ReadAll(io.LimitReader(r.Body, 1))
				if readErr != nil || len(body) != 0 {
					writeError(w, http.StatusBadRequest, "invalid_body", "Request body must be empty")
					return
				}
			}
			profile, err = store.ReadProfile(r.Context(), actorID, profileID)
		}
		if err != nil {
			writeProfileError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, profileResponse{
			ID: profile.ID, Name: profile.Name, Bio: profile.Bio, Version: profile.Version,
		})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/profiles/{profileID}", profileHandler)
	mux.HandleFunc("PUT /v1/profiles/{profileID}", profileHandler)
	return mux
}

func writeProfileError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrProfileNotFound):
		writeError(w, http.StatusNotFound, "profile_not_found", "Profile not found")
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "You can only access your own profile")
	case errors.Is(err, domain.ErrVersionConflict):
		writeError(w, http.StatusConflict, "version_conflict", "Profile has changed since it was read")
	default:
		slog.ErrorContext(r.Context(), "profile request failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
		switch {
		case errors.Is(err, domain.ErrCommitUnconfirmed):
			writeError(w, http.StatusServiceUnavailable, "commit_unconfirmed", "Read the profile to check whether the update was saved")
		case errors.Is(err, domain.ErrTransactionRetryable):
			writeError(w, http.StatusServiceUnavailable, "transaction_retryable", "Database transaction can be retried")
		case errors.Is(err, context.DeadlineExceeded):
			writeError(w, http.StatusGatewayTimeout, "database_timeout", "Database did not respond in time")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "Could not access profile")
		}
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
