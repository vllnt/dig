import "@vllnt/ui/styles.css";
import "@vllnt/ui/themes/default.css";

import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { hasLocale, NextIntlClientProvider } from "next-intl";
import {
  getMessages,
  getTranslations,
  setRequestLocale,
} from "next-intl/server";

import { Providers } from "@/app/providers";
import { routing } from "@/i18n/routing";
import { SITE_URL } from "@/lib/site";

import "@/app/globals.css";

export function generateStaticParams() {
  return routing.locales.map((locale) => ({ locale }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "meta" });
  return {
    description: t("description"),
    metadataBase: new URL(SITE_URL),
    title: t("title"),
  };
}

export default async function LocaleLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  if (!hasLocale(routing.locales, locale)) notFound();

  setRequestLocale(locale);
  // Client components (useTranslations) need their messages in the browser. The
  // cookie-consent banner is the lone such consumer; Server Components read every
  // other namespace through getTranslations. Pass that one namespace so the rest
  // of the catalog stays server-side, out of the RSC payload. Add a namespace
  // here when a new client component needs it.
  const messages = await getMessages();
  const clientMessages = { consent: messages.consent };

  return (
    <html lang={locale} suppressHydrationWarning>
      <body className="min-h-dvh bg-background text-foreground antialiased">
        <NextIntlClientProvider messages={clientMessages}>
          <Providers>{children}</Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
