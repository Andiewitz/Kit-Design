package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"kitdesign/services/user/internal/apperr"
	"kitdesign/services/user/internal/identity"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 10
	config.MinConns = 1
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) Close() {
	s.pool.Close()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func mapNotFound(err error, code string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound(code)
	}
	return err
}

func scanUser(row pgx.Row) (identity.User, error) {
	var user identity.User
	var status string
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&status,
		&user.DisplayName,
		&user.EmailVerifiedAt,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if err != nil {
		return identity.User{}, err
	}
	user.Status = identity.Status(status)
	return user, nil
}

func scanUserRows(rows pgx.Rows) (identity.User, error) {
	var user identity.User
	var status string
	err := rows.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&status,
		&user.DisplayName,
		&user.EmailVerifiedAt,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	)
	if err != nil {
		return identity.User{}, err
	}
	user.Status = identity.Status(status)
	return user, nil
}

const userColumns = `id, email, password_hash, status, display_name, email_verified_at, last_login_at, created_at, updated_at, deleted_at`
