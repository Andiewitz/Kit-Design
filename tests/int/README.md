# Integration Tests

Tests that exercise multiple parts of a system through real boundaries such as HTTP and a database.

## Current suites

- [`user`](user/) - Go HTTP integration tests for registration, verification, login, refresh rotation, recovery, accounts, admin roles, audit events, CSRF, and health checks.

## Rules

- Use real adapters for the boundary under test.
- Isolate data per run and clean up afterward.
- Start required dependencies locally or in CI; do not rely on shared environments.
- Assert observable behavior through HTTP responses, not internal call sequences.
- Name Go targets by system, for example `tests/int/user`.

## Running

```bash
npm run db:up
npm run migrate:up
npm run test:int:user
```

Direct command:

```bash
go -C tests test ./int/user/... -count=1
```

Use `TEST_DATABASE_URL` to isolate the run. The user-service harness truncates identity tables before each suite, so never point it at shared or production data.

## Tooling

- Go testing with `httptest` and a real PostgreSQL store.
- Module path: `kitdesign/services/user/tests`.
- Future client integration tests use Vitest once configured.
