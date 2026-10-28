# Unit Tests

Tests for a single function, component, or module with external dependencies replaced.

## Rules

- Test behavior and public contracts, not implementation details.
- Keep each test independent and fast.
- Replace only the boundary directly under test; prefer real pure functions elsewhere.
- Name Go tests `*_test.go`; name client tests `<subject>.test.ts` or `<subject>.test.tsx`.

## Current suites

Go unit tests live beside the packages they cover:

```bash
npm run test:unit:user
# or
go -C services/user test ./...
```

They cover identity validation, password hashing, random tokens, Ed25519 JWT handling, service authorization rules, registration/login/recovery services, account services, outbox retries, and HTTP middleware.

## Planned tooling

Vitest with React Testing Library for client components. Add the runner before creating client suites.
