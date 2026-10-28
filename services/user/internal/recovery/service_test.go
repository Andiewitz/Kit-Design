package recovery_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/audit"
	"kitdesign/services/user/internal/identity"
	"kitdesign/services/user/internal/mailer"
	"kitdesign/services/user/internal/recovery"
	tokenpkg "kitdesign/services/user/internal/token"
)

type fakeStore struct {
	email   string
	token   recovery.Token
	message mailer.Message
	sent    bool

	resetHash     []byte
	passwordHash  string
	resetUser     identity.User
	resetPassword int
}

func (f *fakeStore) CreatePasswordReset(_ context.Context, email string, token recovery.Token, message mailer.Message) (bool, error) {
	f.email = email
	f.token = token
	f.message = message
	return f.sent, nil
}

func (f *fakeStore) ResetPassword(_ context.Context, hash []byte, passwordHash string) (identity.User, error) {
	f.resetPassword++
	f.resetHash = hash
	f.passwordHash = passwordHash
	return f.resetUser, nil
}

type recorder struct {
	events []audit.Event
}

func (r *recorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

func newService(store *fakeStore) (*recovery.Service, *recorder) {
	events := &recorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return recovery.NewService(store, "http://app.local", events, logger), events
}

func TestForgotPasswordValidation(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service, _ := newService(store)

	if err := service.ForgotPassword(context.Background(), "not-an-email", ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if store.email != "" {
		t.Fatal("store should not be called")
	}
}

func TestForgotPasswordQueuesHashedToken(t *testing.T) {
	t.Parallel()

	store := &fakeStore{sent: true}
	service, events := newService(store)

	if err := service.ForgotPassword(context.Background(), " User@Example.com ", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if store.email != "user@example.com" {
		t.Fatalf("email = %q", store.email)
	}
	if !strings.Contains(store.message.Text, "/reset-password?token=") {
		t.Fatal("reset link missing")
	}
	rawToken := extractToken(t, store.message.Text)
	if !bytes.Equal(store.token.Hash, tokenpkg.Hash(rawToken)) {
		t.Fatal("stored hash does not match token")
	}
	if len(events.events) != 1 || events.events[0].Action != "auth.password_reset_requested" {
		t.Fatalf("events = %+v", events.events)
	}
}

func TestForgotPasswordIgnoresUnknownAccounts(t *testing.T) {
	t.Parallel()

	store := &fakeStore{sent: false}
	service, events := newService(store)

	if err := service.ForgotPassword(context.Background(), "nobody@example.com", ""); err != nil {
		t.Fatal(err)
	}
	if len(events.events) != 0 {
		t.Fatal("unknown account should not be audited")
	}
}

func TestResetPasswordValidation(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service, _ := newService(store)

	if err := service.ResetPassword(context.Background(), "", "a-long-enough-password", ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected token validation error, got %v", err)
	}
	if err := service.ResetPassword(context.Background(), "token", "short", ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected password validation error, got %v", err)
	}
	if store.resetPassword != 0 {
		t.Fatal("store should not be called")
	}
}

func TestResetPasswordHashesNewPassword(t *testing.T) {
	t.Parallel()

	userID := uuid.Must(uuid.NewV7())
	store := &fakeStore{resetUser: identity.User{ID: userID}}
	service, events := newService(store)

	if err := service.ResetPassword(context.Background(), "raw-token", "a-new-long-password", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(store.resetHash, tokenpkg.Hash("raw-token")) {
		t.Fatal("reset token hash mismatch")
	}
	valid, err := identity.VerifyPassword(store.passwordHash, "a-new-long-password")
	if err != nil || !valid {
		t.Fatalf("new password was not hashed: %v", err)
	}
	if len(events.events) != 1 || events.events[0].Action != "auth.password_reset" {
		t.Fatalf("events = %+v", events.events)
	}
	if events.events[0].ActorID == nil || *events.events[0].ActorID != userID {
		t.Fatal("audit actor does not match user")
	}
}

func extractToken(t *testing.T, text string) string {
	t.Helper()
	index := strings.Index(text, "token=")
	if index < 0 {
		t.Fatalf("token not found in %q", text)
	}
	value := text[index+len("token="):]
	if end := strings.IndexAny(value, " \n\r"); end >= 0 {
		value = value[:end]
	}
	return value
}
