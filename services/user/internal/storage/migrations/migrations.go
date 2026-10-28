package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var files embed.FS

func Up(ctx context.Context, databaseURL string) error {
	return run(ctx, databaseURL, func(db *sql.DB) error {
		return goose.UpContext(ctx, db, ".")
	})
}

func Down(ctx context.Context, databaseURL string) error {
	return run(ctx, databaseURL, func(db *sql.DB) error {
		return goose.DownContext(ctx, db, ".")
	})
}

func Status(ctx context.Context, databaseURL string) error {
	return run(ctx, databaseURL, func(db *sql.DB) error {
		return goose.StatusContext(ctx, db, ".")
	})
}

func run(ctx context.Context, databaseURL string, action func(*sql.DB) error) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	goose.SetBaseFS(files)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	return action(db)
}
