# Architecture

dig is a thin CLI over **one content-addressed store** and a **policy engine** that proposes reversible changesets. Dedupe, versioning, isolation, merge, and drift-reconciliation are not separate subsystems — they are all operations on that store.

## Principles

- **Open source, runs fully locally.** No cloud account, no telemetry, no required network. The store, index, policy engine, dedupe, and merge are all on-device.
- **No LLM required.** The deterministic core does the whole job on its own. AI is an *optional* layer for fuzzy judgment, off by default.
- **The harness carries the intelligence, not the model.** dig exposes strong deterministic CLI tools; the model only makes small bounded judgments. This is why a **small local model is enough** — it routes and decides, it never computes structure.
- **Reversibility is the spine.** Every mutation is journaled and `undo`-able, including AI- and auto-applied ones.

---

## Deployment model — CLI-only, multi-KB, composable

```
   one machine
   registry: ~/.config/dig/registry.toml        (names → roots)
     ├─ KB "work"      /data/work/.dig/          store · journal · index · config
     ├─ KB "research"  /data/research/.dig/
     └─ KB "personal"  ~/notes/.dig/

   driver (you, or another harness):
     dig --kb work find "…" --json
     dig --kb work run ingest-contract
```

- **One binary, many KBs.** Each KB is self-contained: its store, journal, index, and `policy / rules / workflows / llm` config live in a `.dig/` directory at the KB root (git-style — config travels with the data). A machine-level registry only maps names → roots.
- **CLI is the sole interface.** No GUI, no importable SDK, no network API. `watch` is a long-running CLI command, not a separate service. Tiny, auditable surface.
- **Composable by design.** Other harnesses drive dig the way a human does — by running commands. Read commands emit `--json`; the command set splits read vs write; exit codes are stable. dig is a *tool other agents use*, not a framework they embed.
- **dig owns its LLM.** dig's internal LLM system (§5) is configured per-KB and independent of any caller's model. Two LLM layers can coexist — the outer agent and dig's own small-model judgments — without coupling.

---

## 1. The single core: a content-addressed store

```
   files on disk                content store
   ─────────────                ─────────────────────────────
   docs/a.pdf  ─hash─▶  blob  b3:9f2a…   (content, stored once)
   docs/b.pdf  ─hash─▶  blob  b3:9f2a…   (same hash → same blob)
   img/x.png   ─hash─▶  blob  b3:71c4…

   manifest  M3  (a versioned tree):
     finance/invoices/2024/acme-1007.pdf → b3:9f2a…
     media/photos/2024/05/x.png          → b3:71c4…
     parent: M2     created-by: work/cleanup
```

- **Blob** = file content, keyed by a content hash (BLAKE3). Stored once.
- **Manifest** = an immutable snapshot of the tree: `path → blob hash + metadata (labels, source, times)`, plus a `parent` pointer.
- **Journal** = the ordered list of manifests (the history).

**Disk is the source of truth; the store is a versioned shadow.** Humans edit the real files with their own tools and never "check in." `dig` observes the live tree, snapshots it into the store, and reconciles. The store records history and powers dedupe / isolation / merge — but it never gatekeeps the files or forces a workflow on people.

### Why four features fall out of one store

| Feature | Mechanism |
|---------|-----------|
| **Dedupe** | Two paths mapping to the same blob hash *are* the duplicate set. Detection is free; `dig dedup` just decides canonical + collapses per policy. |
| **Versioning / undo** | Every mutation writes a new manifest with `parent = previous head`. `dig log` walks parents; `dig undo` sets head back. Content is never deleted while referenced. |
| **Isolation (worktrees)** | A work view = `{base: M_n, draft manifest}`. Forking is O(1) — a pointer, no file copying. |
| **Merge** | A changeset = the diff `base → draft`. Merging is a 3-way reconcile of two changesets against their common base — git's algorithm over `path → hash` entries. |

restic (Go) proves the store; git proves the merge. dig adds the **policy engine** and the **reconcile loop**.

---

## 2. Policy engine

Declarative rules (TOML) compile to a function `file → proposed{path, name, labels}`. Running policy over the head manifest produces a **changeset** (a draft manifest + an op list), never a direct disk mutation. Disk is touched only at commit, atomically, after the journal entry is written.

Deterministic by default. Non-deterministic template fields (`{vendor}` from a PDF) are filled by **opt-in extractor drivers** (regex → OCR → LLM, in that order of preference). The core path stays deterministic and offline.

---

## 3. Reconcile loop (drift + human edits)

