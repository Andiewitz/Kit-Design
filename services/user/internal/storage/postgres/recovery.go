package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/identity"
)

func (s *Store) ResetPassword(ctx context.Context, tokenHash []byte, passwordHash string) (identity.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return identity.User{}, err
	}
	defer tx.Rollback(ctx)

	var tokenID uuid.UUID
	var userID uuid.UUID
	var expiresAt time.Time
	var consumedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, expires_at, consumed_at
		FROM one_time_tokens
		WHERE token_hash = $1 AND purpose = 'password_reset'
		FOR UPDATE
	`, tokenHash).Scan(&tokenID, &userID, &expiresAt, &consumedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.User{}, apperr.New(apperr.KindValidation, "invalid_token", "This reset link is invalid.")
		}
		return identity.User{}, err
	}
	if consumedAt != nil {
		return identity.User{}, apperr.New(apperr.KindValidation, "invalid_token", "This reset link has already been used.")
	}

	now := time.Now().UTC()
	if !expiresAt.After(now) {
		return identity.User{}, apperr.New(apperr.KindValidation, "token_expired", "This reset link has expired.")
	}

	if _, err := tx.Exec(ctx, `UPDATE one_time_tokens SET consumed_at = $2 WHERE id = $1`, tokenID, now); err != nil {
		return identity.User{}, err
	}

	user, err := scanUser(tx.QueryRow(ctx, `
		UPDATE users
		SET password_hash = $2, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+userColumns,
		userID, passwordHash, now,
	))
	if err != nil {
		return identity.User{}, mapNotFound(err, "user_not_found")
	}

	if _, err := tx.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = COALESCE(revoked_at, $2), revocation_reason = COALESCE(revocation_reason, 'password_reset')
		WHERE user_id = $1
	`, userID, now); err != nil {
		return identity.User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return identity.User{}, err
	}
	return user, nil
}
