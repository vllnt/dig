import { buttonVariants } from "@vllnt/ui";
import { useTranslations } from "next-intl";

import { GithubStars } from "@/components/github-stars";
import { Link } from "@/i18n/routing";
import { ARCHITECTURE_URL, GITHUB_URL, ROADMAP_URL } from "@/lib/site";

export default function MarketingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const t = useTranslations("nav");
  const f = useTranslations("footer");

  return (
    <div className="flex min-h-dvh flex-col">
      <header className="sticky top-0 z-40 border-b border-border bg-background/80 backdrop-blur">
        <nav className="mx-auto flex h-16 max-w-5xl items-center justify-between px-6">
          <Link
            className="font-mono text-lg font-semibold tracking-tight"
            href="/"
          >
            dig
          </Link>
          <div className="flex items-center gap-5 text-sm text-muted-foreground">
            <Link
              className="hidden transition-colors hover:text-foreground sm:block"
              href="/integrations"
            >
              {t("integrations")}
            </Link>
            <Link
              className="hidden transition-colors hover:text-foreground sm:block"
              href="/compare"
            >
              {t("compare")}
            </Link>
            <Link
              className="hidden transition-colors hover:text-foreground sm:block"
              href="/learn"
            >
              {t("learn")}
            </Link>
            <Link
              className="hidden transition-colors hover:text-foreground sm:block"
              href="/use-cases"
            >
              {t("use_cases")}
            </Link>
            <Link
              className="hidden transition-colors hover:text-foreground sm:block"
              href="/docs"
            >
              {t("docs")}
            </Link>
            <GithubStars
              className="font-medium text-foreground transition-colors hover:text-foreground/80"
              label={t("github")}
            />
            <Link className={buttonVariants({ size: "sm" })} href="/install">
              {t("install")}
            </Link>
          </div>
        </nav>
      </header>
      <main className="flex-1">{children}</main>
      <footer className="border-t border-border">
        <div className="mx-auto flex max-w-5xl flex-col gap-10 px-6 py-12 text-sm text-muted-foreground sm:flex-row sm:justify-between">
          <div className="flex flex-col gap-1">
            <span className="font-mono font-semibold text-foreground">dig</span>
            <span>{f("tagline")}</span>
            <span>{f("license")}</span>
          </div>
          <div className="grid grid-cols-2 gap-8 sm:grid-cols-3">
            <div className="flex flex-col gap-2">
              <span className="font-medium text-foreground">
                {f("product")}
              </span>
              <Link
                className="transition-colors hover:text-foreground"
                href="/integrations"
              >
                {t("integrations")}
              </Link>
              <Link
                className="transition-colors hover:text-foreground"
                href="/install"
              >
                {t("install")}
              </Link>
              <Link
                className="transition-colors hover:text-foreground"
                href="/docs"
              >
                {t("docs")}
              </Link>
              <Link
                className="transition-colors hover:text-foreground"
                href="/leaderboard"
              >
                {f("benchmarks")}
              </Link>
            </div>
            <div className="flex flex-col gap-2">
              <span className="font-medium text-foreground">
                {f("resources")}
              </span>
              <Link
                className="transition-colors hover:text-foreground"
                href="/compare"
              >
                {t("compare")}
              </Link>
              <Link
                className="transition-colors hover:text-foreground"
                href="/learn"
              >
                {t("learn")}
              </Link>
              <Link
                className="transition-colors hover:text-foreground"
                href="/use-cases"
              >
                {t("use_cases")}
              </Link>
            </div>
            <div className="flex flex-col gap-2">
              <span className="font-medium text-foreground">
                {f("project")}
              </span>
              <a
                className="transition-colors hover:text-foreground"
                href={GITHUB_URL}
                rel="noreferrer"
                target="_blank"
              >
                {f("github")}
              </a>
              <a
                className="transition-colors hover:text-foreground"
                href={ROADMAP_URL}
                rel="noreferrer"
                target="_blank"
              >
                {f("roadmap")}
              </a>
              <a
                className="transition-colors hover:text-foreground"
                href={ARCHITECTURE_URL}
                rel="noreferrer"
                target="_blank"
              >
                {f("architecture")}
              </a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
