# Load Tests

k6 scripts for measuring throughput, latency, and stability under load.

## Rules

- Run only against dedicated test environments.
- Define thresholds for latency and error rate in every script.
- Model realistic traffic mixes and think times.
- Keep scenario data small and generated.
- Name scripts `<scenario>.k6.js`.

## Scenario types

- Smoke: minimal load to confirm the environment works
- Load: expected peak traffic
- Stress: behavior beyond expected capacity
- Soak: sustained traffic to expose leaks and degradation

## Running

Install k6 separately, then run a script directly:

```bash
k6 run tests/k6/<scenario>.k6.js
```

No baseline scenarios are defined yet.
