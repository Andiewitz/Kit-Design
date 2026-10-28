package account_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/account"
	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/audit"
	"kitdesign/services/user/internal/identity"
	"kitdesign/services/user/internal/session"
)

type fakeStore struct {
	user        identity.User
	displayName string
	revokeErr   error
	eventsLimit int
}

func (f *fakeStore) GetUserByID(context.Context, uuid.UUID) (identity.User, error) {
	return f.user, nil
}

func (f *fakeStore) UpdateDisplayName(_ context.Context, id uuid.UUID, name string) (identity.User, error) {
	f.displayName = name
	user := f.user
	user.ID = id
	user.DisplayName = name
	return user, nil
}

func (f *fakeStore) ListSessions(context.Context, uuid.UUID) ([]session.Session, error) {
	return nil, nil
}

func (f *fakeStore) RevokeUserSession(context.Context, uuid.UUID, uuid.UUID) error {
	return f.revokeErr
}

func (f *fakeStore) ListAuditEvents(_ context.Context, _ uuid.UUID, limit int) ([]audit.Event, error) {
	f.eventsLimit = limit
	return nil, nil
}

type recorder struct {
	events []audit.Event
}

func (r *recorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

func newService(store *fakeStore) (*account.Service, *recorder) {
	events := &recorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return account.NewService(store, events, logger), events
}

func TestUpdateProfile(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service, events := newService(store)
	userID := uuid.Must(uuid.NewV7())

	user, err := service.UpdateProfile(context.Background(), userID, "  Ada Lovelace  ", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if store.displayName != "Ada Lovelace" {
		t.Fatalf("display name = %q", store.displayName)
	}
	if user.DisplayName != "Ada Lovelace" {
		t.Fatalf("user display name = %q", user.DisplayName)
	}
	if len(events.events) != 1 || events.events[0].Action != "account.updated" {
		t.Fatalf("events = %+v", events.events)
	}

	if _, err := service.UpdateProfile(context.Background(), userID, "   ", ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestRevokeSession(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service, events := newService(store)
	userID := uuid.Must(uuid.NewV7())
	sessionID := uuid.Must(uuid.NewV7())

	if err := service.RevokeSession(context.Background(), userID, sessionID, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if len(events.events) != 1 || events.events[0].Action != "session.revoked" {
		t.Fatalf("events = %+v", events.events)
	}
	if events.events[0].Metadata["session_id"] != sessionID.String() {
		t.Fatal("audit metadata missing session id")
	}

	store.revokeErr = apperr.NotFound("session_not_found")
	if err := service.RevokeSession(context.Background(), userID, sessionID, ""); err == nil {
		t.Fatal("expected error")
	}
	if len(events.events) != 1 {
		t.Fatal("failed revoke should not be audited")
	}
}

func TestEventsLimit(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service, _ := newService(store)
	userID := uuid.Must(uuid.NewV7())

	if _, err := service.Events(context.Background(), userID, 0); err != nil {
		t.Fatal(err)
	}
	if store.eventsLimit != 50 {
		t.Fatalf("limit = %d, want 50", store.eventsLimit)
	}

	if _, err := service.Events(context.Background(), userID, 500); err != nil {
		t.Fatal(err)
	}
	if store.eventsLimit != 50 {
		t.Fatalf("limit = %d, want 50", store.eventsLimit)
	}
}
