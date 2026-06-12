import { describe, expect, it } from "vitest";

import { COMMANDS, POLICY_SECTIONS, QUICKSTART } from "@/lib/docs";

describe("docs reference data", () => {
  it("covers the core lifecycle commands", () => {
    const usages = COMMANDS.map((c) => c.usage);
    const verbs = ["init", "scan", "find", "org", "reconcile", "undo"];
    const missing = verbs.filter(
      (verb) => !usages.some((u) => u.startsWith(`dig ${verb}`)),
    );
    expect(missing).toEqual([]);
  });

  it("gives every command a usage, summary, and detail", () => {
    const incomplete = COMMANDS.filter(
      (c) => !c.usage.startsWith("dig ") || !c.summary || !c.detail,
    );
    expect(incomplete).toEqual([]);
  });

  it("documents the three policy sections with keys", () => {
    expect(POLICY_SECTIONS.map((s) => s.header)).toEqual([
      "[[rule]]",
      "[dedup]",
      "[retrieval]",
    ]);
    const emptyKeys = POLICY_SECTIONS.filter((s) => s.keys.length === 0);
    expect(emptyKeys).toEqual([]);
  });

  it("keeps the quickstart a non-empty list of dig commands", () => {
    expect(QUICKSTART.length).toBeGreaterThan(0);
    const nonDig = QUICKSTART.filter((step) => !step.cmd.startsWith("dig "));
    expect(nonDig).toEqual([]);
  });
});
