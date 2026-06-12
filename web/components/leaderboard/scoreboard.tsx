import {
  Badge,
  Card,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@vllnt/ui";

import type { Benchmark, RetrievalMode } from "@/lib/leaderboard";

const MODES: readonly { key: RetrievalMode; label: string }[] = [
  { key: "fts", label: "FTS" },
  { key: "vector", label: "Vector" },
  { key: "hybrid", label: "Hybrid" },
];

function pct(value: number): string {
  return `${value.toFixed(1)}%`;
}

/**
 * Renders one benchmark's retrieval scoreboard as a card-wrapped table,
 * highlighting the winning mode per metric row.
 *
 * @param benchmark - the benchmark and its per-metric rows
 * @returns the scoreboard card
 */
export function Scoreboard({
  benchmark,
}: {
  benchmark: Benchmark;
}): React.JSX.Element {
  return (
    <Card className="flex flex-col gap-4 overflow-hidden p-6">
      <div className="flex flex-col gap-1">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <h3 className="text-xl font-semibold tracking-tight">
            {benchmark.name}
          </h3>
          <span className="font-mono text-xs text-muted-foreground">
            {benchmark.model}
          </span>
        </div>
        <p className="text-sm text-muted-foreground">{benchmark.dataset}</p>
      </div>

      <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Metric</TableHead>
              {MODES.map((mode) => (
                <TableHead className="text-right" key={mode.key}>
                  {mode.label}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {benchmark.rows.map((row) => (
              <TableRow key={row.metric}>
                <TableCell className="font-mono text-xs">
                  {row.metric}
                </TableCell>
                {MODES.map((mode) => {
                  const isBest = row.best === mode.key;
                  return (
                    <TableCell
                      className={
                        isBest
                          ? "text-right font-semibold text-foreground"
                          : "text-right text-muted-foreground"
                      }
                      key={mode.key}
                    >
                      {isBest ? (
                        <Badge variant="secondary">{pct(row[mode.key])}</Badge>
                      ) : (
                        pct(row[mode.key])
                      )}
                    </TableCell>
                  );
                })}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <p className="text-sm leading-6 text-muted-foreground">
        {benchmark.note}
      </p>
    </Card>
  );
}
