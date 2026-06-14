# Landscape & positioning

Prior art for `dig` — an **open-source, local-first KB-management harness**: retrieve fast, organize by policy (naming / foldering / labels), dedupe, version everything, **detect & reconcile drift**, and do it **safely in parallel** (isolate → merge → escalate) while humans keep editing with their own tools.

Six camps surround the problem. Each owns one or two slices; **none cover the whole job, none reconcile structural drift, and none operate concurrently with merge + escalation.** That, plus being **open + local + small-model-driven**, is dig's moat.

> Stacks and claims verified May 2026. Treat tool details as a snapshot; check upstream before relying on a specific feature.

```
   organize by  retrieve    dedupe   version/  reconcile  parallel-safe
   policy       (find)               undo      drift      (merge/escalate)
   ──────────────────────────────────────────────────────────────────────
   rule orgs      ~          ~                  ~                          (B)
   AI orgs        ~(naming)                                                (C)
   file movers               ~        ~                                    (A)
   search/dedupe             ✓                                             (D)
   memory tools              ✓(text)                                       (E)
   kb assistants  answer-only — govern access, not structure               (F)
   ──────────────────────────────────────────────────────────────────────
   dig            ✓          ✓        ✓         ✓          ✓        ✓  ← target
```

Orthogonal axis dig also wins: **open source · runs fully local · works with small models** (most of Camp F is closed, cloud, big-model SaaS).

---

## Camp A — file movers / sync

Move and version bytes well. Can't *organize by rules* or *search*.

| Tool | Strategy | Pros | Cons | Stack |
|------|----------|------|------|-------|
| **rclone** | "rsync for cloud" — 70+ backends, sync / copy / mount | huge backend coverage, battle-tested, FUSE mount | no rules engine, no search/index, no versioned undo | Go |
| **rsync** | Delta-transfer sync | ubiquitous, fast deltas | local/SSH only, no organize, no history | C |
| **restic** | Encrypted dedup backup | snapshots, dedupe, encryption — **proves the content-addressed model in Go** | backup/restore-shaped, no organize, no search | Go |
| **s5cmd** | Max-throughput S3 ops | very fast parallel S3 | S3-only, transfer-only | Go |

**Borrow:** restic's content-addressed snapshot store (it's exactly dig's core). **Bar:** rclone backend breadth — don't chase early.

---

## Camp B — rule-based organizers (the closest functional cousins)

Apply naming/foldering rules to a directory. This is the camp dig must beat on the *librarian* axis. All are **single-threaded watch-and-apply** — no index, no versioned undo, no concurrent merge.

| Tool | Strategy | Pros | Cons | Stack |
|------|----------|------|------|-------|
| **organize** (tfeldmann) | YAML rules → filters + actions; simulate before applying | mature, flexible, cross-platform CLI, template engine, dry-run — **closest OSS match** | no index/search, no versioning/undo journal, no dedupe-by-content, single worker, Python runtime | Python |
| **Hazel** (Noodlesoft) | Folder-watching rules + tagging on macOS | polished, battle-tested, Finder tags/colors | macOS-only, GUI, paid, closed, no retrieval, no versioning | (closed, macOS) |
| **File Juggler** | Watch folders, rename/move by rules (Windows) | popular on Windows, content matching | Windows-only, GUI | (closed, Windows) |
| **File Arbor** | Cross-platform rule organizer, "Quick Rules" + Auto Mode | cross-platform, modern UI, cheap one-time license | GUI, closed, no search/versioning/concurrency | (closed) |
| **Maid** | Ruby rules to clean directories | simple ruleset | effectively unmaintained, Ruby | Ruby |

**Takeaway:** organize is the design reference for the policy schema (steal its simulate-first ethos and template engine). dig's differentiation over all of them: an **index** (retrieval), a **journal** (versioned undo), and **concurrency** (parallel workers, merge, escalation).

---

## Camp C — AI organizers (the 2026 wave)

Read file *content* to name/categorize via LLM/OCR. Powerful at the "what should this be called / where does it go" question — but all GUI/desktop, naming-focused, with **no concurrency, no versioning, no merge.**

