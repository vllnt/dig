/**
 * User-docs reference data for the /docs page, synced from README.md and
 * docs/. Keep in sync with the CLI's actual surface (internal/cli) and the
 * policy schema (internal/policy).
 */

/** A CLI command and what it does. */
export type Command = {
  /** Notable flags / behavior. */
  detail: string;
  /** One-line description. */
  summary: string;
  /** Invocation, e.g. `dig find <query>`. */
  usage: string;
};

/** A policy section (a TOML table) and its keys. */
export type PolicySection = {
  /** TOML header, e.g. `[[rule]]` or `[retrieval]`. */
  header: string;
  /** Key/value descriptions. */
  keys: { describe: string; key: string }[];
  /** What the section configures. */
  summary: string;
};

/** A quick-start step. */
export type QuickStep = {
  /** Shell command. */
  cmd: string;
  /** What it does. */
  describe: string;
};

/** The first end-to-end journey, shown verbatim. */
export const QUICKSTART: readonly QuickStep[] = [
  {
    cmd: "dig init ~/library",
    describe: "Create a knowledge base at a directory.",
  },
  {
    cmd: "dig scan",
    describe: "Index files into the content-addressed store (+ search index).",
  },
  {
    cmd: 'dig find "invoice acme 2024"',
    describe: "Search, ranked. Add --mode hybrid for semantic recall.",
  },
  {
    cmd: "dig org --dry-run",
    describe:
      "Preview every move/rename/label your policy would apply — nothing is touched.",
  },
  { cmd: "dig org", describe: "Apply it; each change is journaled." },
  {
    cmd: "dig undo",
    describe: "Step back — the last changeset is reversed, disk and all.",
  },
];

/** The full command surface. */
export const COMMANDS: readonly Command[] = [
  {
    detail: "Writes a per-KB .dig/ directory (store, index, config).",
    summary: "Create a knowledge base at a directory.",
    usage: "dig init <root>",
  },
  {
    detail:
      "--dry-run previews. Rebuilds the search index; queues vectors when retrieval is on.",
    summary: "Index files into the content-addressed store.",
    usage: "dig scan",
  },
  {
    detail:
      "--mode fts|vector|hybrid (default fts, or the policy mode); --json; --limit.",
    summary: "Search the KB, ranked.",
    usage: "dig find <query>",
  },
  {
    detail: "Resumable, per-file commits. Needs a [retrieval] policy.",
    summary: "Drain the semantic-index backlog.",
    usage: "dig embed",
  },
  {
    detail:
      "--at <manifest> pins a point in time; --filter by label/path/date. JSONL with provenance.",
    summary: "Emit a reproducible, manifest-pinned dataset.",
    usage: "dig export",
  },
  {
    detail:
      "--dry-run previews the full plan; conflicts are reported, never forced.",
    summary: "Apply organization policy (move/rename/label).",
    usage: "dig org",
  },
  {
    detail: "--dry-run previews. Never deletes the last copy; ties escalate.",
    summary: "Collapse duplicates per policy.",
    usage: "dig dedup",
  },
  {
    detail: "--json. Surfaces misfiled/misnamed/duplicated/unsorted/pinned.",
    summary: "Report divergence from policy + external edits.",
    usage: "dig drift",
  },
  {
    detail:
      "Auto where rules allow; human moves are pinned and escalated, never overwritten.",
    summary: "Converge the KB to policy, one-shot.",
    usage: "dig reconcile",
  },
  {
    detail:
      "--interval. Drains the semantic backlog per tick. Ctrl-C is a clean stop.",
    summary: "Run continuously: observe, reconcile, escalate.",
    usage: "dig watch",
  },
  {
    detail: "Worktree-like; disjoint changesets merge back automatically.",
    summary: "Open an isolated work view.",
    usage: "dig work <create|list|abort>",
  },
  {
    detail: "Auto-resolves compatible ops; conflicts escalate surgically.",
    summary: "Merge a work view back.",
    usage: "dig merge <name>",
  },
  {
    detail: "Explains rule matches; unknown keys and path-escapes fail loudly.",
    summary: "Lint the policy file.",
    usage: "dig policy validate",
  },
  {
    detail: "--json. Newest first.",
    summary: "Browse change history.",
    usage: "dig log",
  },
  {
    detail:
      "Disk mutations (org/dedup) are reversed; undoing a scan only rewinds history.",
    summary: "Revert the last changeset.",
    usage: "dig undo",
  },
];

/** The policy file reference (.dig/policy.toml). */
export const POLICY_SECTIONS: readonly PolicySection[] = [
  {
    header: "[[rule]]",
    keys: [
      { describe: "Unique rule name.", key: "name" },
      {
        describe:
          "Conditions (all must hold): ext, mime, path glob, content_matches regex, size/date.",
        key: "match",
      },
      {
        describe:
          "Target directory template, KB-root-relative. Vars: {year} {month} {day} {name} {ext}.",
        key: "into",
      },
      { describe: "Target filename template.", key: "rename" },
      { describe: "Labels to apply (accumulate across rules).", key: "label" },
      {
        describe:
          '"" | "auto" | "propose" — in watch, only "auto" rules act unattended.',
        key: "autonomy",
      },
    ],
    summary:
      "Map matching files to a target folder, name, and labels. At least one of into/rename/label is required.",
  },
  {
    header: "[dedup]",
    keys: [
      { describe: "keep-oldest | keep-newest.", key: "strategy" },
      {
        describe: "escalate (default) — never silently delete.",
        key: "on_conflict",
      },
    ],
    summary: "Configure duplicate collapsing.",
  },
  {
    header: "[retrieval]",
    keys: [
      { describe: "off (default) | hybrid | vector.", key: "mode" },
      {
        describe: "Any OpenAI-compatible /embeddings endpoint.",
        key: "base_url",
      },
      { describe: "Embedding model name.", key: "model" },
      {
        describe: "Model task prefixes (model-specific, optional).",
        key: "doc_prefix / query_prefix",
      },
      {
        describe:
          "Env var holding the bearer token — keys never live in the file.",
        key: "api_key_env",
      },
      {
        describe:
          "Tuning knobs (0 = default): rrf_k (60), candidate_factor (4), chunk_size (1000), chunk_overlap (200). Changing chunk size/overlap re-embeds the KB.",
        key: "rrf_k / chunk_size / …",
      },
    ],
    summary:
      "Opt-in semantic retrieval. Off by default — find stays deterministic FTS.",
  },
];
