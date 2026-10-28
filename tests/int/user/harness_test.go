package userint

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"kitdesign/services/user/internal/account"
	"kitdesign/services/user/internal/authorization"
	"kitdesign/services/user/internal/config"
	"kitdesign/services/user/internal/httpapi"
	"kitdesign/services/user/internal/login"
	"kitdesign/services/user/internal/mailer"
	"kitdesign/services/user/internal/recovery"
	"kitdesign/services/user/internal/register"
	"kitdesign/services/user/internal/session"
	"kitdesign/services/user/internal/storage/migrations"
	"kitdesign/services/user/internal/storage/postgres"
)

const testInternalToken = "test-internal-token"

type harness struct {
	server *httptest.Server
	store  *postgres.Store
	pool   *pgxpool.Pool
	client *http.Client
	plain  *http.Client
	config config.Config
	logger *slog.Logger
}

type response struct {
	status int
	header http.Header
	body   []byte
}

type userPayload struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
}

type tokenPayload struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int64        `json:"expires_in"`
	User        *userPayload `json:"user"`
}

type problemPayload struct {
	Title  string `json:"title"`
	Status int    `json:"status"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

func databaseURL() string {
	if value := os.Getenv("TEST_DATABASE_URL"); value != "" {
		return value
	}
	if value := os.Getenv("DATABASE_URL"); value != "" {
		return value
	}
	return "postgres://kit:kit_dev_password@localhost:5432/kit?sslmode=disable"
}

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := migrations.Up(ctx, databaseURL()); err != nil {
		fmt.Fprintln(os.Stderr, "integration tests require a migrated database:", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL())
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("database unavailable: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE TABLE users, sessions, one_time_tokens, audit_events, outbox_events RESTART IDENTITY CASCADE`); err != nil {
		pool.Close()
		t.Fatal(err)
	}

	store, err := postgres.New(ctx, databaseURL())
	if err != nil {
		pool.Close()
		t.Fatal(err)
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		pool.Close()
		store.Close()
		t.Fatal(err)
	}

	cfg := config.Config{
		Env:           "test",
		Port:          "0",
		DatabaseURL:   databaseURL(),
		AppOrigin:     "http://app.local",
		PublicAppURL:  "http://app.local",
		JWTPrivateKey: privateKey,
		JWTKeyID:      "test",
		JWTIssuer:     "kit-user",
		JWTAudience:   "kit-api",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    24 * time.Hour,
		CookieSecure:  false,
		Mailer:        "log",
		InternalToken: testInternalToken,
		RateLimit:     false,
		LogLevel:      slog.LevelError,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tokens, err := session.NewTokenManager(privateKey, cfg.JWTKeyID, cfg.JWTIssuer, cfg.JWTAudience, cfg.AccessTTL)
	if err != nil {
		pool.Close()
		store.Close()
		t.Fatal(err)
	}

	sessionService := session.NewService(store, tokens, cfg.RefreshTTL, store, logger)
	registerService := register.NewService(store, cfg.PublicAppURL, store, logger)
	loginService := login.NewService(store, sessionService, store, logger)
	accountService := account.NewService(store, store, logger)
	recoveryService := recovery.NewService(store, cfg.PublicAppURL, store, logger)
	authorizationService := authorization.NewService(store, store, logger)

	api := httpapi.New(cfg, logger, store, registerService, loginService, sessionService, accountService, recoveryService, authorizationService, tokens)
	server := httptest.NewServer(api.Routes())

	jar, err := cookiejar.New(nil)
	if err != nil {
		server.Close()
		pool.Close()
		store.Close()
		t.Fatal(err)
	}

	h := &harness{
		server: server,
		store:  store,
		pool:   pool,
		client: &http.Client{Jar: jar},
		plain:  &http.Client{},
		config: cfg,
		logger: logger,
	}
	t.Cleanup(func() {
		server.Close()
		store.Close()
		pool.Close()
	})
	return h
}

func (h *harness) request(t *testing.T, method, path string, body any, accessToken string) response {
	t.Helper()
	return h.requestWithHeaders(t, method, path, body, map[string]string{
		"Authorization": bearer(accessToken),
	})
}

