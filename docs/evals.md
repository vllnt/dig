# Retrieval evals — scoreboard

dig measures its own retrieval against the standard memory benchmarks. The eval
harness (`tools/eval`) maps each benchmark onto a real dig KB (one file per
session), indexes it with the production pipeline (FTS5 + the opt-in vector
index), and scores ranked retrieval with standard IR metrics.

**Setup is deliberately modest and fully local**: llama.cpp on CPU, small open
embedding models, zero cloud calls — the configuration any dig user gets on a
laptop with `mode = "hybrid"` in their policy. No reranker model, no LLM in the
loop.

## Method

- **Corpus**: one KB file per conversation session (`speaker: text` lines, DATE
  header). Built via `dig`'s real scan → store → index path, not a bespoke loader.
- **Scoring**: per question, each retrieval mode ranks the question's haystack
  (its session scope); metrics over the evidence sessions (`gold`).
  - `recall@k` — fraction of evidence sessions in the top-k (averaged)
  - `hit@k` — any evidence session in the top-k (MemPalace reports this as
    `recall_any@5`)
  - `ndcg@10`, `mrr` — ranking quality
- **Modes**: `fts` (deterministic FTS5 default), `vector` (embeddings only),
  `hybrid` (FTS ∪ vector fused with RRF, k=60) — exactly what `dig find --mode`
  runs.
- Reproduce: `go run ./tools/eval --bench locomo|longmemeval --data <json>
  --workdir <kb-dir> [--stride k]` against any OpenAI-compatible embedding
  endpoint.

## LoCoMo (locomo10 — 1,536 scoreable questions, 272 session files)

Embedding model: `nomic-embed-text-v1.5` (Q8, llama.cpp, CPU). 2026-06-11.

| metric | fts | vector | **hybrid** |
|---|---|---|---|
| recall@1 | 60.0% | 50.4% | 58.7% |
| recall@3 | 74.8% | 70.2% | **77.7%** |
| recall@5 | 80.4% | 78.1% | **85.3%** |
| recall@10 | 86.0% | 88.2% | **93.2%** |
| hit@1 | 66.3% | 56.8% | 66.0% |
| hit@5 | 87.1% | 83.7% | **91.3%** |
| hit@10 | 91.3% | 92.3% | **97.1%** |
| ndcg@10 | 75.6% | 71.6% | **78.9%** |
| mrr | 75.4% | 68.9% | **76.8%** |

Hybrid beats the FTS baseline on every recall/ranking metric from rank 3 up
(+4.9pts recall@5, +7.2pts recall@10, +5.8pts hit@10). At rank 1 fusion costs
0.3pts hit@1 — the known RRF top-rank dilution; FTS stays available per query
with `--mode fts`.

## LongMemEval-S (full official set: 500 questions, 19,829 unique sessions)

Embedding model: `all-MiniLM-L6-v2` (Q8, llama.cpp, CPU) — the same model class
as ChromaDB's default, i.e. what MemPalace's published 96.6% `recall_any@5`
rides on. 2026-06-12. Corpus: 198MB of chat sessions through dig's real
scan → store → background-embed pipeline (one 20.7h CPU-only indexing run,
resumable, then queries answer in milliseconds).

| metric | fts | vector | **hybrid** |
|---|---|---|---|
| recall@1 | 40.3% | 53.9% | **56.5%** |
| recall@3 | 57.9% | 87.6% | **88.6%** |
| recall@5 | 60.9% | 93.3% | **93.9%** |
| recall@10 | 62.5% | 97.0% | **97.1%** |
| hit@1 | 62.2% | 85.0% | **89.2%** |
| hit@3 | 65.4% | 95.6% | 95.4% |
| hit@5 | 66.8% | 97.8% | **98.0%** |
| hit@10 | 67.2% | 98.8% | **98.8%** |
| ndcg@10 | 61.1% | 90.4% | **92.1%** |
| mrr | 64.1% | 90.5% | **92.8%** |

