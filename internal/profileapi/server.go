package profileapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kmradished/GOsapiens/internal/domain"
)

type Server struct {
	profiles  map[domain.UserID]profileResponse
	mode      atomic.Value
	slowDelay time.Duration
	largeBody string
}

func NewServer(profiles []domain.Profile, slowDelay time.Duration, largeBytes int) *Server {
	server := &Server{
		profiles:  make(map[domain.UserID]profileResponse, len(profiles)),
		slowDelay: slowDelay,
		largeBody: strings.Repeat(" ", largeBytes),
	}
	for _, profile := range profiles {
		server.profiles[profile.ID] = profileResponse{ID: profile.ID, Active: profile.Active}
	}
	server.mode.Store("ok")
	return server
}

func (s *Server) SetMode(mode string) error {
	switch mode {
	case "ok", "missing", "error", "slow", "large":
		s.mode.Store(mode)
		return nil
	default:
		return fmt.Errorf("unknown profile API mode %q", mode)
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /profiles/{id}", s.getProfile)
	mux.HandleFunc("PUT /_mode/{mode}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.SetMode(r.PathValue("mode")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	switch s.mode.Load().(string) {
	case "missing":
		http.NotFound(w, r)
		return
	case "error":
		http.Error(w, "profile API failure", http.StatusInternalServerError)
		return
	case "slow":
		timer := time.NewTimer(s.slowDelay)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
		}
	case "large":
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, s.largeBody); err == nil {
			_ = json.NewEncoder(w).Encode(profileResponse{ID: domain.UserID(r.PathValue("id")), Active: true})
		}
		return
	}

	profile, found := s.profiles[domain.UserID(r.PathValue("id"))]
	if !found {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}
