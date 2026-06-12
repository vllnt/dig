import { Terminal } from "@vllnt/ui";
import type { Metadata } from "next";
import { getTranslations, setRequestLocale } from "next-intl/server";

import { CommandTable } from "@/components/docs/command-table";
import { PolicyReference } from "@/components/docs/policy-reference";
import { QUICKSTART } from "@/lib/docs";
import { buildCanonicalUrl } from "@/lib/site";

const QUICKSTART_LINES = QUICKSTART.flatMap((step) => [
  { content: step.describe, type: "comment" as const },
  { content: step.cmd, type: "command" as const },
]);

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "docs" });
  return {
    alternates: { canonical: buildCanonicalUrl(locale, "/docs") },
    description: t("meta_description"),
    title: t("meta_title"),
  };
}

export default async function DocsPage({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<React.JSX.Element> {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations({ locale, namespace: "docs" });

  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-16 px-6 py-24">
      <header className="flex flex-col gap-4">
        <h1 className="text-balance text-4xl font-semibold tracking-tight sm:text-5xl">
          {t("title")}
        </h1>
        <p className="text-pretty text-lg text-muted-foreground">
          {t("subtitle")}
        </p>
      </header>

      <section className="flex flex-col gap-4" id="quickstart">
        <h2 className="scroll-mt-20 text-2xl font-semibold tracking-tight">
          {t("quickstart_title")}
        </h2>
        <p className="text-muted-foreground">{t("quickstart_body")}</p>
        <Terminal lines={QUICKSTART_LINES} title="quick start" />
      </section>

      <section className="flex flex-col gap-4" id="commands">
        <h2 className="scroll-mt-20 text-2xl font-semibold tracking-tight">
          {t("commands_title")}
        </h2>
        <p className="text-muted-foreground">{t("commands_body")}</p>
        <CommandTable />
      </section>

      <section className="flex flex-col gap-4" id="policy">
        <h2 className="scroll-mt-20 text-2xl font-semibold tracking-tight">
          {t("policy_title")}
        </h2>
        <p className="text-muted-foreground">{t("policy_body")}</p>
        <PolicyReference />
      </section>
    </div>
  );
}
