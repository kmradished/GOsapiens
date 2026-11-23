package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/discoveryhttp"
	"github.com/kmradished/GOsapiens/internal/likehttp"
	"github.com/kmradished/GOsapiens/internal/likes"
	"github.com/kmradished/GOsapiens/internal/postgres"
	"github.com/kmradished/GOsapiens/internal/profileapi"
	"github.com/kmradished/GOsapiens/internal/profilehttp"
	"github.com/kmradished/GOsapiens/internal/webserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := webserver.LoadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	httpClient := webserver.NewHTTPClient(cfg)
	defer httpClient.CloseIdleConnections()
	profiles, err := profileapi.NewClient(cfg.ProfileURL, httpClient, cfg.ProfileMaxBytes)
	if err != nil {
		return fmt.Errorf("profile client: %w", err)
	}
	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	connect, cancelConnect := context.WithTimeout(stop, 3*time.Second)
	pool, err := postgres.Open(connect, os.Getenv("DATABASE_URL"))
	cancelConnect()
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()

	handler := newHandler(cfg, logger, profiles, postgres.New(pool))
	server := webserver.NewHTTPServer(cfg, handler)
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	logger.Info("server started", "address", listener.Addr().String())
	return webserver.Run(stop, server, listener, cfg.ShutdownTimeout)
}

func newHandler(cfg webserver.Config, logger *slog.Logger, profiles likes.ProfileSource, store *postgres.Store) http.Handler {
	// До транзакции проверяем профили. Блокировку проверяет Store.Like под lockPair.
	checker := likes.NewService(profiles, nil)
	like := application.NewServiceWithLikeStore(checker.Check, store)
	discovery := application.NewDiscoveryServiceWithStore(store, nil)

	putLike := likehttp.NewPutLikeHandler(
		like.LikeContext, webserver.CurrentUser, webserver.ValidID,
		func(err error) bool { return errors.Is(err, profileapi.ErrDependency) },
	)
	getPair := likehttp.NewGetPairHandlerContext(like.PairMatchedContext, webserver.CurrentUser, webserver.ValidID)
	deleteLike := likehttp.NewDeleteLikeHandler(store.Unlike, webserver.CurrentUser, webserver.ValidID)
	putBlock := likehttp.NewPutBlockHandler(store.Block, webserver.CurrentUser, webserver.ValidID)
	getDiscovery := discoveryhttp.NewGetDiscoveryContextHandler(discovery.DiscoverContext, webserver.CurrentUser)
	profile := profilehttp.NewHandler(store, webserver.CurrentUser, webserver.ValidID)

	return webserver.NewHandlerWithRoutes(cfg, logger, map[string]http.Handler{
		"PUT /v1/me/likes/{targetID}":    putLike,
		"DELETE /v1/me/likes/{targetID}": deleteLike,
		"GET /v1/me/pairs/{targetID}":    getPair,
		"PUT /v1/me/blocks/{targetID}":   putBlock,
		"GET /v1/me/discovery":           getDiscovery,
		"GET /v1/profiles/{profileID}":   profile,
		"PUT /v1/profiles/{profileID}":   profile,
	})
}
