import { useTranslations } from "next-intl";

import { Terminal } from "./terminal";

const POLICY_SNIPPET = `[[rule]]
name   = "invoices"
match  = { ext = ["pdf"], content_matches = "invoice" }
into   = "finance/invoices/{year}"
rename = "{vendor}-{invoice_no}.pdf"
label  = ["finance", "invoice"]

[dedup]
strategy    = "keep-oldest"
on_conflict = "escalate"   # never silently delete`;

const STEPS = ["step1", "step2", "step3"] as const;

export function HowItWorks() {
  const t = useTranslations("how");

  return (
    <section className="border-t border-border" id="how">
      <div className="mx-auto flex max-w-5xl flex-col gap-12 px-6 py-24">
        <div className="flex flex-col gap-4 text-center">
          <h2 className="text-balance text-3xl font-semibold tracking-tight sm:text-4xl">
            {t("title")}
          </h2>
          <p className="mx-auto max-w-2xl text-pretty text-muted-foreground">
            {t("subtitle")}
          </p>
        </div>
        <div className="grid gap-8 sm:grid-cols-3">
          {STEPS.map((step, index) => (
            <div className="flex flex-col gap-3" key={step}>
              <span className="font-mono text-sm text-muted-foreground">
                0{index + 1}
              </span>
              <h3 className="text-lg font-semibold">{t(`${step}_title`)}</h3>
              <p className="text-sm leading-6 text-muted-foreground">
                {t(`${step}_body`)}
              </p>
            </div>
          ))}
        </div>
        <div className="mx-auto w-full max-w-2xl">
          <Terminal code={POLICY_SNIPPET} title=".dig/policy.toml" />
        </div>
      </div>
    </section>
  );
}
