import { Badge, Button, Terminal } from "@vllnt/ui";
import { useTranslations } from "next-intl";

import { Link } from "@/i18n/routing";
import { GITHUB_URL } from "@/lib/site";

const QUICK_START = [
  { content: "index a library", type: "comment" },
  { content: "dig init ~/library", type: "command" },
  { content: "dig scan", type: "command" },
  { content: "", type: "output" },
  { content: "search, ranked", type: "comment" },
  { content: 'dig find "invoice acme 2024"', type: "command" },
  { content: "", type: "output" },
  { content: "preview, apply, undo", type: "comment" },
  { content: "dig org --dry-run", type: "command" },
  { content: "dig org", type: "command" },
  { content: "dig undo", type: "command" },
] as const;

export function Hero() {
  const t = useTranslations("hero");

  return (
    <section className="mx-auto flex max-w-5xl flex-col items-center gap-8 px-6 pb-24 pt-24 text-center sm:pt-32">
      <Badge variant="secondary">{t("badge")}</Badge>
      <h1 className="max-w-3xl text-balance text-4xl font-semibold tracking-tight sm:text-5xl lg:text-6xl">
        {t("title")}
      </h1>
      <p className="max-w-2xl text-pretty text-lg text-muted-foreground">
        {t("subtitle")}
      </p>
      <div className="flex flex-wrap justify-center gap-3">
        <Button asChild size="lg">
          <Link href="/integrations">{t("cta_agent")}</Link>
        </Button>
        <Button asChild size="lg" variant="outline">
          <Link href="/install">{t("cta_install")}</Link>
        </Button>
        <Button asChild size="lg" variant="outline">
          <a href={GITHUB_URL} rel="noreferrer" target="_blank">
            {t("cta_github")}
          </a>
        </Button>
      </div>
      <Link
        className="text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
        href="/leaderboard"
      >
        {t("proof")}
      </Link>
      <div className="w-full max-w-2xl text-left">
        <Terminal lines={[...QUICK_START]} title={t("terminal_title")} />
      </div>
    </section>
  );
}
