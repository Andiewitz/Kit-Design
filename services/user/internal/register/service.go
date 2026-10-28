package register

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
	tokenPurposeEmailVerify = "email_verify"
	verificationTTL         = 24 * time.Hour
)

type Token struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Hash      []byte
	ExpiresAt time.Time
}

type Store interface {
	CreateRegistration(ctx context.Context, user identity.User, token Token, email mailer.Message) error
	ResendVerification(ctx context.Context, email string, token Token, message mailer.Message) (bool, error)
	ConsumeVerification(ctx context.Context, tokenHash []byte) (identity.User, error)
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

func (s *Service) Register(ctx context.Context, email, password, ip string) (identity.User, error) {
	normalized, err := identity.NormalizeEmail(email)
	if err != nil {
		return identity.User{}, err
	}
	if err := identity.ValidatePassword(password); err != nil {
		return identity.User{}, err
	}

	passwordHash, err := identity.HashPassword(password)
	if err != nil {
		return identity.User{}, fmt.Errorf("hash password: %w", err)
	}

	userID := uuid.Must(uuid.NewV7())
	tokenID := uuid.Must(uuid.NewV7())
	rawToken, tokenHash, err := token.Generate(32)
	if err != nil {
		return identity.User{}, fmt.Errorf("generate verification token: %w", err)
	}

	now := time.Now().UTC()
	user := identity.User{
		ID:           userID,
		Email:        normalized,
		PasswordHash: passwordHash,
		Status:       identity.StatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	record := Token{
		ID:        tokenID,
		UserID:    userID,
		Hash:      tokenHash,
		ExpiresAt: now.Add(verificationTTL),
	}

	message := s.verificationMessage(normalized, rawToken)
	if err := s.store.CreateRegistration(ctx, user, record, message); err != nil {
		return identity.User{}, err
	}

	s.recordAudit(ctx, userID, "auth.register", ip)
	return user, nil
}

func (s *Service) VerifyEmail(ctx context.Context, rawToken, ip string) (identity.User, error) {
	if rawToken == "" {
		return identity.User{}, apperr.Invalid("Verification token is required.")
	}
	user, err := s.store.ConsumeVerification(ctx, token.Hash(rawToken))
	if err != nil {
		return identity.User{}, err
	}
	s.recordAudit(ctx, user.ID, "auth.email_verified", ip)
	return user, nil
}

func (s *Service) ResendVerification(ctx context.Context, email, ip string) error {
	normalized, err := identity.NormalizeEmail(email)
	if err != nil {
		return err
	}

	rawToken, tokenHash, err := token.Generate(32)
	if err != nil {
		return fmt.Errorf("generate verification token: %w", err)
	}
	now := time.Now().UTC()
	record := Token{
		ID:        uuid.Must(uuid.NewV7()),
		Hash:      tokenHash,
		ExpiresAt: now.Add(verificationTTL),
	}
	_, err = s.store.ResendVerification(ctx, normalized, record, s.verificationMessage(normalized, rawToken))
	if err != nil {
		return err
	}
	s.recordAudit(ctx, uuid.Nil, "auth.verification_resent", ip)
	return nil
}

func (s *Service) verificationMessage(email, rawToken string) mailer.Message {
	link := fmt.Sprintf("%s/verify-email?token=%s", s.publicAppURL, rawToken)
	return mailer.Message{
		To:      email,
		Subject: "Verify your Kit Design email",
		Text:    "Confirm your email address to activate your Kit Design account:\n\n" + link + "\n\nThis link expires in 24 hours. If you did not create an account, ignore this email.\n",
	}
}

func (s *Service) recordAudit(ctx context.Context, userID uuid.UUID, action, ip string) {
	var subject *uuid.UUID
	if userID != uuid.Nil {
		subject = &userID
	}
	_ = s.audit.Record(ctx, audit.Event{SubjectID: subject, Action: action, IP: ip})
}
