# Extension system

dig's goal is to be the **pi.dev of KB management**: a small, sharp core with a rich ecosystem around it. Where a company needs to store blobs elsewhere, back up on every change, parse a proprietary format, or add a verb — that's an extension, not a fork.

> pi.dev's *mechanism* (in-process TypeScript modules) does not port to a Go single binary — native Go plugins (`.so`) are version-locked, Linux/macOS-only, and unsandboxed. dig keeps pi.dev's *spirit* (tiny core + ecosystem, manifest-described packages, a registry) with a Go-appropriate, **two-axis** design: typed extension points × tiered transports.

---

## Two axes

```
   WHAT you can extend (typed seams)         HOW it plugs in (transport tiers)
   ─────────────────────────────────        ──────────────────────────────────
   StorageBackend  where blobs live          T0  declarative — TOML + exec/webhook (no code)
   EventSink       react to changes          T1  PATH subcommand  dig-<name>  (any language)
   Extractor       read content (OCR/parse)  T2  gRPC subprocess  (hashicorp/go-plugin)
   Matcher         custom file matching       T3  WASM via wazero  (sandboxed, cap-gated, pure-Go)
   Action          new workflow step
   Command         new `dig <verb>`
   IndexBackend    where the index lives
   LLMProvider     model endpoint (OpenAI-compat — already an interface)
```

You pick the **point** by *what* you're adding and the **tier** by *how much trust/robustness* it needs. The same `StorageBackend` interface can be satisfied by a webhook (T0), a Go binary over gRPC (T2), or a WASM module (T3).

---

## Extension points (the seams)

Each is a small Go interface in the core. First-party implementations are compiled in; third-party ones arrive via a transport.

| Point | Interface (sketch) | Answers |
|-------|--------------------|---------|
| **StorageBackend** | `Put(hash, reader) · Get(hash) · Has(hash) · Delete(hash)` | "save data elsewhere" — S3, GCS, Azure, NAS, IPFS, a company blob store |
| **EventSink** | `OnChangeset(event) error` | "backup system", notify, audit log, webhook, mirror-to-cloud, SIEM |
| **Extractor** | `Extract(file) → fields/text` | new file types, OCR engines, proprietary parsers |
| **Matcher** | `Match(file, args) → bool` | custom selection logic a rule can call |
| **Action** | `Apply(ctx, files) → ops` | new workflow step (`{ encrypt = … }`, `{ transcode = … }`) |
| **Command** | `Run(args) → exit` | brand-new top-level verb `dig <name>` |
| **IndexBackend** | `Index(manifest) · Query(q)` | swap FTS5 for an external search/vector engine |
| **LLMProvider** | OpenAI-compatible HTTP | already pluggable via `[llm] base_url` |

**Design rule:** a company's two stated needs — *store elsewhere* and *back up* — are **one interface each** (`StorageBackend`, `EventSink`), not a bespoke plugin. Most real needs collapse onto these eight seams; resist a generic "do-anything" plugin API.

---

## Transport tiers (how an extension runs)

Ordered by friction/trust. Build them lazily — most extensions never need more than T0/T1.

### T0 — Declarative (no code)
A rule/workflow step references an external command or webhook in config. Covers the long tail of "on change, do X."
```toml
[[event_sink]]
name = "offsite-backup"
on   = "changeset.committed"
exec = "restic backup {changed_paths}"   # or  url = "https://hooks.internal/dig"
```
- **Isolation:** OS process / network. **Trust:** you wrote the command. **Use for:** backups, notifications, mirrors.
- **`exec` sinks are gated:** they run a shell command, so dig only fires them when `DIG_ALLOW_EXEC_SINKS=1` is set (default-deny code execution). `url` webhook sinks need no gate. Sinks *observe* — a sink failure warns, it never rolls back the changeset. (Shipped — see `SECURITY.md`.)

### T1 — PATH subcommand (git-style)
Any executable named `dig-<name>` on `$PATH` becomes `dig <name>`. Zero protocol, any language.
- dig passes context via env + `--json` on stdin; the subcommand emits a changeset proposal on stdout.
- **Isolation:** separate process. **Trust:** installed by user. **Use for:** custom verbs, glue, prototypes.

### T2 — gRPC subprocess (hashicorp/go-plugin)
The Terraform/Vault provider model: the extension is a long-lived binary; dig talks gRPC over a handshake. Typed, versioned, robust; a crash can't take down dig.
- **Isolation:** subprocess, can't corrupt host memory. **Trust:** signed/declared. **Use for:** production `StorageBackend` / `IndexBackend` / heavy `Extractor`.

