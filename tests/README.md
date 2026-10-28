# Tests

All automated tests live here, separated by scope. Go integration tests share the `tests/go.mod` module; future client suites add their own runner.

| Directory | Purpose |
| --- | --- |
| [`int`](int/) | Integration tests across real boundaries |
| [`unit`](unit/) | Isolated unit tests |
| [`e2e`](e2e/) | End-to-end user flows |
| [`k6`](k6/) | Load, stress, and soak tests |
| [`failure`](failure/) | Dependency outage and resilience tests |
| [`security`](security/) | Security behavior and scanning tests |

## Current runners

- Go unit tests live beside the code in `services/user`.
- Go integration tests live in [`int/user`](int/user/).
- k6 scripts are opt-in.

## Running tests

```bash
npm run test:unit:user   # Go unit tests
npm run test:int:user    # PostgreSQL-backed HTTP integration tests
npm run test:user        # all service unit tests
```

Integration tests require a migrated database:

```bash
npm run db:up
npm run migrate:up
```

Document tooling decisions and conventions in [`/docs/tests-docs`](../docs/tests-docs/).

## Rules

- Tests must be deterministic and independent.
- Do not depend on execution order or shared mutable state.
- Never commit credentials, tokens, customer data, or production fixtures.
- Keep load and failure tests opt-in; they should not run in the default fast suite.
- Reset test data per suite and use a dedicated database for integration runs.
