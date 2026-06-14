import { Button, Terminal } from "@vllnt/ui";
import { useTranslations } from "next-intl";

import { CodeSample } from "@/components/docs/code-sample";
import { Link } from "@/i18n/routing";

const MCP_LINES = [
  { content: "Claude Code or Codex:", type: "comment" },
  { content: "claude mcp add dig -- dig mcp", type: "command" },
  { content: "codex plugin marketplace add vllnt/dig", type: "command" },
  { content: "", type: "output" },
  { content: "any other MCP client — config:", type: "comment" },
  {
    content:
      '{ "mcpServers": { "dig": { "command": "dig", "args": ["mcp"] } } }',
    type: "output",
  },
] as const;

const TS_SAMPLE = `import { DigClient } from "@vllnt/dig";

const dig = new DigClient();
const hits = await dig.find("invoice acme 2024", { mode: "hybrid" });
const pack = await dig.recall("billing decision", { budget: 1000 });`;

const PY_SAMPLE = `from dig_client import DigClient

dig = DigClient()
hits = dig.find("invoice acme 2024", mode="hybrid")
pack = dig.recall("billing decision", budget=1000)`;

/** The "Integrate" docs section — MCP, the AI SDK, and the typed SDKs. */
export function Integrate(): React.JSX.Element {
  const t = useTranslations("docs");

  return (
    <section className="flex scroll-mt-20 flex-col gap-6" id="integrate">
      <div className="flex flex-col gap-2">
        <h2 className="text-2xl font-semibold tracking-tight">
          {t("integrate_title")}
        </h2>
        <p className="text-muted-foreground">{t("integrate_body")}</p>
      </div>

      <div className="flex flex-col gap-3">
        <h3 className="text-lg font-semibold">{t("integrate_mcp_title")}</h3>
        <p className="text-sm leading-6 text-muted-foreground">
          {t("integrate_mcp_body")}
        </p>
        <Terminal copyable lines={[...MCP_LINES]} title="mcp" />
      </div>

      <div className="flex flex-col gap-3">
        <h3 className="text-lg font-semibold">{t("integrate_aisdk_title")}</h3>
        <p className="text-sm leading-6 text-muted-foreground">
          {t("integrate_aisdk_body")}
        </p>
        <div>
          <Button asChild size="sm" variant="outline">
            <Link href="/learn/vercel-ai-sdk">
              {`${t("integrate_aisdk_cta")} →`}
            </Link>
          </Button>
        </div>
      </div>

      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-1">
          <h3 className="text-lg font-semibold">{t("integrate_sdk_title")}</h3>
          <p className="text-sm leading-6 text-muted-foreground">
            {t("integrate_sdk_body")}
          </p>
        </div>
        <div className="grid gap-4 md:grid-cols-2">
          <CodeSample code={TS_SAMPLE} lang="TypeScript · @vllnt/dig" />
          <CodeSample code={PY_SAMPLE} lang="Python · dig-client" />
        </div>
        <div className="flex flex-wrap gap-3">
          <Button asChild size="sm" variant="outline">
            <Link href="/learn/typescript-sdk">
              {`${t("integrate_sdk_ts_cta")} →`}
            </Link>
          </Button>
          <Button asChild size="sm" variant="outline">
            <Link href="/learn/python-sdk">
              {`${t("integrate_sdk_py_cta")} →`}
            </Link>
          </Button>
        </div>
      </div>

      <div>
        <Button asChild>
          <Link href="/integrations">{t("integrate_cta")}</Link>
        </Button>
      </div>
    </section>
  );
}
