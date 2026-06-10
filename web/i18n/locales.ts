export const LOCALES = ["en"] as const;

export const DEFAULT_LOCALE = "en";

export type Locale = (typeof LOCALES)[number];
