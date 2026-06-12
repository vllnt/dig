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

## MCP tools (`dig mcp`)

`dig_find`, `dig_drift`, `dig_log`, `dig_export` (read) and `dig_org`,
`dig_reconcile` (preview unless `apply:true`), `dig_undo`. Each takes an optional
`kb` argument. See [architecture.md](architecture.md) §5.

## HTTP endpoints (`dig serve`, loopback only)

`GET /find /drift /log /export` and `POST /org /reconcile /undo`
(`?apply=true` to commit), plus `GET /health`. Query params mirror the CLI
flags (`kb`, `query`, `mode`, `limit`, `filter`, `at`). Official clients:
[`@vllnt/dig`](../clients/typescript) (npm) and
[`vllnt-dig`](../clients/python) (PyPI).

## Portable skill

The canonical instruction set for an agent is
[`skills/dig/SKILL.md`](../skills/dig/SKILL.md). Every harness shim points there
rather than restating the surface.
