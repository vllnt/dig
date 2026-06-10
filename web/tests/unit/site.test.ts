import { afterEach, describe, expect, it, vi } from "vitest";

import { buildCanonicalUrl, GITHUB_URL, SITE_URL } from "@/lib/site";

describe("buildCanonicalUrl", () => {
  it("returns the bare origin for the default locale root", () => {
    expect(buildCanonicalUrl("en", "/")).toBe(SITE_URL);
  });

  it("appends the path for the default locale", () => {
    expect(buildCanonicalUrl("en", "/docs")).toBe(`${SITE_URL}/docs`);
  });

  it("prefixes non-default locales at the root", () => {
    expect(buildCanonicalUrl("fr", "/")).toBe(`${SITE_URL}/fr`);
  });

  it("prefixes non-default locales before the path", () => {
    expect(buildCanonicalUrl("fr", "/docs")).toBe(`${SITE_URL}/fr/docs`);
  });

  it("defaults the path to the root", () => {
    expect(buildCanonicalUrl("en")).toBe(SITE_URL);
  });
});

function restoreSiteEnvironment(
  siteUrl: string | undefined,
  port: string | undefined,
) {
  if (siteUrl === undefined) {
    delete process.env.NEXT_PUBLIC_SITE_URL;
  } else {
    process.env.NEXT_PUBLIC_SITE_URL = siteUrl;
  }
  if (port === undefined) {
    delete process.env.PORT;
  } else {
    process.env.PORT = port;
  }
}

describe("SITE_URL", () => {
  const original = {
    port: process.env.PORT,
    siteUrl: process.env.NEXT_PUBLIC_SITE_URL,
  };

  afterEach(() => {
    restoreSiteEnvironment(original.siteUrl, original.port);
    vi.resetModules();
  });

  it("uses NEXT_PUBLIC_SITE_URL when set", async () => {
    process.env.NEXT_PUBLIC_SITE_URL = "https://dig.vllnt.com";
    vi.resetModules();
    const site = await import("@/lib/site");
    expect(site.SITE_URL).toBe("https://dig.vllnt.com");
  });

  it("falls back to localhost with the PORT env var", async () => {
    delete process.env.NEXT_PUBLIC_SITE_URL;
    process.env.PORT = "4000";
    vi.resetModules();
    const site = await import("@/lib/site");
    expect(site.SITE_URL).toBe("http://localhost:4000");
  });

  it("falls back to the fixed dev port when nothing is set", async () => {
    delete process.env.NEXT_PUBLIC_SITE_URL;
    delete process.env.PORT;
    vi.resetModules();
    const site = await import("@/lib/site");
    expect(site.SITE_URL).toBe("http://localhost:3977");
  });
});

describe("repo links", () => {
  it("points at the public dig repository", () => {
    expect(GITHUB_URL).toBe("https://github.com/vllnt/dig");
  });
});
