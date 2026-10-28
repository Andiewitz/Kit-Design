# User service

- Location: `services/user`
- Runtime: Go 1.27.3
- Datastore: PostgreSQL 18
- Default port: `3001`
- Health: `GET /healthz`, readiness: `GET /readyz`
- Ownership: backend/identity

## Responsibilities

The user service owns account identity, credentials, sessions, role-based permissions, account audit events, and authentication email delivery. Other services may rely on its access tokens for identity claims or call the trusted internal authorization endpoint.

## Architecture

```
client app/web
     |
     | HTTPS, JSON, bearer access token, refresh cookie
     v
services/user/cmd/api
     |
     +-- internal/httpapi        HTTP routing, middleware, handlers
     +-- internal/register       registration and email verification
     +-- internal/login          credential verification and session issue
     +-- internal/session        token issue/parse, refresh rotation, revocation
     +-- internal/recovery       password reset
     +-- internal/account        profile, sessions, audit history
     +-- internal/authorization  admin users/roles and permission checks
     +-- internal/outbox         transactional email worker
     +-- internal/storage        pgx store and goose migrations
     |
     +--> PostgreSQL
     +--> SMTP / Mailpit
```

The service is a modular monolith. Feature packages depend on narrow store interfaces; `internal/storage/postgres` implements them. Cross-feature composition happens in `cmd/api/main.go`.

## Runtime commands

| Command | Purpose |
| --- | --- |
| `go run ./cmd/api` | Start HTTP API and outbox worker |
| `go run ./cmd/migrate up` | Apply migrations |
| `go run ./cmd/migrate down` | Roll back one migration |
| `go run ./cmd/migrate status` | Show migration status |
| `go run ./cmd/admin ...` | Local user and role administration |
| `go run ./services/user/cmd/keygen` | Generate an Ed25519 JWT seed |

The Docker image contains all four binaries under `/app`; the default entrypoint is `/app/api`.

## HTTP behavior

- JSON success responses use `application/json`.
- Errors use `application/problem+json` with `type`, `title`, `status`, `code`, `detail`, and `request_id`.
- `X-Request-ID` is accepted or generated and echoed on every response.
- CORS allows only `APP_ORIGIN` with credentials.
- State-changing requests with an `Origin` header must match `APP_ORIGIN`.
- JSON bodies are capped at 1 MiB and must not contain unknown fields.
- Rate limits are applied per client IP and path for auth endpoints.
- Security headers include `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, and restrictive permissions policy.

## Authentication flows

### Registration

1. Normalize email and validate password.
2. Hash password with Argon2id.
3. Insert a `pending` user, hashed verification token, and queued email in one transaction.
4. Outbox worker sends the verification link.
5. Verification consumes the token, activates the user, and audits the action.

Duplicate registration returns the same `202` response as a new registration.

### Login and sessions

1. Verify email and password. Unknown accounts still run a dummy password hash.
2. Reject non-active accounts.
3. Load roles and permissions.
4. Insert a session with a SHA-256 refresh hash and issue an Ed25519 access token.
5. Set `kit_refresh` on path `/v1/auth`; return the access token and user in JSON.

### Refresh rotation

- Format: `<session-id>.<random-secret>`.
- Every refresh changes the stored hash and issues a new access token.
- Presenting a previously used refresh token revokes the entire session family.
- Expired or revoked sessions cannot refresh.

### Logout and revocation

- Logout revokes the current session.
- Users can revoke individual sessions from `/v1/account/sessions`.
- Suspension and password reset revoke active sessions.
- Access tokens are not introspected per request and remain valid until their 15-minute expiry.

### Password recovery

- Reset requests are accepted for unknown accounts without revealing existence.
- Reset tokens are hashed, single-use, and expire in 30 minutes.
- A successful reset updates the password and revokes all sessions.

## Authorization model

Roles are seeded by migration:

| Role | Permissions |
| --- | --- |
| `user` | `account.read`, `account.update`, `sessions.read`, `sessions.revoke` |
| `admin` | All permissions plus `users.read`, `users.manage`, `roles.manage` |

Permissions are embedded in access tokens and checked by middleware. Role changes apply on the next login or refresh.

Other trusted services can call:

```http
POST /internal/v1/authz/check
X-Internal-Token: <shared secret>
Content-Type: application/json

