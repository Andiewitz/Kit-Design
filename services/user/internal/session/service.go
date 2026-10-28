package session

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/audit"
	"kitdesign/services/user/internal/token"
)

type Session struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	FamilyID         uuid.UUID
	RefreshTokenHash []byte
	UserAgent        string
	IP               string
	CreatedAt        time.Time
	LastUsedAt       time.Time
	ExpiresAt        time.Time
	RevokedAt        *time.Time
}

type Tokens struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	SessionID        uuid.UUID
}

type Store interface {
	CreateSession(ctx context.Context, value Session) error
	GetSession(ctx context.Context, id uuid.UUID) (Session, error)
	RotateSession(ctx context.Context, id uuid.UUID, oldHash, newHash []byte, now time.Time) (bool, error)
	RevokeSession(ctx context.Context, id uuid.UUID, reason string) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID, reason string) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, reason string) error
	UserAuthorization(ctx context.Context, userID uuid.UUID) ([]string, []string, error)
}

type Service struct {
	store      Store
	tokens     *TokenManager
	refreshTTL time.Duration
	audit      audit.Recorder
	logger     *slog.Logger
}

func NewService(store Store, tokens *TokenManager, refreshTTL time.Duration, recorder audit.Recorder, logger *slog.Logger) *Service {
	return &Service{store: store, tokens: tokens, refreshTTL: refreshTTL, audit: recorder, logger: logger}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, roles, permissions []string, userAgent, ip string) (Tokens, error) {
	sessionID := uuid.Must(uuid.NewV7())
	familyID := uuid.Must(uuid.NewV7())
	rawRefresh, refreshHash, err := token.Generate(32)
	if err != nil {
		return Tokens{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(s.refreshTTL)

	if err := s.store.CreateSession(ctx, Session{
		ID:               sessionID,
		UserID:           userID,
		FamilyID:         familyID,
		RefreshTokenHash: refreshHash,
		UserAgent:        truncate(userAgent, 512),
		IP:               truncate(ip, 128),
		CreatedAt:        now,
		LastUsedAt:       now,
		ExpiresAt:        expiresAt,
	}); err != nil {
		return Tokens{}, err
	}

	accessToken, accessExpiresAt, err := s.tokens.Issue(userID, sessionID, roles, permissions)
	if err != nil {
		_ = s.store.RevokeSession(ctx, sessionID, "token_issue_failed")
		return Tokens{}, err
	}

	return Tokens{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     sessionID.String() + "." + rawRefresh,
		RefreshExpiresAt: expiresAt,
		SessionID:        sessionID,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, rawRefresh, ip string) (Tokens, error) {
	sessionID, secret, ok := parseRefreshToken(rawRefresh)
	if !ok {
		return Tokens{}, apperr.Unauthorized("invalid_token")
	}

	current, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		if apperr.IsKind(err, apperr.KindNotFound) {
			return Tokens{}, apperr.Unauthorized("invalid_token")
		}
		return Tokens{}, err
	}

	now := time.Now().UTC()
	if current.RevokedAt != nil || !current.ExpiresAt.After(now) {
		return Tokens{}, apperr.Unauthorized("invalid_token")
	}

	providedHash := token.Hash(secret)
	if subtle.ConstantTimeCompare(current.RefreshTokenHash, providedHash) != 1 {
		_ = s.store.RevokeFamily(ctx, current.FamilyID, "refresh_reuse")
		s.recordAudit(ctx, &current.UserID, "auth.refresh_reuse", ip)
		return Tokens{}, apperr.Unauthorized("invalid_token")
	}

	roles, permissions, err := s.store.UserAuthorization(ctx, current.UserID)
	if err != nil {
		return Tokens{}, err
	}

	newRefresh, newHash, err := token.Generate(32)
	if err != nil {
		return Tokens{}, err
	}

	rotated, err := s.store.RotateSession(ctx, current.ID, current.RefreshTokenHash, newHash, now)
	if err != nil {
		return Tokens{}, err
	}
	if !rotated {
		_ = s.store.RevokeFamily(ctx, current.FamilyID, "refresh_race")
		return Tokens{}, apperr.Unauthorized("invalid_token")
	}

	accessToken, accessExpiresAt, err := s.tokens.Issue(current.UserID, current.ID, roles, permissions)
	if err != nil {
		_ = s.store.RevokeFamily(ctx, current.FamilyID, "token_issue_failed")
		return Tokens{}, err
	}

	return Tokens{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     current.ID.String() + "." + newRefresh,
		RefreshExpiresAt: current.ExpiresAt,
		SessionID:        current.ID,
	}, nil
}

func (s *Service) Revoke(ctx context.Context, sessionID, userID uuid.UUID, reason string) error {
	current, err := s.store.GetSession(ctx, sessionID)
	if err != nil {
		if apperr.IsKind(err, apperr.KindNotFound) {
			return nil
		}
		return err
	}
	if current.UserID != userID {
		return apperr.NotFound("session_not_found")
	}
	return s.store.RevokeSession(ctx, sessionID, reason)
}

func (s *Service) RevokeAll(ctx context.Context, userID uuid.UUID, reason string) error {
	return s.store.RevokeAllForUser(ctx, userID, reason)
}

func (s *Service) recordAudit(ctx context.Context, userID *uuid.UUID, action, ip string) {
	_ = s.audit.Record(ctx, audit.Event{
		ActorID:   userID,
		SubjectID: userID,
		Action:    action,
		IP:        truncate(ip, 128),
	})
}

func parseRefreshToken(raw string) (uuid.UUID, string, bool) {
	sessionIDValue, secret, ok := strings.Cut(raw, ".")
	if !ok || sessionIDValue == "" || secret == "" {
		return uuid.Nil, "", false
	}
	sessionID, err := uuid.Parse(sessionIDValue)
	if err != nil {
		return uuid.Nil, "", false
	}
	return sessionID, secret, true
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
