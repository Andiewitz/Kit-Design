# End-to-End Tests

Tests that drive complete user flows through the running applications, including the browser and backend.

## Rules

- Cover critical journeys, not every combination.
- Use dedicated test accounts and reset state between runs.
- Prefer stable role- and label-based selectors over CSS or test IDs.
- Capture traces and screenshots on failure.
- Name files `<flow>.spec.ts`.

## Candidate flows

- App loads and completes its primary workflow
- Website navigation and key content pages
- Registration, email verification, login, and logout
- Password recovery through an emailed link
- Account profile and session management
- Admin user status and role changes

## Planned tooling

Playwright. No runner is configured yet.
