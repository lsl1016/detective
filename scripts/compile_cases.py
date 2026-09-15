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
GT_KEYS = ("case_id", "title", "truth", "meta_plot", "memory_quiz", "cross_case_dependencies", "cross_case_quiz", "master_truth", "evaluation_targets")


def require(case: dict, key: str, path: Path):
    if key not in case:
        raise ValueError(f"{path.name}: missing key {key}")


def validate_string_list(value, label: str, path: Path):
    if value is None:
        return
    if not isinstance(value, list) or any(not isinstance(x, str) for x in value):
        raise ValueError(f"{path.name}: {label} must be a list of strings")


def validate(case: dict, path: Path):
    for key in RUNTIME_KEYS + ("truth", "memory_quiz"):
        require(case, key, path)
    if not str(case["case_id"]).startswith("CASE-"):
        raise ValueError(f"{path.name}: case_id must start with CASE-")

    ids = set()
    id_line = {}
    for item in case.get("scene", []):
        item_id = item.get("id")
        if not item_id or item_id in ids:
            raise ValueError(f"{path.name}: duplicate/missing scene id {item_id!r}")
        ids.add(item_id)
        id_line[item_id] = "scene"
    for npc in case.get("npcs", []):
        item_id = npc.get("id")
        if not item_id or item_id in ids:
            raise ValueError(f"{path.name}: duplicate/missing npc id {item_id!r}")
        ids.add(item_id)
        id_line[item_id] = "people"
    for item in case.get("archive", []):
        item_id = item.get("id")
        if not item_id or item_id in ids:
            raise ValueError(f"{path.name}: duplicate/missing archive id {item_id!r}")
        ids.add(item_id)
        id_line[item_id] = "archive"
        validate_string_list(item.get("keywords", []), f"archive[{item_id}].keywords", path)

    truth = case["truth"]
    for field in ("key_evidence", "supporting_evidence", "method_required_terms"):
        validate_string_list(truth.get(field, []), f"truth.{field}", path)
    for ref in truth.get("key_evidence", []):
        if ref not in ids:
            raise ValueError(f"{path.name}: truth.key_evidence references unknown id {ref}")
    for ref in truth.get("supporting_evidence", []):
        if ref not in ids:
            raise ValueError(f"{path.name}: truth.supporting_evidence references unknown id {ref}")
    key_lines = {id_line[ref] for ref in truth.get("key_evidence", []) if ref in id_line}
    missing_lines = {"scene", "people", "archive"} - key_lines
    if missing_lines:
        raise ValueError(f"{path.name}: truth.key_evidence must cover all three lines; missing {sorted(missing_lines)}")

    qids = set()
    for q in case.get("memory_quiz", []):
        if not q.get("id") or q["id"] in qids:
            raise ValueError(f"{path.name}: duplicate/missing quiz id")
        qids.add(q["id"])
        if "q" not in q or "a" not in q:
            raise ValueError(f"{path.name}: quiz {q['id']} missing q/a")
        if not isinstance(q["q"], str) or not isinstance(q["a"], str):
            raise ValueError(f"{path.name}: quiz {q['id']} q/a must be strings")
        validate_string_list(q.get("aliases", []), f"quiz[{q['id']}].aliases", path)

    for hint in case.get("meta_plot", []):
        if not hint.get("hint_id"):
            raise ValueError(f"{path.name}: meta_plot hint missing hint_id")
        refs = hint.get("source_refs", [])
        validate_string_list(refs, f"meta_plot[{hint['hint_id']}].source_refs", path)
        if not refs:
            raise ValueError(f"{path.name}: meta_plot {hint['hint_id']} must declare source_refs")
        for ref in refs:
            if ref not in ids:
                raise ValueError(f"{path.name}: meta_plot {hint['hint_id']} references unknown source {ref}")

    for group, values in case.get("evaluation_targets", {}).items():
        validate_string_list(values, f"evaluation_targets.{group}", path)


def case_runtime_ids(case: dict) -> set[str]:
    ids: set[str] = set()
    for item in case.get("scene", []):
        if item.get("id"):
            ids.add(item["id"])
    for item in case.get("npcs", []):
        if item.get("id"):
            ids.add(item["id"])
    for item in case.get("archive", []):
        if item.get("id"):
            ids.add(item["id"])
    return ids


