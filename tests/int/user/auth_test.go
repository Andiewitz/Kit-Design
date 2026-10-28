package userint

import (
	"net/http"
	"testing"
)

func TestRegistrationVerificationAccountAndRefresh(t *testing.T) {
	h := newHarness(t)
	email := "User@Example.com"
	password := "correct horse battery staple"

	user := h.registerAndVerify(t, email, password)
	if user.Email != "user@example.com" {
		t.Fatalf("email = %q", user.Email)
	}
	if user.Status != "active" {
		t.Fatalf("status = %q", user.Status)
	}

	tokens := h.login(t, email, password)
	if tokens.AccessToken == "" || tokens.User == nil {
		t.Fatalf("login payload = %+v", tokens)
	}
	oldRefresh := h.refreshCookie(t)

	account := h.request(t, http.MethodGet, "/v1/account", nil, tokens.AccessToken)
	if account.status != http.StatusOK {
		t.Fatalf("account status = %d: %s", account.status, account.body)
	}
	var accountBody userPayload
	decode(t, account.body, &accountBody)
	if accountBody.Email != "user@example.com" {
		t.Fatalf("account email = %q", accountBody.Email)
	}

	update := h.request(t, http.MethodPatch, "/v1/account", map[string]string{"display_name": "  Ada Lovelace  "}, tokens.AccessToken)
	if update.status != http.StatusOK {
		t.Fatalf("update status = %d: %s", update.status, update.body)
	}
	var updated userPayload
	decode(t, update.body, &updated)
	if updated.DisplayName != "Ada Lovelace" {
		t.Fatalf("display name = %q", updated.DisplayName)
	}

	sessions := h.request(t, http.MethodGet, "/v1/account/sessions", nil, tokens.AccessToken)
	if sessions.status != http.StatusOK {
		t.Fatalf("sessions status = %d: %s", sessions.status, sessions.body)
	}
	var sessionBody struct {
		Sessions []struct {
			ID      string `json:"id"`
			Current bool   `json:"current"`
		} `json:"sessions"`
	}
	decode(t, sessions.body, &sessionBody)
	if len(sessionBody.Sessions) != 1 || !sessionBody.Sessions[0].Current {
		t.Fatalf("sessions = %+v", sessionBody.Sessions)
	}

	events := h.request(t, http.MethodGet, "/v1/account/events", nil, tokens.AccessToken)
	if events.status != http.StatusOK {
		t.Fatalf("events status = %d: %s", events.status, events.body)
	}
	var eventBody struct {
		Events []struct {
			Action string `json:"action"`
		} `json:"events"`
	}
	decode(t, events.body, &eventBody)
	actions := make(map[string]bool)
	for _, event := range eventBody.Events {
		actions[event.Action] = true
	}
	for _, want := range []string{"auth.register", "auth.email_verified", "auth.login", "account.updated"} {
		if !actions[want] {
			t.Fatalf("missing audit action %q in %+v", want, actions)
		}
	}

	refreshed := h.request(t, http.MethodPost, "/v1/auth/refresh", nil, "")
	if refreshed.status != http.StatusOK {
		t.Fatalf("refresh status = %d: %s", refreshed.status, refreshed.body)
	}
	newRefresh := h.refreshCookie(t)
	if newRefresh == oldRefresh {
		t.Fatal("refresh token was not rotated")
	}

	replay := h.refresh(t, oldRefresh)
	if replay.status != http.StatusUnauthorized {
		t.Fatalf("refresh replay status = %d: %s", replay.status, replay.body)
	}
	revoked := h.refresh(t, newRefresh)
	if revoked.status != http.StatusUnauthorized {
		t.Fatalf("family was not revoked: %d: %s", revoked.status, revoked.body)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	h := newHarness(t)
	password := "correct horse battery staple"
	user := h.registerAndVerify(t, "logout@example.com", password)
	tokens := h.login(t, user.Email, password)
	refresh := h.refreshCookie(t)

	result := h.request(t, http.MethodPost, "/v1/auth/logout", nil, tokens.AccessToken)
	if result.status != http.StatusNoContent {
		t.Fatalf("logout status = %d: %s", result.status, result.body)
	}

	result = h.refresh(t, refresh)
	if result.status != http.StatusUnauthorized {
		t.Fatalf("refresh after logout = %d: %s", result.status, result.body)
	}
}

func TestPasswordRecovery(t *testing.T) {
	h := newHarness(t)
	email := "recover@example.com"
	oldPassword := "old correct horse battery"
	newPassword := "new correct horse battery"
	h.registerAndVerify(t, email, oldPassword)
	h.login(t, email, oldPassword)
	oldRefresh := h.refreshCookie(t)

	forgot := h.request(t, http.MethodPost, "/v1/auth/password/forgot", map[string]string{"email": email}, "")
	if forgot.status != http.StatusAccepted {
		t.Fatalf("forgot status = %d: %s", forgot.status, forgot.body)
	}
	resetToken := tokenFromMessage(t, h.latestEmailContaining(t, "Reset"))

	reset := h.request(t, http.MethodPost, "/v1/auth/password/reset", map[string]string{
		"token":    resetToken,
		"password": newPassword,
	}, "")
	if reset.status != http.StatusOK {
		t.Fatalf("reset status = %d: %s", reset.status, reset.body)
	}

	oldLogin := h.request(t, http.MethodPost, "/v1/auth/login", map[string]string{"email": email, "password": oldPassword}, "")
	if oldLogin.status != http.StatusUnauthorized {
		t.Fatalf("old password status = %d: %s", oldLogin.status, oldLogin.body)
	}
	newLogin := h.request(t, http.MethodPost, "/v1/auth/login", map[string]string{"email": email, "password": newPassword}, "")
	if newLogin.status != http.StatusOK {
		t.Fatalf("new password status = %d: %s", newLogin.status, newLogin.body)
	}

	oldRefreshResult := h.refresh(t, oldRefresh)
	if oldRefreshResult.status != http.StatusUnauthorized {
		t.Fatalf("old refresh status = %d: %s", oldRefreshResult.status, oldRefreshResult.body)
	}

	reuse := h.request(t, http.MethodPost, "/v1/auth/password/reset", map[string]string{
		"token":    resetToken,
		"password": "another correct horse battery",
	}, "")
	if reuse.status != http.StatusBadRequest {
		t.Fatalf("reset token reuse status = %d: %s", reuse.status, reuse.body)
	}
}