Policy is a **desired state**; the KB on disk is the **actual state**; **drift is the diff**. dig is a reconciler — the Kubernetes-controller pattern applied to files.

```
   policy (desired) ─┐
                      ├─▶ drift = 3-way diff ─▶ reconcile changeset ─▶ commit (journaled)
   disk  (actual)   ─┤   vs last manifest        (auto | propose | escalate)
                      │
   last manifest ─────┘   (what dig last knew)
```

Two kinds of change arrive between reconciles:

- **dig's own changesets** — tracked (went through the state machine in §4).
- **human direct edits** — untracked: a file appears, gets renamed in Finder, edited in Obsidian, deleted. dig discovers these by diffing current disk against the last manifest, then treats the human as a concurrent writer whose changeset it reconstructs after the fact.

| Disk vs last manifest | Reconcile action |
|---|---|
| new file, matches a rule | file it per policy (auto, journaled) |
| new file, no rule | index it, label `unsorted`, surface in `dig drift` |
| human renamed / moved a file | accept as intent; flag only if it now violates policy |
| human edit conflicts with a pending agent changeset | 3-way merge; escalate if unresolved |
| duplicate introduced | flag in `drift`; collapse per dedup policy on `reconcile` |

**Coexistence contract:**
- never silently override a *deliberate* human change — when human intent and policy disagree, **escalate, don't overwrite**.
- propose-by-default until trusted; `watch` mode earns autonomy rule-by-rule, not all at once.
- one-shot (`dig reconcile`) ships and is trusted **before** continuous (`dig watch`). Never the daemon first.

---

## 4. Concurrency: isolate → merge → escalate

**Target model: multiple autonomous workers** (independent agents) act on one library without corrupting it — so the full 3-way merge + policy-precedence + human-escalation path is required, not just a lock-guarded worker pool. Human direct edits (§3) feed the *same* machinery as another writer.

### Changeset state machine

```
 ┌───────┐  plan    ┌──────────┐ validate ┌────────┐
 │ DRAFT │─────────▶│ PROPOSED │─────────▶│ STAGED │
 └───────┘          └──────────┘          └───┬────┘
   worker        policy produces        dry-run OK,
   opens view    op list                 ready to commit
                                              │ commit vs head (CAS on parent)
                       ┌──────────────────────┼───────────────────────┐
                  disjoint │              overlap │                     │
                           ▼                      ▼                     │
                      ┌────────┐           ┌──────────┐                 │
                      │ MERGED │           │ CONFLICT │                 │
                      └────────┘           └────┬─────┘                 │
                    head advances     policy precedence resolve?         │
                                       ┌──────────┴───────────┐         │
                                  resolved │              unresolved│     │
                                           ▼                       ▼     │
                                      ┌────────┐          ┌────────────┐ │
                                      │ MERGED │          │ ESCALATED  │─┤ human decides
                                      └────────┘          └─────┬──────┘ │
                                                                │        │
                                                  resolve / abort         │
                                              ┌─────────────────┴─────────┘
                                              ▼
                                        ┌──────────┐
                                        │ ABORTED  │  (view discarded, head untouched)
                                        └──────────┘
```

| State | Data shape | Meaning |
|-------|------------|---------|
| DRAFT | `{base manifest, view id}` | worker has an isolated view |
| PROPOSED | `+ op list` | policy produced moves/renames/labels |
| STAGED | `+ draft manifest, dry-run report` | validated, ready to commit |
| MERGED | `head = draft` | committed; head advanced |
| CONFLICT | `+ overlapping ops vs current head` | another writer advanced head first |
| ESCALATED | `+ conflict diff` | policy can't resolve; waiting on human |
| ABORTED | `{}` | view discarded; head never touched |

### Transitions, guards, invalid moves

- `STAGED → MERGED` **guard:** `parent == current head` (CAS) AND ops disjoint from commits since base.
- `STAGED → CONFLICT` when head moved and ops overlap (same path touched).
- `CONFLICT → MERGED` **guard:** policy precedence yields a single deterministic resolution.
- `CONFLICT → ESCALATED` when no deterministic resolution exists.
- **Invalid (rejected):** `DRAFT → MERGED` (never commit unvalidated work); `MERGED → *` (manifests are immutable — a "change to merged" is a *new* changeset); committing on a stale base without re-running the CAS guard.

### Auto-escalation ladder

```
 1. disjoint paths .................. auto-merge (head advances)
 2. same path, compatible ops ....... auto-merge  (e.g. union of labels)
 3. same path, conflicting ops ...... policy precedence picks winner → auto
 4. still conflicting ............... ESCALATE to human
                                      • lock ONLY the conflicting subtree
                                      • the rest of both changesets still merges
```

