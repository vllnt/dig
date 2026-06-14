import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { setRequestLocale } from "next-intl/server";

import { Mdx } from "@/components/content/mdx";
import { JsonLd } from "@/components/seo/json-ld";
import { isCluster, listAllContent, readContent } from "@/lib/content";
import { buildCanonicalUrl, SITE_URL } from "@/lib/site";

export const dynamicParams = false;

export function generateStaticParams(): { cluster: string; slug: string }[] {
  return listAllContent().map((entry) => ({
    cluster: entry.cluster,
    slug: entry.slug,
  }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ cluster: string; locale: string; slug: string }>;
}): Promise<Metadata> {
  const { cluster, locale, slug } = await params;
  if (!isCluster(cluster)) return {};
  const entry = readContent(cluster, slug);
  if (!entry) return {};
  return {
    alternates: { canonical: buildCanonicalUrl(locale, `/${cluster}/${slug}`) },
    description: entry.meta.description,
    title: entry.meta.title,
  };
}

export default async function ContentPage({
  params,
}: {
  params: Promise<{ cluster: string; locale: string; slug: string }>;
}): Promise<React.JSX.Element> {
  const { cluster, locale, slug } = await params;
  setRequestLocale(locale);
  if (!isCluster(cluster)) notFound();
  const entry = readContent(cluster, slug);
  if (!entry) notFound();

  return (
    <article className="mx-auto flex max-w-3xl flex-col px-6 py-24">
      <JsonLd
        data={{
          "@context": "https://schema.org",
          "@type": "TechArticle",
          description: entry.meta.description,
          headline: entry.meta.title,
          url: `${SITE_URL}/${cluster}/${slug}`,
        }}
      />
      <header className="mb-8 flex flex-col gap-3 border-b border-border pb-8">
        <p className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
          {cluster.replace("-", " ")}
        </p>
        <h1 className="text-balance text-4xl font-semibold tracking-tight sm:text-5xl">
          {entry.meta.title}
        </h1>
        <p className="text-pretty text-lg text-muted-foreground">
          {entry.meta.description}
        </p>
      </header>
      <Mdx source={entry.body} />
    </article>
  );
}
