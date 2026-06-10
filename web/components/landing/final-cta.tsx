import { Button } from "@vllnt/ui";
import { useTranslations } from "next-intl";

import { GITHUB_URL, ROADMAP_URL } from "@/lib/site";

export function FinalCta() {
  const t = useTranslations("cta");

  return (
    <section className="border-t border-border">
      <div className="mx-auto flex max-w-3xl flex-col items-center gap-6 px-6 py-24 text-center">
        <h2 className="text-balance text-3xl font-semibold tracking-tight sm:text-4xl">
          {t("title")}
        </h2>
        <p className="max-w-xl text-pretty text-muted-foreground">
          {t("body")}
        </p>
        <div className="flex flex-wrap justify-center gap-3">
          <Button asChild size="lg">
            <a href={GITHUB_URL} rel="noreferrer" target="_blank">
              {t("github")}
            </a>
          </Button>
          <Button asChild size="lg" variant="outline">
            <a href={ROADMAP_URL} rel="noreferrer" target="_blank">
              {t("roadmap")}
            </a>
          </Button>
        </div>
      </div>
    </section>
  );
}
