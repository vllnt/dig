import { createNavigation } from "next-intl/navigation";
import { defineRouting } from "next-intl/routing";

import { DEFAULT_LOCALE, LOCALES } from "./locales";

export const routing = defineRouting({
  defaultLocale: DEFAULT_LOCALE,
  localePrefix: "as-needed",
  locales: LOCALES,
});

export const { getPathname, Link, redirect, usePathname, useRouter } =
  createNavigation(routing);
