import type { MetadataRoute } from "next";

import { DEFAULT_LOCALE, LOCALES } from "@/i18n/locales";
import { buildCanonicalUrl } from "@/lib/site";

export default function sitemap(): MetadataRoute.Sitemap {
  const languages = Object.fromEntries(
    LOCALES.map((locale) => [locale, buildCanonicalUrl(locale, "/")]),
  );

  return [
    {
      alternates: { languages },
      changeFrequency: "weekly",
      lastModified: new Date(),
      priority: 1,
      url: buildCanonicalUrl(DEFAULT_LOCALE, "/"),
    },
  ];
}
