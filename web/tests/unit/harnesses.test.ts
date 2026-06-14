import { describe, expect, it } from "vitest";

import {
  DIG_MCP_SERVER,
  getHarness,
  HARNESS_SLUGS,
  harnessDocumentUrl,
  HARNESSES,
  HARNESSES_BY_STATUS,
  MCP_CONFIG_JSON,
} from "@/lib/harnesses";

describe("harness registry", () => {
  it("has unique, url-safe slugs", () => {
    const slugs = HARNESSES.map((h) => h.slug);
    expect(new Set(slugs).size).toBe(slugs.length);
    slugs.forEach((slug) => {
      expect(slug).toMatch(/^[\da-z-]+$/);
    });
  });

  it("ships the harnesses the selector promises", () => {
    expect(HARNESS_SLUGS).toEqual(
      expect.arrayContaining([
        "claude-code",
        "codex",
        "cursor",
        "mcp",
        "ai-sdk",
      ]),
    );
  });

  it("every harness has a name, an unlocks line, and install steps", () => {
    HARNESSES.forEach((h) => {
      expect(h.name.length).toBeGreaterThan(0);
      expect(h.unlocks.length).toBeGreaterThan(0);
      expect(h.install.length).toBeGreaterThan(0);
      expect(
        h.install.some(
          (line) => line.type === "command" || line.type === "output",
        ),
      ).toBe(true);
    });
  });

  it("every MCP-capable harness registers the canonical dig server", () => {
    HARNESSES.filter((h) => h.mcp).forEach((h) => {
      expect(h.mcp).toEqual(DIG_MCP_SERVER);
    });
  });

  it("WIP harnesses fall back to MCP", () => {
    HARNESSES.filter((h) => h.status === "wip").forEach((h) => {
      expect(h.mcp).toEqual(DIG_MCP_SERVER);
    });
  });
});

describe("HARNESSES_BY_STATUS", () => {
  it("lists every harness, stable before wip", () => {
    expect(HARNESSES_BY_STATUS.length).toBe(HARNESSES.length);
    const firstWip = HARNESSES_BY_STATUS.findIndex((h) => h.status === "wip");
    const lastStable = HARNESSES_BY_STATUS.map((h) => h.status).lastIndexOf(
      "stable",
    );
    if (firstWip !== -1) expect(firstWip).toBeGreaterThan(lastStable);
  });
});

describe("getHarness", () => {
  it("resolves a known slug", () => {
    expect(getHarness("claude-code")?.name).toBe("Claude Code");
  });

  it("returns undefined for an unknown slug", () => {
    expect(getHarness("nope")).toBeUndefined();
  });
});

describe("harnessDocumentUrl", () => {
  it("builds an absolute repo URL on main", () => {
    const claude = getHarness("claude-code");
    expect(claude).toBeDefined();
    if (!claude) return;
    expect(harnessDocumentUrl(claude)).toBe(
      "https://github.com/vllnt/dig/blob/main/.claude-plugin",
    );
  });
});

describe("MCP_CONFIG_JSON", () => {
  it("is valid JSON registering dig over stdio", () => {
    const parsed = JSON.parse(MCP_CONFIG_JSON);
    expect(parsed.mcpServers.dig).toEqual({ args: ["mcp"], command: "dig" });
  });
});
