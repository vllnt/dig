import { describe, expect, it } from "vitest";

import {
  isCluster,
  listAllContent,
  listCluster,
  listSlugs,
  parseContent,
  readContent,
} from "@/lib/content";

describe("isCluster", () => {
  it("accepts known clusters", () => {
    expect(isCluster("compare")).toBe(true);
    expect(isCluster("learn")).toBe(true);
    expect(isCluster("use-cases")).toBe(true);
  });

  it("rejects anything else", () => {
    expect(isCluster("integrations")).toBe(false);
    expect(isCluster("nope")).toBe(false);
  });
});

describe("parseContent", () => {
  it("parses valid frontmatter into a typed entry", () => {
    const raw =
      '---\ntitle: Title\ndescription: Desc\nupdated: "2026-06-14"\n---\nThe body.';
    const parsed = parseContent(raw, "learn", "x");
    expect(parsed).toBeDefined();
    expect(parsed?.meta).toEqual({
      cluster: "learn",
      description: "Desc",
      slug: "x",
      title: "Title",
      updated: "2026-06-14",
    });
    expect(parsed?.body.trim()).toBe("The body.");
  });

  it("omits updated when absent", () => {
    const parsed = parseContent(
      "---\ntitle: T\ndescription: D\n---\nb",
      "learn",
      "x",
    );
    expect(parsed?.meta.updated).toBeUndefined();
  });

  it("returns undefined when the title is missing", () => {
    expect(
      parseContent("---\ndescription: D\n---\nb", "learn", "x"),
    ).toBeUndefined();
  });

  it("returns undefined when the description is missing", () => {
    expect(parseContent("---\ntitle: T\n---\nb", "learn", "x")).toBeUndefined();
  });

  it("returns undefined when a field is the wrong type", () => {
    expect(
      parseContent("---\ntitle: 5\ndescription: D\n---\nb", "learn", "x"),
    ).toBeUndefined();
  });
});

describe("readContent (real files)", () => {
  it("reads a seeded comparison page", () => {
    const entry = readContent("compare", "mem0");
    expect(entry?.meta.title).toContain("mem0");
    expect(entry?.body.length).toBeGreaterThan(0);
  });

  it("returns null for a missing slug", () => {
    expect(readContent("compare", "does-not-exist")).toBeUndefined();
  });
});

describe("listSlugs / listAllContent (real files)", () => {
  it("lists the seeded compare slugs, sorted", () => {
    const slugs = listSlugs("compare");
    expect(slugs).toContain("mem0");
    expect([...slugs]).toEqual([...slugs].sort());
  });

  it("returns every valid entry with required metadata", () => {
    const all = listAllContent();
    expect(all.length).toBeGreaterThanOrEqual(8);
    all.forEach((entry) => {
      expect(entry.title.length).toBeGreaterThan(0);
      expect(entry.description.length).toBeGreaterThan(0);
    });
  });
});

describe("listCluster (real files)", () => {
  it("returns only entries for the given cluster, sorted by slug", () => {
    const entries = listCluster("compare");
    expect(entries.length).toBeGreaterThan(0);
    entries.forEach((entry) => {
      expect(entry.cluster).toBe("compare");
    });
    const slugs = entries.map((entry) => entry.slug);
    expect([...slugs]).toEqual([...slugs].sort());
  });
});
