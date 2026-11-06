package likehttp

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/domain"
	"github.com/kmradished/GOsapiens/internal/likes"
)

type likeResponse struct {
	Matched    bool `json:"matched"`
	Changed    bool `json:"changed"`
	MatchedNow bool `json:"matched_now"`
}

func NewPutLikeHandler(
	like func(context.Context, domain.UserID, domain.UserID) (domain.LikeDecision, error),
	currentUser func(*http.Request) (domain.UserID, bool),
	validID func(domain.UserID) bool,
	isDependencyError func(error) bool,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := currentUser(r)
		if !ok || actorID == "" {
			writeError(w, http.StatusBadRequest, "invalid_demo_user",
				"X-Demo-User is missing or invalid")
			return
		}

		targetID := domain.UserID(r.PathValue("targetID"))
		if targetID == "" || !validID(targetID) {
			writeError(w, http.StatusBadRequest, "invalid_id", "targetID is invalid")
			return
		}

		decision, err := like(r.Context(), actorID, targetID)
		if err != nil {
			writePutLikeError(w, err, isDependencyError)
			return
		}

		writeJSON(w, http.StatusOK, likeResponse{
			Matched:    decision.Matched,
			Changed:    decision.Changed,
			MatchedNow: decision.MatchedNow,
		})
	}
}

func writePutLikeError(w http.ResponseWriter, err error, isDependencyError func(error) bool) {
	if errors.Is(err, domain.ErrProfileNotFound) {
		writeError(w, http.StatusNotFound, "profile_not_found", "Profile not found")
		return
	}

	var denied *application.LikeDeniedError
	if errors.As(err, &denied) {
		switch denied.Reason {
		case likes.ReasonSelfLike:
			writeError(w, http.StatusConflict, "self_like", "Cannot like your own profile")
			return
		case likes.ReasonInactiveProfile:
			writeError(w, http.StatusConflict, "inactive_profile", "Profile is inactive")
			return
		case likes.ReasonBlocked:
			writeError(w, http.StatusForbidden, "blocked", "Like is blocked")
			return
		case likes.ReasonProfileNotFound:
			writeError(w, http.StatusNotFound, "profile_not_found", "Profile not found")
			return
		}
	}

	if isDependencyError(err) {
		var networkErr net.Error
		if errors.Is(err, context.DeadlineExceeded) ||
			(errors.As(err, &networkErr) && networkErr.Timeout()) {
			writeError(w, http.StatusGatewayTimeout, "dependency_timeout",
				"Profile service did not respond in time")
			return
		}

		writeError(w, http.StatusBadGateway, "dependency_unavailable",
			"Profile service is unavailable")
		return
	}

	writeError(w, http.StatusInternalServerError, "internal_error", "Could not apply like")
}
