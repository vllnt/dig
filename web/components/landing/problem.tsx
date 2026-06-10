import { useTranslations } from "next-intl";

export function Problem() {
  const t = useTranslations("problem");

  return (
    <section className="border-t border-border bg-muted/30">
      <div className="mx-auto flex max-w-3xl flex-col gap-6 px-6 py-24 text-center">
        <h2 className="text-balance text-3xl font-semibold tracking-tight sm:text-4xl">
          {t("title")}
        </h2>
        <p className="text-pretty text-lg leading-8 text-muted-foreground">
          {t("body")}
        </p>
        <p className="text-pretty leading-7 text-muted-foreground">
          {t("agitate")}
        </p>
      </div>
    </section>
  );
}
