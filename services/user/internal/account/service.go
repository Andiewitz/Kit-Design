package account

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/audit"
	"kitdesign/services/user/internal/identity"
	"kitdesign/services/user/internal/session"
)

type Store interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (identity.User, error)
	UpdateDisplayName(ctx context.Context, id uuid.UUID, displayName string) (identity.User, error)
	ListSessions(ctx context.Context, userID uuid.UUID) ([]session.Session, error)
	RevokeUserSession(ctx context.Context, userID, sessionID uuid.UUID) error
	ListAuditEvents(ctx context.Context, userID uuid.UUID, limit int) ([]audit.Event, error)
}

type Service struct {
	store  Store
	audit  audit.Recorder
	logger *slog.Logger
}

func NewService(store Store, recorder audit.Recorder, logger *slog.Logger) *Service {
	return &Service{store: store, audit: recorder, logger: logger}
}

func (s *Service) Profile(ctx context.Context, userID uuid.UUID) (identity.User, error) {
	return s.store.GetUserByID(ctx, userID)
}

func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, displayName, ip string) (identity.User, error) {
	normalized, err := identity.NormalizeDisplayName(displayName)
	if err != nil {
		return identity.User{}, err
	}
	user, err := s.store.UpdateDisplayName(ctx, userID, normalized)
	if err != nil {
		return identity.User{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		ActorID:   &userID,
		SubjectID: &userID,
		Action:    "account.updated",
		IP:        ip,
	})
	return user, nil
}

func (s *Service) Sessions(ctx context.Context, userID uuid.UUID) ([]session.Session, error) {
	return s.store.ListSessions(ctx, userID)
}

func (s *Service) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID, ip string) error {
	if err := s.store.RevokeUserSession(ctx, userID, sessionID); err != nil {
		return err
	}
	_ = s.audit.Record(ctx, audit.Event{
		ActorID:   &userID,
		SubjectID: &userID,
		Action:    "session.revoked",
		IP:        ip,
		Metadata:  map[string]any{"session_id": sessionID.String()},
	})
	return nil
}

func (s *Service) Events(ctx context.Context, userID uuid.UUID, limit int) ([]audit.Event, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.store.ListAuditEvents(ctx, userID, limit)
}
