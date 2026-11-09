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

	"github.com/kmradished/GOsapiens/internal/application"
	"github.com/kmradished/GOsapiens/internal/demodata"
	"github.com/kmradished/GOsapiens/internal/discovery/available"
	"github.com/kmradished/GOsapiens/internal/discoveryhttp"
	"github.com/kmradished/GOsapiens/internal/likehttp"
	"github.com/kmradished/GOsapiens/internal/likes"
	"github.com/kmradished/GOsapiens/internal/matching"
	"github.com/kmradished/GOsapiens/internal/memory"
	"github.com/kmradished/GOsapiens/internal/profileapi"
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
	pairs := memory.NewPairStore()
	handler := newHandler(cfg, logger, profiles, pairs)
	server := webserver.NewHTTPServer(cfg, handler)
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	logger.Info("server started", "address", listener.Addr().String())
	return webserver.Run(stop, server, listener, cfg.ShutdownTimeout)
}

func newHandler(cfg webserver.Config, logger *slog.Logger, profiles likes.ProfileSource, pairs *memory.PairStore) http.Handler {
	blocks := demodata.Blocks()
	checker := likes.NewService(profiles, memory.NewLikeState(blocks, nil))
	like := application.NewServiceWithUpdate(checker.Check, pairs, matching.DecideLike)
	discovery := application.NewDiscoveryService(
		memory.NewProfiles(demodata.Profiles()),
		available.NewMemoryExclusions(blocks, nil, nil),
		pairs,
	)
	putLike := likehttp.NewPutLikeHandler(
		like.LikeContext, webserver.CurrentUser, webserver.ValidID,
		func(err error) bool { return errors.Is(err, profileapi.ErrDependency) },
	)
	getPair := likehttp.NewGetPairHandler(like.PairMatched, webserver.CurrentUser, webserver.ValidID)
	getDiscovery := discoveryhttp.NewGetDiscoveryHandler(discovery.Discover, webserver.CurrentUser)
	return webserver.NewHandler(cfg, logger, putLike, getPair, getDiscovery)
}