Escalation is surgical: a conflict on `finance/` never blocks unrelated work on `media/`.

**Complexity:** 7 states, ~11 transitions, ≥3 guards → MEDIUM-HIGH. Implement as an explicit state enum + transition table, not ad-hoc booleans. Needs property-based and race tests (see §7).

---

## 5. The AI layer — optional, local, small-model-first

AI is a thin judgment layer over the deterministic tools, **off by default**. It never touches files directly; it can only *propose* through the same changeset → state-machine path everything else uses.

```
   model endpoint     ◀── OpenAI-compatible API ──▶  dig tool surface
   local runtime:                                     scan · find · hash ·
     Ollama / llama.cpp / LM Studio / vLLM            policy-match · classify-stub ·
   or gateway:                                        propose-changeset · diff · dedup-detect
     LiteLLM / OpenRouter → 100+ providers
        │  picks a tool, returns args
        ▼
   dig executes the tool deterministically, returns structured result, loops
```

- **OpenAI-compatible only.** One client, configurable `base_url` + `model`; no vendor SDK. The endpoint is either a **local runtime** (Ollama / llama.cpp / LM Studio / vLLM — the default, fully on-device) or a **gateway/proxy** (LiteLLM / OpenRouter) that fronts 100+ providers behind the same OpenAI API. Routing, fallback, and cost control live in the gateway — `dig` only ever sees one URL + one model name. A remote endpoint is just a different `base_url`.
- **Why small models suffice:** the hard parts (hashing, matching, moving, merging, journaling) are deterministic tools. The model's job is bounded and local: "does this doc match rule X?", "suggest a name from this text", "are these two notes the same topic?". Strong tools + narrow questions = a 7B-class model is enough.
- **Graceful degradation:** `mode = tools` (function calling) → `mode = json` (constrained JSON for models without tool-calling) → `mode = off` (pure deterministic, no AI). dig stays fully functional at every level.
- **Local-first guarantee:** with `mode = off` or a localhost endpoint, dig makes **zero external network calls**.

```toml
[llm]
mode     = "tools"                      # tools | json | off
base_url = "http://localhost:11434/v1"  # local runtime, or a LiteLLM/OpenRouter gateway URL
model    = "qwen2.5:7b"
api_key_env = "DIG_LLM_API_KEY"         # only for remote/gateway endpoints
```

### Semantic retrieval (shipped — the first AI-layer feature)

`find` is deterministic FTS by default. A `[retrieval]` policy turns on the **vector
index**: file text is chunked, embedded through the same OpenAI-compatible endpoint
contract, and stored in `.dig/vectors.db` as a derived view rebuilt from manifests —
exactly like the FTS index. **Hybrid** mode fuses both rankings with Reciprocal Rank
Fusion; `dig find --mode fts|vector|hybrid` overrides per query.

```toml
[retrieval]
mode     = "hybrid"                     # off (default) | hybrid | vector
base_url = "http://127.0.0.1:8092/v1"   # any OpenAI-compatible /embeddings endpoint
model    = "nomic-embed-text-v1.5"
doc_prefix   = "search_document: "      # model task prefixes (model-specific, optional)
query_prefix = "search_query: "
api_key_env  = "DIG_EMBED_API_KEY"      # only for remote/gateway endpoints

# tuning knobs — 0 / unset = the defaults shown, which reproduce shipped behavior
rrf_k            = 60                    # hybrid fusion constant
candidate_factor = 4                    # per-ranker pool = limit × factor
chunk_size       = 1000                 # document chunk length (chars)
chunk_overlap    = 200                  # overlap between chunks (chars); changing
                                        # chunk_size/overlap re-embeds the KB
```

- **Primitives are config, not code.** The retrieval pipeline's knobs (fusion
  constant, candidate pool, chunking) are `[retrieval]` policy fields — tune them
  without recompiling; unset means the default.
- **Indexing is background work.** A scan never blocks on the endpoint: it syncs
  the docs view instantly and queues unseen blobs; only a small budget embeds
  inline. `dig embed` drains the backlog explicitly (per-file commits —
  interruptible, resumable) and a running `dig watch` drains it every tick.
- **Embeddings are blob-keyed.** The content-addressed store makes incremental
  embedding free: a moved or renamed file re-embeds nothing; only new content costs.
- **Cache invalidation is explicit.** Model / prefix / chunking changes drop the
  vector cache wholesale — mixing embedding spaces would corrupt ranking silently.
