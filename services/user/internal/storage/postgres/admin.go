package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/identity"
)

func (s *Store) ListUsers(ctx context.Context, query string, limit, offset int) ([]identity.User, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM users
		WHERE deleted_at IS NULL
		  AND ($1 = '' OR email ILIKE '%' || $1 || '%' OR display_name ILIKE '%' || $1 || '%')
	`, query).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE deleted_at IS NULL
		  AND ($1 = '' OR email ILIKE '%' || $1 || '%' OR display_name ILIKE '%' || $1 || '%')
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []identity.User
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (s *Store) SetUserStatus(ctx context.Context, id uuid.UUID, status identity.Status) (identity.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return identity.User{}, err
	}
	defer tx.Rollback(ctx)

	user, err := scanUser(tx.QueryRow(ctx, `
		UPDATE users
		SET status = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+userColumns,
		id, string(status),
	))
	if err != nil {
		return identity.User{}, mapNotFound(err, "user_not_found")
	}

	if status == identity.StatusSuspended {
		if _, err := tx.Exec(ctx, `
			UPDATE sessions
			SET revoked_at = COALESCE(revoked_at, now()), revocation_reason = COALESCE(revocation_reason, 'account_suspended')
			WHERE user_id = $1 AND revoked_at IS NULL
		`, id); err != nil {
			return identity.User{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return identity.User{}, err
	}
	return user, nil
}

func (s *Store) GrantRole(ctx context.Context, userID uuid.UUID, role string) error {
	var roleID uuid.UUID
	if err := s.pool.QueryRow(ctx, `SELECT id FROM roles WHERE name = $1`, role).Scan(&roleID); err != nil {
		return mapNotFound(err, "role_not_found")
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, userID, roleID)
	if err != nil {
		var pgErr interface{ SQLState() string }
		if errors.As(err, &pgErr) && pgErr.SQLState() == "23503" {
			return apperr.NotFound("user_not_found")
		}
		return err
	}
	return nil
}

func (s *Store) RevokeRole(ctx context.Context, userID uuid.UUID, role string) error {
	var roleID uuid.UUID
	if err := s.pool.QueryRow(ctx, `SELECT id FROM roles WHERE name = $1`, role).Scan(&roleID); err != nil {
		return mapNotFound(err, "role_not_found")
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`, userID, roleID)
	return err
}

func (s *Store) ListRoles(ctx context.Context) ([]identity.Role, error) {
	rows, err := s.pool.Query(ctx, `SELECT name, description FROM roles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []identity.Role
	for rows.Next() {
		var role identity.Role
		if err := rows.Scan(&role.Name, &role.Description); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (s *Store) CheckPermission(ctx context.Context, userID uuid.UUID, permission string) (bool, error) {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users u
			JOIN user_roles ur ON ur.user_id = u.id
			JOIN role_permissions rp ON rp.role_id = ur.role_id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE u.id = $1
			  AND u.status = 'active'
			  AND u.deleted_at IS NULL
			  AND p.key = $2
		)
	`, userID, permission).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return allowed, err
}
