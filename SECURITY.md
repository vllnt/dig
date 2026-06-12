# Security Policy

## Reporting a vulnerability

Please report security vulnerabilities **privately** — do not open a public issue.

Use GitHub's [private vulnerability reporting](https://github.com/vllnt/dig/security/advisories/new)
("Report a vulnerability" under the repository's **Security** tab). We aim to acknowledge
a report within a few days and to keep you updated as we investigate and fix.

When reporting, please include:

- the `dig` version (`dig --version`) and OS/arch,
- the exact steps or command sequence to reproduce,
- the impact you observed or expect.

## Scope

dig is **local-first and offline by default**: the deterministic core (store, index,
policy, dedupe, merge, undo) makes zero network calls. The most security-relevant areas:

- **Path handling** — policy templates and organize/reconcile must never write outside the
  KB root (path-escape is rejected at validation time).
- **The opt-in AI layer** — embeddings/extraction call a user-configured OpenAI-compatible
  endpoint. With `mode = off` or a localhost endpoint, no data leaves the machine. API keys
  are referenced by environment-variable name and never stored in the policy file.
- **Event sinks** — a KB's `policy.toml` may declare `[[event_sink]]` entries that
  fire on commit. **Webhook** sinks POST JSON to a URL (configured by you).
  **Exec** sinks run a shell command from the policy file and are therefore
  **off by default** — they run only when `DIG_ALLOW_EXEC_SINKS=1` is set, since a
  policy file travels with its KB and an untrusted KB could otherwise execute code
  on `dig scan`. Only enable exec sinks for KBs whose policy you trust.
- **Extensions** (planned) — out-of-tree backends/sinks run with declared capabilities;
  untrusted extensions are sandboxed (WASM) and signed.

## Supported versions

dig is pre-1.0 and under active development. Security fixes target the latest `main` and
the most recent release.
