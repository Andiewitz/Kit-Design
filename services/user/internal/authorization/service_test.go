package authorization_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/audit"
	"kitdesign/services/user/internal/authorization"
	"kitdesign/services/user/internal/identity"
)

type fakeStore struct {
	listQuery  string
	listLimit  int
	listOffset int

	statusUser identity.User
	grantRole  string
	revokeRole string
	permission bool
}

func (f *fakeStore) ListUsers(context.Context, string, int, int) ([]identity.User, int, error) {
	return nil, 0, nil
}

func (f *fakeStore) GetUserByID(context.Context, uuid.UUID) (identity.User, error) {
	return identity.User{}, apperr.NotFound("user_not_found")
}

func (f *fakeStore) SetUserStatus(_ context.Context, id uuid.UUID, status identity.Status) (identity.User, error) {
	f.statusUser = identity.User{ID: id, Status: status}
	return f.statusUser, nil
}

func (f *fakeStore) GrantRole(_ context.Context, _ uuid.UUID, role string) error {
	f.grantRole = role
	return nil
}

func (f *fakeStore) RevokeRole(_ context.Context, _ uuid.UUID, role string) error {
	f.revokeRole = role
	return nil
}

func (f *fakeStore) ListRoles(context.Context) ([]identity.Role, error) {
	return []identity.Role{{Name: "user"}}, nil
}

func (f *fakeStore) CheckPermission(context.Context, uuid.UUID, string) (bool, error) {
	return f.permission, nil
}

type recorder struct {
	events []audit.Event
}

func (r *recorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

func newService(store *fakeStore) (*authorization.Service, *recorder) {
	events := &recorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return authorization.NewService(store, events, logger), events
}

func TestSetStatusValidation(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service, _ := newService(store)
	actor := uuid.Must(uuid.NewV7())

	for _, status := range []identity.Status{identity.StatusPending, identity.StatusDeleted, "unknown"} {
		if _, err := service.SetStatus(context.Background(), actor, actor, status, ""); !apperr.IsKind(err, apperr.KindValidation) {
			t.Fatalf("status %q: expected validation error, got %v", status, err)
		}
	}

	if _, err := service.SetStatus(context.Background(), actor, actor, identity.StatusSuspended, ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected self-suspend to fail, got %v", err)
	}
}

func TestSetStatusRecordsAudit(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service, events := newService(store)
	actor := uuid.Must(uuid.NewV7())
	target := uuid.Must(uuid.NewV7())

	user, err := service.SetStatus(context.Background(), actor, target, identity.StatusSuspended, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if user.Status != identity.StatusSuspended {
		t.Fatalf("status = %q", user.Status)
	}
	if len(events.events) != 1 || events.events[0].Action != "admin.user_status_changed" {
		t.Fatalf("events = %+v", events.events)
	}
	if events.events[0].SubjectID == nil || *events.events[0].SubjectID != target {
		t.Fatal("audit subject does not match target")
	}
}

func TestRoleValidation(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service, _ := newService(store)
	actor := uuid.Must(uuid.NewV7())
	target := uuid.Must(uuid.NewV7())

	if err := service.GrantRole(context.Background(), actor, target, "owner", ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected invalid role error, got %v", err)
	}
	if err := service.RevokeRole(context.Background(), actor, target, "owner", ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected invalid role error, got %v", err)
	}
	if err := service.RevokeRole(context.Background(), actor, actor, "admin", ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected self-revoke to fail, got %v", err)
	}
}

func TestGrantAndRevokeRole(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service, events := newService(store)
	actor := uuid.Must(uuid.NewV7())
	target := uuid.Must(uuid.NewV7())

	if err := service.GrantRole(context.Background(), actor, target, "admin", "10.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if store.grantRole != "admin" {
		t.Fatalf("granted role = %q", store.grantRole)
	}
	if err := service.RevokeRole(context.Background(), actor, target, "admin", "10.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if store.revokeRole != "admin" {
		t.Fatalf("revoked role = %q", store.revokeRole)
	}
	if len(events.events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(events.events))
	}
}

func TestCheckPermission(t *testing.T) {
	t.Parallel()

	store := &fakeStore{permission: true}
	service, _ := newService(store)

	allowed, err := service.CheckPermission(context.Background(), uuid.Must(uuid.NewV7()), "account.read")
	if err != nil || !allowed {
		t.Fatalf("allowed=%v err=%v", allowed, err)
	}
	if _, err := service.CheckPermission(context.Background(), uuid.Must(uuid.NewV7()), ""); !apperr.IsKind(err, apperr.KindValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
