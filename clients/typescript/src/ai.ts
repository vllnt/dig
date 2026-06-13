/**
 * Vercel AI SDK helpers for @dig/client. {@link digTools} turns a
 * {@link DigClient} into a set of AI SDK tools an agent can call directly —
 * search, inspect, and (preview-gated) reorganize a dig knowledge base.
 *
 * `ai` and `zod` are optional peer dependencies; import this entry only when
 * you use the AI SDK.
 *
 * @example
 * import { generateText } from "ai";
 * import { DigClient } from "@dig/client";
 * import { digTools } from "@dig/client/ai";
 *
 * const dig = new DigClient();
 * await generateText({ model, prompt, tools: digTools(dig) });
 */
import { tool, type Tool } from "ai";
import { z } from "zod";

import type { DigClient } from "./index.ts";

const kb = z
  .string()
  .optional()
  .describe("KB name or path; omit to use the daemon's working-directory KB");

/**
 * Build the dig tool set for the Vercel AI SDK from a client. Mutating tools
 * (org, reconcile) preview by default — pass apply: true to commit; every
 * applied change is reversible with the undo tool.
 *
 * @param client - a DigClient pointed at a running `dig serve`
 * @returns a record of AI SDK tools keyed by name
 */
export function digTools(client: DigClient): Record<string, Tool> {
  return {
    dig_find: tool({
      description:
        "Search a dig knowledge base, ranked. mode is fts (default), vector, or hybrid (semantic).",
      parameters: z.object({
        kb,
        query: z.string().describe("the search query"),
        mode: z.enum(["fts", "vector", "hybrid"]).optional(),
        limit: z.number().int().positive().optional(),
      }),
      execute: async ({ query, ...rest }) => client.find(query, rest),
    }),
    dig_recall: tool({
      description:
        "Load a token-budgeted, provenance-tagged context pack from a dig KB for a query — dig as agent memory. mode is fts (default), vector, or hybrid; budget caps the pack in tokens.",
      parameters: z.object({
        kb,
        query: z.string().describe("what to recall"),
        mode: z.enum(["fts", "vector", "hybrid"]).optional(),
        budget: z.number().int().positive().optional().describe("token budget"),
      }),
      execute: async ({ query, ...rest }) => client.recall(query, rest),
    }),
    dig_retain: tool({
      description:
        "Capture content (a decision, a fact, a session) into a dig KB and index it — write to agent memory. Defaults to a dated memory/ path; pass as to choose it. Reversible with dig_undo.",
      parameters: z.object({
        kb,
        content: z.string().describe("the text to remember"),
        as: z.string().optional().describe("target path in the KB"),
      }),
      execute: async ({ content, ...rest }) => client.retain(content, rest),
    }),
    dig_drift: tool({
      description:
        "Report how a dig KB diverges from its policy (misfiled, misnamed, duplicated, unsorted). Read-only.",
      parameters: z.object({ kb }),
      execute: async (args) => client.drift(args),
    }),
    dig_log: tool({
      description: "Browse a dig KB's change history, newest first. Read-only.",
      parameters: z.object({ kb }),
      execute: async (args) => client.log(args),
    }),
    dig_export: tool({
      description:
        "Export a reproducible, provenance-tagged dataset (JSONL) from a dig KB. Read-only.",
      parameters: z.object({
        kb,
        filter: z.string().optional().describe("e.g. 'label:finance path:*.pdf'"),
        at: z.string().optional().describe("pin to a manifest id like M3"),
      }),
      execute: async (args) => client.export(args),
    }),
    dig_org: tool({
      description:
        "Apply organization policy (move/rename/label) to a dig KB. Previews unless apply is true (reversible with dig_undo).",
      parameters: z.object({ kb, apply: z.boolean().optional() }),
      execute: async (args) => client.org(args),
    }),
    dig_reconcile: tool({
      description:
        "Converge a dig KB to its policy. Previews unless apply is true (reversible with dig_undo).",
      parameters: z.object({ kb, apply: z.boolean().optional() }),
      execute: async (args) => client.reconcile(args),
    }),
    dig_undo: tool({
      description: "Revert the last changeset in a dig KB.",
      parameters: z.object({ kb }),
      execute: async (args) => client.undo(args),
    }),
  };
}
