# Tests

All automated tests live here, separated by scope.

| Directory | Purpose |
| --- | --- |
| [`int`](int/) | Integration tests across real boundaries |
| [`unit`](unit/) | Isolated unit tests |
| [`e2e`](e2e/) | End-to-end user flows |
| [`k6`](k6/) | Load, stress, and soak tests |
| [`failure`](failure/) | Dependency outage and resilience tests |
| [`security`](security/) | Security behavior and scanning tests |

## Running tests

No test runner is configured yet. As runners are added, expose one root script per category:

```bash
npm run test:unit
npm run test:int
npm run test:e2e
npm run test:k6
npm run test:failure
npm run test:security
```

Document tooling decisions and conventions in [`/docs/tests-docs`](../docs/tests-docs/).

## Rules

- Tests must be deterministic and independent.
- Do not depend on execution order or shared mutable state.
- Never commit credentials, tokens, customer data, or production fixtures.
- Keep load and failure tests opt-in; they should not run in the default fast suite.
