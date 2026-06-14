import { Badge, Button, Terminal } from "@vllnt/ui";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { setRequestLocale } from "next-intl/server";

import { JsonLd } from "@/components/seo/json-ld";
import { Link } from "@/i18n/routing";
import {
  getHarness,
  HARNESS_SLUGS,
  harnessDocumentUrl,
  MCP_CONFIG_JSON,
} from "@/lib/harnesses";
import { buildCanonicalUrl, GITHUB_URL, SITE_URL } from "@/lib/site";

export const dynamicParams = false;

/** The MCP tool surface dig exposes — same for every harness (integration.md). */
const MCP_TOOLS: readonly { does: string; name: string }[] = [
  { does: "ranked search across the knowledge base", name: "dig_find" },
  {
    does: "token-budgeted, provenance-tagged memory recall",
    name: "dig_recall",
  },
  { does: "capture a note, doc, or session into memory", name: "dig_retain" },
  { does: "report divergence from policy", name: "dig_drift" },
  { does: "read the change history", name: "dig_log" },
  { does: "export a reproducible dataset", name: "dig_export" },
  { does: "reorganize to policy (preview unless apply)", name: "dig_org" },
  { does: "converge to policy (preview unless apply)", name: "dig_reconcile" },
  { does: "step back the last change", name: "dig_undo" },
];

export function generateStaticParams(): { slug: string }[] {
  return HARNESS_SLUGS.map((slug) => ({ slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string; slug: string }>;
}): Promise<Metadata> {
  const { locale, slug } = await params;
  const harness = getHarness(slug);
  if (!harness) return {};
  return {
    alternates: {
      canonical: buildCanonicalUrl(locale, `/integrations/${slug}`),
    },
    description: harness.unlocks,
    title: `dig + ${harness.name} — local, reversible memory for your agent`,
  };
}

export default async function HarnessPage({
  params,
}: {
  params: Promise<{ locale: string; slug: string }>;
}): Promise<React.JSX.Element> {
  const { locale, slug } = await params;
  setRequestLocale(locale);
  const harness = getHarness(slug);
  if (!harness) notFound();

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-12 px-6 py-24">
      <JsonLd
        data={{
          "@context": "https://schema.org",
          "@type": "SoftwareApplication",
          applicationCategory: "DeveloperApplication",
          codeRepository: GITHUB_URL,
          description: harness.unlocks,
          name: `dig + ${harness.name}`,
          offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
          operatingSystem: "macOS, Linux, Windows",
          url: `${SITE_URL}/integrations/${slug}`,
        }}
      />

      <nav className="text-sm text-muted-foreground">
        <Link className="hover:text-foreground" href="/integrations">
          Integrations
        </Link>{" "}
        / {harness.name}
      </nav>

      <header className="flex flex-col gap-4">
        <div className="flex items-center gap-3">
          <h1 className="text-balance text-4xl font-semibold tracking-tight sm:text-5xl">
            dig + {harness.name}
          </h1>
          {harness.status === "wip" ? (
            <Badge variant="secondary">WIP</Badge>
          ) : null}
        </div>
        <p className="text-pretty text-lg text-muted-foreground">
          {harness.unlocks}
        </p>
      </header>

      <section className="flex flex-col gap-3">
        <h2 className="text-xl font-semibold tracking-tight">
          Install — about 10 seconds
        </h2>
        <Terminal copyable lines={[...harness.install]} title={harness.name} />
        {harness.status === "wip" ? (
          <p className="text-sm leading-6 text-muted-foreground">
            The native {harness.name} shim is still landing. Until it does, the
            MCP server gives {harness.name} the full dig surface today.
          </p>
        ) : null}
      </section>

      {harness.mcp ? (
        <section className="flex flex-col gap-3">
          <h2 className="text-xl font-semibold tracking-tight">
            Or register the MCP server directly
          </h2>
          <p className="text-sm leading-6 text-muted-foreground">
            The universal entry: one stdio server, accepted by any MCP-capable
            client.
          </p>
          <Terminal
            copyable
            lines={[{ content: MCP_CONFIG_JSON, type: "output" }]}
            title="mcp config"
          />
        </section>
      ) : null}

      <section className="flex flex-col gap-3">
        <h2 className="text-xl font-semibold tracking-tight">
          What {harness.name} gets
        </h2>
        <p className="text-sm leading-6 text-muted-foreground">
          dig drives the same surface through every path. Read tools never
          change state; mutating tools preview first, and a single dig_undo
          steps any change back.
        </p>
        <dl className="grid gap-x-6 gap-y-3 sm:grid-cols-2">
          {MCP_TOOLS.map((tool) => (
            <div className="flex flex-col" key={tool.name}>
              <dt className="font-mono text-sm">{tool.name}</dt>
              <dd className="text-sm text-muted-foreground">{tool.does}</dd>
            </div>
          ))}
        </dl>
      </section>

      <section className="flex flex-wrap gap-3 border-t border-border pt-12">
        <Button asChild>
          <Link href="/install">Install dig</Link>
        </Button>
        <Button asChild variant="outline">
          <Link href="/integrations">All integrations</Link>
        </Button>
        <Button asChild variant="outline">
          <a
            href={harnessDocumentUrl(harness)}
            rel="noreferrer"
            target="_blank"
          >
            {harness.name} shim on GitHub
          </a>
        </Button>
      </section>
    </div>
  );
}
