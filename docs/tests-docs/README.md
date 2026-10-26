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

## Tooling status

No test runner is configured yet. Record the chosen frameworks, root scripts, and CI commands here before adding the first suite.

Expected tool direction:

- Unit and integration: Vitest
- End-to-end: Playwright
- Load: k6
- Security: dependency audit plus targeted application tests

## Conventions

- Name unit and integration files `<subject>.test.ts` or `<subject>.test.tsx`.
- Name end-to-end files `<flow>.spec.ts`.
- Name load scripts `<scenario>.k6.js`.
- Every fixed bug should add a regression test.
- Tests must be deterministic, isolated, and safe to run repeatedly.
- Use dedicated test credentials and data. Never target production.
- Keep slow, external, and failure-injection suites separate from the default fast run.

## Adding a suite

1. Choose the directory that matches the scope.
2. Add the runner and configuration at the root or nearest workspace.
3. Add root scripts so one command runs each category.
4. Update this document and the relevant `tests/*/README.md`.
