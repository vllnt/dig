import type { MetadataRoute } from "next";

import { DEFAULT_LOCALE, LOCALES } from "@/i18n/locales";
import { buildCanonicalUrl } from "@/lib/site";

/**
 * Public routes (relative to the site root) included in the sitemap, with their
 * crawl priority. Extend this as pages land so crawlers and agents discover
 * every page.
 */
export const ROUTES: readonly { path: string; priority: number }[] = [
  { path: "/", priority: 1 },
  { path: "/docs", priority: 0.9 },
  { path: "/leaderboard", priority: 0.8 },
  { path: "/install", priority: 0.8 },
];

export default function sitemap(): MetadataRoute.Sitemap {
  return ROUTES.map((route) => ({
    alternates: {
      languages: Object.fromEntries(
        LOCALES.map((locale) => [
          locale,
          buildCanonicalUrl(locale, route.path),
        ]),
      ),
    },
    changeFrequency: "weekly",
    lastModified: new Date(),
    priority: route.priority,
    url: buildCanonicalUrl(DEFAULT_LOCALE, route.path),
  }));
}
