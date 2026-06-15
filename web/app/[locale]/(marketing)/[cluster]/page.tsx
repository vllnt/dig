import { Card } from "@vllnt/ui";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getTranslations, setRequestLocale } from "next-intl/server";

import { Link } from "@/i18n/routing";
import { CLUSTERS, isCluster, listCluster } from "@/lib/content";
import { buildCanonicalUrl } from "@/lib/site";

export const dynamicParams = false;

export function generateStaticParams(): { cluster: string }[] {
  return CLUSTERS.map((cluster) => ({ cluster }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ cluster: string; locale: string }>;
}): Promise<Metadata> {
  const { cluster, locale } = await params;
  if (!isCluster(cluster)) return {};
  const t = await getTranslations({ locale, namespace: "clusters" });
  return {
    alternates: { canonical: buildCanonicalUrl(locale, `/${cluster}`) },
    description: t(`${cluster}.meta_description`),
    title: t(`${cluster}.meta_title`),
  };
}

export default async function ClusterHubPage({
  params,
}: {
  params: Promise<{ cluster: string; locale: string }>;
}): Promise<React.JSX.Element> {
  const { cluster, locale } = await params;
  setRequestLocale(locale);
  if (!isCluster(cluster)) notFound();
  const t = await getTranslations({ locale, namespace: "clusters" });
  const entries = listCluster(cluster);

  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-12 px-6 py-24">
      <header className="flex flex-col gap-4">
        <h1 className="text-balance text-4xl font-semibold tracking-tight sm:text-5xl">
          {t(`${cluster}.title`)}
        </h1>
        <p className="max-w-2xl text-pretty text-lg text-muted-foreground">
          {t(`${cluster}.subtitle`)}
        </p>
      </header>

      <div className="grid gap-4 sm:grid-cols-2">
        {entries.map((entry) => (
          <Link
            className="group"
            href={`/${cluster}/${entry.slug}`}
            key={entry.slug}
          >
            <Card className="flex h-full flex-col gap-3 p-6 transition-colors group-hover:border-foreground/30">
              <h2 className="font-semibold">{entry.title}</h2>
              <p className="text-sm leading-6 text-muted-foreground">
                {entry.description}
              </p>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  );
}
