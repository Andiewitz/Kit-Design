package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/identity"
	"kitdesign/services/user/internal/mailer"
	"kitdesign/services/user/internal/outbox"
	"kitdesign/services/user/internal/recovery"
	"kitdesign/services/user/internal/register"
)

func (s *Store) CreateRegistration(ctx context.Context, user identity.User, verification register.Token, message mailer.Message) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, status, display_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
	`, user.ID, user.Email, user.PasswordHash, string(user.Status), user.DisplayName, now)
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.Conflict("email_exists")
		}
		return err
	}

	roleTag, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = 'user'
		ON CONFLICT DO NOTHING
	`, user.ID)
	if err != nil {
		return err
	}
	if roleTag.RowsAffected() != 1 {
		return errors.New("default user role is missing")
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO one_time_tokens (id, user_id, purpose, token_hash, expires_at, created_at)
		VALUES ($1, $2, 'email_verify', $3, $4, $5)
	`, verification.ID, user.ID, verification.Hash, verification.ExpiresAt, now); err != nil {
		return err
	}

	if err := enqueueEmail(ctx, tx, message, now); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Store) ResendVerification(ctx context.Context, email string, verification register.Token, message mailer.Message) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1 AND status = 'pending'`, email).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE one_time_tokens
		SET consumed_at = $2
		WHERE user_id = $1 AND purpose = 'email_verify' AND consumed_at IS NULL
	`, userID, now); err != nil {
		return false, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO one_time_tokens (id, user_id, purpose, token_hash, expires_at, created_at)
		VALUES ($1, $2, 'email_verify', $3, $4, $5)
	`, verification.ID, userID, verification.Hash, verification.ExpiresAt, now); err != nil {
		return false, err
	}

	if err := enqueueEmail(ctx, tx, message, now); err != nil {
		return false, err
	}

	return true, tx.Commit(ctx)
}

func (s *Store) ConsumeVerification(ctx context.Context, tokenHash []byte) (identity.User, error) {
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
		WHERE token_hash = $1 AND purpose = 'email_verify'
		FOR UPDATE
	`, tokenHash).Scan(&tokenID, &userID, &expiresAt, &consumedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return identity.User{}, apperr.New(apperr.KindValidation, "invalid_token", "This verification link is invalid.")
		}
		return identity.User{}, err
	}
	if consumedAt != nil {
		return identity.User{}, apperr.New(apperr.KindValidation, "invalid_token", "This verification link has already been used.")
	}

	now := time.Now().UTC()
	if !expiresAt.After(now) {
		return identity.User{}, apperr.New(apperr.KindValidation, "token_expired", "This verification link has expired.")
	}

	if _, err := tx.Exec(ctx, `UPDATE one_time_tokens SET consumed_at = $2 WHERE id = $1`, tokenID, now); err != nil {
		return identity.User{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET status = 'active', email_verified_at = COALESCE(email_verified_at, $2), updated_at = $2
		WHERE id = $1 AND status = 'pending'
	`, userID, now); err != nil {
		return identity.User{}, err
	}

	user, err := scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, userID))
	if err != nil {
		return identity.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return identity.User{}, err
	}
	return user, nil
}

func (s *Store) FindUserByEmail(ctx context.Context, email string) (identity.User, error) {
	user, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1 AND deleted_at IS NULL`, email))
	if err != nil {
		return identity.User{}, mapNotFound(err, "user_not_found")
	}
	return user, nil
}

func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (identity.User, error) {
	user, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1 AND deleted_at IS NULL`, id))
	if err != nil {
		return identity.User{}, mapNotFound(err, "user_not_found")
	}
	return user, nil
}

func (s *Store) UpdateLastLogin(ctx context.Context, userID uuid.UUID, at time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET last_login_at = $2, updated_at = $2 WHERE id = $1`, userID, at)
	return err
}

func (s *Store) UpdateDisplayName(ctx context.Context, id uuid.UUID, displayName string) (identity.User, error) {
	user, err := scanUser(s.pool.QueryRow(ctx, `
		UPDATE users
		SET display_name = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+userColumns,
		id, displayName,
	))
	if err != nil {
		return identity.User{}, mapNotFound(err, "user_not_found")
	}
	return user, nil
}

func (s *Store) UserAuthorization(ctx context.Context, userID uuid.UUID) ([]string, []string, error) {
	roleRows, err := s.pool.Query(ctx, `
		SELECT r.name
		FROM roles r
		JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = $1
		ORDER BY r.name
	`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer roleRows.Close()

	var roles []string
	for roleRows.Next() {
		var role string
		if err := roleRows.Scan(&role); err != nil {
			return nil, nil, err
		}
		roles = append(roles, role)
	}
	if err := roleRows.Err(); err != nil {
		return nil, nil, err
	}

	permissionRows, err := s.pool.Query(ctx, `
		SELECT DISTINCT p.key
		FROM permissions p
		JOIN role_permissions rp ON rp.permission_id = p.id
		JOIN user_roles ur ON ur.role_id = rp.role_id
		WHERE ur.user_id = $1
		ORDER BY p.key
	`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer permissionRows.Close()

	var permissions []string
	for permissionRows.Next() {
		var permission string
		if err := permissionRows.Scan(&permission); err != nil {
			return nil, nil, err
		}
		permissions = append(permissions, permission)
	}
	if err := permissionRows.Err(); err != nil {
		return nil, nil, err
	}

	return roles, permissions, nil
}

func (s *Store) CreatePasswordReset(ctx context.Context, email string, reset recovery.Token, message mailer.Message) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1 AND status = 'active'`, email).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE one_time_tokens
		SET consumed_at = $2
		WHERE user_id = $1 AND purpose = 'password_reset' AND consumed_at IS NULL
	`, userID, now); err != nil {
		return false, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO one_time_tokens (id, user_id, purpose, token_hash, expires_at, created_at)
		VALUES ($1, $2, 'password_reset', $3, $4, $5)
	`, reset.ID, userID, reset.Hash, reset.ExpiresAt, now); err != nil {
		return false, err
	}

	if err := enqueueEmail(ctx, tx, message, now); err != nil {
		return false, err
	}

	return true, tx.Commit(ctx)
}

func enqueueEmail(ctx context.Context, tx pgx.Tx, message mailer.Message, now time.Time) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (id, topic, payload, created_at)
		VALUES ($1, $2, $3::jsonb, $4)
	`, uuid.Must(uuid.NewV7()), outbox.TopicEmailSend, string(payload), now)
	return err
}
