package userint

import (
	"bytes"
	"net/http"
	"testing"
)

func TestRegistrationDoesNotRevealExistingAccounts(t *testing.T) {
	h := newHarness(t)
	password := "correct horse battery staple"

	first := h.request(t, http.MethodPost, "/v1/auth/register", map[string]string{
		"email":    "duplicate@example.com",
		"password": password,
	}, "")
	if first.status != http.StatusAccepted {
		t.Fatalf("first register = %d: %s", first.status, first.body)
	}

	second := h.request(t, http.MethodPost, "/v1/auth/register", map[string]string{
		"email":    "DUPLICATE@example.com",
		"password": password,
	}, "")
	if second.status != http.StatusAccepted {
		t.Fatalf("second register = %d: %s", second.status, second.body)
	}
	if !bytes.Equal(first.body, second.body) {
		t.Fatalf("responses differ:\nfirst: %s\nsecond: %s", first.body, second.body)
	}

	forgot := h.request(t, http.MethodPost, "/v1/auth/password/forgot", map[string]string{"email": "nobody@example.com"}, "")
	if forgot.status != http.StatusAccepted {
		t.Fatalf("forgot unknown = %d: %s", forgot.status, forgot.body)
	}

	login := h.request(t, http.MethodPost, "/v1/auth/login", map[string]string{
		"email":    "nobody@example.com",
		"password": password,
	}, "")
	if login.status != http.StatusUnauthorized {
		t.Fatalf("unknown login = %d: %s", login.status, login.body)
	}
	var problem problemPayload
	decode(t, login.body, &problem)
	if problem.Code != "invalid_credentials" {
		t.Fatalf("code = %q", problem.Code)
	}

	resend := h.request(t, http.MethodPost, "/v1/auth/email/resend", map[string]string{"email": "nobody@example.com"}, "")
	if resend.status != http.StatusAccepted {
		t.Fatalf("resend unknown = %d: %s", resend.status, resend.body)
	}
}

func TestRequestValidationAndCSRF(t *testing.T) {
	h := newHarness(t)

	csrf := h.rawRequest(t, http.MethodPost, "/v1/auth/register", "application/json", []byte(`{"email":"user@example.com","password":"correct horse battery staple"}`), map[string]string{
		"Origin": "http://evil.local",
	})
	if csrf.status != http.StatusForbidden {
		t.Fatalf("csrf status = %d: %s", csrf.status, csrf.body)
	}
	var problem problemPayload
	decode(t, csrf.body, &problem)
	if problem.Code != "invalid_origin" {
		t.Fatalf("csrf code = %q", problem.Code)
	}

	missingType := h.rawRequest(t, http.MethodPost, "/v1/auth/register", "", []byte(`{"email":"user@example.com","password":"correct horse battery staple"}`), nil)
	if missingType.status != http.StatusBadRequest {
		t.Fatalf("missing content type status = %d: %s", missingType.status, missingType.body)
	}

	weak := h.request(t, http.MethodPost, "/v1/auth/register", map[string]string{
		"email":    "user@example.com",
		"password": "short",
	}, "")
	if weak.status != http.StatusBadRequest {
		t.Fatalf("weak password status = %d: %s", weak.status, weak.body)
	}

	invalidToken := h.request(t, http.MethodPost, "/v1/auth/email/verify", map[string]string{"token": "nope"}, "")
	if invalidToken.status != http.StatusBadRequest {
		t.Fatalf("invalid token status = %d: %s", invalidToken.status, invalidToken.body)
	}

	health := h.request(t, http.MethodGet, "/healthz", nil, "")
	if health.status != http.StatusOK {
		t.Fatalf("health status = %d", health.status)
	}
	ready := h.request(t, http.MethodGet, "/readyz", nil, "")
	if ready.status != http.StatusOK {
		t.Fatalf("ready status = %d: %s", ready.status, ready.body)
	}
}
