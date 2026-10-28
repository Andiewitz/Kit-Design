# Security Tests

Automated checks for security behavior and regressions.

## Scope

- Authentication and session handling
- Authorization and object-level access control
- Input validation, injection, and unsafe deserialization
- Cross-site scripting and request forgery protections
- Secrets and credential exposure
- Dependency vulnerabilities and license risk
- Security headers, CORS, and rate limiting
- Error messages that do not leak internals

## Rules

- Use dedicated test accounts and synthetic data.
- Never run destructive or denial-of-service tests against production.
- Keep scanner output free of live credentials.
- Track every finding as an issue with severity and owner.

## Current coverage

The user-service integration suite covers:

- CSRF origin rejection on state-changing requests.
- Role and permission boundaries for account and admin routes.
- Generic credential errors and duplicate-registration masking.
- Refresh-token replay and family revocation.
- Password-reset token reuse rejection.
- Internal service token rejection.
- Security headers, CORS, and rate-limit responses.

Run it with `npm run test:int:user`.

## Baseline checks

- `npm audit` in CI, with an explicit policy for accepted advisories
- `govulncheck ./...` in `services/user` and `tests`
- Secret scanning on every pull request
- Dependency review for new packages
- Authentication and authorization tests for every protected route

`govulncheck` and secret scanning are not wired into CI yet.
