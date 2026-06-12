import type { Metadata } from "next";
import { getTranslations, setRequestLocale } from "next-intl/server";

import { Scoreboard } from "@/components/leaderboard/scoreboard";
import { BENCHMARKS, EXTERNAL_BASELINES, HEADLINE } from "@/lib/leaderboard";
import { buildCanonicalUrl } from "@/lib/site";

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "leaderboard" });
  return {
    alternates: { canonical: buildCanonicalUrl(locale, "/leaderboard") },
    description: t("meta_description"),
    title: t("meta_title"),
  };
}

export default async function LeaderboardPage({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<React.JSX.Element> {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations({ locale, namespace: "leaderboard" });

  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-16 px-6 py-24">
      <header className="flex flex-col items-center gap-6 text-center">
        <h1 className="max-w-3xl text-balance text-4xl font-semibold tracking-tight sm:text-5xl">
          {t("title")}
        </h1>
        <p className="max-w-2xl text-pretty text-lg text-muted-foreground">
          {t("subtitle")}
        </p>
        <div className="flex flex-col items-center gap-2 rounded-lg border border-border bg-muted/30 px-8 py-6">
          <span className="font-mono text-5xl font-semibold tracking-tight">
            {HEADLINE.digValue.toFixed(1)}%
          </span>
          <span className="text-sm text-muted-foreground">
            {t("headline_label", {
              benchmark: HEADLINE.benchmark,
              metric: HEADLINE.metric,
            })}
          </span>
          <span className="text-sm text-muted-foreground">
            {t("headline_bar", {
              bar: HEADLINE.barValue.toFixed(1),
              system: HEADLINE.barSystem,
            })}
          </span>
        </div>
      </header>

      <section className="flex flex-col gap-8">
        {BENCHMARKS.map((benchmark) => (
          <Scoreboard benchmark={benchmark} key={benchmark.id} />
        ))}
      </section>

      <section className="flex flex-col gap-4">
        <h2 className="text-2xl font-semibold tracking-tight">
          {t("baselines_title")}
        </h2>
        <p className="text-sm text-muted-foreground">{t("baselines_intro")}</p>
        <div className="flex flex-col gap-3">
          {EXTERNAL_BASELINES.map((baseline) => (
            <div
              className="flex flex-col gap-1 border-l-2 border-border pl-4"
              key={`${baseline.system}-${baseline.benchmark}-${baseline.metric}`}
            >
              <div className="flex flex-wrap items-baseline gap-2">
                <span className="font-semibold">{baseline.system}</span>
                <span className="font-mono text-sm text-muted-foreground">
                  {baseline.value.toFixed(1)}% {baseline.metric} ·{" "}
                  {baseline.benchmark}
                </span>
              </div>
              <p className="text-sm leading-6 text-muted-foreground">
                {baseline.note}
              </p>
            </div>
          ))}
        </div>
      </section>

      <section className="flex flex-col gap-3 rounded-lg border border-border bg-muted/20 p-6">
        <h2 className="text-lg font-semibold">{t("method_title")}</h2>
        <p className="text-sm leading-6 text-muted-foreground">
          {t("method_body")}
        </p>
      </section>
    </div>
  );
}