**Hybrid `hit@5` (= MemPalace's `recall_any@5`) = 98.0% vs their 96.6%** —
above the bar on the full official question set, same model class, fully
local. Against MemPalace's scores with its actual architecture enabled
(89.4% rooms / 84.2% compressed) the gap widens to +8.6/+13.8pts. Semantic
modes dominate FTS by +31.2pts hit@5 here: chat sessions paraphrase heavily,
exactly the recall gap the vector index closes — while FTS remains the
deterministic, zero-dependency default.

(A stratified 1-in-8 pilot of this run scored hybrid 96.8% hit@5 — within
sampling error of the full result.)

## BEAM (128K tier: 20 conversations, 5,732 turns, 355 scoreable questions)

Embedding model: `all-MiniLM-L6-v2` (Q8, llama.cpp, CPU). 2026-06-12.
BEAM (ICLR 2026) probes ten memory abilities inside single very-long
conversations; evidence is annotated at TURN granularity (`source_chat_ids`),
so the adapter indexes one file per turn and scores retrieval of the exact
evidence turns — a far stricter target than session-level benchmarks, and
near-duplicate turns inside one conversation make it the unsaturated frontier.
Abstention questions (no retrievable evidence) are excluded.

| metric | fts | vector | hybrid |
|---|---|---|---|
| recall@5 | 31.3% | **36.0%** | 34.8% |
| recall@10 | 39.4% | **46.1%** | 43.4% |
| hit@1 | 23.4% | **29.9%** | 29.6% |
| hit@5 | 49.0% | **54.6%** | 51.8% |
| hit@10 | 58.3% | **64.8%** | 62.3% |
| ndcg@10 | 29.9% | **35.4%** | 34.4% |
| mrr | 35.0% | **41.2%** | 40.9% |

Semantic retrieval beats FTS on every metric (+6.5pts hit@10, +6.2 mrr) —
but here **vector-only edges hybrid**: when the lexical ranking is weak,
RRF fusion dilutes rather than helps. The published BEAM bests (64.1/48.6)
are end-to-end LLM-judged QA scores from full memory pipelines — a different
measurement; these are raw retrieval numbers for the evidence turns.

## Published numbers from alternatives (for context)

| System | Benchmark | Metric | Score | Note |
|---|---|---|---|---|
| MemPalace | LongMemEval | retrieval `recall_any@5` | 96.6% | ChromaDB default embeddings on raw text; with their palace structure enabled: 89.4%, compressed: 84.2% |
| mem0 | LongMemEval | end-to-end QA accuracy (LLM judge) | 94.4 | different metric — answer quality, not retrieval recall |
| mem0 | LoCoMo | end-to-end QA accuracy (LLM judge) | 92.5 | ~6.9k tokens per retrieval call |

**Honesty note**: retrieval recall and LLM-judged QA accuracy are different
measurements; recall is structurally the higher number (finding the document is
easier than answering from it). dig reports retrieval metrics because dig is the
retrieval/management layer — answering belongs to the agent driving it.

## History

| Date | Change | Effect |
|---|---|---|
| 2026-06-11 | First scoreboard: FTS baseline vs vector vs hybrid (RRF k=60, max-pool chunk scoring, 1000/200 chunking) | baseline |
| 2026-06-11 | LoCoMo full (nomic): hybrid beats FTS on every metric from rank 3 (recall@5 85.3 vs 80.4) | hybrid > fts confirmed |
| 2026-06-11 | LongMemEval subset (MiniLM-q8): hybrid hit@5 96.8% — at MemPalace's 96.6% bar | bar reached (subset) |
| 2026-06-12 | LongMemEval FULL 500q (MiniLM-q8): hybrid hit@5 98.0% vs bar 96.6% | **bar beaten** |
| 2026-06-12 | BEAM 128K tier (MiniLM-q8): vector hit@10 64.8% vs FTS 58.3%; vector edges hybrid on turn-level evidence | frontier mapped |