### T3 — WASM (wazero)
A capability-sandboxed module run by a **pure-Go** runtime (wazero — keeps the cgo-free promise). No filesystem/network unless dig grants it.
- **Isolation:** strongest; deny-by-default capabilities. **Trust:** untrusted/community. **Use for:** third-party extractors/matchers from a public registry.

| Tier | Isolation | Lang | Single-binary preserved | Build when |
|------|-----------|------|--------------------------|-----------|
| T0 declarative | process/net | any | yes | day one |
| T1 PATH subcommand | process | any | yes (separate exe) | day one (free) |
| T2 gRPC subprocess | subprocess | Go + gRPC langs | host stays one binary | first real out-of-tree backend |
| T3 WASM | sandbox (cap-gated) | many → wasm | yes (wazero embedded) | untrusted third-party code is real |

---

## Capability & security model

Untrusted extensions are guilty until granted. Every non-first-party extension declares the capabilities it needs; dig enforces them and the user approves on install.

```
capabilities: [ storage:write, net:hooks.internal, read:finance/** ]
```
- **Default deny:** no FS, no net, no exec unless declared (enforced hard at T3, by convention/audit at T0–T2).
- **Scoped to KB:** an extension on KB "work" can't touch KB "research".
- **Through the same spine:** an extension can only ever *propose a changeset*. It never writes files directly — so every extension action is dry-run-able, journaled, and `undo`-able like everything else. **This is the safety guarantee that makes a plugin ecosystem tolerable.**
- **Signing:** registry packages are checksummed; signature verification gates install for T2/T3.

---

## Manifest

Every extension (above T0) ships a manifest — pi.dev-style, describing what it is and what it may do.
```toml
# dig-ext.toml
name        = "s3-store"
version     = "0.2.0"
provides    = "StorageBackend"   # which seam
transport   = "grpc"             # t0 | path | grpc | wasm
entry       = "./dig-s3-store"   # binary / wasm / command
capabilities = ["net:s3.amazonaws.com", "storage:write"]
config_schema = "schema.json"    # validated at install
```

---

## Managing extensions (CLI)

```bash
dig ext list                       # installed, per KB + machine-wide
dig ext add  github.com/acme/dig-s3-store      # from git
dig ext add  oci://registry/dig-backup:1.0     # from a registry
dig ext info s3-store               # manifest, capabilities, signature
dig ext enable  s3-store --kb work  # grant + activate for a KB
dig ext disable s3-store --kb work
dig ext remove  s3-store
```
- Extensions install **per-machine**, are **enabled per-KB** (capabilities granted at enable time).
- Registry distribution mirrors pi.dev: git or an OCI/registry reference; a public catalog later.

---

## What stays in core (never an extension)

The spine cannot be swapped, or the safety guarantees evaporate:
- content-addressed store **semantics** (hashing, manifests, journal) — backends plug in for *where bytes live*, not *how history works*
- the changeset state machine + merge/escalation
- `--dry-run` / `undo`
- the policy/rules/workflow evaluator

Extensions extend the *edges* (storage, events, extraction, matching, verbs, index, model) — never the *spine*.

---

## Phasing (seams first, marketplace last)

These X-phases are extension-specific sub-tracks of the core roadmap in the README: **X0 lands with core P0** (the seams are defined as the core is built — first-party backends *are* the interfaces), and **X1–X4 are the public-transport work grouped under README P7**.

- [x] **X0 — seams (with P0):** the Go interfaces (`StorageBackend`, `EventSink`, `IndexBackend`, …) are defined and first-party impls are compiled in (local FS store, FTS5 + vector index). Shipped.
- [~] **X1 — T0 + T1:** **T0 declarative event sinks (webhook + `DIG_ALLOW_EXEC_SINKS`-gated exec) are shipped**; T1 PATH subcommands (`dig-<name>`) are still to come.
- [ ] **X2 — registry:** `dig ext` commands + manifest + install-from-git.
- [ ] **X3 — T2 gRPC:** first robust out-of-tree backend (e.g. a company blob store).
- [ ] **X4 — T3 WASM + signing:** sandboxed third-party extensions; public catalog.

The two motivating use cases (store elsewhere, backup) land at **X1** — no gRPC/WASM needed. The heavy machinery is built only when the ecosystem demands it.

## References

- [pi.dev packages](https://pi.dev/packages) — the tiny-core-plus-ecosystem model dig mirrors
- [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) — gRPC subprocess plugins (Terraform/Vault)
- [Extism](https://extism.org/) / [wazero](https://wazero.io/) — WASM plugin runtime (pure-Go host)
- [git custom subcommands](https://git.github.io/htmldocs/howto/new-command.html) — PATH-based extension precedent
- [gocloud.dev/blob](https://gocloud.dev/howto/blob/) — first-party `StorageBackend` implementations
