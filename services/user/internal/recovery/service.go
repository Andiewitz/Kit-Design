package recovery

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/audit"
	"kitdesign/services/user/internal/identity"
	"kitdesign/services/user/internal/mailer"
	"kitdesign/services/user/internal/token"
)

const (
	tokenPurposePasswordReset = "password_reset"
	resetTTL                  = 30 * time.Minute
)

type Token struct {
	ID        uuid.UUID
	Hash      []byte
	ExpiresAt time.Time
}

type Store interface {
	CreatePasswordReset(ctx context.Context, email string, token Token, message mailer.Message) (bool, error)
	ResetPassword(ctx context.Context, tokenHash []byte, passwordHash string) (identity.User, error)
}

type Service struct {
	store        Store
	publicAppURL string
	audit        audit.Recorder
	logger       *slog.Logger
}

func NewService(store Store, publicAppURL string, recorder audit.Recorder, logger *slog.Logger) *Service {
	return &Service{store: store, publicAppURL: publicAppURL, audit: recorder, logger: logger}
}

func (s *Service) ForgotPassword(ctx context.Context, email, ip string) error {
	normalized, err := identity.NormalizeEmail(email)
	if err != nil {
		return err
	}

	rawToken, tokenHash, err := token.Generate(32)
	if err != nil {
		return fmt.Errorf("generate password reset token: %w", err)
	}

	now := time.Now().UTC()
	record := Token{
		ID:        uuid.Must(uuid.NewV7()),
		Hash:      tokenHash,
		ExpiresAt: now.Add(resetTTL),
	}
	link := fmt.Sprintf("%s/reset-password?token=%s", s.publicAppURL, rawToken)
	message := mailer.Message{
		To:      normalized,
		Subject: "Reset your Kit Design password",
		Text:    "Reset your Kit Design password using this link:\n\n" + link + "\n\nThis link expires in 30 minutes and can be used once. If you did not request this, ignore this email.\n",
	}

	sent, err := s.store.CreatePasswordReset(ctx, normalized, record, message)
	if err != nil {
		return err
	}
	if sent {
		_ = s.audit.Record(ctx, audit.Event{Action: "auth.password_reset_requested", IP: ip})
	}
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, rawToken, password, ip string) error {
	if rawToken == "" {
		return apperr.Invalid("Reset token is required.")
	}
	if err := identity.ValidatePassword(password); err != nil {
		return err
	}
	passwordHash, err := identity.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user, err := s.store.ResetPassword(ctx, token.Hash(rawToken), passwordHash)
	if err != nil {
		return err
	}
	_ = s.audit.Record(ctx, audit.Event{
		ActorID:   &user.ID,
		SubjectID: &user.ID,
		Action:    "auth.password_reset",
		IP:        ip,
	})
	return nil
}
