# Release system + canary policy (BLOCKING)

dig ships three artifacts — the Go CLI, the `@vllnt/dig` npm SDK, and the
`dig-client` PyPI SDK. Each has one workflow that does both channels.

## Workflows (one file per artifact)

| Artifact | Workflow | Canary (push to `main`) | Stable (`vX.Y.Z` tag) |
|----------|----------|-------------------------|------------------------|
| Go CLI | `canary.yml` (canary) + `release.yml` (stable) | rolling `canary` GitHub prerelease (GoReleaser snapshot) | GoReleaser binaries |
| npm `@vllnt/dig` | `npm.yml` | `@vllnt/dig@{version}-canary.<sha>` (tag `canary`) | `@vllnt/dig@{version}` (tag `latest`) |
| PyPI `dig-client` | `pypi.yml` | `dig-client {version}.dev<run>` | `dig-client {version}` |

- **npm.yml / pypi.yml** each have `quality` → `canary` (on push, gated on the
  `CANARY_ENABLED` repo variable) → `release` (on a published GitHub release or
  `workflow_dispatch`). One file, both channels — modeled on `@vllnt/ui`'s
  `publish.yml`.
- **Auth is OIDC trusted publishing** for npm + PyPI — one trusted publisher per
  package points at its workflow (`npm.yml` / `pypi.yml`) and covers both jobs.
  The CLI canary uses `GITHUB_TOKEN` only. **Exception:** `pypi.yml` auto-detects
  a `PYPI_TOKEN` secret and uses it (twine) when present — a *temporary bridge*
  while the PyPI OIDC pending publisher is unavailable (e.g. org validation
  pending). Prefer OIDC; remove the token once OIDC works. npm stays OIDC-only.
- Full runbook: `docs/RELEASING.md`.

## Canary mode is the default — stable is gated (BLOCKING)

The repo is **public but pre-1.0**. Until the maintainer explicitly says "cut
v1":

- **NEVER cut a stable `vX.Y.Z` git tag** or publish a stable / `latest` / non-dev
  release. That is the only trigger for the `release` jobs and GoReleaser; it is a
  deliberate, maintainer-approved action — never do it on your own.
- **Publish only canary/prerelease**: npm under the `canary` dist-tag, PyPI as
  `.devN`, the CLI as the rolling `canary` prerelease. Versions stay
  `X.Y.Z-canary.<sha>` / `X.Y.Z.dev<run>`.
- `npm i @vllnt/dig` / `pip install dig-client` (bare) are expected to resolve only
  once v1 ships; for now consumers use `@vllnt/dig@canary` / `pip install --pre`.

## Provenance / attestations are ON now that the repo is public

npm `--provenance` and PyPI attestations use sigstore, which **only supports
public source repos**. `vllnt/dig` is public, so:

- **npm.yml** publishes **with** `--provenance` (both OIDC jobs).
- **pypi.yml** sets **`attestations: true`** on its OIDC steps. The `PYPI_TOKEN`
  bridge steps keep `attestations: false` — a token can't mint attestations; only
  OIDC can.

If the repo is ever made private again, turn both back off (sigstore rejects
private repos). OIDC auth itself is unaffected by visibility.

## When v1 is approved (the only stable path)

1. Bump versions: `clients/typescript/package.json`,
   `clients/python/pyproject.toml`; move `CHANGELOG` `[Unreleased]` → `[X.Y.Z]`.
2. Merge that PR (the canary publishes a dress rehearsal).
3. `git tag -a vX.Y.Z -m vX.Y.Z && git push origin vX.Y.Z` → `release.yml`
   (GoReleaser) creates the GitHub Release, whose `published` event fires the
   `release` jobs in `npm.yml` + `pypi.yml` (npm `--tag latest` also moves
   `latest` off the canary).

## Never

- Cut a `vX.Y.Z` tag, or publish a stable/`latest`/non-dev version, without
  explicit maintainer approval.
- Add `--provenance` / `attestations: true` if the repo is ever made private
  again (sigstore rejects private repos).
- Add an `NPM_TOKEN` secret — npm is OIDC-only. (`PYPI_TOKEN` is allowed as a
  documented temporary bridge in `pypi.yml`; remove it once PyPI OIDC works.)
- Force-move or delete a `vX.Y.Z` tag. (The rolling `canary` tag is the only
  moving tag, and only `canary.yml` moves it.)