{"user_id":"<uuid>","permission":"users.read"}
```

The endpoint is disabled when `INTERNAL_SERVICE_TOKEN` is empty and returns `401` for a wrong token.

## Data and migrations

Migrations live in `internal/storage/migrations` and are embedded in the binary with goose. Apply them with the `migrate` command or the Compose `migrate` service. Never edit an applied migration; add the next numbered file.

Tables:

- `users`
- `roles`, `permissions`, `role_permissions`, `user_roles`
- `sessions`
- `one_time_tokens`
- `audit_events`
- `outbox_events`

Indexes cover user status, active sessions, token lookup, audit history, and pending outbox work.

## Email delivery

Registration and recovery handlers write email messages to `outbox_events` in the same transaction as state changes. The worker claims pending rows with `FOR UPDATE SKIP LOCKED`, sends through the configured mailer, and marks success or schedules retry with backoff.

`MAILER=log` records only metadata and does not include message bodies. `MAILER=smtp` uses `SMTP_ADDR` and `MAIL_FROM`. Local Compose uses Mailpit at `localhost:8025` for inspection.

## Observability

- Structured JSON logs include request method, path, status, duration, bytes, IP, and request ID.
- Internal errors are logged with the request ID; clients receive a generic detail.
- Audit events record authentication, account, session, and admin actions.
- Health endpoints support container and load-balancer checks.

No metrics or tracing exporter is configured yet.

## Configuration

| Variable | Default | Notes |
| --- | --- | --- |
| `APP_ENV` | `development` | Production requires explicit database and JWT key |
| `PORT` | `3001` | Listen port |
| `DATABASE_URL` | local dev URL | Required in production |
| `APP_ORIGIN` | `http://localhost:5173` | CORS and origin checks |
| `PUBLIC_APP_URL` | `http://localhost:5173` | Verification and reset links |
| `JWT_PRIVATE_KEY` | random in development | Base64 Ed25519 seed or private key |
| `JWT_KEY_ID` | `dev-1` | Token header key ID |
| `JWT_ISSUER` | `kit-user` | JWT issuer |
| `JWT_AUDIENCE` | `kit-api` | JWT audience |
| `ACCESS_TOKEN_TTL` | `15m` | Must be less than refresh TTL |
| `REFRESH_TOKEN_TTL` | `720h` | Refresh session lifetime |
| `COOKIE_SECURE` | true in production | Requires HTTPS when enabled |
| `COOKIE_DOMAIN` | empty | Optional cookie domain |
| `MAILER` | `log` | `log` or `smtp` |
| `SMTP_ADDR` | `localhost:1025` | SMTP server |
| `MAIL_FROM` | `no-reply@kit.local` | Envelope sender |
| `INTERNAL_SERVICE_TOKEN` | empty | Enables internal authz endpoint |
| `RATE_LIMIT_ENABLED` | `true` | In-memory rate limiting |
| `LOG_LEVEL` | `info` | debug, info, warn, error |
| `AUTO_MIGRATE` | `false` | Apply migrations at API startup |

From the repository root, generate a key with `go run ./services/user/cmd/keygen` and keep it out of git. Compose reads optional values from the repository-root `.env`.

## Deployment notes

- Access tokens are bearer JWTs and are valid until expiry after logout, suspension, or password reset, so keep `ACCESS_TOKEN_TTL` short.
- Internal authorization reflects the account's current status and roles, but permission claims in the clients access token can be up to 15 minutes stale.
- The refresh cookie is hardcoded to `SameSite=Lax`; this works when app and API are the same site. If app and API are on different registrable domains, add a configurable `SameSite=None; Secure` option first.
- IP-based rate limits and audit entries use the TCP peer address. Because they do not trust proxy headers yet, either terminate traffic direct at this service or add trusted-proxy handling before deploying behind a load balancer.
- Outbox email delivery is at least once: a process crash between SMTP send and the successful update can duplicate a message.
- There is no retention job yet for expired sessions, consumed tokens, or sent outbox rows. Schedule cleanup before production data grows.
- Token rotation uses a single Ed25519 key; key rotation requires deploying a new key and letting old access tokens expire.

## Failure behavior

- The API fails to start if the database is unreachable or the JWT key is invalid.
- `/readyz` returns `503` when the database ping fails.
- Email delivery failure leaves the outbox row pending with backoff; it does not roll back registration or recovery.
- Refresh races revoke the session family rather than issuing overlapping tokens.
- Rate limiting is per process and resets on restart.

## Local operations

```bash
npm run db:up
npm run migrate:up
npm run dev:user
```

Or run the complete stack:

```bash
docker compose up --build
```

Admin CLI examples:

```bash
go -C services/user run ./cmd/admin list-users
go -C services/user run ./cmd/admin grant-role <user-id> admin
go -C services/user run ./cmd/admin set-status <user-id> suspended
```

## Related documents

- [Plan](../plans/2026-10-27-user-service.md)
- [Contributor guide](../contributor-guides/go-services.md)
- [Test documentation](../tests-docs/user-service.md)
