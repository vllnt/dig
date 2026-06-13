import { useTranslations } from "next-intl";

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
          <div className="flex items-center gap-6 text-sm text-muted-foreground">
            <a
              className="hidden transition-colors hover:text-foreground sm:block"
              href="#how"
            >
              {t("how")}
            </a>
            <a
              className="hidden transition-colors hover:text-foreground sm:block"
              href="#features"
            >
              {t("features")}
            </a>
            <a
              className="hidden transition-colors hover:text-foreground sm:block"
              href="#faq"
            >
              {t("faq")}
            </a>
            <Link
              className="transition-colors hover:text-foreground"
              href="/integrations"
            >
              {t("integrations")}
            </Link>
            <Link
              className="hidden transition-colors hover:text-foreground sm:block"
              href="/docs"
            >
              {t("docs")}
            </Link>
            <Link
              className="transition-colors hover:text-foreground"
              href="/leaderboard"
            >
              {t("benchmarks")}
            </Link>
            <Link
              className="transition-colors hover:text-foreground"
              href="/install"
            >
              {t("install")}
            </Link>
            <a
              className="font-medium text-foreground transition-colors hover:text-foreground/80"
              href={GITHUB_URL}
              rel="noreferrer"
              target="_blank"
            >
              {t("github")}
            </a>
          </div>
        </nav>
      </header>
      <main className="flex-1">{children}</main>
      <footer className="border-t border-border">
        <div className="mx-auto flex max-w-5xl flex-col gap-4 px-6 py-8 text-sm text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-col gap-1">
            <span className="font-mono font-semibold text-foreground">dig</span>
            <span>{f("tagline")}</span>
            <span>{f("license")}</span>
          </div>
          <div className="flex gap-4">
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
      </footer>
    </div>
  );
}
