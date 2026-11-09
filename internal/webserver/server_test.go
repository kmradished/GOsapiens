package webserver_test

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kmradished/GOsapiens/internal/webserver"
)

func TestServerLimitsDelayedRequestBody(t *testing.T) {
	cfg := webserver.Config{
		CommandTimeout:    100 * time.Millisecond,
		ReadHeaderTimeout: time.Second,
		IdleTimeout:       time.Second,
	}
	entered := make(chan struct{})
	readResult := make(chan error, 1)
	business := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		var firstByte [1]byte
		_, err := r.Body.Read(firstByte[:])
		readResult <- err
		w.WriteHeader(http.StatusBadRequest)
	})
	handler := webserver.NewHandler(cfg, quietLogger(), business, business, business)
	server := httptest.NewUnstartedServer(handler)
	server.Config = webserver.NewHTTPServer(cfg, handler)
	server.Start()
	defer server.Close()

	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	// Отправляем заголовки с неизвестной длиной тела, но само тело задерживаем.
	_, err = fmt.Fprint(conn, "PUT /v1/me/likes/u2 HTTP/1.1\r\n"+
		"Host: localhost\r\nX-Demo-User: u1\r\nTransfer-Encoding: chunked\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	waitSignal(t, entered, "body reader start")
	select {
	case err := <-readResult:
		var timeoutErr net.Error
		if !errors.As(err, &timeoutErr) || !timeoutErr.Timeout() {
			t.Fatalf("body read error = %v; want a timeout", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("body read did not stop after the timeout")
	}

	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodPut})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400 after body timeout", response.StatusCode)
	}
}
