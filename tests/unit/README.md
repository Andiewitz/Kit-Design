# Unit Tests

Tests for a single function, component, or module with all external dependencies replaced.

## Rules

- Test behavior and public contracts, not implementation details.
- Keep each test independent and fast.
- Mock only the boundary directly under test; prefer real pure functions elsewhere.
- Name files `<subject>.test.ts` or `<subject>.test.tsx`.

## Planned tooling

Vitest with the React Testing Library for components. Add the runner before creating suites.
