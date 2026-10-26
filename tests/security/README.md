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

## Baseline checks

- `npm audit` in CI, with an explicit policy for accepted advisories
- Secret scanning on every pull request
- Dependency review for new packages
- Authentication and authorization tests for every protected route

No security runner is configured yet.
