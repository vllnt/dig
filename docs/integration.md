# Integration contract

How any agent harness, SDK, or framework drives dig. This is the base every
shim and adapter builds on — keep it stable.

## Three ways in (pick one)

| Path | Use when | Entry |
|------|----------|-------|
| **MCP** | an MCP-capable harness (Claude, Cursor, the AI SDK, …) | `dig mcp` (stdio JSON-RPC) |
| **HTTP daemon** | an app/SDK in any language | `dig serve` (loopback HTTP+JSON) |
| **Direct CLI** | shelling out / scripts / CI | `dig <command> --json` |

All three are the *same surface* — the MCP server and the daemon are thin
adapters that run the CLI in-process. There is no fourth, divergent API.

## Detect & install

```sh
dig --version            # present? prints "dig version X (commit …, built …)"
# install if missing:
curl -fsSL https://dig.vllnt.com/install.sh | sh   # or: go install github.com/vllnt/dig/cmd/dig@latest
```

A knowledge base is a directory containing a `.dig/` folder. Resolve one with
`--kb <name|path>` or run inside it (dig walks up to find `.dig/`). Create one
with `dig init <dir>`.

## The command surface

| Goal | Command | Side |
|------|---------|------|
| Search, ranked | `dig --kb K find "<q>" --json` (`--mode fts\|vector\|hybrid`, `--limit`) | read |
| Recall budgeted context | `dig --kb K recall "<q>" --json` (`--budget`, `--mode`) | read |
| Capture content | `dig --kb K retain [file]` (or stdin / `--transcript <s.jsonl>`, `--as`) | write |
| Divergence from policy | `dig --kb K drift --json` | read |
| History | `dig --kb K log --json` | read |
| Reproducible dataset | `dig --kb K export --filter "<sel>" --json` | read |
| Reorganize by policy | `dig --kb K org --dry-run` → `dig --kb K org` | write |
| Collapse duplicates | `dig --kb K dedup --dry-run` → `dig --kb K dedup` | write |
| Converge to policy | `dig --kb K reconcile` | write |
| Undo the last change | `dig --kb K undo` | write |

Rules of the contract:

- **Read commands emit `--json`**; parse it, don't scrape human text. Read
  commands never change state.
- **Preview mutations first** (`--dry-run` / the daemon's preview default), then
  apply. Everything applied is reversible with `undo`.
- **Exit code 0 = success**, non-zero = failure with a message on stderr.
- **Local-first**: with no `[retrieval]`/`[llm]` endpoint configured, dig makes
  zero network calls.

## Bootstrapping a KB

The table above is the steady-state surface; an agent setting a KB up also uses:

- **`dig init <dir>`** — create the KB (writes `.dig/`).
- **`dig scan`** — index files already on disk. `retain` only indexes what it
  captures, so existing content needs a `scan` first.
- **`dig embed`** — drain the semantic-index backlog when `[retrieval]` vector or
  hybrid mode is configured (`watch` also drains it per tick).
- **`dig watch`** — run continuously: observe edits, reconcile, surface escalations.

Parallel/isolated work (`dig work` views + `dig merge`) is for harnesses running
many writers on one KB; single-writer integrations don't need it.

## Errors

One error model, because there is one surface (the daemon and MCP server run the
same CLI in-process):

- **CLI** — non-zero exit code; the message is **plain text on stderr**, stdout
  stays empty. `--json` shapes *success* output only, never errors — branch on
  the exit code, don't parse stderr as JSON.
- **HTTP daemon (`dig serve`)** — a failed command is **HTTP 400** with body
  `{"error": "<message>"}`; a wrong method is **405** (`{"error": "use POST"}`).
  `GET /health` is the liveness probe. Success is the command's raw JSON, or
  `{"output": "<text>"}` for commands that emit plain text.
- **MCP (`dig mcp`)** — a failing tool returns a JSON-RPC error; an unknown tool
  name is rejected.

## Recall & capture (memory as a consequence)

Because dig already holds and ranks the KB, it doubles as an agent's recall
layer — capture and recall, reachable through every path above (dig serves the
context; the agent answers):

- **`retain`** captures content — a note, a document, or a rendered agent
  session (`--transcript <session.jsonl>` turns a Claude Code transcript into
  readable markdown) — to a dated `memory/` path, indexed and reversible.
- **`recall`** returns a token-budgeted, provenance-tagged context pack whose
  snippets land on the query-relevant passage, so an agent loads what it knows
  about a topic without overflowing its window.

The Claude Code plugin ships a `SessionEnd` hook that auto-captures finished
sessions (double opt-in: `DIG_RETAIN_SESSIONS=1` and a `.dig` KB at the session
directory). See [architecture.md](architecture.md) §5.

## MCP tools (`dig mcp`)

`dig_find`, `dig_recall`, `dig_drift`, `dig_log`, `dig_export` (read),
`dig_retain` (capture into memory), and `dig_org`, `dig_reconcile` (preview
unless `apply:true`), `dig_undo`. Each takes an optional `kb` argument. See
[architecture.md](architecture.md) §5.

## HTTP endpoints (`dig serve`, loopback only)

`GET /find /recall /drift /log /export` and `POST /retain /org /reconcile /undo`
(`?apply=true` to commit), plus `GET /health`. Query params mirror the CLI
flags (`kb`, `query`, `mode`, `limit`, `budget`, `as`, `filter`, `at`); `/retain`
takes the content as the request body. Official clients:
[`@vllnt/dig`](../clients/typescript) (npm, with `recall()`/`retain()` and AI SDK
`dig_recall`/`dig_retain` tools) and [`dig-client`](../clients/python) (PyPI).

## Portable skill

The canonical instruction set for an agent is
[`skills/dig/SKILL.md`](../skills/dig/SKILL.md). Every harness shim points there
rather than restating the surface.
