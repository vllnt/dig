import { Badge, Button } from "@vllnt/ui";
import { useTranslations } from "next-intl";

import { ARCHITECTURE_URL, GITHUB_URL } from "@/lib/site";

import { Terminal } from "./terminal";

const QUICK_START = `# index a library
dig init ~/library
dig scan

# search, ranked
dig find "invoice acme 2024"

# preview, apply, undo
dig org --dry-run
dig org
dig undo`;

export function Hero() {
  const t = useTranslations("hero");

  return (
    <section className="mx-auto flex max-w-5xl flex-col items-center gap-8 px-6 pb-24 pt-24 text-center sm:pt-32">
      <Badge variant="secondary">{t("badge")}</Badge>
      <h1 className="max-w-3xl text-balance text-5xl font-semibold tracking-tight sm:text-6xl">
        {t("title")}
      </h1>
      <p className="max-w-2xl text-pretty text-lg text-muted-foreground">
        {t("subtitle")}
      </p>
      <div className="flex flex-wrap justify-center gap-3">
        <Button asChild size="lg">
          <a href={GITHUB_URL} rel="noreferrer" target="_blank">
            {t("cta_github")}
          </a>
        </Button>
        <Button asChild size="lg" variant="outline">
          <a href={ARCHITECTURE_URL} rel="noreferrer" target="_blank">
            {t("cta_docs")}
          </a>
        </Button>
      </div>
      <div className="w-full max-w-2xl">
        <Terminal code={QUICK_START} title={t("terminal_title")} />
      </div>
    </section>
  );
}
