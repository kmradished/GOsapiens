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
	mux := http.NewServeMux()
	mux.Handle("PUT /v1/me/likes/{targetID}", demoUser(withBudget(cfg.CommandTimeout, putLike)))
	mux.Handle("GET /v1/me/pairs/{targetID}", demoUser(withBudget(cfg.CommandTimeout, getPair)))
	mux.Handle("GET /v1/me/discovery", demoUser(withBudget(cfg.CommandTimeout, getDiscovery)))
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
