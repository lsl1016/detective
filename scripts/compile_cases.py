#!/usr/bin/env python3
"""Compile human-authored YAML cases into runtime-only casepacks and evaluator ground truth.

Runtime output intentionally excludes truth, meta_plot, memory_quiz and evaluation_targets.
"""
from __future__ import annotations

import json
from pathlib import Path

try:
    import yaml
except ImportError as exc:
    raise SystemExit("PyYAML is required: pip install pyyaml") from exc

ROOT = Path(__file__).resolve().parents[1]
CASES = ROOT / "cases"
CASEPACK = ROOT / "casepack"
GT = ROOT / "eval" / "ground_truth"

RUNTIME_KEYS = ("case_id", "title", "briefing", "scene", "npcs", "archive")
GT_KEYS = ("case_id", "title", "truth", "meta_plot", "memory_quiz", "evaluation_targets")


def require(case: dict, key: str, path: Path):
    if key not in case:
        raise ValueError(f"{path.name}: missing key {key}")


def validate(case: dict, path: Path):
    for key in RUNTIME_KEYS + ("truth", "memory_quiz"):
        require(case, key, path)
    if not str(case["case_id"]).startswith("CASE-"):
        raise ValueError(f"{path.name}: case_id must start with CASE-")

    ids = set()
    for item in case.get("scene", []):
        item_id = item.get("id")
        if not item_id or item_id in ids:
            raise ValueError(f"{path.name}: duplicate/missing scene id {item_id!r}")
        ids.add(item_id)
    for npc in case.get("npcs", []):
        item_id = npc.get("id")
        if not item_id or item_id in ids:
            raise ValueError(f"{path.name}: duplicate/missing npc id {item_id!r}")
        ids.add(item_id)
    for item in case.get("archive", []):
        item_id = item.get("id")
        if not item_id or item_id in ids:
            raise ValueError(f"{path.name}: duplicate/missing archive id {item_id!r}")
        ids.add(item_id)

    truth = case["truth"]
    for ref in truth.get("key_evidence", []):
        if ref not in ids:
            raise ValueError(f"{path.name}: truth.key_evidence references unknown id {ref}")

    qids = set()
    for q in case.get("memory_quiz", []):
        if not q.get("id") or q["id"] in qids:
            raise ValueError(f"{path.name}: duplicate/missing quiz id")
        qids.add(q["id"])
        if "q" not in q or "a" not in q:
            raise ValueError(f"{path.name}: quiz {q['id']} missing q/a")

    for hint in case.get("meta_plot", []):
        if not hint.get("hint_id"):
            raise ValueError(f"{path.name}: meta_plot hint missing hint_id")
        refs = hint.get("source_refs", [])
        if not refs:
            raise ValueError(f"{path.name}: meta_plot {hint['hint_id']} must declare source_refs")
        for ref in refs:
            if ref not in ids:
                raise ValueError(f"{path.name}: meta_plot {hint['hint_id']} references unknown source {ref}")


def main():
    CASEPACK.mkdir(parents=True, exist_ok=True)
    GT.mkdir(parents=True, exist_ok=True)

    files = sorted(CASES.glob("CASE-*.yaml"))
    if not files:
        raise SystemExit("No cases found")

    compiled = []
    for path in files:
        case = yaml.safe_load(path.read_text(encoding="utf-8"))
        validate(case, path)

        runtime = {k: case.get(k) for k in RUNTIME_KEYS}
        ground_truth = {k: case.get(k) for k in GT_KEYS}
        cid = case["case_id"]
        (CASEPACK / f"{cid}.json").write_text(
            json.dumps(runtime, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
        )
        (GT / f"{cid}.json").write_text(
            json.dumps(ground_truth, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
        )
        compiled.append(cid)

    current = CASEPACK / "current.json"
    if not current.exists():
        current.write_text(json.dumps({"case_id": compiled[0]}, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    print(f"compiled {len(compiled)} cases: {', '.join(compiled)}")


if __name__ == "__main__":
    main()