func (h *harness) requestWithHeaders(t *testing.T, method, path string, body any, headers map[string]string) response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", h.config.AppOrigin)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		if value != "" {
			request.Header.Set(key, value)
		}
	}

	result, err := h.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Body.Close()
	payload, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{status: result.StatusCode, header: result.Header, body: payload}
}

func (h *harness) rawRequest(t *testing.T, method, path, contentType string, body []byte, headers map[string]string) response {
	t.Helper()
	request, err := http.NewRequest(method, h.server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", h.config.AppOrigin)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	result, err := h.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Body.Close()
	payload, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{status: result.StatusCode, header: result.Header, body: payload}
}

func (h *harness) refresh(t *testing.T, rawCookie string) response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, h.server.URL+"/v1/auth/refresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", h.config.AppOrigin)
	request.AddCookie(&http.Cookie{Name: "kit_refresh", Value: rawCookie})
	result, err := h.plain.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Body.Close()
	payload, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{status: result.StatusCode, header: result.Header, body: payload}
}

func (h *harness) refreshCookie(t *testing.T) string {
	t.Helper()
	target, err := url.Parse(h.server.URL + "/v1/auth/refresh")
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range h.client.Jar.Cookies(target) {
		if cookie.Name == "kit_refresh" {
			return cookie.Value
		}
	}
	t.Fatal("refresh cookie missing from jar")
	return ""
}

func (h *harness) latestEmail(t *testing.T) mailer.Message {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var payload []byte
	if err := h.pool.QueryRow(ctx, `
		SELECT payload
		FROM outbox_events
		WHERE topic = 'email.send'
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var message mailer.Message
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatal(err)
	}
	return message
}

func (h *harness) latestEmailContaining(t *testing.T, subject string) mailer.Message {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var payload []byte
	if err := h.pool.QueryRow(ctx, `
		SELECT payload
		FROM outbox_events
		WHERE topic = 'email.send'
		  AND payload->>'subject' ILIKE '%' || $1 || '%'
		ORDER BY created_at DESC
		LIMIT 1
	`, subject).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var message mailer.Message
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatal(err)
	}
	return message
}

var tokenPattern = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

func tokenFromMessage(t *testing.T, message mailer.Message) string {
	t.Helper()
	match := tokenPattern.FindStringSubmatch(message.Text)
	if len(match) != 2 {
		t.Fatalf("token not found in message: %q", message.Text)
	}
	return match[1]
}

func (h *harness) registerAndVerify(t *testing.T, email, password string) userPayload {
	t.Helper()
	result := h.request(t, http.MethodPost, "/v1/auth/register", map[string]string{
		"email":    email,
		"password": password,
	}, "")
	if result.status != http.StatusAccepted {
		t.Fatalf("register status = %d: %s", result.status, result.body)
	}
	rawToken := tokenFromMessage(t, h.latestEmail(t))

	result = h.request(t, http.MethodPost, "/v1/auth/email/verify", map[string]string{"token": rawToken}, "")
	if result.status != http.StatusOK {
		t.Fatalf("verify status = %d: %s", result.status, result.body)
	}
	var body struct {
		User userPayload `json:"user"`
	}
	decode(t, result.body, &body)
	return body.User
}

func (h *harness) login(t *testing.T, email, password string) tokenPayload {
	t.Helper()
	result := h.request(t, http.MethodPost, "/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, "")
	if result.status != http.StatusOK {
		t.Fatalf("login status = %d: %s", result.status, result.body)
	}
	var body tokenPayload
	decode(t, result.body, &body)
	return body
}

func (h *harness) grantAdmin(t *testing.T, userID string) {
	t.Helper()
	id, err := uuid.Parse(userID)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.GrantRole(context.Background(), id, "admin"); err != nil {
		t.Fatal(err)
	}
}

func (h *harness) adminToken(t *testing.T, email, password string) tokenPayload {
	t.Helper()
	user := h.registerAndVerify(t, email, password)
	h.grantAdmin(t, user.ID)
	return h.login(t, email, password)
}

func decode(t *testing.T, payload []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(payload, target); err != nil {
		t.Fatalf("decode %q: %v", string(payload), err)
	}
}

func bearer(token string) string {
	if token == "" {
		return ""
	}
	return "Bearer " + token
}
