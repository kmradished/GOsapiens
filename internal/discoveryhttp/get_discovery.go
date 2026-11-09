package discoveryhttp

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/discovery/interests"
	"github.com/kmradished/GOsapiens/internal/domain"
)

type discoveryCard struct {
	CandidateID  domain.UserID `json:"candidate_id"`
	CommonMovies []string      `json:"common_movies"`
	CommonMusic  []string      `json:"common_music"`
	CommonBooks  []string      `json:"common_books"`
}

type discoveryResponse struct {
	Items  []discoveryCard `json:"items"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewGetDiscoveryHandler используется  для создания обработчика HTTP-запросов для получения списка кандидатов для пользователя в сервисе "discovery"
func NewGetDiscoveryHandler(discover func(domain.UserID, application.DiscoveryQuery) ([]interests.AnonymousCard, error), currentUser func(*http.Request) (domain.UserID, bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := currentUser(r)
		if !ok || actorID == "" {
			writeError(w, http.StatusBadRequest, "invalid_demo_user", "X-Demo-User is missing or invalid")
			return
		}
		query, err := parseQuery(r.URL.RawQuery)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
			return
		}
		if !bodyIsEmpty(r) {
			writeError(w, http.StatusBadRequest, "invalid_body", "Request body must be empty")
			return
		}

		cards, err := discover(actorID, query)
		if errors.Is(err, domain.ErrProfileNotFound) {
			writeError(w, http.StatusNotFound, "profile_not_found", "Profile not found")
			return
		}
		if err != nil {
			slog.ErrorContext(r.Context(), "discovery failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Could not discover candidates")
			return
		}

		items := make([]discoveryCard, 0, len(cards))
		for _, card := range cards {
			items = append(items, discoveryCard{
				CandidateID:  card.CandidateID,
				CommonMovies: card.CommonMovies,
				CommonMusic:  card.CommonMusic,
				CommonBooks:  card.CommonBooks,
			})
		}
		writeJSON(w, http.StatusOK, discoveryResponse{Items: items, Limit: query.Limit, Offset: query.Offset})
	}
}

func parseQuery(rawQuery string) (application.DiscoveryQuery, error) {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return application.DiscoveryQuery{}, errors.New("Malformed query parameters")
	}

	query := application.DiscoveryQuery{Limit: application.DefaultDiscoveryLimit}
	for name, entries := range values {
		if len(entries) != 1 {
			return application.DiscoveryQuery{}, errors.New("Each query parameter must appear once")
		}
		value := entries[0]
		switch name {
		case "interest":
			query.Interest = strings.TrimSpace(value)
			if query.Interest == "" {
				return application.DiscoveryQuery{}, errors.New("interest must not be empty")
			}
		case "limit":
			query.Limit, err = strconv.Atoi(value)
			if err != nil {
				return application.DiscoveryQuery{}, errors.New("limit must be an integer")
			}
		case "offset":
			query.Offset, err = strconv.Atoi(value)
			if err != nil {
				return application.DiscoveryQuery{}, errors.New("offset must be an integer")
			}
		default:
			return application.DiscoveryQuery{}, errors.New("Only interest, limit and offset are supported")
		}
	}
	if !query.ValidPage() {
		return application.DiscoveryQuery{}, errors.New("limit must be between 1 and 50 and offset must be non-negative")
	}
	return query, nil
}

func bodyIsEmpty(r *http.Request) bool {
	if r.ContentLength > 0 {
		return false
	}
	if r.Body == nil {
		return true
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 1))
	return err == nil && len(data) == 0
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
