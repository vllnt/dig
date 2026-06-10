# Testing Policy (BLOCKING)

The default answer to "should this have a test?" is **yes**. Code without a test
is code we cannot change with confidence.

## What to test (the shape)

- **Behavior, not implementation.** Test what a caller/user observes, not private internals.
- **Integration + E2E carry the weight** for product code — they catch the bugs users hit.
- **Unit tests pin tricky logic** — parsers, math, state machines, edge-case branches.
- Don't chase 100% unit coverage of glue code; cover the behavior that matters.

## When a test is REQUIRED (not optional)

| Change | Required test |
|--------|---------------|
| New user-facing feature/flow | E2E test of the full journey (see `e2e.md`) |
| UI added/changed | UI verification in a real browser/device (see `ui.md`) |
| Bug fix | Regression test that FAILS before the fix, PASSES after |
| New/changed public function, API, or type | Unit/contract test incl. >=1 error path |
| Auth, payments, data mutations | >=1 adversarial case, not just happy path |

## TDD default

RED -> GREEN -> REFACTOR. Write the failing test first when the behavior is known.
For exploratory spikes, backfill tests before the spike becomes real code.

## Gates

- Tests must pass before any task is "done".
- Coverage gate is BLOCKING — see `coverage.md`.
- Mocking your own code is BANNED — see `no-mocking.md`.

## Anti-patterns (NEVER)

- "Too hard to test" -> restructure the code so it's testable, or escalate.
- Asserting only that an element/object exists — assert the outcome.
- Deleting or skipping a failing test to make the suite go green.
