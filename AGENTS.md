# AGENTS.md

Canonical instructions for AI agents working in this repository. Harness-specific files
(`CLAUDE.md`, `GEMINI.md`, …) may mirror or point here.

## What this repo is

dig — an open, local, reversible **data + retrieval + memory primitive for AI agents**.
A Go CLI over a content-addressed store: organize a knowledge base to a declarative policy,
retrieve it (FTS + opt-in semantic/hybrid), and reconcile drift — all reversibly.

## Layout

| Path | What |
|------|------|
| `cmd/dig` | CLI entry point — thin; all logic lives in `internal/` |
| `internal/store` | Content-addressed store: BLAKE3 blobs, bbolt manifests + journal |
| `internal/index` | FTS5 search index (pure-Go sqlite), rebuilt from manifests |
| `internal/vector`, `internal/retrieval` | Opt-in vector index + hybrid (RRF) retrieval |
| `internal/policy` | Declarative TOML policy: match/organize/dedup/retrieval rules |
| `internal/organize`, `internal/drift`, `internal/watch` | Restructure, reconcile, continuous harness |
| `internal/cli` | Cobra command tree |
| `tools/eval` | Benchmark harness (dev tooling, not shipped) |
| `docs/` | Architecture, extensions, landscape, evals |
| `web/` | Landing page (separate Next.js app — see its own rules) |

## Build & verify (the gate)

```bash
go build ./...
go vet ./...
go test -race ./...            # must be green
golangci-lint run ./...        # must be 0 issues
gofmt -l cmd internal tools    # must be empty
```

Run the full gate before declaring any change done. CI enforces it on every PR.

## Non-negotiable invariants

- **Reversibility** — every disk mutation goes through the changeset → journal path so
  `dig undo` reverses it. Never write outside that path.
- **Determinism by default** — the core is deterministic and offline; AI is opt-in and only
  ever *proposes* through the same spine. `mode = off` or a localhost endpoint = zero
  network calls.
- **Derived views** — FTS and vector indexes are rebuilt from manifests; never a source of
  truth. A moved/renamed file re-embeds nothing (content addressing).
- **Behavior tests, no self-mocking** — real temp dirs / DB / embedding endpoint (or the
  in-repo fake at the one external boundary). Test outcomes, not internals.
- **Surgical diffs** — every changed line traces to the task; no drive-by refactors.

## Workflow

Branch from `main` (protected) → change with tests → full gate → PR with a conventional-commit
title → squash-merge on green. Bug fix ⇒ a regression test that fails before the fix.

## Roadmap

`ROADMAP.md` tracks phases by outcome slug with stable `slug.N` task IDs. Work the active
phase in order.