| Tool | Strategy | Pros | Cons | Stack |
|------|----------|------|------|-------|
| **NameQuick** | AI + OCR reads files, generates descriptive names; pairs with Hazel for moving | strong content-aware renaming (extracts vendor/invoice/date) | macOS, GUI, naming-only (relies on Hazel to move), no versioning | (closed, macOS) |
| **Sortio** | Natural-language prompts → sort by name/type/metadata/content | flexible NL rules, pattern-adaptive | GUI/SaaS, closed, no versioning/concurrency | (closed) |
| **Riffo** | AI-driven renaming + management | content-aware, polished | closed, desktop, naming-centric | (closed) |
| **ai-file-sorter** (hyperfield) | Cross-platform desktop, content-aware sort/rename, local **or** remote LLMs, preview-first | open source, local-LLM option, user-controlled preview | desktop GUI, no index/versioning/concurrency | C++ / Qt |
| **Filex AI / RenameClick** | Privacy-first folder-scoped AI organize; watch + auto-flow | per-folder control, watch automation | GUI, closed, no versioning/merge | (closed) |

**Takeaway:** this validates that **content-aware naming belongs as an opt-in extractor driver** in dig (`{vendor}` resolved by an LLM), feeding the deterministic policy engine — not as the core path. dig keeps the safe, reversible, concurrent execution these tools lack.

---

## Camp D — search, dedupe & data tooling (feature references)

Steal capabilities, not positioning.

| Tool | Strategy | Pros | Cons | Stack |
|------|----------|------|------|-------|
| **fd** | Fast `find` replacement | speed, ergonomics | local FS only | Rust |
| **ripgrep** | Fast content search | fastest grep, gitignore-aware | content-only, local | Rust |
| **fzf** | Interactive fuzzy filter | composable, universal | not an index | Go |
| **fclones / jdupes / rdfind / czkawka** | Dedupe by content hash | best-in-class dedupe | single-purpose | Rust / C |
| **TMSU** | Tag-based virtual filesystem | tagging + query, **Go** | tagging only, no organize/version | Go |
| **DuckDB** | Query files in-place with SQL | query CSV/Parquet/JSON directly | analytics, not file mgmt | C++ |
| **DVC / git-annex** | Version data/large files for ML | reproducible, content-addressed datasets | VCS-coupled, manual staging, no organize/policy/drift | Python / Haskell |
| **datasette / dlt** | Explore / pipe data | instant UI; many connectors | read or pipeline, not management | Python |

**Borrow:** fd/ripgrep ergonomics → `dig find`; fclones dedupe → `dig dedup`; DuckDB "query in place" → future `dig query`; TMSU tagging model → `dig label`.

**Dataset-versioning adjacency (DVC / git-annex):** dig's content-addressed store makes it a natural *reproducible dataset* source — `dig export` pins a manifest and emits deduped, policy-filtered, provenance-tagged JSONL for ML training. The difference: DVC versions whatever you manually `dvc add`; dig *continuously organizes and dedupes the KB by policy*, so the dataset is a clean, labeled view by construction. dig feeds trainers (axolotl, unsloth, llama-factory, MLX); it **never trains** — GPU/CUDA/PyTorch is antithetical to a cgo-free, small-model, local binary, and that space churns monthly. Training tools are out of scope, intentionally.

---

## Camp E — AI memory / retrieval harnesses

Semantic recall of *text* for AI agents. Adjacent, not a direct rival — but the camp dig's name-twin sits near. Mostly Python + a vector DB.

| Tool | Strategy | Pros | Cons | Stack |
|------|----------|------|------|-------|
| **MemPalace** | Local-first verbatim "palace" (Wings/Rooms/Drawers); init → mine → search | no API key, verbatim (no lossy summarize), benchmarked recall | embedding/vector overhead, AI-agent-shaped, not file management | Python + ChromaDB |
| **mem0** | Pluggable memory layer (vector + graph) | drop-in SDK, hybrid store | library not CLI, needs a vector DB | Python (+ TS SDK) |
| **Letta** (ex-MemGPT) | Agent with tiered self-managed memory | autonomous memory paging | heavy agent framework, not a file tool | Python |
| **Zep** | Temporal knowledge graph memory (entity resolution, contradiction detection, validity windows) | strong temporal/graph model, enterprise-grade | cloud/SaaS-leaning, graph engine to operate, closed core, not file management | Python/Go service |
| **MCP memory servers** (the 2026 wave) | Per-harness stateful context tools exposed over MCP | trivial to plug into Claude/Cursor; large + growing set | thin, bespoke, no versioning/merge/provenance; storage varies wildly | various |

