---
name: dig
description: >-
  Drive a dig knowledge base from any agent harness — organize files to a
  policy, search them (deterministic FTS + opt-in semantic/hybrid), reconcile
  drift, and remember across sessions, all locally and reversibly. Use when the
  user wants to find/organize/dedupe/label/version/reconcile a directory of
  files, or wants a local, reversible memory/retrieval layer. Triggers: "dig",
  "organize my files", "find in my notes/KB", "dedupe", "reconcile drift",
  "knowledge base", "local memory for my agent".
---

# dig — the local, reversible KB primitive

dig is a Go CLI over a content-addressed store. Every change is journaled and
`undo`-able. It runs fully locally; AI is opt-in. Drive it the way a human does
— by running commands — or over MCP (`dig mcp`). This is the canonical
instruction set; harness-specific shims point here.

## Detect & install

```sh
dig --version || true            # present?  if not:
curl -fsSL https://dig.vllnt.com/install.sh | sh   # or: go install github.com/vllnt/dig/cmd/dig@latest
```

A KB lives in a directory with a `.dig/` folder. Resolve one with `--kb <path>`
or run inside it (dig walks up to find `.dig/`). Create one with `dig init <dir>`.

## The surface (read commands emit `--json`)

| Goal | Command |
|------|---------|
| Index a directory | `dig init <dir>` then `dig --kb <dir> scan` |
| Search (ranked) | `dig --kb <dir> find "<query>" --json` — add `--mode hybrid` for semantic recall |
| Recall budgeted memory | `dig --kb <dir> recall "<query>" --json` — token-budgeted (`--budget`), provenance-tagged context pack |
| Capture into memory | `dig --kb <dir> retain [file]` (or stdin, or `--transcript <session.jsonl>`) → dated `memory/` path (`--as`) |
| See divergence from policy | `dig --kb <dir> drift --json` |
| Reorganize by policy | `dig --kb <dir> org --dry-run` (preview) → `dig --kb <dir> org` (apply) |
| Collapse duplicates | `dig --kb <dir> dedup --dry-run` → `dig --kb <dir> dedup` |
| Converge to policy | `dig --kb <dir> reconcile` |
| Export a dataset | `dig --kb <dir> export --filter "label:finance" --json` |
| History | `dig --kb <dir> log --json` |
| Undo the last change | `dig --kb <dir> undo` |

**Always preview mutations with `--dry-run` first, then apply.** Anything applied
is reversible with `dig undo`. Read commands (`find`, `drift`, `log`, `export`)
never change state.

## Policy

Organization is declarative TOML at `.dig/policy.toml` (`[[rule]]` match → into/
rename/label; `[dedup]`; opt-in `[retrieval]` for semantic search). Validate with
`dig policy validate`. See https://dig.vllnt.com/docs.

## Via MCP (preferred for agents)

`dig mcp` runs an MCP server over stdio exposing `dig_find`, `dig_recall`,
`dig_drift`, `dig_log`, `dig_export` (read), `dig_retain` (capture into memory),
`dig_org`, `dig_reconcile` (preview by default; `apply: true` commits), and
`dig_undo`. Register it with the harness's MCP config and call the tools
directly. `dig_retain` + `dig_recall` make dig the agent's memory layer.

## Remember sessions (opt-in)

When installed as the Claude Code plugin, a `SessionEnd` hook renders each
finished session and `dig retain`s it into `memory/sessions/`. It is **double
opt-in** and fail-open: it captures only when `DIG_RETAIN_SESSIONS=1` is set
**and** the session's directory is inside a `.dig` KB, and it never blocks a
session. Recall a past session with `dig recall "<topic>"`.

## Rules

- Never invent file moves — let policy + `dig org`/`reconcile` decide; preview first.
- Prefer `--json` and parse it; don't scrape human output.
- Local-first: with no `[retrieval]`/`[llm]` endpoint configured, dig makes zero
  network calls.
- Benchmarks + method: https://dig.vllnt.com/leaderboard.
