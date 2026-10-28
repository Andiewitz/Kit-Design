# Go services

Guide for the Go service module at `services/user`.

## Prerequisites

- Go 1.27.3
- Docker with Compose for PostgreSQL and Mailpit
- Node.js 22.23.4 or newer for the client workspaces

Verify the toolchain:

```bash
go version
# go version go1.27.3 linux/amd64
```

If `/usr/local/go/bin` is not on `PATH`, add it or invoke that binary directly. The `go` command can also auto-download the toolchain declared in `go.mod` when `GOTOOLCHAIN=auto`.

## Repository layout

- `services/user/cmd/api` - HTTP server and outbox worker
- `services/user/cmd/migrate` - migration runner
- `services/user/cmd/admin` - local administration CLI
- `services/user/cmd/keygen` - Ed25519 key generation
- `services/user/internal` - feature packages, HTTP layer, storage, migrations
- `tests/int/user` - black-box HTTP integration tests
- `go.work` - workspace containing the service and test modules
- `compose.yaml` - PostgreSQL, Mailpit, migration, and API services

`go.work` is committed deliberately so the service and test modules resolve together; `go.work.sum` stays local. Do not add machine-specific `replace` directives.

The test module is `kitdesign/services/user/tests` even though it lives at the repository root. This keeps its import path under the service module so integration tests can import service packages while remaining a separate module.

## Setup

```bash
npm install
cp .env.example .env
set -a
. ./.env
set +a
go run ./services/user/cmd/keygen
```

Set `JWT_PRIVATE_KEY` from the generated seed when you need stable tokens across restarts:

```bash
export JWT_PRIVATE_KEY="$(cat .secrets/jwt_ed25519.key)"
```

Leaving it empty in development uses an ephemeral key and logs a warning.

## Local workflow

```bash
npm run db:up
npm run migrate:up
npm run dev:user
```

The API listens on `http://localhost:3001`. Mailpit is at `http://localhost:8025`.

Run the complete container stack:

```bash
docker compose up --build
```

The `migrate` service runs once and must complete before `user` starts.

## Environment

Copy `.env.example` to `.env`; Compose reads it automatically. For local Go runs, export the values:

```bash
set -a
source .env
set +a
```

The API does not load `.env` itself. Never commit `.env` or `.secrets/`.

## Database and migrations

Migrations are SQL files in `services/user/internal/storage/migrations` and are embedded in the binary. Use goose markers:

```sql
-- +goose Up
-- statements

-- +goose Down
-- statements
```

Add the next number, for example `0002_add_feature.sql`. Never rewrite a migration that has been applied. Apply and inspect:

```bash
go -C services/user run ./cmd/migrate up
go -C services/user run ./cmd/migrate status
go -C services/user run ./cmd/migrate down
```

`down` rolls back one migration. Use it only in development or with a reviewed rollback plan.

## Testing

Fast unit tests:

```bash
go -C services/user test ./...
```

Integration tests require PostgreSQL and truncate the identity tables before each suite:

```bash
npm run db:up
npm run migrate:up
npm run test:int:user
```

Use `TEST_DATABASE_URL` to point integration tests at a dedicated database. Never run them against shared or production data.

Useful checks:

```bash
go -C services/user vet ./...
go -C services/user test -race ./...
go -C tests test ./int/user/... -count=1
```

## Adding a feature

1. Add or extend a domain package under `services/user/internal`.
2. Keep dependencies behind interfaces defined by the consumer.
3. Implement persistence in `internal/storage/postgres`.
4. Wire the dependency in `cmd/api/main.go`.
5. Add HTTP routes and handlers in `internal/httpapi`.
6. Add unit tests near the package and integration coverage in `tests/int/user` for observable behavior.
7. Add a migration if the schema changes.
8. Update `docs/systems/user-service.md` and `docs/tests-docs/user-service.md`.

## Error handling

- Return `*apperr.Error` for expected failures; set a stable machine-readable code.
- Validation errors use `apperr.Invalid`.
- Do not leak internal errors or SQL details to clients.
- `writeError` maps types to HTTP status and logs internal failures with the request ID.
- Audit security-relevant state changes, but do not fail the request if audit insertion fails.

## Security rules

- Never log passwords, raw tokens, authorization headers, or JWT seeds.
- Hash password and one-time tokens; store only hashes.
- Use constant-time comparison for shared secrets.
- Keep state-changing endpoints behind origin checks and authentication.
- Enforce authorization in the service even when the client hides UI.
- Add rate limits to credential and email endpoints.

## Administration

```bash
go -C services/user run ./cmd/admin list-users
go -C services/user run ./cmd/admin list-roles
go -C services/user run ./cmd/admin grant-role <user-id> admin
go -C services/user run ./cmd/admin revoke-role <user-id> admin
go -C services/user run ./cmd/admin set-status <user-id> suspended
```

The CLI connects with `DATABASE_URL` and writes audit events for its changes.

## Troubleshooting

- `database system is starting up`: wait for the Compose health check, then rerun migrations.
- `relation "users" does not exist`: run `npm run migrate:up`.
- `JWT_PRIVATE_KEY is not set`: expected in development; set a generated key for stable sessions.
- Refresh always fails: confirm the client sends the `kit_refresh` cookie to `/v1/auth` and uses the latest rotated value.
- Mailpit is empty: check `MAILER=smtp`, `SMTP_ADDR`, and outbox worker logs.
