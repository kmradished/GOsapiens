package webserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/kmradished/GOsapiens/internal/domain"
)

type userKey struct{}

func CurrentUser(r *http.Request) (domain.UserID, bool) {
	id, ok := r.Context().Value(userKey{}).(domain.UserID)
	return id, ok
}

// ValidID использует один формат для demo-пользователя и targetID.
func ValidID(id domain.UserID) bool {
	text := string(id)
	if text == "" || len(text) > 64 || strings.TrimSpace(text) != text || strings.Contains(text, "/") {
		return false
	}
	for _, ch := range text {
		if unicode.IsControl(ch) {
			return false
		}
	}
	return true
}

func demoUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := r.Header.Values("X-Demo-User")
		if len(values) != 1 || !ValidID(domain.UserID(values[0])) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{"invalid_demo_user", "Укажите корректный X-Demo-User"})
			return
		}
		ctx := context.WithValue(r.Context(), userKey{}, domain.UserID(values[0]))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func withBudget(budget time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), budget)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	var counter atomic.Uint64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strconv.FormatUint(counter.Add(1), 10)
		w.Header().Set("X-Request-ID", id)
		started := time.Now()
		writer := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		if writer.status == 0 {
			writer.status = http.StatusOK
		}
		logger.InfoContext(r.Context(), "http request",
			"request_id", id, "method", r.Method, "path", r.URL.Path,
			"status", writer.status, "duration", time.Since(started))
	})
}
