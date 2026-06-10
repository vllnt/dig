# Roadmap — dig

> Open, local, CLI-only agent harness that keeps knowledge bases organized to your policy — reversible, parallel-safe, extensible.

**Now:** public-release
**Last updated:** 2026-06-10

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
- [ ] site-launch.3 Install strategy — curl installer script + Homebrew tap + `go install`, all fed by GoReleaser artifacts (needs public-release.3), install page on the site
- [ ] site-launch.4 User docs — quickstart, policy/rules/workflows reference, command reference, synced from README + docs/
- [ ] site-launch.5 llms.txt + llms-full.txt published on the site for agent consumption

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

## Later

- Workflows engine — [[workflow]] multi-step ingest procedures committing as one changeset
- Import-aware source-code reorganization (currently an explicit non-goal)
- dig query — DuckDB-style query-in-place over KB files
- Semantic / vector search as opt-in IndexBackend driver
- Vision-model OCR fallback when tesseract absent
- Migrate branch protection to GitHub rulesets if collaborators join
- Homebrew tap once binaries are public
