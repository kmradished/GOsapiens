package webserver_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/kmradished/GOsapiens/internal/webserver"
)

func waitSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for %s", name)
	}
}

func TestRunShutdown(t *testing.T) {
	for _, forceClose := range []bool{false, true} {
		name := "waits for handler"
		if forceClose {
			name = "closes after grace"
		}
		t.Run(name, func(t *testing.T) {
			entered := make(chan struct{})
			exited := make(chan struct{})
			draining := make(chan struct{})
			allowFinish, release := context.WithCancel(context.Background())
			defer release()
			srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				defer close(exited)
				select {
				case <-allowFinish.Done():
				case <-r.Context().Done():
				}
			})}
			srv.RegisterOnShutdown(func() { close(draining) })
			t.Cleanup(func() { _ = srv.Close() })
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			stop, cancel := context.WithCancel(context.Background())
			defer cancel()
			grace := time.Second
			if forceClose {
				grace = 0 // Срок уже истёк: результат не зависит от задержек машины.
			}
			result := make(chan error, 1)
			go func() { result <- webserver.Run(stop, srv, listener, grace) }()
			client := &http.Client{Timeout: 3 * time.Second}
			defer client.CloseIdleConnections()
			clientDone := make(chan error, 1)
			go func() {
				res, err := client.Get("http://" + listener.Addr().String())
				if err == nil {
					_, err = io.Copy(io.Discard, res.Body)
					_ = res.Body.Close()
				}
				clientDone <- err
			}()
			waitSignal(t, entered, "handler start")
			cancel()
			waitSignal(t, draining, "Shutdown start")
			if !forceClose {
				select {
				case err := <-result:
					t.Fatalf("Run returned while handler was active: %v", err)
				default:
				}
				release()
			}
			select {
			case err := <-result:
				if forceClose && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("Run error = %v; want grace timeout", err)
				}
				if !forceClose && err != nil {
					t.Fatalf("graceful Run error = %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("Run did not finish")
			}
			waitSignal(t, exited, "handler completion")
			select {
			case err := <-clientDone:
				if !forceClose && err != nil {
					t.Fatalf("active request failed during graceful shutdown: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("client did not finish")
			}
		})
	}
}
