# @vllnt/dig

TypeScript client for a local [dig](https://github.com/vllnt/dig) daemon — search,
organize, reconcile, and export a knowledge base over HTTP. Dependency-free
(native `fetch`), local-first.

## Install

```sh
npm i @vllnt/dig
```

Start a daemon next to your KB (dig binary from https://dig.vllnt.com):

```sh
dig serve            # binds 127.0.0.1:3978 (loopback only)
```

## Use

```ts
import { DigClient } from "@vllnt/dig";

const dig = new DigClient(); // defaults to http://127.0.0.1:3978

// search — fts (default), vector, or hybrid (semantic)
const hits = await dig.find("invoice acme 2024", { mode: "hybrid", limit: 5 });

// agent memory — capture, then recall a token-budgeted pack
await dig.retain(sessionMarkdown, { as: "memory/sessions/today.md" });
const pack = await dig.recall("billing ledger decision", { budget: 1000 });

// reorganize by policy — preview, then apply (reversible)
await dig.org({ apply: false }); // preview the plan
await dig.org({ apply: true }); // commit it
await dig.undo(); // step back

// reproducible dataset export (JSONL text)
const jsonl = await dig.export({ filter: "label:finance" });

// read-only inspection
await dig.drift();
await dig.log();
```

Target a specific KB with `{ kb: "/path/or/name" }` on any call; omit it to use
the KB at the daemon's working directory. Errors throw a `DigError` carrying the
HTTP status.

## API

| Method | HTTP | Notes |
|--------|------|-------|
| `find(query, opts)` | GET /find | `mode`, `limit` |
| `recall(query, opts)` | GET /recall | `mode`, `budget`; token-budgeted memory pack |
| `retain(content, opts)` | POST /retain | `as`; capture into memory (reversible) |
| `drift(opts)` | GET /drift | read-only |
| `log(opts)` | GET /log | read-only |
| `export(opts)` | GET /export | `filter`, `at`; returns JSONL string |
| `org(opts)` | POST /org | `apply` (default preview) |
| `reconcile(opts)` | POST /reconcile | `apply` (default preview) |
| `undo(opts)` | POST /undo | reverts the last changeset |
| `health()` | GET /health | daemon liveness + version |

## Vercel AI SDK

`@vllnt/dig/ai` turns a client into AI SDK tools an agent can call (`ai` + `zod`
are optional peer deps):

```ts
import { generateText } from "ai";
import { DigClient } from "@vllnt/dig";
import { digTools } from "@vllnt/dig/ai";

const dig = new DigClient();
await generateText({
  model,
  prompt: "What invoices are in my KB? Organize them if needed.",
  // dig_find, dig_recall, dig_retain, dig_drift, dig_log, dig_export, dig_org, dig_reconcile, dig_undo
  tools: digTools(dig),
});
```

`dig_recall` + `dig_retain` give the agent memory: it can write a decision to
the KB and load a token-budgeted pack back on a later turn.

```ts
await generateText({ model, prompt, tools: digTools(dig) });
// the agent calls dig_retain to remember, dig_recall to recall — reversibly
```

Mutating tools (`dig_org`, `dig_reconcile`) preview by default; the agent passes
`apply: true` to commit, and every change is reversible via `dig_undo`.

The client speaks the same contract as `dig serve`, which is a thin adapter over
the dig CLI — so the SDK never drifts from the tool.
