# /// script
# requires-python = ">=3.10"
# dependencies = ["pyarrow"]
# ///
"""Convert a BEAM parquet split (huggingface.co/datasets/Mohammadta/BEAM) into
the JSON shape tools/eval's --bench beam adapter consumes.

BEAM ships conversations as parquet with python-repr probing_questions; each
question's source_chat_ids names the evidence TURN ids inside its own
conversation. Abstention questions carry no evidence and are skipped here —
retrieval recall needs a retrievable answer.

Usage:
    uv run tools/eval/beam_convert.py <split.parquet> <out.json>
"""

import ast
import json
import sys

import pyarrow.parquet as pq


def evidence_ids(value) -> list[int]:
    """Flatten source_chat_ids: list of ints, or dict of lists (temporal,
    knowledge-update questions split evidence by role)."""
    if isinstance(value, int):
        return [value]
    if isinstance(value, list):
        out: list[int] = []
        for v in value:
            out.extend(evidence_ids(v))
        return out
    if isinstance(value, dict):
        out = []
        for v in value.values():
            out.extend(evidence_ids(v))
        return out
    return []


def main() -> None:
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    table = pq.read_table(sys.argv[1])
    convs = []
    for row in table.to_pylist():
        cid = row["conversation_id"]
        turns = [
            {"id": t["id"], "role": t["role"], "content": t["content"]}
            for batch in row["chat"]
            for t in batch
        ]
        known = {t["id"] for t in turns}
        questions = []
        probing = ast.literal_eval(row["probing_questions"])
        for ability, qs in probing.items():
            if ability == "abstention":
                continue
            for i, q in enumerate(qs):
                ids = [e for e in evidence_ids(q.get("source_chat_ids")) if e in known]
                if not ids:
                    continue
                questions.append(
                    {
                        "qid": f"{cid}-{ability}-{i}",
                        "type": ability,
                        "question": q["question"],
                        "evidence_ids": sorted(set(ids)),
                    }
                )
        convs.append({"conversation_id": cid, "turns": turns, "questions": questions})
    with open(sys.argv[2], "w") as f:
        json.dump(convs, f)
    nq = sum(len(c["questions"]) for c in convs)
    nt = sum(len(c["turns"]) for c in convs)
    print(f"{len(convs)} conversations, {nt} turns, {nq} scoreable questions")


if __name__ == "__main__":
    main()
