# Coverage (BLOCKING gate)

Coverage is a **floor that stops regressions**, not a trophy. The gate blocks
merges; the adversarial-case rule below is what actually buys quality.

## The gate

- **Lines AND branches >= 90%** (default; raise per-package as it matures).
- Configured in the runner (`vitest.config.ts` thresholds / `--cov-fail-under`).
- Wire `test:coverage` into CI as a **required status check** so it blocks merges.

## Beyond the number

- Every `if/else`, `switch`, and error branch on a critical path is exercised.
- Critical modules (auth, payments, data mutations): >=1 **adversarial** test each
  (malformed input, boundary, unauthorized, concurrent), not just happy path.
- New code must not lower the project's coverage.

## Mutation testing (advisory, recommended for critical modules)

High line-coverage with weak assertions still passes when logic is wrong. Run a
mutation tester (Stryker for JS/TS, mutmut for Python) on critical modules: each
mutation (flipped condition, removed line) should make at least one test fail.

## Anti-patterns (NEVER)

- Padding coverage with tests that assert nothing.
- Excluding real source from coverage to clear the gate.
- Lowering the threshold to make a red build green.
