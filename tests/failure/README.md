# Failure and Resilience Tests

Tests for what happens when dependencies fail or the runtime degrades. These are deliberate fault-injection suites, not happy-path checks.

## Scenarios to cover

- Database unavailable, slow, or dropped mid-transaction
- Service process crash and restart
- Upstream timeouts and connection resets
- Partial network failure or DNS failure
- Disk full or read-only filesystem
- Invalid or exhausted configuration
- Traffic spikes beyond capacity

## Expected behavior

- Fail fast with bounded retries and timeouts
- Return clear errors without leaking internals
- Preserve data integrity and avoid duplicate side effects
- Report degraded health through `/health` or equivalent
- Emit actionable logs and metrics
- Recover automatically when the dependency returns

## Rules

- Run in isolated environments with disposable data.
- Record recovery time and any manual steps required.
- Add a regression test for every incident that reaches production.
- Never inject faults into production without an approved plan.
