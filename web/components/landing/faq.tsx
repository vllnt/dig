import { useTranslations } from "next-intl";

const QUESTIONS = [1, 2, 3, 4, 5] as const;

export function Faq() {
  const t = useTranslations("faq");

  return (
    <section className="border-t border-border bg-muted/30" id="faq">
      <div className="mx-auto flex max-w-3xl flex-col gap-12 px-6 py-24">
        <h2 className="text-balance text-center text-3xl font-semibold tracking-tight sm:text-4xl">
          {t("title")}
        </h2>
        <dl className="flex flex-col gap-8">
          {QUESTIONS.map((n) => (
            <div className="flex flex-col gap-2" key={n}>
              <dt className="font-semibold">{t(`q${n}`)}</dt>
              <dd className="leading-7 text-muted-foreground">{t(`a${n}`)}</dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}