def validate_cross_case(all_cases: dict[str, dict], paths: dict[str, Path]):
    for cid, case in all_cases.items():
        path = paths[cid]
        current_ids = case_runtime_ids(case)
        dep_ids = set()
        for dep in case.get("cross_case_dependencies", []) or []:
            dep_id = dep.get("id")
            if not isinstance(dep_id, str) or not dep_id or dep_id in dep_ids:
                raise ValueError(f"{path.name}: duplicate/missing cross_case dependency id {dep_id!r}")
            dep_ids.add(dep_id)
            source_case = dep.get("source_case")
            if source_case not in all_cases:
                raise ValueError(f"{path.name}: dependency {dep_id} references unknown source_case {source_case!r}")
            validate_string_list(dep.get("source_refs", []), f"dependency[{dep_id}].source_refs", path)
            validate_string_list(dep.get("current_refs", []), f"dependency[{dep_id}].current_refs", path)
            source_ids = case_runtime_ids(all_cases[source_case])
            for ref in dep.get("source_refs", []):
                if ref not in source_ids:
                    raise ValueError(f"{path.name}: dependency {dep_id} source ref {source_case}:{ref} does not exist")
            for ref in dep.get("current_refs", []):
                if ref not in current_ids:
                    raise ValueError(f"{path.name}: dependency {dep_id} current ref {ref} does not exist")
            for field in ("historical_fact", "high_value_judgment"):
                if not isinstance(dep.get(field), str) or not dep.get(field):
                    raise ValueError(f"{path.name}: dependency {dep_id} missing string {field}")
            for field in ("required_for_single_case", "required_for_master"):
                if field in dep and not isinstance(dep[field], bool):
                    raise ValueError(f"{path.name}: dependency {dep_id}.{field} must be boolean")

        quiz_ids = set()
        for q in case.get("cross_case_quiz", []) or []:
            qid = q.get("id")
            if not isinstance(qid, str) or not qid or qid in quiz_ids:
                raise ValueError(f"{path.name}: duplicate/missing cross_case quiz id {qid!r}")
            quiz_ids.add(qid)
            for field in ("q", "a", "source_case"):
                if not isinstance(q.get(field), str) or not q.get(field):
                    raise ValueError(f"{path.name}: cross_case quiz {qid} missing string {field}")
            validate_string_list(q.get("aliases", []), f"cross_case_quiz[{qid}].aliases", path)
            validate_string_list(q.get("source_refs", []), f"cross_case_quiz[{qid}].source_refs", path)
            validate_string_list(q.get("current_refs", []), f"cross_case_quiz[{qid}].current_refs", path)
            source_case = q["source_case"]
            if source_case not in all_cases:
                raise ValueError(f"{path.name}: cross_case quiz {qid} references unknown source_case {source_case!r}")
            source_ids = case_runtime_ids(all_cases[source_case])
            for ref in q.get("source_refs", []):
                if ref not in source_ids:
                    raise ValueError(f"{path.name}: cross_case quiz {qid} source ref {source_case}:{ref} does not exist")
            for ref in q.get("current_refs", []):
                if ref not in current_ids:
                    raise ValueError(f"{path.name}: cross_case quiz {qid} current ref {ref} does not exist")
            if "historical_only" in q and not isinstance(q["historical_only"], bool):
                raise ValueError(f"{path.name}: cross_case quiz {qid}.historical_only must be boolean")

        master = case.get("master_truth")
        if master is not None:
            if not isinstance(master, dict):
                raise ValueError(f"{path.name}: master_truth must be an object")
            for field in ("mastermind", "network_name", "thesis"):
                if not isinstance(master.get(field), str) or not master.get(field):
                    raise ValueError(f"{path.name}: master_truth missing string {field}")
            validate_string_list(master.get("required_dependency_ids", []), "master_truth.required_dependency_ids", path)
            validate_string_list(master.get("required_terms", []), "master_truth.required_terms", path)
            unknown = [x for x in master.get("required_dependency_ids", []) if x not in dep_ids]
            if unknown:
                raise ValueError(f"{path.name}: master_truth references unknown dependency ids {unknown}")


def main():
    CASEPACK.mkdir(parents=True, exist_ok=True)
    GT.mkdir(parents=True, exist_ok=True)

    files = sorted(CASES.glob("CASE-*.yaml"))
    if not files:
        raise SystemExit("No cases found")

    compiled = []
    all_cases: dict[str, dict] = {}
    paths: dict[str, Path] = {}
    for path in files:
        case = yaml.safe_load(path.read_text(encoding="utf-8"))
        validate(case, path)
        cid = case["case_id"]
        if cid in all_cases:
            raise ValueError(f"duplicate case_id {cid}")
        all_cases[cid] = case
        paths[cid] = path

    validate_cross_case(all_cases, paths)

    for cid in sorted(all_cases):
        case = all_cases[cid]
        runtime = {k: case.get(k) for k in RUNTIME_KEYS}
        ground_truth = {k: case.get(k) for k in GT_KEYS if case.get(k) is not None}
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
