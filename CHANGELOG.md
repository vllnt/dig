# Changelog

All notable changes to dig are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`dig retain [file]`** — the agent-memory capture primitive: writes content (a
  file argument, stdin, or a rendered session via `--transcript`) into the KB at a
  dated, content-addressed `memory/` path (`--as` to override, `--date` for
  reproducible captures), then scans + indexes it as a reversible changeset, so
  `dig find`/`dig recall` surface it. Path-escape guarded.
- **Session retention** — `dig retain --transcript <session.jsonl>` renders a Claude
  Code transcript to readable markdown (user + assistant turns, tool calls
  summarized; thinking, tool output, system reminders, and injected skill bodies
  dropped). A **SessionEnd plugin hook** (`hooks/retain-session.sh`) auto-captures
  finished sessions into `memory/sessions/` — double opt-in (`DIG_RETAIN_SESSIONS=1`
  **and** a `.dig` KB at the session's directory) and fail-open, so it can never
  block or break a session.
- **`dig recall <query>`** — the agent-memory recall primitive: a token-budgeted
  (`--budget`), provenance-tagged context pack ranked from the KB (text or
  `--json`), so an agent loads relevant memory without overflowing its context.
  Snippets land on the query-relevant **window** of each matched document (not its
  head), so recalling a long captured session returns the matching exchange.
- **Semantic + hybrid retrieval** — opt-in vector index behind a `[retrieval]` policy,
  embeddings via any OpenAI-compatible endpoint, stored in `.dig/vectors.db` as a derived
  view with a blob-keyed cache. `dig find --mode fts|vector|hybrid`; hybrid fuses FTS and
  vector rankings with Reciprocal Rank Fusion. FTS stays the deterministic default.
- **Background semantic indexing** — scans queue unseen blobs instantly; `dig embed` drains
  the backlog with per-file commits (interruptible, resumable) and `dig watch` drains it per
  tick. An unreachable endpoint degrades gracefully and never blocks the deterministic spine.
- **Multilingual / cross-lingual recall** — validated via config only (`model = "bge-m3"`):
  a query in one language retrieves documents written in another.
- **Benchmark eval harness** (`tools/eval`) — LongMemEval, LoCoMo, and BEAM adapters scoring
  retrieval through the real pipeline (recall@k, hit@k, NDCG@10, MRR). Full LongMemEval-S:
  hybrid hit@5 **98.0%** vs the published 96.6% bar. Scoreboard in `docs/evals.md`.
- **`dig mcp`** — run dig as a Model Context Protocol server over stdio, exposing
  the CLI surface (find, recall, drift, log, export read-only; retain to capture
  into memory; org/reconcile preview-by-default with an apply flag; undo) as tools
  any MCP client can drive. `dig_retain` + `dig_recall` make dig an agent's memory
  layer over MCP — capture a session, load a budgeted pack back, all reversible.
- **`dig serve`** — localhost HTTP+JSON daemon over the CLI contract (GET
  /find /drift /log /export, POST /org /reconcile /undo, apply-gated), so apps
  and SDKs embed dig without shelling out. Binds loopback only — never public.
- **`@vllnt/dig` TypeScript SDK** (`clients/typescript`) — dependency-free typed
  client over the daemon; CI builds + tests it against a real `dig serve`, and an
  npm-publish workflow ships it on release (gated on `NPM_TOKEN`).
- **`vllnt-dig` Python SDK** (`clients/python`) — stdlib-only client over the
  daemon, same surface; CI-tested against a real `dig serve`; a PyPI-publish
  workflow ships it on release (gated on `PYPI_TOKEN`).
- **Claude Code plugin** (`.claude-plugin/`) — `/plugin marketplace add vllnt/dig`
  then `/plugin install dig@dig` bundles the dig skill + the `dig mcp` server.
- **AI SDK tools** (`@vllnt/dig/ai`) — `digTools(client)` returns Vercel AI SDK
  `tool()` definitions for the dig surface, so an agent can search/organize a KB
  (mutations apply-gated, reversible). `ai` + `zod` are optional peer deps.
- **Configurable retrieval primitives** — `[retrieval]` policy gains `rrf_k`,
  `candidate_factor`, `chunk_size`, `chunk_overlap` tuning knobs (0 = default,
  reproducing shipped behavior); changing chunk size/overlap re-embeds the KB.
- **Event sinks** — `[[event_sink]]` policy entries fire on every committed
  changeset: `webhook` POSTs the event JSON; `exec` runs a command (off unless
  `DIG_ALLOW_EXEC_SINKS=1`). Sinks observe — a sink failure warns, never rolls
  back the commit.
- **`dig --version`** — build metadata (version, commit, date).
- **Release tooling** — GoReleaser cross-compiles checksummed binaries for
  linux/darwin/windows × amd64/arm64; a `vX.Y.Z` tag publishes a GitHub release.

### Changed

- `main` branch protection hardened (`enforce_admins`, required CI checks) ahead of going
  public.

[Unreleased]: https://github.com/vllnt/dig/commits/main
