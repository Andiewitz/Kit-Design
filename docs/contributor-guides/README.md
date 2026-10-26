# Contributor Guides

## Prerequisites

- Node.js 22.23.4 or newer
- npm 11 or newer

## Setup

```bash
npm install
```

All dependencies are installed once from the repository root. Each workspace declares its own dependencies.

## Repository layout

- `client/app` - React application built with Vite and TypeScript (`@kit/app`)
- `client/web` - Astro website (`@kit/web`)
- `services` - Node.js and TypeScript HTTP service (`@kit/services`)
- `tests` - unit, integration, end-to-end, load, failure, and security tests
- `docs` - issues, plans, system notes, post-mortems, and test documentation

## Development commands

```bash
npm run dev:app
npm run dev:web
npm run dev:services
```

The services health endpoint is available at `http://localhost:3001/health`.

## Before opening a pull request

```bash
npm run typecheck
npm run lint
npm run build
```

Add the relevant test suites once their runners are configured. Never commit secrets, credentials, or production data.

## Where to document work

- Bug or regression: `docs/issues`
- Upcoming design or implementation work: `docs/plans`
- Incident follow-up: `docs/post-mortems`
- Architecture or ownership changes: `docs/systems`
- Test strategy or harness changes: `docs/tests-docs`
