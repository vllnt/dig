import { DEFAULT_LOCALE } from "@/i18n/locales";

export const SITE_URL =
  process.env.NEXT_PUBLIC_SITE_URL ??
  `http://localhost:${process.env.PORT ?? "3977"}`;

export const GITHUB_URL = "https://github.com/vllnt/dig";
export const ROADMAP_URL = `${GITHUB_URL}/blob/main/ROADMAP.md`;
export const ARCHITECTURE_URL = `${GITHUB_URL}/blob/main/docs/architecture.md`;

/**
 * Build the absolute canonical URL for a locale + path, honoring the
 * `as-needed` locale prefix (default locale unprefixed).
 *
 * @param locale - a locale code (one of `LOCALES` in app code)
 * @param path - pathname starting with `/` (default `/`)
 * @returns absolute URL on `SITE_URL`
 * @example buildCanonicalUrl('en', '/') // https://dig.vllnt.com
 */
export function buildCanonicalUrl(locale: string, path = "/"): string {
  const prefix = locale === DEFAULT_LOCALE ? "" : `/${locale}`;
  const suffix = path === "/" ? "" : path;
  return `${SITE_URL}${prefix}${suffix}`;
}
