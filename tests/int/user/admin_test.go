package userint

import (
	"net/http"
	"testing"
)

func TestAdminAuthorizationAndRoles(t *testing.T) {
	h := newHarness(t)
	password := "correct horse battery staple"

	admin := h.adminToken(t, "admin@example.com", password)
	user := h.registerAndVerify(t, "member@example.com", password)
	userToken := h.login(t, user.Email, password)

	forbidden := h.request(t, http.MethodGet, "/v1/admin/users", nil, userToken.AccessToken)
	if forbidden.status != http.StatusForbidden {
		t.Fatalf("regular user list status = %d: %s", forbidden.status, forbidden.body)
	}

	listed := h.request(t, http.MethodGet, "/v1/admin/users", nil, admin.AccessToken)
	if listed.status != http.StatusOK {
		t.Fatalf("admin list status = %d: %s", listed.status, listed.body)
	}
	var listBody struct {
		Users []userPayload `json:"users"`
		Total int           `json:"total"`
	}
	decode(t, listed.body, &listBody)
	if listBody.Total != 2 || len(listBody.Users) != 2 {
		t.Fatalf("users = %+v total=%d", listBody.Users, listBody.Total)
	}

	suspended := h.request(t, http.MethodPatch, "/v1/admin/users/"+user.ID+"/status", map[string]string{"status": "suspended"}, admin.AccessToken)
	if suspended.status != http.StatusOK {
		t.Fatalf("suspend status = %d: %s", suspended.status, suspended.body)
	}
	var suspendedUser userPayload
	decode(t, suspended.body, &suspendedUser)
	if suspendedUser.Status != "suspended" {
		t.Fatalf("status = %q", suspendedUser.Status)
	}

	blocked := h.request(t, http.MethodPost, "/v1/auth/login", map[string]string{"email": user.Email, "password": password}, "")
	if blocked.status != http.StatusForbidden {
		t.Fatalf("suspended login status = %d: %s", blocked.status, blocked.body)
	}

	suspendedAuthz := h.requestWithHeaders(t, http.MethodPost, "/internal/v1/authz/check", map[string]string{
		"user_id":    user.ID,
		"permission": "account.read",
	}, map[string]string{"X-Internal-Token": testInternalToken})
	var suspendedBody struct {
		Allowed bool `json:"allowed"`
	}
	decode(t, suspendedAuthz.body, &suspendedBody)
	if suspendedBody.Allowed {
		t.Fatal("suspended account should not pass internal authorization")
	}

	grant := h.request(t, http.MethodPost, "/v1/admin/users/"+user.ID+"/roles", map[string]string{"role": "admin"}, admin.AccessToken)
	if grant.status != http.StatusNoContent {
		t.Fatalf("grant status = %d: %s", grant.status, grant.body)
	}

	activate := h.request(t, http.MethodPatch, "/v1/admin/users/"+user.ID+"/status", map[string]string{"status": "active"}, admin.AccessToken)
	if activate.status != http.StatusOK {
		t.Fatalf("activate status = %d: %s", activate.status, activate.body)
	}

	promoted := h.login(t, user.Email, password)
	allowed := h.request(t, http.MethodGet, "/v1/admin/users", nil, promoted.AccessToken)
	if allowed.status != http.StatusOK {
		t.Fatalf("promoted user status = %d: %s", allowed.status, allowed.body)
	}

	checked := h.requestWithHeaders(t, http.MethodPost, "/internal/v1/authz/check", map[string]string{
		"user_id":    user.ID,
		"permission": "users.read",
	}, map[string]string{"X-Internal-Token": testInternalToken})
	if checked.status != http.StatusOK {
		t.Fatalf("authz status = %d: %s", checked.status, checked.body)
	}
	var authzBody struct {
		Allowed bool `json:"allowed"`
	}
	decode(t, checked.body, &authzBody)
	if !authzBody.Allowed {
		t.Fatal("expected users.read to be allowed")
	}

	roles := h.request(t, http.MethodGet, "/v1/admin/roles", nil, admin.AccessToken)
	if roles.status != http.StatusOK {
		t.Fatalf("roles status = %d: %s", roles.status, roles.body)
	}

	revoke := h.request(t, http.MethodDelete, "/v1/admin/users/"+user.ID+"/roles/admin", nil, admin.AccessToken)
	if revoke.status != http.StatusNoContent {
		t.Fatalf("revoke status = %d: %s", revoke.status, revoke.body)
	}

	demoted := h.login(t, user.Email, password)
	denied := h.request(t, http.MethodGet, "/v1/admin/users", nil, demoted.AccessToken)
	if denied.status != http.StatusForbidden {
		t.Fatalf("demoted user status = %d: %s", denied.status, denied.body)
	}

	checked = h.requestWithHeaders(t, http.MethodPost, "/internal/v1/authz/check", map[string]string{
		"user_id":    user.ID,
		"permission": "users.read",
	}, map[string]string{"X-Internal-Token": testInternalToken})
	decode(t, checked.body, &authzBody)
	if authzBody.Allowed {
		t.Fatal("expected users.read to be denied after role removal")
	}

	badToken := h.requestWithHeaders(t, http.MethodPost, "/internal/v1/authz/check", map[string]string{
		"user_id":    user.ID,
		"permission": "users.read",
	}, map[string]string{"X-Internal-Token": "wrong"})
	if badToken.status != http.StatusUnauthorized {
		t.Fatalf("wrong internal token status = %d: %s", badToken.status, badToken.body)
	}
}
