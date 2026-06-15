import { Badge, Button, Terminal } from "@vllnt/ui";
import type { Metadata } from "next";
import { getTranslations, setRequestLocale } from "next-intl/server";

import { HarnessPicker } from "@/components/integrations/harness-picker";
import { buildCanonicalUrl, GITHUB_URL } from "@/lib/site";

const RELEASES_URL = `${GITHUB_URL}/releases`;

const QUICK_INSTALL = [
  {
    content: "curl -fsSL https://dig.vllnt.com/install.sh | sh",
    type: "command",
  },
] as const;

const GO_INSTALL = [
  {
    content: "go install github.com/vllnt/dig/cmd/dig@latest",
    type: "command",
  },
] as const;

const FIRST_RUN = [
  { content: "point dig at a directory and index it", type: "comment" },
  { content: "dig init ~/library", type: "command" },
  { content: "dig scan", type: "command" },
  { content: "", type: "output" },
  { content: "search, ranked", type: "comment" },
  { content: 'dig find "invoice acme 2024"', type: "command" },
  { content: "", type: "output" },
  { content: "preview a reorg, apply it, step back", type: "comment" },
  { content: "dig org --dry-run", type: "command" },
  { content: "dig org", type: "command" },
  { content: "dig undo", type: "command" },
] as const;

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "install" });
  return {
    alternates: { canonical: buildCanonicalUrl(locale, "/install") },
    description: t("meta_description"),
    title: t("meta_title"),
  };
}

export default async function InstallPage({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<React.JSX.Element> {
  const { locale } = await params;
  setRequestLocale(locale);
  const t = await getTranslations({ locale, namespace: "install" });

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-12 px-6 py-24">
      <header className="flex flex-col gap-4">
        <Badge className="w-fit" variant="secondary">
          {t("status_badge")}
        </Badge>
        <h1 className="text-balance text-4xl font-semibold tracking-tight sm:text-5xl">
          {t("title")}
        </h1>
        <p className="text-pretty text-lg text-muted-foreground">
          {t("subtitle")}
        </p>
        <p className="text-pretty text-sm leading-6 text-muted-foreground">
          {t("status_note")}
        </p>
      </header>

      <section className="flex flex-col gap-4 rounded-lg border border-border bg-muted/30 p-6">
        <div className="flex flex-col gap-2">
          <h2 className="text-xl font-semibold tracking-tight">
            {t("agent_title")}
          </h2>
          <p className="text-sm leading-6 text-muted-foreground">
            {t("agent_note")}
          </p>
        </div>
        <HarnessPicker />
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="text-xl font-semibold tracking-tight">
          {t("primary_title")}
        </h2>
        <Terminal copyable lines={[...QUICK_INSTALL]} title="install" />
        <p className="text-sm leading-6 text-muted-foreground">
          {t("primary_note")}
        </p>
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="text-xl font-semibold tracking-tight">
          {t("go_title")}
        </h2>
        <Terminal copyable lines={[...GO_INSTALL]} title="go" />
        <p className="text-sm leading-6 text-muted-foreground">
          {t("go_note")}
        </p>
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="text-xl font-semibold tracking-tight">
          {t("binary_title")}
        </h2>
        <p className="text-sm leading-6 text-muted-foreground">
          {t("binary_note")}
        </p>
        <div>
          <Button asChild variant="outline">
            <a href={RELEASES_URL} rel="noreferrer" target="_blank">
              {t("binary_cta")}
            </a>
          </Button>
        </div>
      </section>

      <section className="flex flex-col gap-3 border-t border-border pt-12">
        <h2 className="text-xl font-semibold tracking-tight">
          {t("next_title")}
        </h2>
        <Terminal lines={[...FIRST_RUN]} title="quick start" />
        <p className="text-sm leading-6 text-muted-foreground">
          {t("next_note")}
        </p>
      </section>
    </div>
  );
}
