package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/config"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDecodeJSON(t *testing.T) {
	t.Parallel()

	type payload struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name        string
		contentType string
		body        string
		wantErr     bool
	}{
		{name: "valid", contentType: "application/json", body: `{"name":"kit"}`},
		{name: "missing content type", body: `{"name":"kit"}`, wantErr: true},
		{name: "unknown field", contentType: "application/json", body: `{"name":"kit","extra":true}`, wantErr: true},
		{name: "multiple objects", contentType: "application/json", body: `{"name":"kit"}{"name":"other"}`, wantErr: true},
		{name: "empty body", contentType: "application/json", body: ``, wantErr: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(test.body))
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			var value payload
			err := decodeJSON(httptest.NewRecorder(), request, &value)
			if test.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHasPermission(t *testing.T) {
	t.Parallel()

	permissions := []string{"account.read", "sessions.read"}
	if !hasPermission(permissions, "account.read") {
		t.Fatal("expected permission")
	}
	if hasPermission(permissions, "admin.users.read") {
		t.Fatal("unexpected permission")
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:4321"
	if got := clientIP(request); got != "127.0.0.1" {
		t.Fatalf("got %q", got)
	}
	request.RemoteAddr = "not-a-host-port"
	if got := clientIP(request); got != "not-a-host-port" {
		t.Fatalf("got %q", got)
	}
}

func TestLimiter(t *testing.T) {
	t.Parallel()

	disabled := newLimiter(false)
	for i := 0; i < 100; i++ {
		if !disabled.Allow("key", rate.Limit(1), 1) {
			t.Fatal("disabled limiter should always allow")
		}
	}

	enabled := newLimiter(true)
	if !enabled.Allow("key", rate.Limit(1), 2) {
		t.Fatal("first request should be allowed")
	}
	if !enabled.Allow("key", rate.Limit(1), 2) {
		t.Fatal("second request should be allowed")
	}
	if enabled.Allow("key", rate.Limit(1), 2) {
		t.Fatal("third request should be limited")
	}
	if !enabled.Allow("other", rate.Limit(1), 1) {
		t.Fatal("different key should be allowed")
	}
}

func TestCORS(t *testing.T) {
	t.Parallel()

	server := &Server{
		config: config.Config{AppOrigin: "http://app.local"},
		logger: discardLogger(),
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := server.cors(next)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "http://app.local")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://app.local" {
		t.Fatal("missing allow origin")
	}
	if recorder.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("missing credentials header")
	}

	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "http://evil.local")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unexpected allow origin")
	}

	request = httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "http://app.local")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	server := &Server{logger: discardLogger()}
	handler := server.securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}
	if recorder.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing frame options")
	}
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	writeError(recorder, request, apperr.NotFound("user_not_found"), discardLogger())

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get("Content-Type") != "application/problem+json; charset=utf-8" {
		t.Fatalf("content type = %q", recorder.Header().Get("Content-Type"))
	}
	var body problemBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "user_not_found" || body.Status != http.StatusNotFound {
		t.Fatalf("body = %+v", body)
	}
}

func TestCheckOrigin(t *testing.T) {
	t.Parallel()

	server := &Server{config: config.Config{AppOrigin: "http://app.local"}, logger: discardLogger()}

	request := httptest.NewRequest(http.MethodPost, "/", nil)
	recorder := httptest.NewRecorder()
	if !server.checkOrigin(recorder, request) {
		t.Fatal("missing origin should be allowed")
	}

	request.Header.Set("Origin", "http://app.local")
	if !server.checkOrigin(httptest.NewRecorder(), request) {
		t.Fatal("matching origin should be allowed")
	}

	request.Header.Set("Origin", "http://evil.local")
	recorder = httptest.NewRecorder()
	if server.checkOrigin(recorder, request) {
		t.Fatal("mismatched origin should be rejected")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestRequestID(t *testing.T) {
	t.Parallel()

	server := &Server{logger: discardLogger()}
	handler := server.withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestIDFrom(r.Context()) == "" {
			t.Error("request id missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", "known-id")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Header().Get("X-Request-ID") != "known-id" {
		t.Fatalf("request id = %q", recorder.Header().Get("X-Request-ID"))
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("generated request id missing")
	}
}

func TestRateLimit(t *testing.T) {
	t.Parallel()

	server := &Server{
		logger:  discardLogger(),
		limiter: newLimiter(true),
	}
	handler := server.rateLimit(rate.Limit(1), 1, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	request.RemoteAddr = "127.0.0.1:1000"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("first status = %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "60" {
		t.Fatalf("retry-after = %q", recorder.Header().Get("Retry-After"))
	}
}
