# Releasing dig

dig ships two channels for three artifacts — the **Go CLI**, the **`@vllnt/dig`**
npm SDK, and the **`dig-client`** PyPI SDK:

| Artifact | Workflow | Canary (push to `main`) | Stable (`vX.Y.Z` tag) |
|----------|----------|-------------------------|------------------------|
| **npm** `@vllnt/dig` | `npm.yml` | `@vllnt/dig@canary` | `@vllnt/dig@latest` (release / dispatch) |
| **Go CLI** | `canary.yml` + `release.yml` | rolling `canary` prerelease | GoReleaser binaries |
| **PyPI** `dig-client` | `pypi.yml` | `.devN` prerelease | stable on release |

npm and PyPI each keep canary **and** release in one file (`npm.yml` / `pypi.yml`),
mirroring `@vllnt/ui`'s `publish.yml`. The Go CLI canary is `canary.yml`; its
stable release is `release.yml` (GoReleaser, on a tag).

> **Provenance:** npm `--provenance` and PyPI attestations require a **public**
> source repo (sigstore). `vllnt/dig` is public, so `npm.yml` publishes with
> `--provenance` and `pypi.yml`'s OIDC steps set `attestations: true` (the
> token-bridge steps keep it false — a token can't attest). OIDC auth is unaffected.

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
- **PyPI** (`pypi.yml` → `canary` job) — publishes `dig-client` as a PEP 440 dev
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
2. **PyPI** — on pypi.org, add a *Trusted Publisher* (or a **pending publisher**,
   since `dig-client` isn't published yet) for project `dig-client`: repo
   `vllnt/dig`, workflow **`pypi.yml`**. No PyPI org/scope needed — just an
   account; the project name is claimed on first publish.
   - **Token bridge (while OIDC is unavailable).** If the pending publisher is
     blocked (e.g. PyPI org validation pending), set an account-scoped token
     instead: `gh secret set PYPI_TOKEN`. `pypi.yml` auto-detects it and uses
     twine; remove the secret once OIDC works. Trigger a canary publish on demand
     with `gh workflow run pypi.yml`. npm stays OIDC-only.
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
  publishes `@vllnt/dig@latest` — moving `latest` off any earlier canary) and the
  **`release` job in `pypi.yml`** (OIDC, publishes `dig-client`). Both reuse the
  same trusted publisher as their canary job — no tokens to manage.

## Homebrew (wired at go-public + v1)

Homebrew is **not** active yet — it needs a public repo and a stable release, so
it is a deliberate go-public step, not a canary channel. When ready:

1. **Create a public tap repo** `vllnt/homebrew-tap` (an empty public repo is
   enough; GoReleaser fills it).
2. **Add a `HOMEBREW_TAP_TOKEN` secret** — a fine-grained PAT with *Contents:
   write* on `vllnt/homebrew-tap` (the release's `GITHUB_TOKEN` can't push to a
   different repo). Wire it into `release.yml`'s env.
3. **Add the GoReleaser block** and choose the shape:
   - **Formula** (`brews:`) — works on macOS **and** Linux `brew`, but GoReleaser
     has **deprecated** `brews` (Homebrew is moving binary installs to casks).
   - **Cask** (`homebrew_casks:`) — not deprecated, but **macOS-only**.

   For a cross-platform CLI, a formula in a custom tap is still valid; pick based
   on Homebrew's deprecation state at v1. Use `skip_upload: auto` so it only
   pushes on a stable `vX.Y.Z` (never on the canary/snapshot).
4. Cut the first `vX.Y.Z` → GoReleaser builds the binaries and pushes the formula
   so `brew install vllnt/tap/dig` resolves.

Blocked until: the repo (or at least its release assets) is **public** — `brew`
downloads the release tarballs — and a **stable release** exists (canary-only
policy gates the tag; see `.claude/rules/release.md`).

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
