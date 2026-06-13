# Releasing dig

dig ships two channels for three artifacts — the **Go CLI**, the **`@vllnt/dig`**
npm SDK, and the **`dig-client`** PyPI SDK:

| Artifact | Workflow | Canary (push to `main`) | Stable (`vX.Y.Z` tag) |
|----------|----------|-------------------------|------------------------|
| **npm** `@vllnt/dig` | `npm.yml` | `@vllnt/dig@canary` | `@vllnt/dig@latest` (release / dispatch) |
| **Go CLI** | `canary.yml` + `release.yml` | rolling `canary` prerelease | GoReleaser binaries |
| **PyPI** `dig-client` | `canary.yml` + `pypi-publish.yml` | `.devN` prerelease | stable on release |

npm canary **and** release live in one file (`npm.yml`), mirroring `@vllnt/ui`'s
`publish.yml`. The Go CLI and PyPI keep their own workflows.

The canary channel is the bleeding edge — a dress rehearsal of every release,
built from the exact commit on `main`. **Not for production.**

## Canary (automatic, on every push to `main`)

Each artifact's workflow runs a quality gate, then publishes a canary:

- **CLI** (`canary.yml`) — GoReleaser builds a snapshot (`release --snapshot
  --clean`, cross-compiled linux/darwin/windows × amd64/arm64). The artifacts are
  uploaded to a single **rolling `canary` GitHub prerelease** moved to the current
  commit each push. The binary self-identifies: `dig --version` →
  `X.Y.Z-canary.<short-sha>`.
- **npm** (`npm.yml` → `canary` job) — publishes
  `@vllnt/dig@{version}-canary.<short-sha>` under the `canary` dist-tag.
- **PyPI** (`canary.yml` → `pypi` job) — publishes `dig-client` as a PEP 440 dev
  release (`{version}.dev{run-number}`), which `pip` only installs with `--pre`.

Consume the latest canary:

```bash
# CLI — download from the rolling prerelease
#   https://github.com/vllnt/dig/releases/tag/canary
npm add @vllnt/dig@canary
pip install --pre dig-client
```

### One-time setup for the npm + PyPI canaries (OIDC)

The CLI canary needs no setup — `GITHUB_TOKEN` is sufficient and it runs on the
first merge. The npm and PyPI canaries publish via **OIDC trusted publishing**
(no long-lived token), so they are **gated on the `CANARY_ENABLED` repo
variable** and stay dormant until you:

0. **First npm publish (manual, once).** npm Trusted Publishing can only be
   configured on a package that already exists, so claim the name with a manual
   first publish: `clients/typescript/scripts/first-publish.sh` (needs
   `npm login`). It builds + tests against a real `dig serve` and publishes the
   first `@vllnt/dig@…-canary.<sha>` under the `canary` tag. (`dig-client` on
   PyPI works the same — its first upload can also be a manual `twine upload`.)
1. **npm** — on npmjs.org, add a *Trusted Publisher* for `@vllnt/dig`: GitHub
   Actions, repo `vllnt/dig`, workflow **`npm.yml`** (covers both the canary and
   the release jobs).
2. **PyPI** — on pypi.org, add a *Trusted Publisher* for `dig-client`: repo
   `vllnt/dig`, workflow `canary.yml`.
3. Set the repo variable: `gh variable set CANARY_ENABLED --body true`.

Until `CANARY_ENABLED` is `true`, the `npm` and `pypi` jobs are skipped (the
workflow stays green); the CLI canary publishes regardless.

## Stable release (tag-driven)

Stable releases stay on the existing tag pipeline — the canary channel does not
change it.

### 1. Prepare the version in a PR

- Bump the version: `clients/typescript/package.json`, `clients/python/pyproject.toml`,
  and (for visibility) the CLI is stamped from the git tag by GoReleaser.
- Move the matching `CHANGELOG.md` entries from `[Unreleased]` to a dated
  `[X.Y.Z]` section. Keep the package CHANGELOGs in sync.
- Merge the PR. The canary on `main` immediately publishes
  `…-canary.<sha>` / `@vllnt/dig@canary` — a dress rehearsal of the release.

### 2. Cut the tag

```bash
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

That triggers:

- **`release.yml`** → GoReleaser builds the cross-compiled binaries + checksums
  and creates the **GitHub Release** for `vX.Y.Z` (changelog from Conventional
  Commits).
- The published-release event triggers the **`release` job in `npm.yml`** (OIDC,
  publishes `@vllnt/dig@latest` — moving `latest` off any earlier canary) and
  **`pypi-publish.yml`** (publishes `dig-client`; still `PYPI_TOKEN`-gated,
  skips gracefully when unset). The npm release reuses the same `npm.yml` trusted
  publisher as the canary, so there is no token to manage.

## Versioning policy

[SemVer](https://semver.org). A change is **major** only if a shipped public
contract breaks: CLI command/flag/JSON output, an exported SDK symbol, the HTTP
endpoint shape, or the policy schema. Adding commands or endpoints is a **minor**
bump.

## Rollback

- **npm:** `npm deprecate @vllnt/dig@{bad} "reason"` then publish a patch. Avoid
  unpublishing (npm disallows it after 72h and it breaks downstream users).
- **PyPI:** yank the bad release (`pip` stops resolving it) and publish a patch.
  Do not delete.
- **CLI:** the rolling `canary` prerelease self-heals on the next push; for a bad
  stable tag, publish a higher patch tag — never force-move a `vX.Y.Z` tag.
