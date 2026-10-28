package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"kitdesign/services/user/internal/storage/migrations"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	direction := flags.String("direction", "up", "migration direction: up, down, or status")
	if err := flags.Parse(args); err != nil {
		return err
	}

	action := *direction
	if flags.NArg() > 0 {
		action = flags.Arg(0)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://kit:kit_dev_password@localhost:5432/kit?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch action {
	case "up":
		return migrations.Up(ctx, databaseURL)
	case "down":
		return migrations.Down(ctx, databaseURL)
	case "status":
		return migrations.Status(ctx, databaseURL)
	default:
		return fmt.Errorf("unknown direction %q", action)
	}
}
