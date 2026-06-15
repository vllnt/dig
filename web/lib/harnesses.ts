/**
 * The harness registry — one typed source of truth for every agent/editor that
 * can drive dig. Powers the homepage selector, the `/integrations` hub, each
 * `/integrations/[slug]` page, and the sitemap. Every entry maps to something
 * that ships in the repo today (`.claude-plugin/`, `.cursor/`, `AGENTS.md`,
 * `GEMINI.md`, `dig mcp`, the SDKs) — no aspirational integrations.
 *
 * Keep facts in sync with `docs/integration.md` and `.claude-plugin/plugin.json`.
 */

import { GITHUB_URL } from "@/lib/site";

/** Whether a harness integration ships today or is still landing. */
export type HarnessStatus = "stable" | "wip";

/** A single line in an install snippet, rendered by the `Terminal` component. */
export type InstallLine = {
  /** The literal text. */
  content: string;
  /** `command` = typed/run, `comment` = annotation, `output` = result/config. */
  type: "command" | "comment" | "output";
};

/** An MCP stdio server registration (the universal entry for MCP clients). */
export type McpServer = {
  /** Arguments passed to it. */
  args: readonly string[];
  /** Executable to spawn. */
  command: string;
};

/** One agent harness and how it drives dig. */
export type Harness = {
  /** Where the in-repo shim / contract lives, for "learn more". */
  docPath: string;
  /** The 10-second install, as terminal lines. */
  install: readonly InstallLine[];
  /** The `dig mcp` server config, when this harness consumes MCP. */
  mcp?: McpServer;
  /** Display name, e.g. `Claude Code`. */
  name: string;
  /** URL + lookup key, e.g. `claude-code`. */
  slug: string;
  /** Does the integration ship today, or is it WIP (MCP fallback shown)? */
  status: HarnessStatus;
  /** One line: what plugging dig in unlocks for this harness. */
  unlocks: string;
};

/** The canonical `dig mcp` stdio server — the universal entry for MCP clients. */
export const DIG_MCP_SERVER: McpServer = {
  args: ["mcp"],
  command: "dig",
} as const;

/**
 * The MCP config block most clients accept (Claude, Cursor, Codex, …). Shown
 * verbatim as the fallback for any MCP-capable harness.
 */
export const MCP_CONFIG_JSON = JSON.stringify(
  { mcpServers: { dig: DIG_MCP_SERVER } },
  undefined,
  2,
);

