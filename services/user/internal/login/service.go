package login

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/audit"
	"kitdesign/services/user/internal/identity"
	"kitdesign/services/user/internal/session"
)

type Store interface {
	FindUserByEmail(ctx context.Context, email string) (identity.User, error)
	UpdateLastLogin(ctx context.Context, userID uuid.UUID, at time.Time) error
	UserAuthorization(ctx context.Context, userID uuid.UUID) ([]string, []string, error)
}

type SessionIssuer interface {
	Create(ctx context.Context, userID uuid.UUID, roles, permissions []string, userAgent, ip string) (session.Tokens, error)
}

type Service struct {
	store    Store
	sessions SessionIssuer
	audit    audit.Recorder
	logger   *slog.Logger
}

func NewService(store Store, sessions SessionIssuer, recorder audit.Recorder, logger *slog.Logger) *Service {
	return &Service{store: store, sessions: sessions, audit: recorder, logger: logger}
}

func (s *Service) Login(ctx context.Context, email, password, userAgent, ip string) (session.Tokens, identity.User, error) {
	normalized, err := identity.NormalizeEmail(email)
	if err != nil {
		return session.Tokens{}, identity.User{}, apperr.Unauthorized("invalid_credentials")
	}

	user, err := s.store.FindUserByEmail(ctx, normalized)
	if err != nil {
		if apperr.IsKind(err, apperr.KindNotFound) {
			_, _ = identity.VerifyPassword(dummyHash(), password)
			s.recordAudit(ctx, nil, "auth.login_failed", ip)
			return session.Tokens{}, identity.User{}, apperr.Unauthorized("invalid_credentials")
		}
		return session.Tokens{}, identity.User{}, err
	}

	valid, err := identity.VerifyPassword(user.PasswordHash, password)
	if err != nil || !valid {
		s.recordAudit(ctx, &user.ID, "auth.login_failed", ip)
		return session.Tokens{}, identity.User{}, apperr.Unauthorized("invalid_credentials")
	}

	if user.Status != identity.StatusActive {
		return session.Tokens{}, identity.User{}, apperr.Forbidden("account_not_active")
	}

	roles, permissions, err := s.store.UserAuthorization(ctx, user.ID)
	if err != nil {
		return session.Tokens{}, identity.User{}, err
	}

	tokens, err := s.sessions.Create(ctx, user.ID, roles, permissions, userAgent, ip)
	if err != nil {
		return session.Tokens{}, identity.User{}, err
	}

	now := time.Now().UTC()
	if err := s.store.UpdateLastLogin(ctx, user.ID, now); err != nil {
		s.logger.Warn("failed to update last login", "user_id", user.ID, "error", err)
	}

	s.recordAudit(ctx, &user.ID, "auth.login", ip)
	return tokens, user, nil
}

func (s *Service) recordAudit(ctx context.Context, userID *uuid.UUID, action, ip string) {
	_ = s.audit.Record(ctx, audit.Event{SubjectID: userID, Action: action, IP: ip})
}

var dummyHash = sync.OnceValue(func() string {
	hash, err := identity.HashPassword("kit-design-dummy-password-value")
	if err != nil {
		panic(err)
	}
	return hash
})
