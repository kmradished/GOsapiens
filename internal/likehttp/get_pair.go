package likehttp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/kmradished/GOsapiens/internal/domain"
)

type pairResponse struct {
	TargetID domain.UserID `json:"target_id"`
	Matched  bool          `json:"matched"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewGetPairHandler сохраняет поддержку обработчиков чтения ЛР № 3.
func NewGetPairHandler(
	readPair func(actorID, targetID domain.UserID) (bool, error),
	currentUser func(*http.Request) (domain.UserID, bool),
	validID func(domain.UserID) bool,
) http.HandlerFunc {
	return NewGetPairHandlerContext(
		func(_ context.Context, actorID, targetID domain.UserID) (bool, error) {
			return readPair(actorID, targetID)
		},
		currentUser,
		validID,
	)
}

// NewGetPairHandlerContext передаёт контекст запроса в чтение БД.
func NewGetPairHandlerContext(
	readPair func(context.Context, domain.UserID, domain.UserID) (bool, error),
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

		matched, err := readPair(r.Context(), actorID, targetID)
		if err != nil {
			slog.ErrorContext(r.Context(), "read pair failed",
				"request_id", w.Header().Get("X-Request-ID"), "error", err)
			if writeDatabaseError(w, err) {
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error",
				"Could not read pair state")
			return
		}

		writeJSON(w, http.StatusOK, pairResponse{
			TargetID: targetID,
			Matched:  matched,
		})
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
