# Test Documentation

Test strategy and conventions for the suites in [`/tests`](../../tests/).

## Layout

| Directory | Scope |
| --- | --- |
| `tests/unit` | Single functions, components, or modules in isolation |
| `tests/int` | Integration tests across real boundaries such as HTTP and persistence |
| `tests/e2e` | Complete user flows through the running applications |
| `tests/k6` | Load, stress, and soak tests with k6 |
| `tests/failure` | Dependency outages, crashes, timeouts, and degraded operation |
| `tests/security` | Authentication, authorization, input handling, secrets, and dependency risk |

## Current tooling

- Go unit tests: `go test` in `services/user`, files named `*_test.go`.
- Go integration tests: `tests/int/user`, run from the `kitdesign/services/user/tests` module against PostgreSQL.
- Load: k6 scripts in `tests/k6`, opt-in.
- Client unit tests: Vitest when the client suites are added.
- End-to-end: Playwright when browser flows are added.

## Commands

```bash
npm run test:unit:user
npm run test:int:user
npm run test:user
```

Integration tests require `npm run db:up` and `npm run migrate:up`. Use `TEST_DATABASE_URL` for a dedicated test database; the harness truncates identity tables.

## Conventions

- Name Go tests `*_test.go` and use table tests for validation and error matrices.
- Name integration targets by system, for example `tests/int/user`.
- Name end-to-end files `<flow>.spec.ts` and load scripts `<scenario>.k6.js`.
- Every fixed bug should add a regression test.
- Tests must be deterministic, isolated, and safe to run repeatedly.
- Use dedicated test credentials and data. Never target production.
- Keep slow, external, and failure-injection suites separate from the default fast run.

## Per-system documentation

- [User service](user-service.md)

## Adding a suite

1. Choose the directory that matches the scope.
2. Add the runner and configuration at the root or nearest module.
3. Add root scripts so one command runs each category.
4. Update this document and the relevant `tests/*/README.md`.
