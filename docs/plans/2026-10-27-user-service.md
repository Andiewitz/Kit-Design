# User service: registration, login, account, and authorization

- Status: in progress
- Owner: engineering
- Created: 2026-10-27
- Target: first backend service milestone

## Problem

The repository has client applications but no backend service. Accounts need a durable identity store, secure sessions, account self-service, and role-based authorization before product work can depend on authenticated users.

## Goals

- Provide a Go service at `services/user` with registration, email verification, login, refresh, logout, password recovery, account profile, session management, and authorization checks.
- Store data in PostgreSQL with versioned migrations.
- Use short-lived Ed25519 access tokens and rotating refresh tokens in an HttpOnly cookie.
- Enforce role-based permissions in the service and expose a trusted internal authorization check.
- Send verification and reset email through a transactional outbox.
- Document architecture, operations, contributor workflow, and test strategy in their existing documentation categories.

## Non-goals

- Social login, MFA, passkeys, or organization tenancy.
- A general-purpose policy engine.
- Client-side authentication UI in this change.
- Production email provider integration beyond SMTP configuration.
- Fine-grained per-object permissions.

## Proposal

### Runtime

- Go 1.27.3 module: `kitdesign/services/user`
- HTTP service on port `3001`
- PostgreSQL 18
- Mailpit for local SMTP capture
- Docker image containing `api`, `migrate`, `admin`, and `keygen`

### API surface

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/v1/auth/register` | Create a pending account and queue verification email |
| `POST` | `/v1/auth/email/verify` | Consume a verification token and activate the account |
| `POST` | `/v1/auth/email/resend` | Reissue a verification email without revealing account existence |
| `POST` | `/v1/auth/login` | Verify credentials and issue tokens |
| `POST` | `/v1/auth/refresh` | Rotate the refresh token and issue a new access token |
| `POST` | `/v1/auth/logout` | Revoke the current session |
| `POST` | `/v1/auth/password/forgot` | Queue a password reset email |
| `POST` | `/v1/auth/password/reset` | Consume a reset token, change the password, revoke sessions |
| `GET` | `/v1/account` | Read the authenticated profile |
| `PATCH` | `/v1/account` | Update display name |
| `GET` | `/v1/account/sessions` | List active sessions |
| `DELETE` | `/v1/account/sessions/{id}` | Revoke one session |
| `GET` | `/v1/account/events` | Read recent account audit events |
| `GET` | `/v1/admin/users` | Search and list users |
| `PATCH` | `/v1/admin/users/{id}/status` | Activate or suspend a user |
| `POST` | `/v1/admin/users/{id}/roles` | Grant a role |
| `DELETE` | `/v1/admin/users/{id}/roles/{role}` | Revoke a role |
| `GET` | `/v1/admin/roles` | List roles |
| `POST` | `/internal/v1/authz/check` | Trusted permission check for other services |
| `GET` | `/healthz` | Liveness |
| `GET` | `/readyz` | Database readiness |

### Data model

- `users`: normalized email, Argon2id hash, status, display name, verification and login timestamps.
- `roles`, `permissions`, `role_permissions`, `user_roles`: `user` and `admin` roles with seeded permissions.
- `sessions`: refresh-token hash, family, client metadata, expiry, revocation.
- `one_time_tokens`: hashed email verification and password reset tokens.
- `audit_events`: actor, subject, action, IP, metadata.
- `outbox_events`: queued email delivery with retry state.

### Security decisions

- Passwords: Argon2id, 64 MiB, 3 iterations, parallelism 2; 12 to 128 characters.
- Access tokens: Ed25519 JWT, 15 minutes, include roles, permissions, and session ID.
- Refresh tokens: 32 random bytes, SHA-256 hash stored, 30-day expiry, rotated on every use.
- Refresh reuse revokes the session family.
- Cookies: HttpOnly, SameSite=Lax, path `/v1/auth`; Secure enabled outside development.
- Broader account endpoints use `Authorization: Bearer`.
- State-changing endpoints reject mismatched `Origin` values.
- JSON bodies require `Content-Type: application/json`, reject unknown fields, and are limited to 1 MiB.
- Authentication responses are intentionally generic to reduce account enumeration.
- Internal authorization checks require a shared `INTERNAL_SERVICE_TOKEN`.

### Alternatives considered

- **Node/TypeScript service** - rejected for stronger Go concurrency, static deployment artifact, and built-in HTTP tooling.
- **Opaque access tokens with database lookup** - rejected to keep read paths stateless; refresh tokens remain revocable.
- **Cookie-only sessions for all requests** - rejected to avoid CSRF exposure on every API call and to support non-browser clients.
- **Direct SMTP send in request handlers** - rejected; outbox keeps registration/recovery transactions durable.
- **Separate services per auth concern** - rejected as premature for the first milestone.

## Risks

- Access tokens remain valid until expiry after suspension, logout, or password reset. Keep the 15-minute TTL and document the window.
- A shared internal token is a coarse trust boundary. Rotate it and keep the authz endpoint private.
- Argon2id parameters consume memory under load; rate limits and container limits must be sized together.
- Email delivery is eventually consistent through the outbox.
- Role changes do not alter already-issued access tokens; clients must refresh to see new permissions.
- The current rate limiter is per process and in-memory, so it does not coordinate across replicas.

## Validation

- Unit tests cover validation, password hashing, token generation, JWT parsing, service rules, outbox retries, and HTTP middleware.
- Integration tests run the HTTP API against PostgreSQL and cover registration, verification, login, refresh rotation/reuse, logout, recovery, admin roles, session listing, audit events, CSRF, and account enumeration behavior.
- `go vet` and `go test ./...` run in the service module; integration tests run from `tests`.
- Docker image builds and Compose smoke test confirmed `/healthz`, `/readyz`, registration, outbox delivery, and Mailpit receipt.

## Rollout and rollback

1. Merge the service and migration.
2. Apply migration `0001_init.sql` with the `migrate` command or Compose migration service.
3. Generate and inject `JWT_PRIVATE_KEY`; set `INTERNAL_SERVICE_TOKEN`.
4. Deploy the API and configure the client `APP_ORIGIN`.
5. Roll back the API image first; roll back the migration only if no account data must be preserved.

## Open questions

- Which production email provider and bounce handling will replace local SMTP?
- Should suspension invalidate access tokens immediately through a version claim or per-request status lookup?
- Is a distributed rate limiter needed at the first multi-replica deployment?
- When should the client application expose account and admin screens?
