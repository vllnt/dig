# Roadmap — dig

> Open, local, CLI-only agent harness that keeps knowledge bases organized to your policy — reversible, parallel-safe, extensible.

**Now:** semantic-retrieval (pulled ahead of queue — user decision 2026-06-11); public-release next
**Last updated:** 2026-06-11

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

**Goal:** dig proves itself on a real messy KB and beats MemPalace on every file-management function, with found friction fixed.
**Exit criteria:** Full lifecycle exercised hands-on; every MemPalace command verified and matched/beaten or honestly split; friction issues closed; matrix in docs/landscape.md.

- [x] dogfood-hardening.1 Dogfood full lifecycle on a realistic KB + dig's own docs/ (5 frictions logged → #3 #4 #5)
- [x] dogfood-hardening.2 Verify all 14 MemPalace functions hands-on, same corpus; measured matrix in landscape.md (38bd24f)
- [x] dogfood-hardening.3 Index file content from blob store + AND→OR natural-query fallback (#3, 2966f8a)
- [x] dogfood-hardening.4 Watch surfaces standing items once; label ops render +label (#4)
- [x] dogfood-hardening.5 Label-only rules accumulate; placement rules stay first-wins (#5)

## public-release [ACTIVE]

**Goal:** The repo goes public hardened, licensed, and installable.
**Exit criteria:** License committed, main protection hardened per #2, binaries published, oss-readiness gate passes.

- [x] public-release.1 LICENSE committed — MIT (user decision 2026-06)
- [ ] public-release.2 Harden main — enforce_admins on, CI required checks, remove guard exemption (#2)
- [ ] public-release.3 GoReleaser — cross-compiled binaries, checksums, install docs
- [ ] public-release.4 oss-readiness audit — llms.txt, contributing, release notes

## site-launch [PLANNED]

**Goal:** dig.vllnt.com is live — the web app ships, users can install dig through a clear strategy, and user docs are published.
**Exit criteria:** Site resolves at dig.vllnt.com; a newcomer installs dig from it in under a minute via a documented channel; quickstart + policy reference + command docs published and synced with the CLI.

- [ ] site-launch.1 Land the web app — merge worktree `dig-landing` (web/ Next.js), builds green
- [ ] site-launch.2 Deploy to dig.vllnt.com — hosting + DNS + CI deploy on main
- [ ] site-launch.3 Install strategy — primary: `curl -fsSL https://dig.vllnt.com/install.sh | sh` (installer served at site root `/install.sh`); secondary: Homebrew tap + `go install`; all fed by GoReleaser artifacts (needs public-release.3); install page + homepage install CTA on the site
- [ ] site-launch.4 User docs — quickstart, policy/rules/workflows reference, command reference, synced from README + docs/
- [ ] site-launch.5 llms.txt + llms-full.txt published on the site for agent consumption
- [ ] site-launch.6 Leaderboard page — benchmark scores (LongMemEval, LoCoMo, BEAM + cost pairs) rendered from eval-harness's docs/evals.md scoreboard, vs MemPalace/mem0/ByteRover published numbers
- [ ] site-launch.7 Crawlability — robots.txt + sitemap.xml (sitemap URL declared in robots.txt); both resolve at site root

## eval-harness [PLANNED]

**Goal:** dig measures itself — one repeatable loop that dogfoods the full lifecycle on generated corpora and scores retrieval against the standard memory/IR benchmarks, continuously.
**Exit criteria:** One command runs the full loop (generate KB → lifecycle regression → benchmark scores → scoreboard diff); scores + cost pairs tracked in-repo; a score regression fails CI.

- [ ] eval-harness.1 Corpus generator — deterministic synthetic messy KBs (S/M/L; dupes, binaries, renames, nested chaos), seeded for reproducibility
- [ ] eval-harness.2 Lifecycle loop — automated full-journey regression (scan→find→org→dedup→drift→reconcile→watch→export→undo) over generated corpora, asserting the core invariants (byte-identical undo, idempotency, no lost ops)
- [ ] eval-harness.3 Retrieval metrics core — recall@k, NDCG@10, MRR over labeled query sets; pairs reporting (score + latency + footprint) per the field standard
- [ ] eval-harness.4 Benchmark adapters — LongMemEval, LoCoMo, BEAM ingestion (sessions → KB files) + official scoring
- [ ] eval-harness.5 FTS baseline scoreboard — pre-vector scores on every adapter, published in docs/evals.md (the bar semantic-retrieval must beat)
- [ ] eval-harness.6 CI eval loop — nightly + on-demand workflow; scoreboard diff posted; regression gates block

## harness-plugins [PLANNED]

**Goal:** dig ships skill-first — one portable dig skill is the canonical instruction set and each agent harness gets a thin shim that points at it, so any agent can manage a KB out of the box.
**Exit criteria:** The portable skill drives a KB (find/org/reconcile/export) unchanged across every listed harness; each harness shim is a thin pointer to it; one shared integration contract keeps surfaces consistent.

- [ ] harness-plugins.1 Integration contract — one doc: how a harness drives dig (--json surfaces, exit codes, dig detection/install), the base every shim builds on
- [ ] harness-plugins.8 Portable dig skill — skills/dig/SKILL.md, the canonical instruction set (when to reach for dig, --json surfaces, detect/install); every harness shim points here (codebase-intelligence model)
- [ ] harness-plugins.2 claude-code shim — `.claude` skill + slash commands, thin pointer to the portable skill (hp.8)
- [ ] harness-plugins.9 cursor shim — `.cursor/rules/dig.mdc` rule, thin auto-generated pointer (between markers) to the portable skill
- [ ] harness-plugins.3 pi shim — pi.dev package, thin pointer to the portable skill
- [ ] harness-plugins.4 codex shim — thin pointer to the portable skill
- [ ] harness-plugins.5 openclaw shim — thin pointer to the portable skill
- [ ] harness-plugins.6 hermes shim — thin pointer to the portable skill
- [ ] harness-plugins.7 MCP server — expose the CLI surface (find/org/drift/reconcile/export) as MCP tools, thin wrapper over the integration contract
- [ ] harness-plugins.10 Agent entry docs — AGENTS.md (cross-harness standard) + GEMINI.md beside the existing CLAUDE.md, each pointing at the portable skill

## public-extensibility [PLANNED]

**Goal:** Third parties extend dig without forking — backup and store-elsewhere land first.
**Exit criteria:** A T0 event_sink backup fires on commit; dig-<name> PATH subcommands resolve; dig ext installs a manifest-described extension from git.

- [ ] public-extensibility.1 T0 declarative event sinks — exec/webhook on changeset.committed
- [ ] public-extensibility.2 T1 PATH subcommands — dig-<name> resolution + changeset-proposal contract
- [ ] public-extensibility.3 dig ext CLI — manifest, capabilities, install-from-git, enable per KB
- [ ] public-extensibility.4 T2 gRPC subprocess backend — first out-of-tree StorageBackend
- [ ] public-extensibility.5 T3 WASM (wazero) + signing — sandboxed untrusted extensions

## remote-reach [PLANNED]

**Goal:** KBs live on remote storage and AI drivers plug in opt-in.
**Exit criteria:** A KB stores blobs in S3-compatible storage via gocloud; extraction falls back regex → PDF text → OCR → LLM; mode=off stays fully deterministic.

- [ ] remote-reach.1 gocloud.dev/blob StorageBackend (S3/GCS/Azure)
- [ ] remote-reach.2 SFTP StorageBackend (pkg/sftp)
- [ ] remote-reach.3 OpenAI-compatible LLM client — base_url + model, tools/json/off modes
- [ ] remote-reach.4 Extraction pipeline — PDF text layer (pure-Go) + tesseract OCR shell-out
- [ ] remote-reach.5 Opt-in extractor/classifier drivers wired into rules ({vendor} fields)

## semantic-retrieval [ACTIVE]

**Goal:** Opt-in semantic search closes the one gap MemPalace kept — paraphrase recall with zero shared terms — without touching the deterministic FTS default.
**Exit criteria:** Vector IndexBackend driver works against a local embedding endpoint; hybrid FTS+vector with reranking beats FTS-only on the eval set; dig scores published on LongMemEval and BEAM.

- [x] semantic-retrieval.1 Vector IndexBackend driver — opt-in `[retrieval]` policy, embeddings via the OpenAI-compatible endpoint, blob-keyed cache in .dig/vectors.db, FTS stays default (2026-06-11)
- [x] semantic-retrieval.2 Hybrid retrieval + reranking — FTS ∪ vector, RRF fusion, `dig find --mode fts|vector|hybrid`; hybrid beats FTS on LoCoMo (recall@5 85.3 vs 80.4) — model reranker stays optional/future (2026-06-11)
- [x] semantic-retrieval.3 Published LongMemEval score via eval-harness — full 500-question set: hybrid hit@5 98.0% BEATS MemPalace's 96.6% bar (+31.2pts over FTS baseline); scoreboard in docs/evals.md (2026-06-12)
- [ ] semantic-retrieval.4 Published BEAM score via eval-harness (unsaturated frontier — 64.1/48.6 are today's best)
- [x] semantic-retrieval.5 Published LoCoMo score via eval-harness — scoreboard in docs/evals.md (2026-06-11)

## entity-graph [PLANNED]

**Goal:** dig understands WHO and WHAT is in the KB — entities resolved across files, relations queryable — feeding labels, dedupe, and find.
**Exit criteria:** "ACME" == "ACME Corp" across invoices/notes/contracts; entity labels applied by policy; relations (file↔entity↔entity) queryable via the CLI.

- [ ] entity-graph.1 Entity extraction — extractor-pipeline stage (regex/LLM), entities stored as manifest metadata
- [ ] entity-graph.2 Entity resolution — same-entity detection across mentions (deterministic rules first, LLM judgment opt-in)
- [ ] entity-graph.3 Knowledge graph — entity/relation store derived from manifests, `dig entities` / graph-aware `find`
- [ ] entity-graph.4 Policy hooks — match/label/file by entity ({vendor} from resolved entities, not just regex)

## Later

- Policy presets — ready-made `policy.toml` templates + `dig init --template <name>` selector so a newcomer organizes by a known method instead of hand-writing TOML. v1 set: PARA (Projects/Areas/Resources/Archives → into-rules), Johnny Decimal (numbered areas/categories → rename templates), Zettelkasten (flat atomic notes + `{id}-` prefix; filing layer only — backlinks are [[entity-graph]], not filing), Palace (nested Wings/Rooms/Drawers container layout — the deterministic folder scheme, not the recall mnemonic). Gate: every preset must compile to a deterministic folder/label layout the org engine enforces; mnemonics that don't map to a layout are out.
- Workflows engine — [[workflow]] multi-step ingest procedures committing as one changeset
- Import-aware source-code reorganization (currently an explicit non-goal)
- dig query — DuckDB-style query-in-place over KB files
- Vision-model OCR fallback when tesseract absent
- Migrate branch protection to GitHub rulesets if collaborators join
- Homebrew tap once binaries are public
