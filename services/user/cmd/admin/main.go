package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"kitdesign/services/user/internal/audit"
	"kitdesign/services/user/internal/identity"
	"kitdesign/services/user/internal/storage/postgres"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "admin:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("a command is required")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://kit:kit_dev_password@localhost:5432/kit?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	store, err := postgres.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	switch args[0] {
	case "list-users":
		return listUsers(ctx, store, args[1:])
	case "list-roles":
		return listRoles(ctx, store)
	case "set-status":
		return setStatus(ctx, store, args[1:])
	case "grant-role":
		return grantRole(ctx, store, args[1:])
	case "revoke-role":
		return revokeRole(ctx, store, args[1:])
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: admin <command> [arguments]

commands:
  list-users [-query text] [-limit 50]
  list-roles
  set-status <user-id> <active|suspended>
  grant-role <user-id> <user|admin>
  revoke-role <user-id> <user|admin>`)
}

func listUsers(ctx context.Context, store *postgres.Store, args []string) error {
	flags := flag.NewFlagSet("list-users", flag.ContinueOnError)
	query := flags.String("query", "", "search email or display name")
	limit := flags.Int("limit", 50, "maximum results")
	if err := flags.Parse(args); err != nil {
		return err
	}

	users, total, err := store.ListUsers(ctx, *query, *limit, 0)
	if err != nil {
		return err
	}
	output := make([]userOutput, 0, len(users))
	for _, user := range users {
		output = append(output, outputUser(user))
	}
	return printJSON(map[string]any{"users": output, "total": total})
}

func listRoles(ctx context.Context, store *postgres.Store) error {
	roles, err := store.ListRoles(ctx)
	if err != nil {
		return err
	}
	return printJSON(map[string]any{"roles": roles})
}

func setStatus(ctx context.Context, store *postgres.Store, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("set-status requires <user-id> <active|suspended>")
	}
	id, err := uuid.Parse(args[0])
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	status := identity.Status(args[1])
	if status != identity.StatusActive && status != identity.StatusSuspended {
		return fmt.Errorf("status must be active or suspended")
	}

	user, err := store.SetUserStatus(ctx, id, status)
	if err != nil {
		return err
	}
	_ = store.Record(ctx, audit.Event{
		SubjectID: &id,
		Action:    "admin.cli_status_changed",
		Metadata:  map[string]any{"status": string(status)},
	})
	return printJSON(outputUser(user))
}

func grantRole(ctx context.Context, store *postgres.Store, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("grant-role requires <user-id> <user|admin>")
	}
	id, role, err := parseRoleArgs(args)
	if err != nil {
		return err
	}
	if err := store.GrantRole(ctx, id, role); err != nil {
		return err
	}
	_ = store.Record(ctx, audit.Event{
		SubjectID: &id,
		Action:    "admin.cli_role_granted",
		Metadata:  map[string]any{"role": role},
	})
	fmt.Printf("granted role %s to %s\n", role, id)
	return nil
}

func revokeRole(ctx context.Context, store *postgres.Store, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("revoke-role requires <user-id> <user|admin>")
	}
	id, role, err := parseRoleArgs(args)
	if err != nil {
		return err
	}
	if err := store.RevokeRole(ctx, id, role); err != nil {
		return err
	}
	_ = store.Record(ctx, audit.Event{
		SubjectID: &id,
		Action:    "admin.cli_role_revoked",
		Metadata:  map[string]any{"role": role},
	})
	fmt.Printf("revoked role %s from %s\n", role, id)
	return nil
}

func parseRoleArgs(args []string) (uuid.UUID, string, error) {
	id, err := uuid.Parse(args[0])
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("invalid user id: %w", err)
	}
	role := args[1]
	if role != "user" && role != "admin" {
		return uuid.Nil, "", fmt.Errorf("role must be user or admin")
	}
	return id, role, nil
}

type userOutput struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

func outputUser(user identity.User) userOutput {
	return userOutput{
		ID:          user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Status:      string(user.Status),
		CreatedAt:   user.CreatedAt.Format(time.RFC3339),
	}
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
