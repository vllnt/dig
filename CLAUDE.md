# dig

Agent harness that keeps a knowledge base in order. Go CLI (`cmd/`, `internal/`) + landing page (`web/`).

## Layout

| Path | What |
|------|------|
| `cmd/`, `internal/` | Go CLI — the product |
| `docs/` | Architecture, extensions, landscape docs |
| `web/` | Landing page (dig.vllnt.com) — Next.js App Router on the @vllnt stack |

## Rules (web/)

- @web/.claude/rules/testing/policy.md
- @web/.claude/rules/testing/no-mocking.md
- @web/.claude/rules/testing/e2e.md
- @web/.claude/rules/testing/ui.md
- @web/.claude/rules/testing/coverage.md

## Dev server (web/)

- Port: 3977 (fixed — avoids collision with other local projects)
- Local: `pnpm -C web dev` → http://localhost:3977
- Expose (device/cross-machine/preview testing): ALWAYS `tailscale serve` — see @.claude/rules/dev-server.md
- NEVER `next dev -H 0.0.0.0`, ngrok, or a public tunnel for routine dev.

## Issues

- Issue intake = structured templates only, **blank issues OFF** (BLOCKING): @.claude/rules/issues.md
- Templates in `.github/ISSUE_TEMPLATE/`; security goes to private advisories, never a public issue.

## Release

- Release system + **canary-only policy** (BLOCKING): @.claude/rules/release.md
- One workflow per artifact: `npm.yml` / `pypi.yml` (canary + release, OIDC), `canary.yml` + `release.yml` (CLI). Runbook: `docs/RELEASING.md`.
- The repo is public but pre-1.0 → **canary mode only**. Never cut a stable `vX.Y.Z` tag or publish `latest`/non-dev without explicit maintainer approval. Provenance/attestations are ON now that the repo is public.
