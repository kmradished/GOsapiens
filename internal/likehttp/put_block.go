package likehttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/kmradished/GOsapiens/internal/domain"
)

type blockResponse struct {
	Changed bool `json:"changed"`
	Matched bool `json:"matched"`
}

func NewPutBlockHandler(
	block func(context.Context, domain.UserID, domain.UserID) (domain.PairChange, error),
	currentUser func(*http.Request) (domain.UserID, bool),
	validID func(domain.UserID) bool,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := currentUser(r)
		if !ok || actorID == "" {
			writeError(w, http.StatusBadRequest, "invalid_demo_user", "X-Demo-User is missing or invalid")
			return
		}
		targetID := domain.UserID(r.PathValue("targetID"))
		if targetID == "" || !validID(targetID) {
			writeError(w, http.StatusBadRequest, "invalid_id", "targetID is invalid")
			return
		}
		if !validateEmptyInput(w, r) {
			return
		}
		if actorID == targetID {
			writeError(w, http.StatusBadRequest, "invalid_id", "Cannot block your own profile")
			return
		}
		change, err := block(r.Context(), actorID, targetID)
		if err != nil {
			slog.ErrorContext(r.Context(), "block failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
			writeBlockError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, blockResponse{Changed: change.Changed, Matched: change.Matched})
	}
}

func writeBlockError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidPair):
		writeError(w, http.StatusBadRequest, "invalid_id", "Invalid pair")
	case errors.Is(err, domain.ErrProfileNotFound):
		writeError(w, http.StatusNotFound, "profile_not_found", "Profile not found")
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Operation is forbidden")
	case errors.Is(err, domain.ErrCommitUnconfirmed):
		writeError(w, http.StatusServiceUnavailable, "commit_unconfirmed", "Read state or repeat the same block command")
	case errors.Is(err, domain.ErrTransactionRetryable):
		writeError(w, http.StatusServiceUnavailable, "transaction_retryable", "Repeat the same block command")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeError(w, http.StatusGatewayTimeout, "database_timeout", "Database operation timed out")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Could not block user")
	}
}
