import { describe, expect, it } from "vitest";

import {
  BENCHMARKS,
  EXTERNAL_BASELINES,
  HEADLINE,
  type RetrievalMode,
} from "@/lib/leaderboard";

const MODES: readonly RetrievalMode[] = ["fts", "vector", "hybrid"];
const ROWS = BENCHMARKS.flatMap((b) => b.rows);

describe("leaderboard data", () => {
  it("ships at least the three published benchmarks", () => {
    expect(BENCHMARKS.length).toBeGreaterThanOrEqual(3);
    expect(BENCHMARKS.map((b) => b.id)).toContain("longmemeval");
  });

  it("marks the actual winning mode as best on every row", () => {
    const wrong = ROWS.filter(
      (row) => row[row.best] !== Math.max(row.fts, row.vector, row.hybrid),
    );
    expect(wrong).toEqual([]);
  });

  it("keeps every score a percentage in [0, 100]", () => {
    const outOfRange = ROWS.flatMap((row) =>
      MODES.filter((mode) => row[mode] < 0 || row[mode] > 100),
    );
    expect(outOfRange).toEqual([]);
  });

  it("gives every benchmark a model, dataset, note, and rows", () => {
    const incomplete = BENCHMARKS.filter(
      (b) => !b.model || !b.dataset || !b.note || b.rows.length === 0,
    );
    expect(incomplete).toEqual([]);
  });

  it("headline beats the published bar", () => {
    expect(HEADLINE.digValue).toBeGreaterThan(HEADLINE.barValue);
  });

  it("matches the headline to a real row in its benchmark", () => {
    const lme = BENCHMARKS.find((b) => b.name.startsWith("LongMemEval"));
    const row = lme?.rows.find((r) => r.metric === HEADLINE.metric);
    expect(row?.hybrid).toBe(HEADLINE.digValue);
  });

  it("documents every external baseline with a comparability note", () => {
    expect(EXTERNAL_BASELINES.length).toBeGreaterThan(0);
    const undocumented = EXTERNAL_BASELINES.filter(
      (b) => !b.note || b.value <= 0,
    );
    expect(undocumented).toEqual([]);
  });
});
