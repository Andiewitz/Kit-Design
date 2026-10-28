package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/session"
)

const sessionColumns = `id, user_id, family_id, refresh_token_hash, user_agent, ip, created_at, last_used_at, expires_at, revoked_at`

func scanSession(row pgx.Row) (session.Session, error) {
	var value session.Session
	err := row.Scan(
		&value.ID,
		&value.UserID,
		&value.FamilyID,
		&value.RefreshTokenHash,
		&value.UserAgent,
		&value.IP,
		&value.CreatedAt,
		&value.LastUsedAt,
		&value.ExpiresAt,
		&value.RevokedAt,
	)
	return value, err
}

func scanSessionRows(rows pgx.Rows) (session.Session, error) {
	var value session.Session
	err := rows.Scan(
		&value.ID,
		&value.UserID,
		&value.FamilyID,
		&value.RefreshTokenHash,
		&value.UserAgent,
		&value.IP,
		&value.CreatedAt,
		&value.LastUsedAt,
		&value.ExpiresAt,
		&value.RevokedAt,
	)
	return value, err
}

func (s *Store) CreateSession(ctx context.Context, value session.Session) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, family_id, refresh_token_hash, user_agent, ip, created_at, last_used_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, value.ID, value.UserID, value.FamilyID, value.RefreshTokenHash, value.UserAgent, value.IP, value.CreatedAt, value.LastUsedAt, value.ExpiresAt)
	return err
}

func (s *Store) GetSession(ctx context.Context, id uuid.UUID) (session.Session, error) {
	value, err := scanSession(s.pool.QueryRow(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE id = $1`, id))
	if err != nil {
		return session.Session{}, mapNotFound(err, "session_not_found")
	}
	return value, nil
}

func (s *Store) RotateSession(ctx context.Context, id uuid.UUID, oldHash, newHash []byte, now time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET refresh_token_hash = $3, last_used_at = $4
		WHERE id = $1 AND refresh_token_hash = $2 AND revoked_at IS NULL AND expires_at > $4
	`, id, oldHash, newHash, now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) RevokeSession(ctx context.Context, id uuid.UUID, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = COALESCE(revoked_at, now()), revocation_reason = COALESCE(revocation_reason, $2)
		WHERE id = $1
	`, id, reason)
	return err
}

func (s *Store) RevokeFamily(ctx context.Context, familyID uuid.UUID, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = COALESCE(revoked_at, now()), revocation_reason = COALESCE(revocation_reason, $2)
		WHERE family_id = $1
	`, familyID, reason)
	return err
}

func (s *Store) RevokeAllForUser(ctx context.Context, userID uuid.UUID, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = COALESCE(revoked_at, now()), revocation_reason = COALESCE(revocation_reason, $2)
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, reason)
	return err
}

func (s *Store) ListSessions(ctx context.Context, userID uuid.UUID) ([]session.Session, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+sessionColumns+`
		FROM sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
		ORDER BY last_used_at DESC
		LIMIT 50
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []session.Session
	for rows.Next() {
		value, err := scanSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, value)
	}
	return sessions, rows.Err()
}

func (s *Store) RevokeUserSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = now(), revocation_reason = 'user_revoked'
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
	`, sessionID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1 AND user_id = $2)`, sessionID, userID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return apperr.NotFound("session_not_found")
	}
	return nil
}
