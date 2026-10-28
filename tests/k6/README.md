# Load Tests

k6 scripts for measuring throughput, latency, and stability under load.

## Scripts

- [`user-session.k6.js`](user-session.k6.js) - login, account read, refresh, and logout with a pre-verified account.

## Rules

- Run only against dedicated test environments.
- Define thresholds for latency and error rate in every script.
- Model realistic traffic mixes and think times.
- Keep scenario data small and generated.
- Name scripts `<scenario>.k6.js`.

## Running

Install k6 separately, then run a script directly:

```bash
USER_EMAIL=load@example.com \
USER_PASSWORD='correct horse battery staple' \
BASE_URL=http://localhost:3001 \
APP_ORIGIN=http://localhost:5173 \
k6 run tests/k6/user-session.k6.js
```

The account must already be verified. Create it through registration, or use the admin CLI after local setup.

The default one VU per seven seconds stays under the service rate limits. Raise `VUS` only with RATE_LIMIT_ENABLED=false or an allowlisted source; otherwise login and refresh 429s will fail the thresholds.

## Scenario types

- Smoke: minimal load to confirm the environment works
- Load: expected peak traffic
- Stress: behavior beyond expected capacity
- Soak: sustained traffic to expose leaks and degradation
