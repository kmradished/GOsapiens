package webserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Run использует порядок остановки из примера C5 преподавателя.
func Run(stop context.Context, srv *http.Server, listener net.Listener, grace time.Duration) error {
	served := make(chan error, 1)
	go func() { served <- srv.Serve(listener) }()
	select {
	case err := <-served:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-stop.Done():
	}

	// Контекст остановки уже отменён, поэтому для ожидания нужен отдельный.
	drain, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	if err := srv.Shutdown(drain); err != nil {
		closeErr := srv.Close()
		<-served
		return errors.Join(fmt.Errorf("shutdown: %w", err), closeErr)
	}
	err := <-served
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
