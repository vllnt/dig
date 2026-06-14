import { useTranslations } from "next-intl";

import { HarnessPicker } from "@/components/integrations/harness-picker";
import { HARNESSES_BY_STATUS } from "@/lib/harnesses";

const WORKS_WITH = HARNESSES_BY_STATUS.map((h) => h.name).join(" · ");

export function Integrations() {
  const t = useTranslations("home_integrations");

  return (
    <section
      className="scroll-mt-16 border-t border-border bg-muted/30"
      id="integrations"
    >
      <div className="mx-auto flex max-w-5xl flex-col gap-10 px-6 py-24">
        <div className="flex flex-col gap-4 text-center">
          <h2 className="text-balance text-3xl font-semibold tracking-tight sm:text-4xl">
            {t("title")}
          </h2>
          <p className="mx-auto max-w-2xl text-pretty text-muted-foreground">
            {t("subtitle")}
          </p>
        </div>
        <div className="mx-auto w-full max-w-2xl rounded-lg border border-border bg-background p-6">
          <HarnessPicker label={t("picker_label")} />
        </div>
        <p className="text-center text-sm text-muted-foreground">
          {t("works_with")} {WORKS_WITH}
        </p>
      </div>
    </section>
  );
}
