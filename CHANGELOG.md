# Changelog

All notable changes to dig are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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
  the CLI surface (find, drift, log, export read-only; org/reconcile preview-by-
  default with an apply flag; undo) as tools any MCP client can drive.
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
