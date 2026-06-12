# dig

> The open, local, reversible **data layer for AI agents** — it keeps a knowledge base in order *and* serves as your agent's memory. You set the policy (folder structure, naming, labels, no duplicates); `dig`'s agents enforce it, **detect drift, fix it, and version every change** so nothing is ever lost. It **retrieves fast** (hybrid full-text + semantic), **remembers across sessions**, and plugs into any agent or framework via **MCP + native SDKs**. Humans keep editing with their own tools — `dig` reconciles around them instead of locking them out, and runs many agents in parallel without colliding. **Open source, runs fully on your machine, works with any OpenAI-compatible model — including a small local one.**

A company's or a person's knowledge base rots: files land in the wrong place, names drift from convention, duplicates pile up, structure erodes. Keeping it tidy is real, recurring work most people would rather **delegate**. `dig` is that delegate — an agent harness that does the librarian's whole job (**find, organize, dedupe, label, version, reconcile**) over one content-addressed core, safely, even while humans and other agents touch the same library.

Most tools do one slice: some move bytes, some apply naming rules, some lint prose, some answer questions about your docs, some version. None *manage the structure of a living knowledge base* and keep it converged on your policy.

`dig` aims to be **the pi.dev of KB management** — a small, sharp core with a rich extension ecosystem. Need to store blobs in your own object store, back up on every change, parse a proprietary format, or add a command? That's an extension, not a fork.

> **Status: early scaffold.** This README describes the intended design. Nearly everything is _(planned)_. Expect breaking changes.

---

## What dig does

```
        ┌──────────────────────────────────────────────┐
        │              dig — file librarian              │
        └──────────────────────────────────────────────┘
   retrieve        organize        dedupe       version       parallel-safe
   find fast    rules: name/     no copies     full history   isolate · merge
   & ranked     move/label        kept          + undo         · escalate
        └──────────────┴──────────────┬──────────────┴──────────────┘
                                       ▼
                     ┌───────────────────────────────────┐
                     │   one content-addressed store       │
                     │   blobs by hash + tree manifests    │
                     └───────────────────────────────────┘
```

- **Retrieve fast.** Indexed, ranked `find` across the whole library.
- **Organize by policy.** You declare the rules (naming conventions, folder layout, where things belong); `dig` makes the tree match — readable, like a librarian shelving books.
- **No duplicates.** Identical content is detected by construction (same hash) and collapsed per policy.
- **Version everything.** Every change is recorded; history is browsable; any change is reversible (`dig undo`).
- **Detect & fix drift.** Policy is a *desired state*. `dig` continuously compares it to the actual KB, reports what has drifted (misfiled, misnamed, duplicated, unlabeled), and reconciles — automatically where safe, by proposal where not.
- **Coexist with humans.** People keep using their notes app, Finder, Drive, their editor. `dig` observes those direct edits, folds them into history, and reconciles them against policy — it never demands you go "through" it, and never silently overrides a deliberate human change (it escalates instead).
- **Parallel-safe.** Multiple agents operate in isolated views, merge back automatically when they don't overlap, and **escalate to a human** only when a real conflict can't be resolved by policy.

Why these aren't separate features: a single content-addressed store gives dedupe, versioning, cheap isolation, and mergeable changesets *for free*. See [docs/architecture.md](docs/architecture.md).

### Scope