- **Graceful degradation.** An unreachable endpoint never blocks the deterministic
  spine: `scan` warns and continues (FTS stays fresh), `find --mode fts` always works,
  and semantic modes fail loudly with the endpoint error.
- **Model selection is pure config** — `[retrieval] model` + prefixes, no code. Validated
  on llama.cpp (CPU): `nomic-embed-text-v1.5` (English, prefixes `search_document: `/
  `search_query: `), `all-MiniLM-L6-v2` (English, light, no prefixes), `bge-m3`
  (multilingual + cross-lingual — a German query finds an English note — no prefixes).
  Changing the model drops the vector cache (fingerprint) and re-embeds on the same
  background path. Beware third-party GGUF conversions of XLM-R-based embedders:
  broken pooling/tokenizer conversions produce uniform ~0.9 cosines; validate ranking
  on a handful of known pairs before trusting one (gpustack's bge-m3 GGUF is sound).
- Scores against the standard memory benchmarks live in [evals.md](evals.md).

### Memory falls out of it (capture + recall)

Because the same store + retrieval that organize files already hold and rank
everything, an agent's memory falls out for free — capture and recall, two
primitives on the deterministic spine, no new storage model. dig serves the
recall; the agent does the answering:

```
  capture                                   recall
  retain ─┬─ file / stdin                   recall "<q>" ──▶ rank (FTS/vector/hybrid)
          └─ --transcript <session.jsonl>                       │
                  │ render to markdown                          ▼
                  ▼ (user+assistant turns,                  best-window snippet per hit
            memory/<date>/<hash>.md            ◀──────────  (lands on the matching
                  │ scan + commit (reversible)               passage, not the head)
                  ▼                                            │
            FTS (+ vector) index  ───────────────────────────▶ token-budgeted,
                                                               provenance-tagged pack
```

- **`retain` is the one capture entry**, harness-agnostic: it writes content to a
  dated `memory/` path and commits it as a reversible *observe* changeset, so
  `dig undo` rewinds the index but never deletes the file (the same guarantee that
  makes undoing a `scan` safe). `--transcript` is an input adapter that renders a
  Claude Code session JSONL to readable markdown (turns kept; thinking, tool
  output, system reminders, and injected skill bodies dropped) — keeping the
  Claude-specific parsing out of the generic command.
- **`recall` is `find` plus budgeting**: it ranks the KB, then returns the
  query-relevant *window* of each hit (shared chunker + term-coverage scoring)
  capped to a token budget, pinned to the head manifest so a pack is reproducible.
- **Dogfooding is a `SessionEnd` hook** (the Claude Code plugin): it renders the
  finished session and `dig retain`s it, double opt-in (`DIG_RETAIN_SESSIONS=1`
  and a `.dig` KB at the session directory) and fail-open so it can never block a
  session. Sessions become their own searchable memory.
- **One surface, every path:** capture/recall are reachable as CLI commands, MCP
  tools (`dig_retain`/`dig_recall`), daemon endpoints (`POST /retain`, `GET
  /recall`), SDK methods, and AI SDK tools — all the in-process CLI, so none drift.

### Extraction pipeline (feeds the AI layer)

Content-based decisions need text. Extraction runs cheapest-first, so the model — and any network — are last resorts:

```
metadata / regex  →  PDF text layer  →  OCR (scanned PDF / image)  →  LLM judgment
   (free)             (pure-Go)          (external tool, optional)     (small model)
```

- **Digital PDFs:** pure-Go text-layer extraction, no dependencies.
- **Scanned PDFs & images:** rasterize pages → shell out to `tesseract` (+ poppler/pdfium). External tools, detected at runtime — this is how dig adds OCR while **staying cgo-free**. If absent, OCR steps escalate rather than fail (graceful degradation).
- **The LLM only ever sees extracted text, never pixels** — the reason a 7B-class model suffices.

---

## 6. Go components

| Component | Implementation |
|-----------|----------------|
| Content store | BLAKE3 hashing + `bbolt` manifests/journal (core, not pluggable); **blob bytes go to a `StorageBackend`** |
| StorageBackend | interface — first-party: local sharded dir + `gocloud.dev/blob` (S3/GCS/Azure); third-party via extension |
| Index | `IndexBackend` interface — first-party: SQLite FTS5 via `modernc.org/sqlite` (pure-Go); swappable |
| EventSink | interface — fires on changeset commit; first-party: webhook/exec; third-party for backup/audit/mirror |
| Policy engine | koanf-loaded rules → compiled matchers/templates |
| Reconciler | scan → diff vs last manifest → drift report → changeset |
| Lock manager | per-subtree advisory locks (bbolt-backed) |
| Commit | optimistic CAS on manifest `parent`; mismatch → merge |
| Merge | custom 3-way reconcile over `path → hash` entries |
| Workers | `errgroup` bounded pool; `context` cancellation for abort/escalation |
| LLM driver | minimal OpenAI-compatible HTTP client (chat + tool-calling); no SDK; pluggable `base_url` |
| Text extraction | pure-Go PDF text layer (`ledongthuc/pdf`); metadata readers |
| OCR (opt-in) | shell out to `tesseract` + poppler/pdfium for scanned PDFs/images; runtime-detected, cgo-free |
| KB registry | machine-level names → roots map; per-KB `.dig/` store + config |
| Dataset export | reads a pinned manifest → streams JSONL/records with per-row provenance (`src` blob hash + `manifest` id); pure read path |

All pure-Go → single static binary, no cgo, cross-compiles everywhere, runs offline.

---

## 7. Testing posture (BLOCKING dimensions)

The store + concurrency + reconcile layers are exactly the code that destroys trust if wrong:

- **Concurrency:** N parallel workers on overlapping/disjoint subtrees → correct final manifest, no lost ops, no torn writes. Race detector on.
- **Idempotency:** committing the same changeset twice = one head advance (CAS makes the second a no-op or clean conflict). Re-running `reconcile` on a converged KB = no-op.
- **Data integrity:** undo restores byte-identical content; no blob deleted while referenced; journal is append-only.
- **State machine:** every valid transition exercised; every invalid transition rejected; escalation locks only the conflicting subtree.
- **Reconcile / drift:** human-edit scenarios (rename, delete, dupe, conflicting edit) produce the right action from the §3 table; deliberate human changes are never silently overwritten.
- **AI layer:** deterministic behaviour with `mode = off`; AI proposals always pass through dry-run + journal; a wrong model suggestion is always reversible.
- **Property-based:** random op sequences → invariants hold (head always has a valid parent chain; dedupe never deletes the last copy).

---

## 8. What is explicitly NOT in the core

- **Source-code reorganization** — non-goal. dig manages document/asset libraries; auto-moving source breaks imports/builds and needs language-aware refactoring (a different product). Policy should exclude code trees.
- **RAG / Q&A assistant** — non-goal. dig governs *structure*, not answers. Retrieval serves management, not end-user search (that's Glean / Dust / Onyx).
- **Model training / fine-tuning** — non-goal. dig is the *data layer* (`dig export` emits reproducible, manifest-pinned, deduped, policy-filtered datasets) and the model *consumer* (`[llm]` endpoint), never the trainer. Training needs GPU/CUDA/PyTorch — incompatible with a cgo-free, small-model, local-deterministic binary. Hand the dataset to an external trainer (axolotl / unsloth / llama-factory / MLX), optionally via a workflow `exec` step.
- **Cloud dependency / telemetry** — none. dig runs fully locally; the only optional network hop is a user-configured LLM endpoint, which itself defaults to localhost.
- **Mandatory AI** — the deterministic core is complete without a model.
- **Embeddings / semantic search** — opt-in driver. Default index is FTS5.
- **Remote storage backends** (object storage via `gocloud.dev/blob`; SFTP via `pkg/sftp`) — later phase, implemented as `StorageBackend`s; the store/manifest model is transport-agnostic.

These exclusions preserve the open, local, deterministic, single-binary guarantee for the core librarian path.

---

## 9. Extensibility seam

The core is built against interfaces from day one — first-party implementations (local store, FTS5 index, regex/OCR extractors, webhook event sink) are themselves implementations of the public extension points. This means the spine is extensible by construction, while the public plugin *transports* (gRPC, WASM, registry) ship later.

```
              core (spine — NOT pluggable)
   ┌───────────────────────────────────────────────┐
   │ hashing · manifests · journal · changeset SM    │
   │ merge/escalate · dry-run · undo · policy eval    │
   └───────────────────────────────────────────────┘
        │ calls typed interfaces (the seams) ▼
   StorageBackend · IndexBackend · EventSink · Extractor · Matcher · Action · Command · LLMProvider
        │ satisfied by ▼
   first-party (compiled in)        third-party (T0 webhook · T1 PATH · T2 gRPC · T3 WASM)
```

**Invariant that makes a plugin ecosystem safe:** an extension can only *propose a changeset*; it never mutates the store or disk directly. Every extension action therefore inherits dry-run, journaling, and `undo`. Capabilities are declared per extension and enforced default-deny, scoped per KB. Full design in [extensions.md](extensions.md).
