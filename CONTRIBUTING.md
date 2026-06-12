# Contributing to dig

Thanks for your interest. dig is an open, local, reversible data/retrieval/memory
primitive for AI agents — a Go CLI over a content-addressed store. This guide covers
how to build it, the conventions we hold, and how to land a change.

## Project layout

| Path | What |
|------|------|
| `cmd/dig` | CLI entry point (thin — all logic in `internal/`) |
| `internal/` | The product: store, index, vector, retrieval, policy, organize, drift, watch, cli |
| `tools/eval` | Benchmark harness (LongMemEval, LoCoMo, BEAM) — dev tooling, not shipped |
| `docs/` | Architecture, extensions, landscape, evals |
| `web/` | Landing page (dig.vllnt.com) — separate Next.js app |

## Build & test

```bash
go build ./...                 # build everything
go test ./...                  # run the suite
go test -race ./...            # the bar for merge — must be green
golangci-lint run ./...        # lint — must be 0 issues
gofmt -l cmd internal tools    # must print nothing
```

The full gate before any change is "done": `gofmt` → `go vet` → `go build` →
`go test -race` → `golangci-lint`. CI enforces all of it on every PR.

## Conventions

- **Reversibility is the spine.** Every state mutation goes through the changeset →
  journal path so `dig undo` can reverse it. Never write to disk outside that path.
- **Determinism by default.** The core (hash, match, move, merge, FTS) is deterministic
  and offline. AI is an opt-in layer that only ever *proposes* through the same spine.
- **Derived views, never sources of truth.** The FTS and vector indexes are rebuilt from
  manifests; they never originate data.
- **Behavior tests, real instances.** Test what a caller observes. No mocking of code we
  own — use real temp dirs, a real local DB, a real embedding endpoint (or the in-repo
  fake at the one true network boundary). See `web/.claude/rules/testing/` for the full
  policy; the Go side follows the same shape.
- **Surgical changes.** Every changed line traces to the change's purpose. No drive-by
  refactors in a feature PR.

## Workflow

1. Branch from `main` (`feat/...`, `fix/...`). `main` is protected — no direct pushes.
2. Make the change with tests. New behavior = new test; bug fix = regression test that
   fails before, passes after.
3. Run the full gate locally.
4. Open a PR with a conventional-commit title (`feat(scope): …`, `fix(scope): …`).
   CI (build · vet · test · lint · docs) must pass — it's a required check.
5. Squash-merge once green.

## Commit messages

Conventional Commits. The scope is the package or area (`retrieval`, `eval`, `release`,
`policy`, …). The body explains *why*, not just *what*.

## Reporting bugs / proposing features

Open an issue using the templates in `.github/ISSUE_TEMPLATE/`. For a bug, include the
exact command, the `dig` version (`dig --version`), and what you expected vs. saw.

## Security

Do not file security issues publicly — see [SECURITY.md](SECURITY.md).
