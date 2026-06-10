# End-to-End Testing (BLOCKING)

Every user-facing flow has an E2E test that drives a **real running app** the way
a user would.

## Required for

- Each new feature/flow (signup, checkout, search, settings, ...).
- Each multi-step journey — test the FULL flow, not isolated steps.
- Each critical path (auth, payments, data create/update/delete).

## What "E2E" means here

navigate -> interact -> assert outcome -> navigate away -> return -> assert persistence.

| User action | Assert |
|-------------|--------|
| Submit a form | success feedback shown AND data persisted |
| Trigger an error | meaningful error shown AND user can recover |
| Create/edit data | survives reload and re-navigation |
| Access gated content unauthenticated | redirected/blocked |

- Happy path: ALWAYS.
- Error path: ALWAYS at least one, with recovery.
- Edge cases when applicable (empty state, boundaries, long input, offline).

## Tooling (detect from stack)

- Web: Playwright against the real app (`webServer` boots it).
- Mobile (Expo/RN native): Maestro flows under `maestro/flows/`.
- Any backend language: Playwright (JS) or the language's HTTP/E2E harness — drive
  the deployed/booted service, not mocks.

## Anti-patterns (NEVER)

- Using curl/HTTP status as a substitute for a real E2E flow.
- Mocking the app under test (see `no-mocking.md`).
- Testing only that a route returns 200.
