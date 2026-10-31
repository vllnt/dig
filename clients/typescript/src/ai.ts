/**
 * Vercel AI SDK helpers for @vllnt/dig. {@link digTools} turns a
 * {@link DigClient} into a set of AI SDK tools an agent can call directly —
 * search, inspect, and (preview-gated) reorganize a dig knowledge base.
 *
 * `ai` and `zod` are optional peer dependencies; import this entry only when
 * you use the AI SDK.
 *
 * @example
 * import { generateText } from "ai";
 * import { DigClient } from "@vllnt/dig";
 * import { digTools } from "@vllnt/dig/ai";
 *
 * const dig = new DigClient();
 * await generateText({ model, prompt, tools: digTools(dig) });
 */
import type { Tool } from "ai";
import { z } from "zod";

import type { DigClient } from "./index.ts";

const kb = z
  .string()
  .optional()
  .describe("KB name or path; omit to use the daemon's working-directory KB");

/**
 * Define one tool. ai 4 reads the schema from `parameters`, ai 5 from
 * `inputSchema`; `tool()` is an identity function in both majors, so a plain
 * object carrying both keys works on either without a version switch.
 */
function define<S extends z.ZodTypeAny>(
  description: string,
  schema: S,
  execute: (args: z.infer<S>) => Promise<unknown>,
): Tool {
  return { description, parameters: schema, inputSchema: schema, execute } as unknown as Tool;
}

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
    dig_find: define(
      "Search a dig knowledge base, ranked. mode is fts (default), vector, or hybrid (semantic).",
      z.object({
        kb,
        query: z.string().describe("the search query"),
        mode: z.enum(["fts", "vector", "hybrid"]).optional(),
        limit: z.number().int().positive().optional(),
      }),
      async ({ query, ...rest }) => client.find(query, rest),
    ),
    dig_recall: define(
      "Load a token-budgeted, provenance-tagged context pack from a dig KB for a query — dig as agent memory. mode is fts (default), vector, or hybrid; budget caps the pack in tokens.",
      z.object({
        kb,
        query: z.string().describe("what to recall"),
        mode: z.enum(["fts", "vector", "hybrid"]).optional(),
        budget: z.number().int().positive().optional().describe("token budget"),
      }),
      async ({ query, ...rest }) => client.recall(query, rest),
    ),
    dig_retain: define(
      "Capture content (a decision, a fact, a session) into a dig KB and index it — write to agent memory. Defaults to a dated memory/ path; pass as to choose it. Reversible with dig_undo.",
      z.object({
        kb,
        content: z.string().describe("the text to remember"),
        as: z.string().optional().describe("target path in the KB"),
      }),
      async ({ content, ...rest }) => client.retain(content, rest),
    ),
    dig_drift: define(
      "Report how a dig KB diverges from its policy (misfiled, misnamed, duplicated, unsorted). Read-only.",
      z.object({ kb }),
      async (args) => client.drift(args),
    ),
    dig_log: define(
      "Browse a dig KB's change history, newest first. Read-only.",
      z.object({ kb }),
      async (args) => client.log(args),
    ),
    dig_export: define(
      "Export a reproducible, provenance-tagged dataset (JSONL) from a dig KB. Read-only.",
      z.object({
        kb,
        filter: z.string().optional().describe("e.g. 'label:finance path:*.pdf'"),
        at: z.string().optional().describe("pin to a manifest id like M3"),
      }),
      async (args) => client.export(args),
    ),
    dig_org: define(
      "Apply organization policy (move/rename/label) to a dig KB. Previews unless apply is true (reversible with dig_undo).",
      z.object({ kb, apply: z.boolean().optional() }),
      async (args) => client.org(args),
    ),
    dig_reconcile: define(
      "Converge a dig KB to its policy. Previews unless apply is true (reversible with dig_undo).",
      z.object({ kb, apply: z.boolean().optional() }),
      async (args) => client.reconcile(args),
    ),
    dig_undo: define(
      "Revert the last changeset in a dig KB.",
      z.object({ kb }),
      async (args) => client.undo(args),
    ),
  };
}
