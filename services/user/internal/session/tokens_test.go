package session_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/session"
)

func newManager(t *testing.T, audience string) *session.TokenManager {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := session.NewTokenManager(privateKey, "test-key", "kit-user", audience, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func TestTokenManagerIssueAndParse(t *testing.T) {
	t.Parallel()

	manager := newManager(t, "kit-api")
	userID := uuid.Must(uuid.NewV7())
	sessionID := uuid.Must(uuid.NewV7())

	raw, expiresAt, err := manager.Issue(userID, sessionID, []string{"user"}, []string{"account.read"})
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" {
		t.Fatal("empty token")
	}
	if time.Until(expiresAt) <= 0 {
		t.Fatal("token is already expired")
	}

	claims, err := manager.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != userID.String() {
		t.Fatalf("subject = %q", claims.Subject)
	}
	if claims.SessionID != sessionID.String() {
		t.Fatalf("session = %q", claims.SessionID)
	}
	if len(claims.Permissions) != 1 || claims.Permissions[0] != "account.read" {
		t.Fatalf("permissions = %v", claims.Permissions)
	}
}

func TestTokenManagerRejectsTampering(t *testing.T) {
	t.Parallel()

	manager := newManager(t, "kit-api")
	raw, _, err := manager.Issue(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	tampered := raw[:len(raw)-2] + "aa"
	if _, err := manager.Parse(tampered); !apperr.IsKind(err, apperr.KindUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestTokenManagerRejectsWrongAudience(t *testing.T) {
	t.Parallel()

	issuer := newManager(t, "kit-api")
	verifier := newManager(t, "other-api")

	raw, _, err := issuer.Issue(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Parse(raw); err == nil {
		t.Fatal("expected wrong audience to fail")
	}
}

func TestTokenManagerRejectsWrongKey(t *testing.T) {
	t.Parallel()

	issuer := newManager(t, "kit-api")
	verifier := newManager(t, "kit-api")

	raw, _, err := issuer.Issue(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Parse(raw); err == nil {
		t.Fatal("expected token signed by another key to fail")
	}
}

func TestNewTokenManagerValidation(t *testing.T) {
	t.Parallel()

	if _, err := session.NewTokenManager(nil, "key", "issuer", "audience", time.Minute); err == nil {
		t.Fatal("expected nil key to fail")
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.NewTokenManager(privateKey, "key", "issuer", "audience", 0); err == nil {
		t.Fatal("expected zero TTL to fail")
	}
}
