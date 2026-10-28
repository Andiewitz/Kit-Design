package register_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/audit"
	"kitdesign/services/user/internal/identity"
	"kitdesign/services/user/internal/mailer"
	"kitdesign/services/user/internal/register"
	tokenpkg "kitdesign/services/user/internal/token"
)

type fakeStore struct {
	createdUser    identity.User
	createdToken   register.Token
	createdMessage mailer.Message
	createCalls    int

	resentToken   register.Token
	resentMessage mailer.Message

	verifiedUser identity.User
	consumedHash []byte
}

func (f *fakeStore) CreateRegistration(_ context.Context, user identity.User, token register.Token, message mailer.Message) error {
	f.createCalls++
	f.createdUser = user
	f.createdToken = token
	f.createdMessage = message
	return nil
}

func (f *fakeStore) ResendVerification(_ context.Context, _ string, token register.Token, message mailer.Message) (bool, error) {
	f.resentToken = token
	f.resentMessage = message
	return true, nil
}

func (f *fakeStore) ConsumeVerification(_ context.Context, hash []byte) (identity.User, error) {
	f.consumedHash = hash
	return f.verifiedUser, nil
}

type recorder struct {
	events []audit.Event
}

func (r *recorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

func newService(store *fakeStore) (*register.Service, *recorder) {
	events := &recorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return register.NewService(store, "http://app.local", events, logger), events
}

func TestRegisterValidation(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service, _ := newService(store)

	if _, err := service.Register(context.Background(), "not-an-email", "a-long-enough-password", ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected email validation error, got %v", err)
	}
	if _, err := service.Register(context.Background(), "user@example.com", "short", ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected password validation error, got %v", err)
	}
	if store.createCalls != 0 {
		t.Fatal("store should not be called for invalid input")
	}
}

func TestRegisterCreatesPendingUserAndHashedToken(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service, events := newService(store)

	user, err := service.Register(context.Background(), "  User@Example.COM ", "a-long-enough-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	if user.Email != "user@example.com" {
		t.Fatalf("email = %q", user.Email)
	}
	if user.Status != identity.StatusPending {
		t.Fatalf("status = %q", user.Status)
	}
	if user.PasswordHash == "" || strings.Contains(user.PasswordHash, "a-long-enough-password") {
		t.Fatal("password was not hashed")
	}
	if !strings.Contains(store.createdMessage.Text, "/verify-email?token=") {
		t.Fatal("verification link missing from message")
	}

	rawToken := extractToken(t, store.createdMessage.Text)
	if !bytes.Equal(store.createdToken.Hash, tokenpkg.Hash(rawToken)) {
		t.Fatal("stored token hash does not match token in the email")
	}
	if time.Until(store.createdToken.ExpiresAt) <= 0 {
		t.Fatal("verification token is already expired")
	}
	if len(events.events) != 1 || events.events[0].Action != "auth.register" {
		t.Fatalf("events = %+v", events.events)
	}
}

func TestVerifyEmail(t *testing.T) {
	t.Parallel()

	userID := uuid.Must(uuid.NewV7())
	store := &fakeStore{verifiedUser: identity.User{ID: userID, Status: identity.StatusActive}}
	service, events := newService(store)

	if _, err := service.VerifyEmail(context.Background(), "", ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}

	user, err := service.VerifyEmail(context.Background(), "raw-token", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != userID {
		t.Fatalf("user id = %s", user.ID)
	}
	if !bytes.Equal(store.consumedHash, tokenpkg.Hash("raw-token")) {
		t.Fatal("verification hash does not match the provided token")
	}
	if len(events.events) != 1 || events.events[0].Action != "auth.email_verified" {
		t.Fatalf("events = %+v", events.events)
	}
}

func TestResendVerification(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service, events := newService(store)

	if err := service.ResendVerification(context.Background(), "bad", ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if err := service.ResendVerification(context.Background(), "User@Example.com", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(store.resentMessage.Text, "/verify-email?token=") {
		t.Fatal("verification link missing from resent message")
	}
	rawToken := extractToken(t, store.resentMessage.Text)
	if !bytes.Equal(store.resentToken.Hash, tokenpkg.Hash(rawToken)) {
		t.Fatal("resent token hash does not match the email")
	}
	if len(events.events) != 1 || events.events[0].Action != "auth.verification_resent" {
		t.Fatalf("events = %+v", events.events)
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
