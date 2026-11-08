package likehttp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kmradished/GOsapiens/internal/domain"
)

type observedBodyReader struct {
	source    io.Reader
	bytesRead int
}

func (b *observedBodyReader) Read(p []byte) (int, error) {
	n, err := b.source.Read(p)
	b.bytesRead += n
	return n, err
}

type brokenBodyReader struct{}

func (brokenBodyReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestPutLikeBodyValidation(t *testing.T) {
	tests := []struct {
		name          string
		body          io.Reader
		contentLength int64
		wantStatus    int
	}{
		{
			name:       "no body",
			wantStatus: http.StatusOK,
		},
		{
			name: "empty body with unknown length",
			body: strings.NewReader(""), contentLength: -1,
			wantStatus: http.StatusOK,
		},
		{
			name: "known nonempty body",
			body: strings.NewReader("{}"), contentLength: 2,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "unknown length nonempty body",
			body: strings.NewReader("{}"), contentLength: -1,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "large body is not fully read",
			body: strings.NewReader(strings.Repeat("x", 8192)), contentLength: -1,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "body read error",
			body: brokenBodyReader{}, contentLength: -1,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			like := func(context.Context, domain.UserID, domain.UserID) (domain.LikeDecision, error) {
				calls++
				return domain.LikeDecision{}, nil
			}
			currentUser := func(*http.Request) (domain.UserID, bool) {
				return "u1", true
			}
			validID := func(id domain.UserID) bool { return id == "u2" }
			isDependencyError := func(error) bool { return false }

			mux := http.NewServeMux()
			mux.HandleFunc("PUT /v1/me/likes/{targetID}",
				NewPutLikeHandler(like, currentUser, validID, isDependencyError))

			req := httptest.NewRequest(http.MethodPut, "/v1/me/likes/u2", nil)
			body := &observedBodyReader{source: tt.body}
			if tt.body != nil {
				req.Body = io.NopCloser(body)
				req.ContentLength = tt.contentLength
			}
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s",
					rec.Code, tt.wantStatus, rec.Body.String())
			}

			wantCalls := 0
			if tt.wantStatus == http.StatusOK {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Errorf("like called %d times, want %d", calls, wantCalls)
			}
			if body.bytesRead > 1 {
				t.Errorf("read %d body bytes, want at most 1", body.bytesRead)
			}
			if tt.wantStatus == http.StatusBadRequest {
				var response map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatalf("invalid error JSON: %v", err)
				}
				if response["code"] != "invalid_body" {
					t.Errorf("error code = %v, want invalid_body", response["code"])
				}
			}
		})
	}
}