const HARNESSES_DATA = [
  {
    docPath: "/.claude-plugin",
    install: [
      {
        content:
          "add dig to Claude Code — installs the skill + the dig mcp server",
        type: "comment",
      },
      { content: "claude plugin marketplace add vllnt/dig", type: "command" },
      { content: "claude plugin install dig@dig", type: "command" },
    ],
    mcp: DIG_MCP_SERVER,
    name: "Claude Code",
    slug: "claude-code",
    status: "stable",
    unlocks:
      "Two CLI commands add the dig plugin to Claude Code — bundling the dig skill, the `dig mcp` server, and a SessionEnd hook that captures finished sessions into memory.",
  },
  {
    docPath: "/.codex-plugin",
    install: [
      {
        content: "add dig to Codex — installs the skill + the dig mcp server",
        type: "comment",
      },
      { content: "codex plugin marketplace add vllnt/dig", type: "command" },
    ],
    mcp: DIG_MCP_SERVER,
    name: "Codex",
    slug: "codex",
    status: "stable",
    unlocks:
      "One CLI command adds the dig plugin to Codex — it bundles the dig skill and the `dig mcp` server, ready to use with no extra config.",
  },
  {
    docPath: "/.cursor/rules/dig.mdc",
    install: [
      {
        content:
          "Cursor: add the dig MCP server (Settings → MCP, or ~/.cursor/mcp.json)",
        type: "comment",
      },
      {
        content:
          '{ "mcpServers": { "dig": { "command": "dig", "args": ["mcp"] } } }',
        type: "output",
      },
    ],
    mcp: DIG_MCP_SERVER,
    name: "Cursor",
    slug: "cursor",
    status: "stable",
    unlocks:
      "A `.cursor/rules/dig.mdc` rule points the agent at the portable dig skill; register the `dig mcp` server to give Cursor the full tool surface.",
  },
  {
    docPath: "/docs/integration.md",
    install: [
      { content: "Claude Code or Codex — one command:", type: "comment" },
      { content: "claude mcp add dig -- dig mcp", type: "command" },
      { content: "codex mcp add dig -- dig mcp", type: "command" },
      { content: "", type: "output" },
      { content: "any other MCP client — config:", type: "comment" },
      {
        content:
          '{ "mcpServers": { "dig": { "command": "dig", "args": ["mcp"] } } }',
        type: "output",
      },
    ],
    mcp: DIG_MCP_SERVER,
    name: "Any MCP client",
    slug: "mcp",
    status: "stable",
    unlocks:
      "One stdio server — `dig mcp` — exposes find, recall, retain, drift, log, export, org, reconcile, and undo as MCP tools to any compatible client.",
  },
  {
    docPath: "/GEMINI.md",
    install: [
      {
        content: "Gemini reads GEMINI.md; register the dig MCP server too",
        type: "comment",
      },
      {
        content:
          '{ "mcpServers": { "dig": { "command": "dig", "args": ["mcp"] } } }',
        type: "output",
      },
    ],
    mcp: DIG_MCP_SERVER,
    name: "Gemini CLI",
    slug: "gemini",
    status: "stable",
    unlocks:
      "A `GEMINI.md` entry doc points the Gemini CLI at the portable dig skill and the same MCP / daemon surface every other harness uses.",
  },
  {
    docPath: "/clients/typescript",
    install: [
      {
        content: "install the SDK, expose dig as AI SDK tools",
        type: "comment",
      },
      { content: "npm i @vllnt/dig@canary", type: "command" },
      { content: "", type: "output" },
      { content: 'import { digTools } from "@vllnt/dig/ai"', type: "output" },
    ],
    name: "Vercel AI SDK",
    slug: "ai-sdk",
    status: "stable",
    unlocks:
      "`@vllnt/dig/ai` exports `digTools(client)` — typed AI SDK tool defs (incl. `dig_recall` / `dig_retain`) so the model reads and writes dig as memory.",
  },
  {
    docPath: "/skills/dig/SKILL.md",
    install: [
      {
        content: "pi shim is landing — drive dig over MCP meanwhile",
        type: "comment",
      },
      {
        content:
          '{ "mcpServers": { "dig": { "command": "dig", "args": ["mcp"] } } }',
        type: "output",
      },
    ],
    mcp: DIG_MCP_SERVER,
    name: "Pi",
    slug: "pi",
    status: "wip",
    unlocks:
      "A pi.dev package pointing at the portable dig skill is in progress. Until it lands, any pi agent drives dig through the `dig mcp` server.",
  },
  {
    docPath: "/skills/dig/SKILL.md",
    install: [
      {
        content: "opencode shim is landing — drive dig over MCP meanwhile",
        type: "comment",
      },
      {
        content:
          '{ "mcpServers": { "dig": { "command": "dig", "args": ["mcp"] } } }',
        type: "output",
      },
    ],
    mcp: DIG_MCP_SERVER,
    name: "opencode",
    slug: "opencode",
    status: "wip",
    unlocks:
      "A thin opencode pointer to the portable dig skill is in progress. opencode speaks MCP today, so `dig mcp` gives it the full surface now.",
  },
] as const satisfies readonly Harness[];

/** Every harness in the registry (widened so optional fields are accessible). */
export const HARNESSES: readonly Harness[] = HARNESSES_DATA;

const statusRank = (status: HarnessStatus): number =>
  status === "stable" ? 0 : 1;

/** Every harness, in display order (stable first, then WIP). */
export const HARNESSES_BY_STATUS: readonly Harness[] = [...HARNESSES].sort(
  (a, b) => statusRank(a.status) - statusRank(b.status),
);

/** The harness shown before the visitor picks one (first registry entry). */
export const DEFAULT_HARNESS: Harness = HARNESSES_DATA[0];

/** All harness slugs — for `generateStaticParams` and the sitemap. */
export const HARNESS_SLUGS: readonly string[] = HARNESSES.map((h) => h.slug);

/**
 * Resolve a harness by slug.
 *
 * @param slug - a harness slug (e.g. `claude-code`)
 * @returns the harness, or `undefined` if no such slug exists
 */
export function getHarness(slug: string): Harness | undefined {
  return HARNESSES.find((h) => h.slug === slug);
}

/** Absolute repo URL for a harness's in-repo shim / doc. */
export function harnessDocumentUrl(harness: Harness): string {
  return `${GITHUB_URL}/blob/main${harness.docPath}`;
}
