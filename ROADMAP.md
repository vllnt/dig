# Roadmap — dig

> The open, local, reversible **data + retrieval + memory primitive for AI agents** — organize a knowledge base to *your* mental model, retrieve it fast, remember across sessions, and plug it into any agent or framework (MCP first, then native SDKs). Configurable and extensible at every stage, bring-your-own model (PARA, GTD, Memory Palace, or your own). Your own system end-to-end — and the data layer others build on.

**Now:** eval-harness (CI eval loop · corpus generator). semantic-retrieval + public-release + site-launch DONE; BEAM larger-tier scores backfilling in background.
**Last updated:** 2026-06-12

## vision-docs [DONE 2026-05]

**Goal:** Project vision, architecture, competitive landscape, and extension design captured as versioned docs in a configured repo.
**Exit criteria:** README + 3 design docs internally consistent, repo private with protection + squash-only, CI link-check green.

- [x] vision-docs.1 Write README (concept, scope, commands, policy/workflows, stack, roadmap)
- [x] vision-docs.2 Write docs/architecture.md (content store, reconcile loop, concurrency state machine, AI layer)
- [x] vision-docs.3 Write docs/landscape.md (6-camp prior art + extensibility models)
- [x] vision-docs.4 Write docs/extensions.md (8 typed seams × 4 transport tiers, capability model)
- [x] vision-docs.5 Create private repo bntvllnt/dig — topics, squash-only, auto-delete, main protection
- [x] vision-docs.6 Add CI — docs link/anchor check active, Go pipeline dormant until go.mod (#1)
- [x] vision-docs.7 Document LLM endpoint = any OpenAI-compatible runtime or gateway (LiteLLM)

## foundation [DONE 2026-05]

**Goal:** The reversible spine — content-addressed store, search index, and core CLI — before any destructive feature.
**Exit criteria:** init/scan/find/log/undo work end-to-end on a real KB; undo restores prior manifest; tests pass with -race; CI green 4/4.

- [x] foundation.1 Content store — BLAKE3 blobs behind StorageBackend seam, bbolt manifests + journal
- [x] foundation.2 FTS5 index behind IndexBackend seam (pure-Go sqlite), rebuilt from manifests
- [x] foundation.3 KB resolution — per-KB .dig dir, --kb flag, walk-up discovery
- [x] foundation.4 Scan walker — hash, store, manifest entries, skip .dig
- [x] foundation.5 Cobra CLI — init, scan --dry-run, find --json, log, undo
- [x] foundation.6 Tests (12, -race) + golangci-lint v2 green + CI green on main

## organize [DONE 2026-06]

**Goal:** dig restructures a KB per declarative rules — the first destructive feature, fully reversible.
**Exit criteria:** TOML rules move/rename/label files; --dry-run previews the full plan; org → undo restores byte-identical state; unmatched files untouched and labeled unsorted.

- [x] organize.1 Policy schema — TOML [[rule]] match/into/rename/label, parser + validation
- [x] organize.2 Matchers — ext, mime, path glob, content_matches (plaintext), size/date
- [x] organize.3 Template engine — {year} {month} {name} {ext} from entry metadata
- [x] organize.4 Changeset builder — rules over head manifest → op list (move/rename/label)
- [x] organize.5 dig org --dry-run — render the full op plan, touch nothing
- [x] organize.6 dig org apply — atomic disk ops + journal commit + index rebuild
- [x] organize.7 dig policy validate — lint the policy file, explain rule matches
- [x] organize.8 E2E — org → undo byte-identical; unmatched → unsorted label; idempotent re-run

## dedupe [DONE 2026-06]

**Goal:** Duplicates are detected from the store and collapsed per policy — never silently deleted.
**Exit criteria:** dig dedup reports duplicate sets, collapses per strategy, escalates on conflict, and undo restores every copy.

- [x] dedupe.1 Duplicate-set detection — same blob, multiple paths, from head manifest
- [x] dedupe.2 [dedup] policy — strategy keep-oldest/keep-newest, mtime tie → escalate
- [x] dedupe.3 dig dedup --dry-run + apply (journaled changeset)
- [x] dedupe.4 Tests — never deletes the last copy; undo restores collapsed duplicates

## dataset-export [DONE 2026-06]

**Goal:** A KB slice exports as a reproducible, provenance-tagged dataset for ML training.
**Exit criteria:** dig export --at @M re-emits a byte-identical dataset months later; every row carries src blob + manifest id.

- [x] dataset-export.1 Filter language — label:, path glob, date ranges
- [x] dataset-export.2 JSONL emitter with per-row provenance (src, manifest)
- [x] dataset-export.3 --at manifest pinning + determinism test (same manifest → same bytes)

## drift-reconcile [DONE 2026-06]

**Goal:** dig measures desired-vs-actual drift and converges the KB one-shot, folding human edits in as a concurrent writer.
**Exit criteria:** dig drift reports misfiled/misnamed/duplicated/unsorted; reconcile converges per the coexistence contract; re-running reconcile on a converged KB is a no-op.

- [x] drift-reconcile.1 Scan-diff — disk vs last manifest → reconstructed human changeset (renames via blob identity; labels survive)
- [x] drift-reconcile.2 dig drift report — external edits + policy violations + pinned + unsorted + duplicates, --json
- [x] drift-reconcile.3 dig reconcile — one-shot converge; human moves pinned (dig:pinned) → standing escalation, never overwritten
- [x] drift-reconcile.4 Tests — human rename accepted as intent; violation flagged never overwritten; idempotency

## parallel-views [DONE 2026-06]

**Goal:** Multiple workers operate on isolated views and disjoint changesets merge back automatically.
**Exit criteria:** N concurrent workers on disjoint subtrees all merge clean with no lost ops; race detector green.

- [x] parallel-views.1 Work views — {base manifest, draft} fork as a pointer
- [x] parallel-views.2 Changeset state machine — explicit transition table (DRAFT→PROPOSED→STAGED→MERGED/CONFLICT/ESCALATED/ABORTED)
- [x] parallel-views.3 CAS commit + disjointness check in one serialized tx; overlap → CONFLICT, head untouched
- [x] parallel-views.4 dig work <create|list|abort> / dig merge CLI
- [x] parallel-views.5 Race tests — 8 concurrent workers, all merges land, no lost ops, history chain intact

## conflict-escalation [DONE 2026-06]

**Goal:** Overlapping changesets resolve by policy precedence or escalate surgically to a human.
**Exit criteria:** Escalation ladder holds — compatible ops union, precedence picks deterministically, unresolved conflicts lock only their subtree while the rest merges.

- [x] conflict-escalation.1 Compatible-op union — label union, blob-follow retarget, same-target noop, vacated-target apply
- [x] conflict-escalation.2 Policy precedence — Entry.Rule provenance + ViewOp.Rule; earlier rule wins deterministically, weaker drops
- [x] conflict-escalation.3 ESCALATED state + dig work resolve --mine|--theirs; escalations visible in work list
- [x] conflict-escalation.4 Partial merge — clean ops land, only conflicted remainder held (finance/ never blocks media/)

## watch-harness [DONE 2026-06]

**Goal:** dig runs continuously — observes edits, reconciles, escalates — with autonomy earned rule-by-rule.
**Exit criteria:** dig watch converges a live KB unattended; trusted rules auto-apply, the rest propose; escalation queue is surfaced and actionable.

- [x] watch-harness.1 Polling watch loop → reconcile per tick (quiet tick commits nothing; inotify trigger in Later)
- [x] watch-harness.2 Per-rule autonomy — autonomy = "auto" | "propose"; watch applies auto-only, one-shot = consent
- [x] watch-harness.3 dig watch [--interval] + escalation queue surfaced (ESCALATED views, pins, proposals, pending dups)
- [x] watch-harness.4 Soak test — files dropped mid-watch converge; manual-rule files propose and stay put; clean cancel

## dogfood-hardening [DONE 2026-06]

**Goal:** dig proves itself on a real messy KB and beats the prior art on every file-management function, with found friction fixed.
**Exit criteria:** Full lifecycle exercised hands-on; every prior-art command verified and matched/beaten or honestly split; friction issues closed; matrix in docs/landscape.md.

- [x] dogfood-hardening.1 Dogfood full lifecycle on a realistic KB + dig's own docs/ (5 frictions logged → #3 #4 #5)
- [x] dogfood-hardening.2 Verify all 14 prior-art file functions hands-on, same corpus; measured matrix in landscape.md (38bd24f)
- [x] dogfood-hardening.3 Index file content from blob store + AND→OR natural-query fallback (#3, 2966f8a)
- [x] dogfood-hardening.4 Watch surfaces standing items once; label ops render +label (#4)
- [x] dogfood-hardening.5 Label-only rules accumulate; placement rules stay first-wins (#5)

## semantic-retrieval [ACTIVE]

**Goal:** Opt-in semantic search closes the one retrieval gap that remained — paraphrase recall with zero shared terms — without touching the deterministic FTS default.
**Exit criteria:** Vector IndexBackend driver works against a local embedding endpoint; hybrid FTS+vector with reranking beats FTS-only on the eval set; dig scores published on LongMemEval and BEAM.

- [x] semantic-retrieval.1 Vector IndexBackend driver — opt-in `[retrieval]` policy, embeddings via the OpenAI-compatible endpoint, blob-keyed cache in .dig/vectors.db, FTS stays default (2026-06-11)
- [x] semantic-retrieval.2 Hybrid retrieval + reranking — FTS ∪ vector, RRF fusion, `dig find --mode fts|vector|hybrid`; hybrid beats FTS on LoCoMo (recall@5 85.3 vs 80.4) — model reranker stays optional/future (2026-06-11)
- [x] semantic-retrieval.3 Published LongMemEval score via eval-harness — full official 500-question set: hybrid hit@5 98.0% vs the published 96.6% bar (**beaten**), fully local; well clear of the prior art's structured/compressed scores too (docs/evals.md, 2026-06-12)
- [~] semantic-retrieval.4 Published BEAM score via eval-harness — **the keystone target**: unsaturated (64.1/48.6 best, LLM-judged QA), 1M–10M tokens where context-stuffing is impossible; the contested public frontier, where the agent-memory pivot (contradiction/temporal/reading) is proven. 128K-tier retrieval published (vector hit@10 64.8% vs FTS 58.3%, docs/evals.md, 2026-06-12); 500K tier running; 1M/10M + QA pipeline open
- [x] semantic-retrieval.5 Published LoCoMo score via eval-harness — scoreboard in docs/evals.md (2026-06-11)
- [x] semantic-retrieval.6 Multilingual recall — bge-m3 validated via config only: 5/5 cross-lingual queries (DE/ES/FR/EN, query lang ≠ doc lang) hit rank-1 through the real pipeline; gated live test (TestLiveMultilingualRecall) + model-selection guidance in architecture.md (2026-06-12)

## public-release [DONE 2026-06-12]

**Goal:** The repo goes public hardened, licensed, and installable.
**Exit criteria:** License committed, main protection hardened per #2, binaries published, oss-readiness gate passes.

- [x] public-release.1 LICENSE committed — MIT (user decision 2026-06)
- [x] public-release.2 Harden main — enforce_admins on, required CI checks (docs/go/lint), 0 approvals (solo, CI-gated); dig never in guard exempt; closed #2 (2026-06-12)
- [x] public-release.3 GoReleaser — cross-compiled checksummed binaries (linux/darwin/windows × amd64/arm64), release.yml on tag, `dig --version`, install docs; snapshot verified (#12, 2026-06-12)
- [x] public-release.4 oss-readiness audit — CONTRIBUTING/SECURITY/CODE_OF_CONDUCT/CHANGELOG/AGENTS, llms.txt + llms-full.txt, issue+PR templates; all BLOCKING+WARN gaps closed (#13, 2026-06-12)

## site-launch [DONE 2026-06-12]

**Goal:** dig.vllnt.com is live — the web app ships, users can install dig through a clear strategy, and user docs are published.
**Exit criteria:** Site resolves at dig.vllnt.com; a newcomer installs dig from it in under a minute via a documented channel; quickstart + policy reference + command docs published and synced with the CLI.

- [x] site-launch.1 Land the web app — merge worktree `dig-landing` (web/ Next.js), builds green (7b6600e, 22fbf92)
- [x] site-launch.2 Deploy to dig.vllnt.com — onboarded to ntk (prod + previews, 50659e6); site verified live (landing renders, full content) 2026-06-12
- [x] site-launch.3 Install strategy — installer at site root `/install.sh` (OS/arch detect, latest-release fetch, checksum verify); `/install` page with copyable curl + `go install` + binaries; homepage primary "Install dig" CTA; stale "early scaffold" copy refreshed. Homebrew tap tracked in distribution.5. Fed by GoReleaser artifacts (public-release.3 ✓) (2026-06-12)
- [x] site-launch.4 User docs — /docs page: quickstart, full command reference (15 commands), policy reference ([[rule]]/[dedup]/[retrieval]), synced from README + internal/cli + internal/policy; nav-linked, Playwright E2E + 3-viewport visual verified (2026-06-12)
- [x] site-launch.5 llms.txt + llms-full.txt published on the site for agent consumption (served at site root, 2026-06-12)
- [x] site-launch.6 Leaderboard page — /leaderboard renders LongMemEval/LoCoMo/BEAM scoreboards from docs/evals.md (hybrid hit@5 98.0% headline) + published baselines + method; nav-linked, server-rendered, Playwright E2E + 3-viewport visual verified (2026-06-12)
- [x] site-launch.7 Crawlability — robots.txt (declares sitemap) + sitemap.xml both resolve at site root (web app routes, verified in build 2026-06-12)

## eval-harness [PLANNED]

**Goal:** dig measures itself — one repeatable loop that dogfoods the full lifecycle on generated corpora and scores retrieval against the standard memory/IR benchmarks, continuously.
**Exit criteria:** One command runs the full loop (generate KB → lifecycle regression → benchmark scores → scoreboard diff); scores + cost pairs tracked in-repo; a score regression fails CI.

- [ ] eval-harness.1 Corpus generator — deterministic synthetic messy KBs (S/M/L; dupes, binaries, renames, nested chaos), seeded for reproducibility
- [ ] eval-harness.2 Lifecycle loop — automated full-journey regression (scan→find→org→dedup→drift→reconcile→watch→export→undo) over generated corpora, asserting the core invariants (byte-identical undo, idempotency, no lost ops)
- [~] eval-harness.3 Retrieval metrics core — recall@k, NDCG@10, MRR over labeled query sets; pairs reporting (score + latency + footprint) per the field standard (built in tools/eval, uncommitted — pulled forward to score semantic-retrieval)
- [~] eval-harness.4 Benchmark adapters — LongMemEval, LoCoMo, BEAM, MemoryAgentBench, MemBench ingestion (sessions → KB files) + official scoring (LongMemEval + LoCoMo done in tools/eval; BEAM + the two cognitive benchmarks pending — they uniquely test selective-forgetting, contradiction, test-time-learning, and capacity-under-growth, mapping to entity-graph + agent-memory)
- [~] eval-harness.5 FTS baseline scoreboard — pre-vector scores on every adapter, published in docs/evals.md (the bar semantic-retrieval must beat) (FTS/vector/hybrid scoreboard in docs/evals.md, uncommitted)
- [ ] eval-harness.6 CI eval loop — nightly + on-demand workflow; scoreboard diff posted; regression gates block

## harness-plugins [PLANNED]

**Goal:** dig ships skill-first — one portable dig skill is the canonical instruction set and each agent harness gets a thin shim that points at it, so any agent can manage a KB out of the box.
**Exit criteria:** The portable skill drives a KB (find/org/reconcile/export) unchanged across every listed harness; each harness shim is a thin pointer to it; one shared integration contract keeps surfaces consistent.

- [ ] harness-plugins.1 Integration contract — one doc: how a harness drives dig (--json surfaces, exit codes, dig detection/install), the base every shim builds on
- [ ] harness-plugins.8 Portable dig skill — skills/dig/SKILL.md, the canonical instruction set (when to reach for dig, --json surfaces, detect/install); every harness shim points here (codebase-intelligence model)
- [ ] harness-plugins.2 claude-code plugin — official `.claude-plugin/plugin.json` bundling `commands/` (dig-find/org/drift/export over the existing CLI) + `skills/dig` (hp.8) + the retention hook (agent-memory.1, SessionEnd → KB) + the MCP server (hp.7); v1 ships over the working CLI today, MCP/retention gated on those tasks. Paths via `${CLAUDE_PLUGIN_ROOT}`, component dirs at root
- [ ] harness-plugins.9 cursor shim — `.cursor/rules/dig.mdc` rule, thin auto-generated pointer (between markers) to the portable skill
- [ ] harness-plugins.3 pi shim — pi.dev package, thin pointer to the portable skill
- [ ] harness-plugins.4 codex shim — thin pointer to the portable skill
- [ ] harness-plugins.5 openclaw shim — thin pointer to the portable skill
- [ ] harness-plugins.6 hermes shim — thin pointer to the portable skill
- [ ] harness-plugins.7 dig MCP server (`dig mcp`) — THE keystone integration: expose find/org/drift/reconcile/export + retain/recall (agent-memory) as MCP tools over stdio + HTTP/SSE; one server reaches the whole MCP ecosystem (Claude/ChatGPT/Gemini/Cursor/VS Code/JetBrains + AI SDK/Mastra/LangChain). Highest-leverage task — build before per-framework adapters (see integrations phase)
- [ ] harness-plugins.10 Agent entry docs — AGENTS.md (cross-harness standard) + GEMINI.md beside the existing CLAUDE.md, each pointing at the portable skill
- [ ] harness-plugins.11 gemini-cli shim — thin pointer to the portable skill
- [ ] harness-plugins.12 antigravity shim — thin pointer to the portable skill
- [ ] harness-plugins.13 Plugin marketplace — `.claude-plugin/marketplace.json` in the repo so `/plugin marketplace add vllnt/dig` → `/plugin install dig@dig`; semver bump per release for updates (per the official Claude Code plugin spec)

## public-extensibility [PLANNED]

**Goal:** Third parties extend dig without forking via the 8 typed seams, and every built-in primitive is user-configurable without code — backup and store-elsewhere land first.
**Exit criteria:** A T0 event_sink backup fires on commit; dig-<name> PATH subcommands resolve; dig ext installs a manifest-described extension from git.

- [ ] public-extensibility.1 T0 declarative event sinks — exec/webhook on changeset.committed
- [ ] public-extensibility.2 T1 PATH subcommands — dig-<name> resolution + changeset-proposal contract
- [ ] public-extensibility.3 dig ext CLI — manifest, capabilities, install-from-git, enable per KB
- [ ] public-extensibility.4 T2 gRPC subprocess backend — first out-of-tree StorageBackend
- [ ] public-extensibility.5 T3 WASM (wazero) + signing — sandboxed untrusted extensions
- [ ] public-extensibility.6 Configurable primitives — expose the built-in retrieval pipeline knobs (mode, RRF k, chunk size/overlap, top-k, candidate pool, reranker) as `[retrieval]` policy config, so primitives are tuned without recompiling (the configure-without-code half of the seams)
- [ ] public-extensibility.7 Programmatic API — a stable Go SDK + an optional localhost HTTP daemon over the CLI/--json contract, so apps embed dig without shelling out; local-only, no hosted service (closes the SDK/REST gap without breaking local-first)

## mental-models [PLANNED]

**Goal:** dig is organization-model-agnostic — the user picks the mental model a KB is organized by (PARA, Johnny Decimal, Zettelkasten, Memory Palace) or brings their own, where prior tools lock every user into one model. A model is a policy preset (config on the core rules evaluator), not code — so it inherits dry-run, journal, and undo for free.
**Exit criteria:** `dig init --preset <name>` organizes a fresh KB by any built-in model; a user-authored preset loads from a path or git repo and validates; switching models is a reversible reconcile; every preset compiles to a deterministic folder/label layout the org engine enforces.

- [ ] mental-models.1 Preset format — a model is a named, portable bundle of [[rule]] policy + folder skeleton, authored as .toml (config, not an extension seam). GATE: must compile to a deterministic folder/label layout; pure mnemonics that don't map to a layout are out
- [ ] mental-models.2 Built-in models — PARA (Projects/Areas/Resources/Archives → into-rules), Johnny Decimal (numbered areas/categories → rename templates), Zettelkasten (flat atomic notes + {id}- prefix; filing layer only — backlinks are [[entity-graph]], not filing), Memory Palace (nested Wings/Rooms/Drawers container layout — the folder scheme, not the recall mnemonic), shipped under presets/, applied via `dig init --preset <name>`
- [ ] mental-models.3 Bring-your-own — load a preset from a path or git repo, validated against the policy schema on load (strict, like core policy)
- [ ] mental-models.4 Reversible switch — `dig preset apply <name>` reorganizes through the changeset spine; undo restores the prior structure byte-identical
- [ ] mental-models.5 Registry + authoring docs — `dig preset list`/inspect installed models; user guide for authoring a custom model

## remote-reach [PLANNED]

**Goal:** KBs live on remote storage and AI drivers plug in opt-in.
**Exit criteria:** A KB stores blobs in S3-compatible storage via gocloud; extraction falls back regex → PDF text → OCR → LLM; mode=off stays fully deterministic.

- [ ] remote-reach.1 gocloud.dev/blob StorageBackend (S3/GCS/Azure)
- [ ] remote-reach.2 SFTP StorageBackend (pkg/sftp)
- [ ] remote-reach.3 OpenAI-compatible LLM client — base_url + model, tools/json/off modes
- [ ] remote-reach.4 Extraction pipeline — PDF text layer (pure-Go) + tesseract OCR shell-out
- [ ] remote-reach.5 Opt-in extractor/classifier drivers wired into rules ({vendor} fields)

## entity-graph [PLANNED]

**Goal:** dig understands WHO and WHAT is in the KB — entities resolved across files, relations queryable — feeding labels, dedupe, and find.
**Exit criteria:** "ACME" == "ACME Corp" across invoices/notes/contracts; entity labels applied by policy; relations (file↔entity↔entity) queryable via the CLI.

- [ ] entity-graph.1 Entity extraction — extractor-pipeline stage (regex/LLM), entities stored as manifest metadata
- [ ] entity-graph.2 Entity resolution — same-entity detection across mentions (deterministic rules first, LLM judgment opt-in)
- [ ] entity-graph.3 Knowledge graph — entity/relation store derived from manifests, `dig entities` / graph-aware `find`
- [ ] entity-graph.4 Policy hooks — match/label/file by entity ({vendor} from resolved entities, not just regex)
- [ ] entity-graph.5 Temporal validity — entities/facts/relations carry validity windows; an update invalidates by end-date, never deletes; `find --at` answers point-in-time (the knowledge-updates + temporal axis)
- [ ] entity-graph.6 Contradiction detection — surface conflicting facts (same subject, incompatible values across sources) as standing escalations; never auto-resolve
- [ ] entity-graph.7 Ontology/schema-driven graph — declare entity + relation types, validate the KG against the schema, support typed queries (closes the ontology-graph gap)

## composable-pipeline [PLANNED]

**Goal:** dig's memory pipeline is an ordered chain of named stages — ingest→extract→chunk→enrich→embed→index→organize→consolidate→forget on the write path, understand→retrieve→fuse→rerank→read on the read path — where every stage is configurable by policy, the earned subset is swappable via a typed seam, and the reconcile/undo spine is never swappable so dry-run and undo cover every stage.
**Exit criteria:** retrieval runs as a configured `pipeline = [...]` of registered stages, not a hardcoded mode switch; each stage's params live in opaque per-stage policy tables that pass strict validation; the missing read-side stages (query-expansion, rerank, reading) exist and are benchmarked end-to-end; a third party registers a stage without forking.

- [ ] composable-pipeline.1 Stage registry + pipeline spine — replace the hardcoded retrieval Mode switch with a configured `pipeline = [stage, …]` over a stage registry; built-ins compiled in, order declared in policy
- [ ] composable-pipeline.2 Opaque per-stage params — `[stage.<name>]` tables decoded raw and validated by the owning stage, so config/preset/plugin params coexist with core strict-unknown-key rejection (each stage stays strict on its own keys)
- [ ] composable-pipeline.3 Query-expansion stage — query rewrite + time-aware window filtering on the read path (LongMemEval temporal lever, +6.8–11.3% on the temporal subset)
- [ ] composable-pipeline.4 Rerank stage — pluggable reranker (cross-encoder / model / MMR diversity) over the fused candidate pool, off by default (graduates semantic-retrieval.2's optional/future reranker)
- [ ] composable-pipeline.5 Reading/synthesis stage — opt-in LLM reader (chain-of-note + JSON context) turning retrieved items into an answer, scored as QA accuracy not just hit@k (LongMemEval +10 QA points); per the 2026-06-12 eval, dig's 98.0% is retrieval recall — reading is the path to the answer-quality (QA) number the field reports (~94.4)
- [ ] composable-pipeline.6 Design law in docs — record the three mechanisms (param config · preset · typed seam) and the rule: configure every stage, extend only the earned subset, never the spine; cross-ref docs/extensions.md
- [ ] composable-pipeline.7 Time-decay scoring — opt-in recency-weighted ranking stage (configurable half-life), off by default; newer memories rank higher
- [ ] composable-pipeline.8 Query sanitization — prompt-injection defense on the query-understand stage: strip/escape instruction-like spans before retrieval and reading
- [ ] composable-pipeline.9 Graph-traversal retrieval — add graph-walk as a first-class retrieval strategy fused with FTS ∪ vector (a multi-strategy retrieval model) over the entity-graph; cross-ref entity-graph.3 (closes the graph-retrieval gap)

## agent-memory [PLANNED]

**Goal:** dig is the user's own agent memory — it captures their agent sessions, serves token-budgeted recall to any harness, and syncs across devices, so they never reach for an external memory tool (the sovereignty bet — see the reversed non-goal in docs/landscape.md). Tasks ordered by dogfood value: capture + recall first.
**Exit criteria:** an opt-in hook retains Claude Code sessions into a KB; `dig recall <q>` returns a token-budgeted, provenance-tagged context bundle; the same memory is reachable from a second device and through MCP; dig's own daily agent work runs on it.

- [ ] agent-memory.1 Session retention — opt-in hooks capture agent conversations + tool traces into the KB as dated files (Claude Code first, then Codex/Cursor/Gemini); the user's sessions become their own searchable memory (session-capture hooks)
- [ ] agent-memory.2 Transcript split — segment captured sessions into turn/round units at index time so recall lands on the exact exchange (feeds composable-pipeline chunking)
- [ ] agent-memory.3 Recall context-pack — `dig recall <query>` emits a token-budgeted, importance-ranked L0/L1 context bundle, deterministic + provenance-tagged — the budgeted-recall primitive, built on the composable-pipeline reading stage
- [ ] agent-memory.4 Multi-device sync — the KB syncs across devices via a remote StorageBackend (gocloud/SFTP from remote-reach) + the changeset merge spine; same memory everywhere
- [ ] agent-memory.5 MCP memory tools — expose retain + recall as MCP tools so any harness uses dig as its memory layer (extends harness-plugins.7)
- [ ] agent-memory.6 Memory extraction + consolidation — opt-in: an LLM distills salient facts from retained sessions into labeled memory entries; updates ADD/UPDATE/MERGE with conflict → escalate (never silent), all as reversible changesets; raw sessions stay the source of truth — extraction is an additive index, not a replacement (THE architectural fork — closes the memory-extraction/consolidation gap)
- [ ] agent-memory.7 Tiered + agent-writable memory — memory tiers (hot/recall/archive) + an agent-writable surface (via MCP) where every edit is a reversible changeset, not free-form mutation (closes the self-editing/tiered-memory gap, dig-native)
- [ ] agent-memory.8 Memory namespacing — scope memory per agent/user/session within a KB (on the work-views isolation), so multiple agents share one dig without bleeding context (closes the multi-tenant memory gap)

## integrations [PLANNED]

**Goal:** dig is a drop-in data/memory backend for every agent framework and language — MCP first (one server → most of the ecosystem), then TS + Python SDKs over the local API, then native adapters implementing each framework's Retriever/Memory/VectorStore. dig stays the data layer, never the agent or the model.
**Exit criteria:** an agent built on the Vercel AI SDK, Mastra, and LangChain (Python) each uses dig as its memory/retriever — via MCP or a native adapter — with no dig-specific glue beyond install, all against the same KB.

- [ ] integrations.1 MCP-first reach — land `dig mcp` (harness-plugins.7) as the universal entry; verify it drives a KB from Claude + the Vercel AI SDK unchanged
- [ ] integrations.2 TypeScript/JS SDK — thin client over the localhost daemon (public-extensibility.7) + AI SDK `tool()` helpers; Node/Bun/Deno
- [ ] integrations.3 Python SDK — thin client over the daemon; the ML/agent default (LangChain/LlamaIndex/CrewAI build on it)
- [ ] integrations.4 Vercel AI SDK adapter — MCP wiring + memory middleware (recall-before / retain-after), published example
- [ ] integrations.5 Mastra adapter — dig as Memory + RAG store (MCP-native)
- [ ] integrations.6 LangChain / LangGraph adapter (py + js) — Retriever · VectorStore · BaseMemory · Tool
- [ ] integrations.7 LlamaIndex adapter (py + ts) — BaseRetriever · VectorStore
- [ ] integrations.8 JVM + .NET adapters — Spring AI / LangChain4j (EmbeddingStore · ContentRetriever · ChatMemory), Semantic Kernel
- [ ] integrations.9 Go SDK — native in-process package (dig is Go; nearly free)
- [ ] integrations.10 Neutral interface spec (GATED) — extract a transport-agnostic data/memory interface from the shipped adapters so any backend is swappable; the seed of an open standard — only after ≥2 framework adapters ship and a real adopter (the platform) runs on it (standards are earned, not declared)

## distribution [PLANNED]

**Goal:** dig is discoverable and one-command installable in every channel devs and agents already browse — package registries, the MCP registry, and framework directories — each listing pointing at the published 98% proof (site-launch.6). Passive reach: the ecosystem finds dig without us pushing. (Binary install lives in site-launch.3 / public-release.3; this phase is the agentic + SDK channels on top.)
**Exit criteria:** `npm i @dig/client`, `pip install dig`, and `brew install dig` all resolve; `dig mcp` is in the public MCP registry; dig appears in the LangChain / LlamaIndex / Vercel AI SDK directories; every entry links the benchmark leaderboard.

- [ ] distribution.1 SDK registries — publish the TS SDK (`@dig/client`, npm) + Python SDK (`dig`, PyPI) from the integrations phase, semver + CI-published; the front door for agent devs (npm/pip is how the agentic world installs, not curl|sh)
- [ ] distribution.2 MCP registry listing — submit `dig mcp` (harness-plugins.7) to the public MCP server registry → passive discovery by every MCP client, one listing reaches the ecosystem
- [ ] distribution.3 Framework directories — list dig in LangChain integrations · LlamaHub · Vercel AI SDK providers · Mastra registry (riding the integrations adapters), where devs browse for a memory/retriever backend
- [ ] distribution.4 Catalogs + awesome-lists — submit to awesome-mcp · awesome-ai-agents · awesome-ai-memory · awesome-selfhosted; each entry links the leaderboard (the trust hook that earns the click)
- [ ] distribution.5 Homebrew tap — `brew install dig` via a `vllnt/homebrew-dig` tap, formula fed by GoReleaser artifacts (public-release.3); the Mac/Linux CLI default alongside the curl installer (site-launch.3). The three front doors: `brew install dig` · `npm i @dig/client` · `pip install dig`

## Later

- Indexing throughput — the full LongMemEval index ran 20.7h CPU-only on 198MB (queries stay ms); batched/parallel/GPU or a faster embedder to cut cold-start indexing
- Workflows engine — [[workflow]] multi-step ingest procedures committing as one changeset
- Import-aware source-code reorganization (currently an explicit non-goal)
- dig query — DuckDB-style query-in-place over KB files
- Vision-model OCR fallback when tesseract absent
- Migrate branch protection to GitHub rulesets if collaborators join
