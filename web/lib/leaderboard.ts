/**
 * Benchmark scoreboard data for the leaderboard page, mirroring
 * `docs/evals.md`. All dig numbers are retrieval recall through dig's real
 * scan → store → index pipeline, fully local on CPU with small open embedding
 * models — no reranker, no LLM in the loop. Keep this in sync with evals.md.
 */

/** A retrieval mode dig can run a query in. */
export type RetrievalMode = "fts" | "hybrid" | "vector";

/** One metric row across the three retrieval modes (percentages, 0–100). */
export type ScoreRow = {
  /** Which mode wins this row — drives the visual highlight. */
  best: RetrievalMode;
  /** Deterministic full-text search (the default). */
  fts: number;
  /** FTS ∪ vector fused with Reciprocal Rank Fusion. */
  hybrid: number;
  /** Metric label, e.g. "hit@5" or "recall@10". */
  metric: string;
  /** Semantic vector retrieval. */
  vector: number;
};

/** A benchmark with dig's per-metric scores. */
export type Benchmark = {
  /** One-line dataset description (size, scope). */
  dataset: string;
  /** Stable id for keys/anchors. */
  id: string;
  /** Embedding model used (llama.cpp, CPU). */
  model: string;
  /** Display name. */
  name: string;
  /** Short framing note shown under the table. */
  note: string;
  /** Metric rows. */
  rows: ScoreRow[];
};

/** A published score from another system, shown for context. */
export type ExternalBaseline = {
  /** Benchmark name. */
  benchmark: string;
  /** Metric reported. */
  metric: string;
  /** Comparability caveat for this score. */
  note: string;
  /** System name. */
  system: string;
  /** Score (0–100). */
  value: number;
};

/** The headline result, surfaced above the tables. */
export const HEADLINE = {
  barSystem: "MemPalace",
  barValue: 96.6,
  benchmark: "LongMemEval-S (full 500 questions)",
  digValue: 98,
  metric: "hit@5",
} as const;

/** dig's measured benchmarks. */
export const BENCHMARKS: readonly Benchmark[] = [
  {
    dataset: "Full official set — 500 questions, 19,829 sessions",
    id: "longmemeval",
    model: "all-MiniLM-L6-v2 (Q8)",
    name: "LongMemEval-S",
    note: "Hybrid hit@5 of 98.0% clears MemPalace's published 96.6% recall_any@5 on the same model class, fully local. Chat sessions paraphrase heavily — exactly the recall gap the vector index closes over FTS (+31.2pts).",
    rows: [
      {
        best: "hybrid",
        fts: 60.9,
        hybrid: 93.9,
        metric: "recall@5",
        vector: 93.3,
      },
      {
        best: "hybrid",
        fts: 62.5,
        hybrid: 97.1,
        metric: "recall@10",
        vector: 97,
      },
      {
        best: "hybrid",
        fts: 66.8,
        hybrid: 98,
        metric: "hit@5",
        vector: 97.8,
      },
      {
        best: "hybrid",
        fts: 67.2,
        hybrid: 98.8,
        metric: "hit@10",
        vector: 98.8,
      },
      {
        best: "hybrid",
        fts: 61.1,
        hybrid: 92.1,
        metric: "ndcg@10",
        vector: 90.4,
      },
      { best: "hybrid", fts: 64.1, hybrid: 92.8, metric: "mrr", vector: 90.5 },
    ],
  },
  {
    dataset: "1,536 questions, multi-session conversations",
    id: "locomo",
    model: "nomic-embed-text-v1.5 (Q8)",
    name: "LoCoMo",
    note: "Hybrid beats the deterministic FTS baseline on every metric from rank 3 up (+4.9pts recall@5, +5.8pts hit@10). FTS stays the zero-dependency default, available per query.",
    rows: [
      {
        best: "hybrid",
        fts: 80.4,
        hybrid: 85.3,
        metric: "recall@5",
        vector: 78.1,
      },
      {
        best: "hybrid",
        fts: 86,
        hybrid: 93.2,
        metric: "recall@10",
        vector: 88.2,
      },
      {
        best: "hybrid",
        fts: 87.1,
        hybrid: 91.3,
        metric: "hit@5",
        vector: 83.7,
      },
      {
        best: "hybrid",
        fts: 91.3,
        hybrid: 97.1,
        metric: "hit@10",
        vector: 92.3,
      },
      {
        best: "hybrid",
        fts: 75.6,
        hybrid: 78.9,
        metric: "ndcg@10",
        vector: 71.6,
      },
      { best: "hybrid", fts: 75.4, hybrid: 76.8, metric: "mrr", vector: 68.9 },
    ],
  },
  {
    dataset: "355 questions, 5,732 turns — turn-level evidence",
    id: "beam",
    model: "all-MiniLM-L6-v2 (Q8)",
    name: "BEAM (128K tier)",
    note: "The unsaturated frontier (ICLR 2026): one long conversation, near-duplicate turns, evidence annotated per turn. Semantic beats FTS on every metric; here vector edges hybrid — weak lexical rankings dilute RRF. Larger tiers are backfilling.",
    rows: [
      {
        best: "vector",
        fts: 31.3,
        hybrid: 34.8,
        metric: "recall@5",
        vector: 36,
      },
      {
        best: "vector",
        fts: 39.4,
        hybrid: 43.4,
        metric: "recall@10",
        vector: 46.1,
      },
      {
        best: "vector",
        fts: 49,
        hybrid: 51.8,
        metric: "hit@5",
        vector: 54.6,
      },
      {
        best: "vector",
        fts: 58.3,
        hybrid: 62.3,
        metric: "hit@10",
        vector: 64.8,
      },
      {
        best: "vector",
        fts: 29.9,
        hybrid: 34.4,
        metric: "ndcg@10",
        vector: 35.4,
      },
      { best: "vector", fts: 35, hybrid: 40.9, metric: "mrr", vector: 41.2 },
    ],
  },
];

/** Published numbers from other systems, shown for honest context. */
export const EXTERNAL_BASELINES: readonly ExternalBaseline[] = [
  {
    benchmark: "LongMemEval",
    metric: "recall_any@5",
    note: "Same metric as dig's hit@5 — its default-embeddings number; with its own structure enabled it drops to 89.4% (rooms) / 84.2% (compressed).",
    system: "MemPalace",
    value: 96.6,
  },
  {
    benchmark: "LongMemEval",
    metric: "QA accuracy (LLM judge)",
    note: "Different metric — end-to-end answer quality, not retrieval recall. Structurally lower than recall; not directly comparable.",
    system: "mem0",
    value: 94.4,
  },
  {
    benchmark: "LoCoMo",
    metric: "QA accuracy (LLM judge)",
    note: "Different metric — answer quality at ~6.9k tokens per query.",
    system: "mem0",
    value: 92.5,
  },
];