**Borrow:** the `init → index → search` UX; Zep's temporal-validity + contradiction model (dig's roadmap entity-graph maps facts to validity windows and surfaces contradictions as *escalations*, not silent overwrites). **Reject:** mandatory embeddings (semantic search is an opt-in driver in dig; default index stays SQLite FTS5) and a separate graph engine to operate. **Note:** this is the camp growing fastest into dig's path — and the one most exposed on dig's axes (versioned undo, merge, provenance, local-first). It's covered here as adjacent, but for the agent-memory positioning it is the *primary* battlefield, not a footnote.

### Full functionality matrix: dig vs MemPalace, verified hands-on (2026-06)

Every MemPalace 1.x command exercised on an identical 13-file messy KB (invoices as .pdf, notes as .md, duplicates, binary blobs); dig re-measured after closing #3/#4/#5.

| MemPalace function (verified) | What it did on the corpus | dig equivalent (measured) | Verdict |
|---|---|---|---|
| `init` (room detection) | detected 2 rooms from folders, interactive prompt | `init` + `scan`: 13/13 files, 0.05 s, no prompts | **dig** — full coverage, scriptable |
| `mine` (ingest) | 24 s, **5/13 files** — every PDF/txt/jpg skipped | `scan`: all files, hashed + deduped by construction, 0.05 s | **dig** |
| `search` (hybrid cosine+bm25) | ~1.9 s, md-only corpus slice | `find`: FTS over paths + labels + **content**, AND→OR fallback for natural questions; same top hit on "who did I talk to about contract renewal", 11 ms, PDFs included | **dig** — same answer, 170× faster, whole corpus |
| `sweep` (catch-up miner) | 0 new (idempotent re-mine) | `scan` / `reconcile` are idempotent by design (no-op commits never created) | **dig** — same property, plus journaled |
| `sync` (prune deleted sources) | dry-run default, `--apply` removes stale drawers | `drift` + `reconcile`: absorbs deletes/renames/edits into versioned history; renames detected by content identity | **dig** — sync is a subset of reconcile, without history |
| `compress` (AAAK token reduction) | **grew** the corpus: 95t → 116t ("0.8×") | `export` streams capped text slices; no lossy compression by design | **dig** (on this corpus) — and dig never mutates content |
| `wake-up` (agent context) | ~151-token L0/L1 summary of mined drawers | `export --filter … --json` / `find --json`: deterministic, provenance-tagged context for any harness | **split** — MemPalace's curated "story" is a real, distinct feature; dig provides raw, pinned, machine-consumable context |
| `split` (transcript chunking) | n/a on this corpus (AI-session transcripts only) | out of scope — dig manages files, not chat transcripts | **n/a** — different problem |
| `hook` / `instructions` / `mcp` | Claude/Codex integration surfaces | CLI-first by design: `--json` + stable exit codes; any harness shells out (MCP wrapper = extensibility phase) | **split** — MemPalace ships turnkey agent glue today; dig's contract is broader but DIY until P-extensibility |
| `repair` / `repair-status` | HNSW vs sqlite divergence check (0 here) | index is a **derived view** — `scan` rebuilds it from manifests; nothing to diverge | **dig** — repair is unnecessary by construction |
| `migrate` / `migrate-wings` | store schema migrations | versioned manifests + append-only journal; store semantics in core | **dig** — history is the data model, not a migration target |
| `status` | drawer counts per wing/room | `log` (history) + `work list` (views) + `drift` (divergence) | **dig** — richer state, three lenses |
| — (no equivalent) | — | organize/rename/label by policy, dedupe, byte-identical undo, human-coexistence (pinning), parallel views + merge/escalation, continuous watch, reproducible dataset export | **dig only** |

**Measured footprint:** dig 12 MB single binary, 244 KB store · MemPalace 330 MB venv, 620 KB palace. Ingest 0.05 s vs 24 s. Query 11 ms vs 1.9 s.

