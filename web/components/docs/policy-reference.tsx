import { Card } from "@vllnt/ui";

import { POLICY_SECTIONS } from "@/lib/docs";

/**
 * Renders the policy-file (.dig/policy.toml) reference as a card per section.
 *
 * @returns the policy-reference cards
 */
export function PolicyReference(): React.JSX.Element {
  return (
    <div className="flex flex-col gap-4">
      {POLICY_SECTIONS.map((section) => (
        <Card className="flex flex-col gap-3 p-6" key={section.header}>
          <div className="flex flex-col gap-1">
            <h3 className="font-mono text-sm font-semibold">
              {section.header}
            </h3>
            <p className="text-sm text-muted-foreground">{section.summary}</p>
          </div>
          <dl className="flex flex-col gap-2">
            {section.keys.map((entry) => (
              <div
                className="flex flex-col gap-0.5 sm:flex-row sm:gap-3"
                key={entry.key}
              >
                <dt className="font-mono text-xs text-foreground sm:w-44 sm:shrink-0">
                  {entry.key}
                </dt>
                <dd className="text-sm text-muted-foreground">
                  {entry.describe}
                </dd>
              </div>
            ))}
          </dl>
        </Card>
      ))}
    </div>
  );
}
