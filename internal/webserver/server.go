package webserver

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
)

// NewHTTPClient создаётся один раз при сборке приложения и используется повторно.
func NewHTTPClient(cfg Config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: cfg.DialTimeout}).DialContext
	transport.ResponseHeaderTimeout = cfg.ResponseHeaderTimeout
	transport.TLSHandshakeTimeout = cfg.TLSHandshakeTimeout
	return &http.Client{
		Transport: transport,
		Timeout:   cfg.ClientTimeout,
	}
}

// NewHandler получает готовые бизнес-обработчики, не создавая их зависимости.
func NewHandler(cfg Config, logger *slog.Logger, putLike, getPair, getDiscovery http.Handler) http.Handler {
	return NewHandlerWithBlock(cfg, logger, putLike, getPair, getDiscovery, nil)
}

func NewHandlerWithBlock(cfg Config, logger *slog.Logger, putLike, getPair, getDiscovery, putBlock http.Handler) http.Handler {
	return NewHandlerWithRoutes(cfg, logger, map[string]http.Handler{
		"PUT /v1/me/likes/{targetID}":  putLike,
		"GET /v1/me/pairs/{targetID}":  getPair,
		"GET /v1/me/discovery":         getDiscovery,
		"PUT /v1/me/blocks/{targetID}": putBlock,
	})
}

func NewHandlerWithRoutes(cfg Config, logger *slog.Logger, routes map[string]http.Handler) http.Handler {
	mux := http.NewServeMux()
	for pattern, handler := range routes {
		if handler != nil {
			mux.Handle(pattern, demoUser(withBudget(cfg.CommandTimeout, handler)))
		}
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	return requestLog(logger, mux)
}

func NewHTTPServer(cfg Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadTimeout:       cfg.CommandTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    16 * 1024,
	}
}