**Honest read:** dig now wins or matches on every function MemPalace has for *files*, plus the entire management surface MemPalace lacks. MemPalace's two historic strengths are now both contested: true embedding semantics — dig's opt-in vector driver is **built and benchmarked** (hybrid hit@5 98.0% vs MemPalace's 96.6% on the full LongMemEval-S set, 2026-06); and turnkey agent-memory glue (session retention, wake-up context, MCP server, transcript splitting) — which dig **exposes as thin glue over the same store** (the `agent-memory` roadmap phase), not a memory engine of its own. dig is not a Camp-E product: it serves the recall *substrate* these tools sit on. The reason is sovereignty — use your own system end-to-end, never rent the recall layer.

---

## Camp F — KB assistants & governance (the framing twin)

The camp dig is most likely to be *compared* to — "an AI for our knowledge base." But they **connect-and-answer** (RAG/Q&A) and govern *access / sensitivity / retention*; they never restructure the files. Mostly closed, cloud, connector-based, large-model.

| Tool | Strategy | Pros | Cons (vs dig) | Stack / model |
|------|----------|------|---------------|---------------|
| **Glean** | Enterprise Work AI: search + agents + active data governance (scans/remediates oversharing, retention) | strong governance loop, broad connectors, real drift remediation — **for access/compliance** | governs *access*, not *structure*; doesn't reorganize/rename/dedupe files; closed, cloud SaaS, large models | closed SaaS, cloud |
| **Dust** | Enterprise AI agents over internal knowledge + semantic layer | polished agent platform, multi-model, connectors | answers/automates, doesn't manage file structure or versioning; closed SaaS | closed SaaS, cloud |
| **Onyx** (ex-Danswer) | Open-source enterprise search / Gen-AI assistant over company docs | **open source, self-hostable**, connectors, RAG | retrieval/Q&A, not structure management; no organize/dedupe/version/reconcile | OSS (Python), self-host |
| **Vale** | Prose/markup linter — policy-as-code for *writing style*, CLI | **closest "policy-as-code on a KB" precedent; CLI; written in Go** | content/prose only, **read-only** (flags, never fixes), no structure/version/dedupe | Go (OSS) |
| **Governance-aware agents** (2026 category) | Agents enforce *data policies*, detect *policy drift*, real-time violation engines | validates the drift + enforcement thesis | govern data *compliance* (PII/retention/access), not file *structure*; enterprise platforms | mixed, cloud |

**Takeaway:** this camp proves the *demand* (AI managing a KB, enforcement, drift) and the *risk* (everyone interprets it as access-governance + Q&A, closed + cloud + big-model). dig occupies the empty structural lane: **manage and fix the physical organization of the KB**, open + local + small-model. Vale is the spiritual ancestor — "policy-as-code for a KB, in Go" — but dig *reconciles* where Vale only *flags*.

---

## Extensibility models (how dig should be a platform)

dig aims to be **the pi.dev of KB management** — tiny core, rich ecosystem. These are the prior-art *mechanisms* for making a tool extensible; dig borrows from each rather than inventing one. (Detail in [extensions.md](extensions.md).)

| Model | Mechanism | Pros | Cons | dig uses it for |
|-------|-----------|------|------|-----------------|
| **pi.dev** | In-process TS modules + npm/git packages + catalog | frictionless authoring, vibrant ecosystem, the spirit dig wants | mechanism is JS-runtime-specific — **does not port to a Go binary** | the *model* (tiny core, manifest packages, registry), not the mechanism |
| **Terraform / Vault** (`hashicorp/go-plugin`) | gRPC subprocess plugins | battle-tested, crash-isolated, typed, multi-language | a binary per plugin, RPC overhead | **T2** — robust out-of-tree storage/index/extractor backends |
| **Helm / Extism** | WASM modules (wazero host) | sandboxed, capability-gated, portable, **pure-Go host (cgo-free)** | wasm toolchain, data-marshalling cost | **T3** — untrusted third-party extensions from a public catalog |
| **git** | PATH executables `git-<subcmd>` | zero protocol, any language, trivial | no isolation, no typed contract | **T1** — custom verbs and glue |
| **Go native `plugin`** | `.so` dynamic loading | in-process, fast | version-locked to compiler, Linux/macOS-only, unsandboxed — **a trap** | rejected |

**Takeaway:** dig keeps pi.dev's *spirit* (small core + ecosystem + manifest packages + registry) but implements it with Go-native transports (PATH / gRPC / WASM) layered over **typed extension points**, so "store elsewhere" and "backup" are one interface each — not a bespoke plugin host.

## Strategic read

- **The whole-job gap.** rclone moves, organize shelves, restic versions, fclones dedupes, MemPalace finds — five tools, five mental models, and you still have to choose a canonical copy and pray nothing runs twice. dig is the one harness that does the librarian's full job over one store.
- **Concurrency is the moat.** Every organizer in Camps B/C is a single-threaded watch loop, *precisely because* concurrent destructive file ops are brutally hard. dig's isolate → merge → escalate model is the hardest part and the thing nobody else offers.
- **Reversibility is the spine, not a feature.** A librarian that renames/moves/dedupes is destructive. The journal + `--dry-run` + `undo` must exist before any organize feature, or users won't trust it on real files. This dictates the phasing (P0 = store + journal + undo, before P1 = organize).
- **Drift is the recurring job.** A KB doesn't stay tidy; it rots as humans edit it. Camp F validates "detect drift + enforce policy" as a real category — but for compliance, not structure. dig reconciles *structural* drift (misfiled, misnamed, duplicated) and coexists with the human edits that cause it.
- **AI stays opt-in, local, and small.** Camp C proves content-aware naming is valuable; Camp F proves the closed/cloud/big-model version is the default trap. dig inverts it: deterministic core, AI as an opt-in layer over strong tools, OpenAI-compatible endpoint defaulting to localhost — so a 7B model is enough and `off` still works.
- **Open + local is a moat, not a footnote.** Camp F's leaders (Glean, Dust) are closed cloud SaaS on enterprise data. An open, self-hosted, no-telemetry tool that never sends the KB anywhere is a different value proposition for anyone who can't or won't ship their knowledge base to a vendor.
- **Extensibility is the platform bet.** Camps A–F are products; dig aims to be a *platform* — the pi.dev of KB management. Typed seams (storage, events, extraction, …) mean a company adapts dig (own object store, own backup) without forking, and the safety spine (every extension only *proposes a changeset*) makes a third-party ecosystem tolerable. That combination — open + local + extensible + reversible — is what none of the six camps offer.
- **One-line positioning:** *"an open, local, extensible harness that keeps your knowledge base organized to your rules — detects drift, fixes it reversibly, runs in parallel, works with a small local model; a KB kept that clean doubles as recall that doesn't rot, so you never rent the memory layer."*

## v0 wedge

Smallest slice that earns trust and proves the core: **content store + journal + `init` / `scan` / `find` + `--dry-run`/`undo`.** Retrieval works, and every future destructive feature already has its safety net. Organize (P1), dedupe (P2), and the parallel/merge machinery (P4–P5) layer on once the spine is rock-solid.

## Sources

- Rule organizers: [organize](https://github.com/tfeldmann/organize) · [Hazel](https://www.noodlesoft.com/) · [File Juggler](https://www.filejuggler.com/) · [File Arbor](https://filearbor.com/) · [Hazel alternatives (AlternativeTo)](https://alternativeto.net/software/hazel/)
- AI organizers: [NameQuick](https://www.namequick.app/) · [Sortio](https://www.getsortio.com/) · [Riffo](https://riffo.ai/) · [ai-file-sorter](https://github.com/hyperfield/ai-file-sorter) · [Filex AI](https://filexai.com/) · [RenameClick](https://rename.click/)
- File movers: [rclone](https://rclone.org/) · [restic](https://restic.net/) · [s5cmd](https://github.com/peak/s5cmd)
- Search / dedupe / data: [fd](https://github.com/sharkdp/fd) · [ripgrep](https://github.com/BurntSushi/ripgrep) · [fzf](https://github.com/junegunn/fzf) · [fclones](https://github.com/pkolaczk/fclones) · [TMSU](https://tmsu.org/) · [DuckDB](https://duckdb.org/) · [DVC](https://dvc.org/)
- Memory tools: [MemPalace](https://github.com/MemPalace/mempalace) · [mem0](https://github.com/mem0ai/mem0) · [Letta](https://www.letta.com/)
- KB assistants / governance: [Glean](https://www.glean.com/) · [Dust](https://dust.tt/) · [Onyx (ex-Danswer)](https://www.onyx.app/) · [Vale](https://vale.sh/)
- LLM endpoints (OpenAI-compatible) — local runtimes: [Ollama](https://ollama.com/) · [llama.cpp](https://github.com/ggml-org/llama.cpp) · [LM Studio](https://lmstudio.ai/) · [vLLM](https://github.com/vllm-project/vllm); gateways/proxies: [LiteLLM](https://github.com/BerriAI/litellm) · [OpenRouter](https://openrouter.ai/)
- Extensibility models: [pi.dev packages](https://pi.dev/packages) · [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) · [Extism](https://extism.org/) · [wazero](https://wazero.io/) · [git custom subcommands](https://git.github.io/htmldocs/howto/new-command.html)
