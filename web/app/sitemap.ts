import type { MetadataRoute } from "next";

import { DEFAULT_LOCALE, LOCALES } from "@/i18n/locales";
import { CLUSTERS, listAllContent } from "@/lib/content";
import { HARNESS_SLUGS } from "@/lib/harnesses";
import { buildCanonicalUrl } from "@/lib/site";

/** One sitemap entry; the default export expands each per-locale. */
type Route = { path: string; priority: number };

/**
 * Static public routes (relative to the site root) with their crawl priority.
 * `harnessRoutes` and `contentRoutes` add the integration and SEO pages, so
 * crawlers and agents find every page without a hand-maintained list.
 */
export const ROUTES: readonly Route[] = [
  { path: "/", priority: 1 },
  { path: "/docs", priority: 0.9 },
  { path: "/integrations", priority: 0.9 },
  { path: "/leaderboard", priority: 0.8 },
  { path: "/install", priority: 0.8 },
];

/** Per-harness integration pages, generated from the registry. */
function harnessRoutes(): Route[] {
  return HARNESS_SLUGS.map((slug) => ({
    path: `/integrations/${slug}`,
    priority: 0.7,
  }));
}

/** Cluster hub index pages (/compare, /learn, /use-cases). */
function clusterHubRoutes(): Route[] {
  return CLUSTERS.map((cluster) => ({ path: `/${cluster}`, priority: 0.7 }));
}

/** MDX SEO pages (compare / use-cases / learn), generated from `content/`. */
function contentRoutes(): Route[] {
  return listAllContent().map((entry) => ({
    path: `/${entry.cluster}/${entry.slug}`,
    priority: 0.6,
  }));
}

/** Every route in the sitemap, static + dynamic. */
export function allRoutes(): Route[] {
  return [
    ...ROUTES,
    ...clusterHubRoutes(),
    ...harnessRoutes(),
    ...contentRoutes(),
  ];
}

export default function sitemap(): MetadataRoute.Sitemap {
  return allRoutes().map((route) => ({
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