- **Manages knowledge bases** — document/asset libraries: PDFs, media, notes, datasets, research, downloads. Files that are safe to move, rename, and relabel.
- **Manages structure, does not answer questions.** `dig` governs *where things live and what they're called*. It is **not** a RAG / Q&A assistant — that's a different product category. Retrieval in `dig` serves *management* (find the files a rule applies to), not end-user search. Keeping this lane is deliberate; a tool that both restructures files and answers questions does neither well.
- **Feeds model training, does not train.** A clean, deduped, labeled, versioned KB *is* a training dataset — so `dig export` emits one reproducibly (see [Datasets](#datasets-for-ml-training)). Actually fine-tuning a model (GPU / CUDA / PyTorch) is an explicit non-goal: it would break the cgo-free single-binary + small-model architecture. dig is the data layer at the start of the pipeline and the model *consumer* at the end — never the trainer in the middle.
- **Restructures fully, never ad hoc.** Within a library `dig` reshapes hierarchies, moves, renames, dedupes — but only as policy / rules / workflows direct, always reversibly.
- **Not a code refactoring tool (for now).** Restructuring *source* trees breaks imports/builds and needs language-aware analysis — a future import-aware workflow, not the initial scope. Point `dig` at a repo's *assets*, not its source.
- **Source of truth is the disk, not `dig`.** Humans edit files directly with their own tools; `dig`'s store is a versioned shadow it reconciles against — never a gatekeeper you must check in through.
- **Open, local, AI-optional.** Open source, runs fully on-device, no telemetry. AI is opt-in, points at any OpenAI-compatible endpoint (localhost by default), and a small local model suffices — the deterministic core works with no model at all.
- **Parallel model: multiple autonomous agents** on one library — the design assumption behind the full isolate → merge → escalate machinery.
- **CLI-only.** The command line is the *sole* interface — no GUI, no importable SDK. This is what makes `dig` composable: any larger agent harness drives it by calling the CLI.
- **Multi-KB.** One machine hosts many knowledge bases. Each is configured independently (its own policy / rules / workflows / LLM); a machine-level registry tracks them.
- **Extensible.** Storage, events/backup, extraction, matching, workflow steps, commands, the index, and the model endpoint are all typed extension points (eight seams). The core spine (store semantics, changeset state machine, undo) is not extensible — extensions plug into the edges, never the spine.

---

## Why

```
        Without dig                          With dig
  ┌──────────────────────┐           ┌──────────────────────┐
  │ renaming by rule      │           │                       │
  │ deduping by hash      │           │   one policy file      │
  │ manual renaming       │   ───▶    │   dig org              │
  │ manual version control│           │   dig undo / log       │
  │ "don't run two at once"│          │   dig work (parallel)  │
  └──────────────────────┘           └──────────────────────┘
   N tools, no safety net              1 tool, fully reversible
```

The thing that makes destructive file management trustworthy is **reversibility**, not features. `dig` treats history, dry-run, and undo as the foundation — everything else is built on top so you can let it loose on real files.

---

## Install

```bash
# with Go (any platform)
go install github.com/vllnt/dig/cmd/dig@latest

# or grab a prebuilt binary from a release
#   https://github.com/vllnt/dig/releases  (linux/macOS/windows × amd64/arm64)
# verify checksums.txt, extract, put `dig` on your PATH
```

Releases are cross-compiled and checksummed by [GoReleaser](.goreleaser.yaml); a
tag `vX.Y.Z` builds and publishes them. A `curl | sh` installer served from
[dig.vllnt.com](https://dig.vllnt.com) lands with the site (roadmap `site-launch.3`).

### Drive dig from your agent

- **Claude Code plugin** (bundles the dig skill + the `dig mcp` server):
  ```
  /plugin marketplace add vllnt/dig
  /plugin install dig@dig
  ```
- **MCP (any client)**: register `dig mcp` (stdio) — see `.claude-plugin/`.
- **SDKs over `dig serve`**: `npm i @vllnt/dig` · `pip install vllnt-dig`.

## Quick start

```bash
# index a library (builds the content-addressed store + search index)
dig init ~/library
dig scan

# search
dig find "invoice acme 2024"

# preview what your policy would change — nothing is touched
dig org --dry-run

# apply it; every move/rename/label is recorded
dig org

# made a mess? step back
dig undo
dig log
```

---

## Commands

| Command | Does | Status |
|---------|------|--------|
| `dig init <root>` | Create a library at a directory | planned |
| `dig scan` | Index files into the content-addressed store | planned |
| `dig find <query>` | Search the library, ranked results. FTS by default; opt-in semantic + hybrid (`--mode`, `[retrieval]` policy) | planned |
| `dig retain [file]` | Capture content (a file, stdin, or a rendered agent session via `--transcript`) into the KB and index it — the agent-memory capture primitive; dated `memory/` path by default (`--as`, `--date`) | planned |
| `dig recall <query>` | Emit a token-budgeted, provenance-tagged context pack for a query — the agent-memory recall primitive (`--budget`, `--mode`, `--json`) | planned |
| `dig embed` | Drain the semantic-index backlog in the background (resumable; `watch` also drains it per tick) | planned |
| `dig mcp` | Run dig as an MCP server (stdio) — exposes find/recall/retain/drift/log/export + org/reconcile/undo as tools for any agent harness (dig as a memory layer) | planned |
| `dig serve` | Run a localhost HTTP+JSON daemon over the CLI — for SDKs/apps that embed dig without shelling out (loopback only) | planned |
| `dig export` | Emit a reproducible, manifest-pinned dataset (JSONL etc.) for ML training | planned |
| `dig org` | Apply organization policy (move / rename / label). `--dry-run` previews | planned |
| `dig run <workflow>` | Execute a named workflow; commits its steps as one reversible changeset | planned |
| `dig dedup` | Find duplicates and collapse per policy | planned |
| `dig label <selector>` | Apply labels/tags per policy | planned |
| `dig drift` | Report how the KB diverges from policy (misfiled, misnamed, duplicated, unlabeled) | planned |
| `dig reconcile` | Bring the KB back to policy — auto where safe, proposals where not. `--dry-run` previews | planned |
| `dig watch` | Run as a harness: observe edits + reconcile continuously, escalate when unsure | planned |
| `dig log` | Browse change history | planned |
| `dig undo [ref]` | Revert the last changeset (or a specific one) | planned |
| `dig work <name>` | Open an isolated work view (worktree-like) | planned |
| `dig merge <work>` | Merge a work view back; auto-resolve or escalate | planned |
| `dig policy` | Edit / validate the organization policy | planned |
| `dig kb <list\|add\|remove>` | Manage the knowledge bases registered on this machine | planned |
| `dig ext <list\|add\|enable\|remove>` | Manage extensions (storage backends, event sinks, extractors, commands) | planned |
| `dig config` | View and edit configuration | planned |

Run `dig <command> --help` for flags. Commands target a KB via `--kb <name>` (or the KB rooted at the current directory). **Read** commands (`find`, `export`, `drift`, `log`) support `--json` for other harnesses to consume; **write** commands (`org`, `run`, `dedup`, `label`, `reconcile`, `undo`, `merge`) default to dry-run-friendly output and always write to the history journal.

---

## Policy, rules & workflows

`dig` actively restructures the library — it reshapes folder hierarchies, renames, dedupes, relabels — but **never ad hoc**. Every change is driven by one of three governance primitives, and every change is journaled and reversible:

- **Rules** — `match → action`. Stateless: where a file belongs, what it's named, which labels it gets.
- **Policy** — the desired-state spec: the full rule set plus invariants (naming convention, no duplicates, retention). Defines what "organized" *means*; `dig drift` is measured against it.
- **Workflows** — ordered, multi-step, optionally triggered/stateful procedures. Where real restructuring and agent steps live: *ingest contract → extract parties → file under client → label → version → notify*. Steps can call tools, extractors, or an LLM; the whole workflow commits as **one** reversible changeset.

```toml
# rules — declarative placement
[[rule]]
name   = "invoices"
match  = { ext = ["pdf"], content_matches = "invoice" }
into   = "finance/invoices/{year}"
rename = "{vendor}-{invoice_no}.pdf"
label  = ["finance", "invoice"]

[dedup]
strategy    = "keep-oldest"   # which copy is canonical
on_conflict = "escalate"      # never silently delete

# workflows — ordered steps, can call extractors/agents, commit atomically
[[workflow]]
name  = "ingest-contract"
on    = "new_file in inbox/ matching *.pdf"
steps = [
  { extract = ["parties", "effective_date"] },   # regex → OCR → LLM
  { apply_rule = "contracts" },
  { label = ["legal"] },
  { version = true },
]

# AI — optional, OpenAI-compatible (any endpoint that speaks the OpenAI API)
[llm]
mode     = "tools"                      # tools | json | off   (off = pure deterministic)
base_url = "http://localhost:11434/v1"  # local runtime (Ollama / llama.cpp / LM Studio / vLLM)
                                        # or a gateway (LiteLLM / OpenRouter) → 100+ providers
model    = "qwen2.5:7b"                  # a small local model is enough
```

`dig` speaks **only the OpenAI API shape** and never bundles a vendor SDK, so `base_url` accepts two kinds of endpoint:

- **A local runtime** — Ollama, llama.cpp, LM Studio, vLLM. Default; keeps everything on-device.
- **A gateway/proxy** — [LiteLLM](https://github.com/BerriAI/litellm), OpenRouter. One OpenAI-compatible URL fronting 100+ providers (Claude, GPT, Gemini, Bedrock, …), with per-model routing, fallback, and cost controls handled by the gateway, not `dig`.

Either way `dig` sees one URL + one model name. Pointing at a remote endpoint trades the local-only guarantee for provider reach — your choice, per KB.

### Reading content (extraction & OCR)

To file or name by *content* (the vendor on an invoice, the parties on a contract), `dig` must read the file. The extraction pipeline runs cheapest-first:

```
metadata/regex → PDF text layer → OCR (scanned PDFs / images) → LLM judgment
   (free)         (pure-Go)        (external tool)               (small model)
```

- **Digital PDFs** carry a text layer — extracted in pure Go, no dependencies.
- **Scanned PDFs and images** need **OCR**: `dig` rasterizes pages and shells out to `tesseract` (plus a rasterizer like poppler / pdfium). These are *optional external tools* — `dig` stays a pure-Go single binary, detects them at runtime, and if they're absent the OCR step escalates instead of failing.
- The **LLM only ever sees clean extracted text**, never raw pixels — which is exactly why a small local model is enough.

**AI is never on the core path:** with `[llm] mode = off`, `dig` runs fully deterministic and offline; the model only makes small bounded judgments while `dig`'s tools do the structural work. See [docs/architecture.md](docs/architecture.md).

---

## Architecture

`dig` is a thin command layer over a **content-addressed store** (blobs keyed by content hash + versioned tree manifests — a git-style model) and a **policy engine** that proposes changesets the store applies atomically and reversibly.

```
┌──────────────────────────────────────────────┐
│                   dig CLI                       │  commands, dry-run, --json
└───────────────────────┬────────────────────────┘
            ┌────────────┼─────────────┐
       ┌────▼────┐  ┌────▼────┐  ┌─────▼─────┐
       │ policy  │  │  index   │  │ concurrency│   propose · search · isolate
       │ engine  │  │ (FTS5)   │  │  control   │   merge · escalate
       └────┬────┘  └────┬────┘  └─────┬─────┘
            └────────────┼─────────────┘
                  ┌──────▼───────┐
                  │ content store │  blobs(hash) + tree manifests + journal
                  └──────────────┘
```

**Concurrency** is the hard, novel part: each unit of work runs against an isolated manifest view, produces a changeset, and merges back. Disjoint changes auto-merge; overlaps are resolved by policy precedence; anything still conflicting is **escalated to a human** while the rest proceeds. Full state machine and escalation ladder in [docs/architecture.md](docs/architecture.md).

---

## Multiple KBs & use from other harnesses

```
   one machine
   ├─ registry  (~/.config/dig/registry.toml — names → roots)
   │
   ├─ KB "work"     /data/work/.dig/   ← store · journal · index · config
   ├─ KB "research" /data/research/.dig/
   └─ KB "personal" ~/notes/.dig/
              ▲
   ┌──────────┴───────────┐
   │   another harness     │  shells out:  dig --kb work find … --json
   │  (agent / script /    │               dig --kb work run ingest-contract
   │   bigger system)      │
   └──────────────────────┘
```

- **Per-KB config.** Each KB keeps its own `policy / rules / workflows / LLM` in a `.dig/` folder at its root — portable and independently customizable. A machine-level registry just maps names to roots.
- **CLI is the only interface.** No GUI, no library to import. Other harnesses use `dig` exactly the way you do: by running commands. `--json` on read commands makes output machine-consumable; exit codes are stable.
- **dig owns its own LLM.** When embedded in a bigger harness, `dig` still uses *its* configured (local, OpenAI-compatible) model for its internal judgments — it does not depend on, and is not coupled to, the caller's model. The outer harness orchestrates; `dig` manages the KB.

---

## Extensions

A small core; everything at the edges is pluggable. Eight typed seams, four transport tiers — you pick the seam by *what* you're adding and the tier by *how much trust/robustness* it needs.

```
   WHAT (typed seams)                         HOW (transport tiers)
   ────────────────────────────────          ───────────────────────────────
   StorageBackend  where blobs live  ◀─┐      T0  declarative — TOML + exec/webhook (no code)
   EventSink       react to changes  ◀─┤      T1  PATH subcommand  dig-<name>  (any language)
   Extractor       read content (OCR)        T2  gRPC subprocess  (hashicorp/go-plugin)
   Matcher         custom matching            T3  WASM via wazero  (sandboxed, cap-gated)
   Action          new workflow step
   Command         new `dig <verb>`        ┌─ company X: "save data elsewhere" → StorageBackend
   IndexBackend    where search lives      └─ company X: "backup system"        → EventSink
   LLMProvider     model endpoint
```

The two classic company needs map to **one interface each**, no bespoke plugin:

```toml
# back up on every change — T0, zero code
[[event_sink]]
name = "offsite-backup"
on   = "changeset.committed"
exec = "backup-tool backup {changed_paths}"

# store blobs in a company object store — a StorageBackend extension
[[storage]]
name = "acme-store"
ext  = "dig-s3-store"     # gRPC backend installed via `dig ext add`
```

**Safety:** an extension can only ever *propose a changeset* — it never writes files directly. So every extension action is dry-run-able, journaled, and `undo`-able like everything else. Untrusted extensions declare capabilities (`storage:write`, `net:…`, `read:finance/**`); the core enforces them, default-deny. Full design — interfaces, tiers, capability model, manifest, registry — in [docs/extensions.md](docs/extensions.md).

---

## Datasets for ML training

A clean, deduped, labeled, **versioned** KB is exactly what a training run wants — and dig's content-addressed store makes that dataset *reproducible* in a way ad-hoc `find | jq` pipelines can't be. dig is the **data layer**, not the trainer.

```
   KB ──▶ dig export ──▶ dataset.jsonl ──▶ [ external trainer ] ──▶ model
          (pin a manifest,                  axolotl · unsloth ·       │
           dedup, filter,                    llama-factory · MLX      │
           carry provenance)                                          ▼
                                              dig points its [llm] endpoint here
```

```bash
dig export --kb work \
  --filter label:legal \      # policy-driven selection
  --format jsonl \
  --at @M42 \                 # pin to a manifest = reproducible
  > dataset.jsonl
```

Each record carries provenance back to the content hash and the manifest it came from:
```json
{"text": "…extracted text…", "labels": ["legal"], "src": "b3:9f2a…", "manifest": "M42"}
```

- **Reproducible.** Same manifest → byte-identical dataset. A model's training data is pinned to a version you can diff and re-emit months later.
- **Deduped by construction.** No near-duplicate documents skewing the run — the store already collapsed them.
- **Policy-filtered.** Export exactly the slice your rules/labels define (`label:legal`, `into:finance/**`, date ranges).
- **Provenance-tracked.** Every row traces to its source blob and KB version — auditable, and the basis for honoring deletions/retention in derived datasets.

**Why dig stops here:** training needs GPU/CUDA/PyTorch and hours of compute — antithetical to a cgo-free, small-model, local-deterministic binary. Hand `dataset.jsonl` to any external trainer (optionally via a workflow `exec` step), then point `[llm] base_url` at the result to make dig's own extraction/classification sharper on that KB. The pipeline closes; the architecture stays intact.

---

## Tech stack

Go, single static binary, **cgo-free** so it cross-compiles to every OS/arch without a toolchain.

| Concern | Choice | Notes |
|---------|--------|-------|
| Language | Go 1.22+ | single binary, concurrent IO, widest storage SDKs |
| CLI | `spf13/cobra` | subcommands, completions, man-page generation |
| Config / policy | `knadh/koanf` + TOML | declarative rules; env + file + flag merge |
| Content store | content-addressed blobs + `bbolt` for manifests/journal | dedupe + versioning + isolation from one store |
| Index / search | SQLite FTS5 via `modernc.org/sqlite` | pure-Go (no cgo), one index file, SQL + full-text |
| Concurrency | `errgroup` + subtree lock manager + manifest CAS | parallel workers, optimistic merge, no collisions |
| Merge | custom 3-way tree-manifest merge | git-style: disjoint auto, overlap → conflict |
| AI (optional) | minimal OpenAI-compatible HTTP client | no vendor SDK; localhost by default; small-model-first; `mode` off/json/tools |
| Text extraction | pure-Go PDF text layer (`ledongthuc/pdf`) | digital PDFs, no deps |
| OCR (optional) | shell out to `tesseract` + poppler/pdfium | scanned PDFs / images; external tools, detected at runtime; keeps core cgo-free |
| Extensions — gRPC | `hashicorp/go-plugin` | T2: out-of-tree backends as subprocesses; crash-isolated |
| Extensions — WASM | `tetratelabs/wazero` | T3: sandboxed, capability-gated third-party modules; pure-Go (no cgo) |
| Storage backends | `gocloud.dev/blob` | first-party `StorageBackend`: one API over S3 / GCS / Azure / local |
| SSH / SFTP (later) | `golang.org/x/crypto/ssh` + `pkg/sftp` | remote storage backend |
| Logging | `log/slog` | stdlib, structured |
| Output | `text/tabwriter` + `encoding/json` | human tables + `--json` |
| Test | `testing` + `testify` + golden files | + concurrency/merge property tests |
| Lint / Release | `golangci-lint` · GoReleaser + GitHub Actions | cross-compiled binaries, checksums, Homebrew tap |

Semantic search and content-based naming are **opt-in drivers**, not core — the default stays deterministic and single-binary.

---

## Prior art & positioning

Six camps surround the problem; none cover it whole:

- **File movers** — transfer/version bytes, but can't *search* or *organize by rules*.
- **Rule organizers** — apply naming/foldering rules, but single-threaded, no index, no versioned undo, no merge.
- **AI organizers** — read content to name/sort, but GUI, no concurrency, no versioning.
- **Search/dedupe** — one capability each, local only.
- **KB assistants** — connect to docs and *answer questions*; govern access/sensitivity — but never restructure the files.
- **Doc linters / governance agents** — enforce *prose style* or *data compliance* and flag drift — read-only, no structural fix, no versioning.

dig's unfilled gap: **policy-driven structure + drift detection + reconcile + full versioning + safe parallel operation that coexists with human edits — on an open, local, extensible core.** Everyone else answers, flags, or moves; nobody *manages the structure of a living KB and keeps it converged.* The extensibility model borrows from Terraform providers (gRPC plugins), Helm/Extism (WASM), git (PATH subcommands), and pi.dev (tiny core + package ecosystem). Full breakdown — strategy, pros/cons, stack per tool — in [docs/landscape.md](docs/landscape.md).

---

## Roadmap

Phased so the safety spine exists before destructive features, and one-shot before continuous:

- [ ] **P0 — foundation:** content store + manifests + journal · `init` / `scan` / `find` · `--dry-run` + `undo` everywhere
- [ ] **P1 — organize:** policy engine · `org` (rename / move / label), single worker, fully reversible
- [ ] **P2 — dedupe:** `dedup` (free once content-addressed)
- [ ] **P2.5 — export:** `dig export` — reproducible, manifest-pinned, deduped, policy-filtered datasets (data layer for ML; trivial once store + labels exist)
- [ ] **P3 — drift + reconcile:** `drift` (desired vs actual) · `reconcile` (one-shot) · detect external human edits via scan-diff
- [ ] **P4 — parallel:** isolated work views · auto-merge of disjoint changesets
- [ ] **P5 — conflicts:** policy precedence · human escalation
- [ ] **P6 — harness:** `watch` (continuous observe + reconcile loop) · agent orchestration · escalation queue
- [ ] **P7 — public extensibility:** T0 event-sinks (backup) + T1 PATH subcommands · `dig ext` + manifest/registry · then T2 gRPC, T3 WASM + signing (see [docs/extensions.md](docs/extensions.md))
- [ ] **P8 — reach:** remote storage backends (SFTP, object storage) · opt-in AI extractor/classifier/search drivers

Note: the **extension seam interfaces** are defined from P0, not P7 — first-party backends (local store, FTS5 index, regex/OCR extractors) are themselves implementations of those interfaces, so the core is built extensible from the start. P7 only adds the *public plugin transports* (third-party code) on top of seams that already exist.

---

## Contributing

Early days — design feedback welcome, especially on the policy schema and the merge/escalation model. Open an issue before large changes so we can align on the changeset interface.

## License

[MIT](LICENSE).
