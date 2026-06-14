import { Badge, Button, buttonVariants, Terminal } from "@vllnt/ui";
import { useTranslations } from "next-intl";

import { GithubStars } from "@/components/github-stars";
import { Link } from "@/i18n/routing";

const INSTALL = [
  { content: "1 · install the dig CLI", type: "comment" },
  {
    content: "curl -fsSL https://dig.vllnt.com/install.sh | sh",
    type: "command",
  },
  { content: "", type: "output" },
  {
    content:
      "2 · add it to your agent — bundles the skill + the dig mcp server",
    type: "comment",
  },
  { content: "claude plugin marketplace add vllnt/dig", type: "command" },
  { content: "claude plugin install dig@dig", type: "command" },
  { content: "", type: "output" },
  {
    content: "3 · ready — dig is on your PATH and in your agent",
    type: "comment",
  },
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
      <div className="flex flex-wrap items-center justify-center gap-3">
        <Button asChild size="lg">
          <Link href="/install">{t("cta_install")}</Link>
        </Button>
        <GithubStars
          className={buttonVariants({ size: "lg", variant: "outline" })}
          label={t("cta_github")}
        />
      </div>
      <Link
        className="text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
        href="/leaderboard"
      >
        {t("proof")}
      </Link>
      <div className="w-full max-w-2xl text-left">
        <Terminal copyable lines={[...INSTALL]} title={t("terminal_title")} />
      </div>
    </section>
  );
}
