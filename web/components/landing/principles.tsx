import { useTranslations } from "next-intl";

const PRINCIPLES = ["p1", "p2", "p3", "p4", "p5", "p6"] as const;

export function Principles() {
  const t = useTranslations("principles");

  return (
    <section className="border-t border-border">
      <div className="mx-auto flex max-w-5xl flex-col gap-12 px-6 py-24">
        <h2 className="text-balance text-center text-3xl font-semibold tracking-tight sm:text-4xl">
          {t("title")}
        </h2>
        <dl className="grid gap-x-8 gap-y-8 sm:grid-cols-2 lg:grid-cols-3">
          {PRINCIPLES.map((principle) => (
            <div className="flex flex-col gap-2" key={principle}>
              <dt className="font-semibold">{t(`${principle}_title`)}</dt>
              <dd className="text-sm leading-6 text-muted-foreground">
                {t(`${principle}_body`)}
              </dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}
