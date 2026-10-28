# Systems

Architecture, ownership, and operational notes for each system in the repository.

## Current systems

### @kit/app

- Location: `client/app`
- Runtime: browser
- Stack: React 19, Vite, TypeScript
- Responsibility: interactive product interface

### @kit/web

- Location: `client/web`
- Runtime: static build
- Stack: Astro 7
- Responsibility: public website and content

### User service

- Location: `services/user`
- Runtime: Go 1.27.3
- Stack: PostgreSQL 18, pgx, goose, Ed25519 JWT
- Responsibility: identity, sessions, accounts, roles, audit events
- Health checks: `GET /healthz`, `GET /readyz` on port `3001`
- Documentation: [`user-service.md`](user-service.md)

## Dependency ownership

- Client workspaces declare dependencies in their own `package.json`; npm hoists to the root `node_modules` and records versions in `package-lock.json`.
- The Go service declares dependencies in `services/user/go.mod`; the test module declares its own in `tests/go.mod`.
- `go.work` ties the service and test modules together for local development.
- Container images are built from `services/user/Dockerfile` and run the compiled Go binaries.

## Adding a system

1. Create the system directory with its own manifest and lockfile where applicable.
2. For a Node workspace, add the directory to `workspaces` in the root `package.json`.
3. For a Go module, add it to `go.work` and document its module path.
4. Declare dependencies in that system, not at the repository root.
5. Add a system document here with runtime, ownership, health checks, and failure behavior.
6. Add tests in the matching `/tests` category and document commands.
