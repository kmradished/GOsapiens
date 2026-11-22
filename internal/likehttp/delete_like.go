package likehttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/kmradished/GOsapiens/internal/domain"
)

type unlikeResponse struct {
	Changed bool `json:"changed"`
	Matched bool `json:"matched"`
}

func NewDeleteLikeHandler(
	unlike func(context.Context, domain.UserID, domain.UserID) (domain.PairChange, error),
	currentUser func(*http.Request) (domain.UserID, bool),
	validID func(domain.UserID) bool,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := currentUser(r)
		if !ok || actorID == "" {
			writeError(w, http.StatusBadRequest, "invalid_demo_user",
				"X-Demo-User is missing or invalid")
			return
		}

		targetID := domain.UserID(r.PathValue("targetID"))
		if targetID == "" || !validID(targetID) || targetID == actorID {
			writeError(w, http.StatusBadRequest, "invalid_id",
				"targetID must identify another user")
			return
		}

		if !validateEmptyInput(w, r) {
			return
		}

		change, err := unlike(r.Context(), actorID, targetID)
		if err != nil {
			slog.ErrorContext(r.Context(), "unlike failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
			writeDeleteLikeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, unlikeResponse{
			Changed: change.Changed,
			Matched: change.Matched,
		})
	}
}

func writeDeleteLikeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidPair):
		writeError(w, http.StatusBadRequest, "invalid_id", "targetID must identify another user")
	case errors.Is(err, domain.ErrProfileNotFound):
		writeError(w, http.StatusNotFound, "profile_not_found", "Profile not found")
	case errors.Is(err, domain.ErrCommitUnconfirmed):
		writeError(w, http.StatusServiceUnavailable, "commit_unconfirmed", "Database commit could not be confirmed")
	case errors.Is(err, domain.ErrTransactionRetryable):
		writeError(w, http.StatusServiceUnavailable, "transaction_retryable", "Database transaction can be retried")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, "database_timeout", "Database did not respond in time")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Could not remove like")
	}
}
