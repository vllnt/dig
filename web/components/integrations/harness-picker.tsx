"use client";

import { useState } from "react";

import {
  Badge,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Terminal,
} from "@vllnt/ui";

import { Link } from "@/i18n/routing";
import {
  DEFAULT_HARNESS,
  getHarness,
  HARNESSES_BY_STATUS,
} from "@/lib/harnesses";

/**
 * The "easy way to select your agent" — a dropdown over the harness registry
 * that renders the exact 10-second install for whatever the visitor picks.
 * Client component — the interactive surface on an otherwise static site.
 */
export function HarnessPicker({
  label = "Your agent",
}: {
  /** Caption shown beside the dropdown. */
  label?: string;
}): React.JSX.Element {
  const [slug, setSlug] = useState(DEFAULT_HARNESS.slug);
  const harness = getHarness(slug) ?? DEFAULT_HARNESS;

  return (
    <div className="flex w-full flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <span className="text-sm text-muted-foreground">{label}</span>
        <Select onValueChange={setSlug} value={slug}>
          <SelectTrigger aria-label="Select your agent" className="w-60">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {HARNESSES_BY_STATUS.map((h) => (
              <SelectItem key={h.slug} value={h.slug}>
                {h.name}
                {h.status === "wip" ? " · WIP" : ""}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {harness.status === "wip" ? (
          <Badge variant="secondary">WIP · MCP fallback</Badge>
        ) : null}
      </div>

      <p className="text-sm leading-6 text-muted-foreground">
        {harness.unlocks}
      </p>

      <Terminal
        copyable
        lines={[...harness.install]}
        title={`install · ${harness.name}`}
      />

      <Link
        className="text-sm font-medium text-primary underline-offset-4 hover:underline"
        href={`/integrations/${harness.slug}`}
      >
        {`Full ${harness.name} guide →`}
      </Link>
    </div>
  );
}
