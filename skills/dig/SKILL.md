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

`dig mcp` runs an MCP server over stdio exposing `dig_find`, `dig_drift`,
`dig_log`, `dig_export` (read), `dig_org`, `dig_reconcile` (preview by default;
`apply: true` commits), and `dig_undo`. Register it with the harness's MCP
config and call the tools directly.

## Rules

- Never invent file moves — let policy + `dig org`/`reconcile` decide; preview first.
- Prefer `--json` and parse it; don't scrape human output.
- Local-first: with no `[retrieval]`/`[llm]` endpoint configured, dig makes zero
  network calls.
- Benchmarks + method: https://dig.vllnt.com/leaderboard.
