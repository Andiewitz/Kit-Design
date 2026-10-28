package login_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/audit"
	"kitdesign/services/user/internal/identity"
	"kitdesign/services/user/internal/login"
	"kitdesign/services/user/internal/session"
)

type fakeStore struct {
	user    identity.User
	findErr error

	lastLoginID uuid.UUID
	lastLoginAt time.Time
}

func (f *fakeStore) FindUserByEmail(context.Context, string) (identity.User, error) {
	if f.findErr != nil {
		return identity.User{}, f.findErr
	}
	return f.user, nil
}

func (f *fakeStore) UpdateLastLogin(_ context.Context, userID uuid.UUID, at time.Time) error {
	f.lastLoginID = userID
	f.lastLoginAt = at
	return nil
}

func (f *fakeStore) UserAuthorization(context.Context, uuid.UUID) ([]string, []string, error) {
	return []string{"user"}, []string{"account.read"}, nil
}

type fakeIssuer struct {
	calls       int
	userID      uuid.UUID
	roles       []string
	permissions []string
}

func (f *fakeIssuer) Create(_ context.Context, userID uuid.UUID, roles, permissions []string, _, _ string) (session.Tokens, error) {
	f.calls++
	f.userID = userID
	f.roles = roles
	f.permissions = permissions
	return session.Tokens{
		AccessToken:      "access",
		AccessExpiresAt:  time.Now().Add(15 * time.Minute),
		RefreshToken:     "refresh",
		RefreshExpiresAt: time.Now().Add(24 * time.Hour),
		SessionID:        uuid.Must(uuid.NewV7()),
	}, nil
}

type recorder struct {
	events []audit.Event
}

func (r *recorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

func newService(store *fakeStore, issuer *fakeIssuer) (*login.Service, *recorder) {
	events := &recorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return login.NewService(store, issuer, events, logger), events
}

func activeUser(t *testing.T, password string) identity.User {
	t.Helper()
	hash, err := identity.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	return identity.User{
		ID:           uuid.Must(uuid.NewV7()),
		Email:        "user@example.com",
		PasswordHash: hash,
		Status:       identity.StatusActive,
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	t.Parallel()

	store := &fakeStore{user: activeUser(t, "a-long-enough-password")}
	issuer := &fakeIssuer{}
	service, events := newService(store, issuer)

	if _, _, err := service.Login(context.Background(), "user@example.com", "wrong-password", "agent", ""); !apperr.IsKind(err, apperr.KindUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
	if issuer.calls != 0 {
		t.Fatal("session should not be issued")
	}
	if len(events.events) != 1 || events.events[0].Action != "auth.login_failed" {
		t.Fatalf("events = %+v", events.events)
	}
}

func TestLoginUnknownUserIsIndistinguishable(t *testing.T) {
	t.Parallel()

	store := &fakeStore{findErr: apperr.NotFound("user_not_found")}
	issuer := &fakeIssuer{}
	service, _ := newService(store, issuer)

	_, _, err := service.Login(context.Background(), "nobody@example.com", "a-long-enough-password", "agent", "")
	if !apperr.IsKind(err, apperr.KindUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
	if apperr.Code(err) != "invalid_credentials" {
		t.Fatalf("code = %q", apperr.Code(err))
	}
}

func TestLoginRejectsInactiveAccount(t *testing.T) {
	t.Parallel()

	user := activeUser(t, "a-long-enough-password")
	user.Status = identity.StatusPending
	store := &fakeStore{user: user}
	issuer := &fakeIssuer{}
	service, _ := newService(store, issuer)

	if _, _, err := service.Login(context.Background(), "user@example.com", "a-long-enough-password", "agent", ""); !apperr.IsKind(err, apperr.KindForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if issuer.calls != 0 {
		t.Fatal("session should not be issued")
	}
}

func TestLoginSuccess(t *testing.T) {
	t.Parallel()

	user := activeUser(t, "a-long-enough-password")
	store := &fakeStore{user: user}
	issuer := &fakeIssuer{}
	service, events := newService(store, issuer)

	tokens, gotUser, err := service.Login(context.Background(), " USER@example.com ", "a-long-enough-password", "agent", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("tokens are empty")
	}
	if gotUser.ID != user.ID {
		t.Fatalf("user = %s", gotUser.ID)
	}
	if issuer.calls != 1 || issuer.userID != user.ID {
		t.Fatalf("issuer calls=%d user=%s", issuer.calls, issuer.userID)
	}
	if len(issuer.roles) != 1 || issuer.roles[0] != "user" {
		t.Fatalf("roles = %v", issuer.roles)
	}
	if store.lastLoginID != user.ID {
		t.Fatal("last login was not updated")
	}
	if len(events.events) != 1 || events.events[0].Action != "auth.login" {
		t.Fatalf("events = %+v", events.events)
	}
}
