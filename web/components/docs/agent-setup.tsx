import { Button } from "@vllnt/ui";
import { useTranslations } from "next-intl";

import { Link } from "@/i18n/routing";

/** What an agent with the dig skill does when asked to set dig up. */
const AGENT_ACTIONS = [
  "detect dig and install the CLI if it's missing",
  "dig init a knowledge base on the directory you point at",
  "draft and validate a .dig/policy.toml from your conventions",
  "scan, preview org --dry-run, apply, and reconcile drift",
  "keep every change reversible with dig undo",
] as const;

/** Example prompts a user can hand their agent. */
const PROMPTS = [
  "Set up dig on ~/Documents and file invoices under finance/{year}.",
  "Index my notes, make them searchable, and collapse duplicates.",
  "Remember this session so you can recall it next time.",
] as const;

/** The "let your agent set it up" docs section — skill install + delegation. */
export function AgentSetup(): React.JSX.Element {
  const t = useTranslations("docs");

  return (
    <section className="flex scroll-mt-20 flex-col gap-5" id="agent-setup">
      <div className="flex flex-col gap-2">
        <h2 className="text-2xl font-semibold tracking-tight">
          {t("agent_title")}
        </h2>
        <p className="text-muted-foreground">{t("agent_body")}</p>
      </div>

      <div className="flex flex-col gap-2">
        <h3 className="text-lg font-semibold">{t("agent_install_title")}</h3>
        <p className="text-sm leading-6 text-muted-foreground">
          {t("agent_install_body")}
        </p>
      </div>

      <div className="flex flex-col gap-3">
        <h3 className="text-lg font-semibold">{t("agent_ask_title")}</h3>
        <p className="text-sm leading-6 text-muted-foreground">
          {t("agent_ask_body")}
        </p>
        <ul className="ml-5 flex list-disc flex-col gap-1 text-sm text-muted-foreground">
          {AGENT_ACTIONS.map((action) => (
            <li key={action}>{action}</li>
          ))}
        </ul>
      </div>

      <div className="flex flex-col gap-2 rounded-lg border border-border bg-muted/30 p-4">
        <span className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
          {t("agent_prompts_label")}
        </span>
        <ul className="flex flex-col gap-1.5">
          {PROMPTS.map((prompt) => (
            <li className="text-sm italic text-foreground/80" key={prompt}>
              “{prompt}”
            </li>
          ))}
        </ul>
      </div>

      <div>
        <Button asChild variant="outline">
          <Link href="/integrations">{t("agent_cta")}</Link>
        </Button>
      </div>
    </section>
  );
}
