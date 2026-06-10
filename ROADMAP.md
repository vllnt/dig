# Roadmap — dig

> Open, local, CLI-only agent harness that keeps knowledge bases organized to your policy — reversible, parallel-safe, extensible.

**Now:** dedupe
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

## dedupe [ACTIVE]

**Goal:** Duplicates are detected from the store and collapsed per policy — never silently deleted.
**Exit criteria:** dig dedup reports duplicate sets, collapses per strategy, escalates on conflict, and undo restores every copy.

- [ ] dedupe.1 Duplicate-set detection — same blob, multiple paths, from head manifest
- [ ] dedupe.2 [dedup] policy — strategy keep-oldest/newest/path-priority, on_conflict escalate
- [ ] dedupe.3 dig dedup --dry-run + apply (journaled changeset)
- [ ] dedupe.4 Tests — never deletes the last copy; undo restores collapsed duplicates

## dataset-export [PLANNED]

**Goal:** A KB slice exports as a reproducible, provenance-tagged dataset for ML training.
**Exit criteria:** dig export --at @M re-emits a byte-identical dataset months later; every row carries src blob + manifest id.

- [ ] dataset-export.1 Filter language — label:, path glob, date ranges
- [ ] dataset-export.2 JSONL emitter with per-row provenance (src, manifest)
- [ ] dataset-export.3 --at manifest pinning + determinism test (same manifest → same bytes)

## drift-reconcile [PLANNED]

**Goal:** dig measures desired-vs-actual drift and converges the KB one-shot, folding human edits in as a concurrent writer.
**Exit criteria:** dig drift reports misfiled/misnamed/duplicated/unsorted; reconcile converges per the coexistence contract; re-running reconcile on a converged KB is a no-op.

- [ ] drift-reconcile.1 Scan-diff — disk vs last manifest → reconstructed human changeset
- [ ] drift-reconcile.2 dig drift report — policy violations + unsorted + duplicates, --json
- [ ] drift-reconcile.3 dig reconcile — one-shot converge, auto/propose split per rule
- [ ] drift-reconcile.4 Tests — human rename accepted as intent; violation flagged never overwritten; idempotency

## parallel-views [PLANNED]

**Goal:** Multiple workers operate on isolated views and disjoint changesets merge back automatically.
**Exit criteria:** N concurrent workers on disjoint subtrees all merge clean with no lost ops; race detector green.

- [ ] parallel-views.1 Work views — {base manifest, draft} fork as a pointer
- [ ] parallel-views.2 Changeset state machine — explicit transition table (DRAFT→PROPOSED→STAGED→MERGED/ABORTED)
- [ ] parallel-views.3 CAS commit on manifest parent + 3-way merge for disjoint changesets
- [ ] parallel-views.4 dig work / dig merge CLI
- [ ] parallel-views.5 Property + race tests — N workers, correct final manifest, no torn writes

## conflict-escalation [PLANNED]

**Goal:** Overlapping changesets resolve by policy precedence or escalate surgically to a human.
**Exit criteria:** Escalation ladder holds — compatible ops union, precedence picks deterministically, unresolved conflicts lock only their subtree while the rest merges.

- [ ] conflict-escalation.1 Overlap detection + compatible-op union (e.g. label merges)
- [ ] conflict-escalation.2 Policy precedence resolution
- [ ] conflict-escalation.3 ESCALATED state + escalation queue + resolve/abort CLI
- [ ] conflict-escalation.4 Subtree-scoped locking + tests (conflict on finance/ never blocks media/)

## watch-harness [PLANNED]

**Goal:** dig runs continuously — observes edits, reconciles, escalates — with autonomy earned rule-by-rule.
**Exit criteria:** dig watch converges a live KB unattended; trusted rules auto-apply, the rest propose; escalation queue is surfaced and actionable.

- [ ] watch-harness.1 Filesystem watcher → incremental scan-diff
- [ ] watch-harness.2 Per-rule autonomy config — propose | auto
- [ ] watch-harness.3 dig watch long-running command + escalation queue surfacing
- [ ] watch-harness.4 Soak test — sustained human + agent edits, no divergence

## public-release [PLANNED]

**Goal:** The repo goes public hardened, licensed, and installable.
**Exit criteria:** License committed, main protection hardened per #2, binaries published, oss-readiness gate passes.

- [ ] public-release.1 Decide + commit LICENSE — Apache-2.0 vs AGPL-3.0
- [ ] public-release.2 Harden main — enforce_admins on, CI required checks, remove guard exemption (#2)
- [ ] public-release.3 GoReleaser — cross-compiled binaries, checksums, install docs
- [ ] public-release.4 oss-readiness audit — llms.txt, contributing, release notes

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
