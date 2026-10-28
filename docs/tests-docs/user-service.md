# User service tests

Test strategy for the Go user service at `services/user`.

## Layers

| Layer | Location | Scope |
| --- | --- | --- |
| Unit | `services/user/internal/**/*_test.go` | Validation, hashing, JWT, service rules, middleware, outbox retries |
| Integration | `tests/int/user` | Real HTTP handler stack against PostgreSQL |
| Load | `tests/k6/user-session.k6.js` | Opt-in login/account/refresh/logout throughput |
| Failure and security | `tests/failure`, `tests/security` | Documented scenarios; expand as the service matures |

## Unit tests

Run from the service module:

```bash
go -C services/user test ./...
```

Coverage includes:

- Email, password, display-name, and status validation.
- Argon2id hash and verification, including malformed hashes.
- Random token generation and deterministic hashing.
- Ed25519 JWT issue/parse, tampering, wrong audience, and wrong key.
- Registration validation, token hashing, and audit events.
- Login success, invalid credentials, unknown accounts, and inactive accounts.
- Authorization role/status rules and self-protection.
- Account profile updates, session revoke audit, and event limits.
- Outbox send, retry, unsupported topic, and invalid payload.
- HTTP middleware: JSON decoding, CORS, security headers, request IDs, origin checks, rate limits, and problem responses.

## Integration tests

Location: `tests/int/user`

The suite builds the real HTTP router with real services and a real PostgreSQL store. It uses `httptest` and a cookie jar; it does not mock the database.

Run:

```bash
npm run db:up
npm run migrate:up
npm run test:int:user
```

Direct command:

```bash
go -C tests test ./int/user/... -count=1
```

Environment:

- `TEST_DATABASE_URL` preferred.
- Falls back to `DATABASE_URL`.
- Falls back to the local Compose URL.

The harness truncates `users`, `sessions`, `one_time_tokens`, `audit_events`, and `outbox_events` before each harness instance. Roles and permissions seeded by migration remain. Never point these tests at shared or production data.

Covered flows:

- Registration, duplicate-email masking, verification, login, profile read/update.
- Session listing, current-session marker, logout, and refresh.
- Refresh rotation, old-token replay, and family revocation.
- Password reset, old-password rejection, session revocation, and reset-token reuse rejection.
- Admin user listing, suspend/activate, role grant/revoke, and permission enforcement.
- Internal authorization checks and invalid internal token rejection.
- Audit event visibility for account actions.
- CSRF origin rejection, content-type enforcement, weak password rejection, invalid token handling.
- Health and readiness endpoints.

Email is not sent during integration tests. The outbox payload is read directly so verification and reset tokens can be extracted.

## Load tests

`tests/k6/user-session.k6.js` exercises login, account read, refresh, and logout against a dedicated environment with a pre-verified account.

```bash
USER_EMAIL=load@example.com \
USER_PASSWORD='correct horse battery staple' \
BASE_URL=http://localhost:3001 \
APP_ORIGIN=http://localhost:5173 \
k6 run tests/k6/user-session.k6.js
```

Load tests are opt-in and must not target production or shared development data.

## Security coverage

Integration tests currently cover origin validation, authorization boundaries, generic credential errors, reset-token reuse, refresh reuse, and internal-token rejection. Add `govulncheck` and secret scanning to CI before production. See [`tests/security/README.md`](../../tests/security/README.md).

## Failure coverage

Unit tests cover outbox send failure and retry scheduling. Remaining failure scenarios to automate: database outage during request handling, migration failure, SMTP timeout, graceful shutdown, and overloaded rate-limit behavior. See [`tests/failure/README.md`](../../tests/failure/README.md).

## Conventions

- Use Go table tests for validation and error matrices.
- Test exported behavior and HTTP contracts; avoid asserting private call sequences.
- Keep integration data synthetic and reset per harness.
- Fixed auth or authorization bugs require a regression test.
- Do not log or commit raw tokens, cookies, passwords, or JWT seeds.
- Run `go vet ./...` and `go test -race ./...` before merging backend changes.

## CI commands

```bash
npm run lint
npm run test:user
npm run test:int:user
npm run build:user
```
