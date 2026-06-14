import { Badge, Card } from "@vllnt/ui";
import type { Metadata } from "next";
import { getTranslations, setRequestLocale } from "next-intl/server";

import { Link } from "@/i18n/routing";
import { HARNESSES_BY_STATUS } from "@/lib/harnesses";
import { buildCanonicalUrl } from "@/lib/site";

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "integrations" });
  return {
    alternates: { canonical: buildCanonicalUrl(locale, "/integrations") },
    description: t("meta_description"),
    title: t("meta_title"),
  };
}

export default async function IntegrationsPage({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<React.JSX.Element> {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations({ locale, namespace: "integrations" });

  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-12 px-6 py-24">
      <header className="flex flex-col gap-4">
        <h1 className="text-balance text-4xl font-semibold tracking-tight sm:text-5xl">
          {t("title")}
        </h1>
        <p className="max-w-2xl text-pretty text-lg text-muted-foreground">
          {t("subtitle")}
        </p>
      </header>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {HARNESSES_BY_STATUS.map((harness) => (
          <Link
            className="group"
            href={`/integrations/${harness.slug}`}
            key={harness.slug}
          >
            <Card className="flex h-full flex-col gap-3 p-6 transition-colors group-hover:border-foreground/30">
              <div className="flex items-center justify-between gap-2">
                <h2 className="font-semibold">{harness.name}</h2>
                <Badge
                  variant={harness.status === "wip" ? "secondary" : "outline"}
                >
                  {harness.status === "wip"
                    ? t("wip_label")
                    : t("stable_label")}
                </Badge>
              </div>
              <p className="text-sm leading-6 text-muted-foreground">
                {harness.unlocks}
              </p>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  );
}
