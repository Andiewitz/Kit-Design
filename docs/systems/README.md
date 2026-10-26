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

### @kit/services

- Location: `services`
- Runtime: Node.js 22.23.4 or newer
- Stack: TypeScript, native Node HTTP server
- Responsibility: backend API surface
- Health check: `GET /health` on port `3001` by default, configurable with `PORT`

## Dependency ownership

Each workspace declares dependencies in its own `package.json`. npm hoists the installed tree to the root `node_modules` and records resolved versions in the root `package-lock.json`.

## Adding a system

1. Create the system directory with its own `package.json`.
2. Add the directory to the `workspaces` array in the root `package.json`.
3. Declare dependencies in that workspace, not at the root.
4. Document the system here, including runtime, ownership, and health checks.
