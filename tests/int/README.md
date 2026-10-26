# Integration Tests

Tests that exercise multiple parts of a system through real boundaries such as HTTP, the filesystem, or a database.

## Rules

- Use real adapters for the boundary under test.
- Isolate data per run and clean up afterward.
- Start required dependencies locally or in CI; do not rely on shared environments.
- Assert observable behavior, not internal call sequences.
- Name files `<system>-<behavior>.test.ts`.

## Typical targets

- Services routes and error handling
- Database queries and migrations, once persistence is added
- Client data flows against a test API

## Planned tooling

Vitest with per-suite setup. Add the runner and environment configuration before creating suites.
