# No Mocking Your Own Code (BLOCKING)

A test that mocks the thing it claims to test proves nothing. Mocks rot, drift
from reality, and turn green while production breaks.

## The rule

- **NEVER mock code you own** — your modules, services, DB layer, components.
- **Mock ONLY true third-party network boundaries you do not control**, and only
  when hitting them for real is impossible/expensive/non-deterministic:
  payment gateways, email/SMS providers, external paid APIs.
- Even then, prefer a **fake/sandbox/local emulator** over a hand-written mock.

## Use real instances instead

| Instead of mocking... | Use |
|-----------------------|-----|
| Your database | Real local DB or Testcontainers; transaction-rollback per test |
| HTTP between your services | The real services (compose / test harness) |
| The browser / DOM | A real browser via Playwright (see `ui.md`) |
| Filesystem | A real temp dir (`mkdtemp`), cleaned up after |
| Time/clock | Injectable clock you control — not a mock of `Date` everywhere |
| Randomness | Seeded RNG |

## Allowed test doubles

- **Fakes** (working in-memory implementation) > stubs > mocks, in that order.
- A thin adapter at the external boundary may be stubbed — the adapter itself
  still gets one real integration test.

## Anti-patterns (NEVER)

- `mock('../../src/...')` on your own module.
- Asserting a mock was called instead of asserting the real outcome.
- Over-mocking until the test only exercises the mocks.
