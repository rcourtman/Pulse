#!/usr/bin/env python3
"""Machine audit for v6 status.json.

Validates the live machine state schema, resolves evidence references across the
active repos, and derives lane evidence health from actual proof presence.
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import sys
from typing import Any

from canonical_completion_guard import load_subsystem_rules


REPO_ROOT = Path(__file__).resolve().parents[2]
STATUS_PATH = REPO_ROOT / "docs" / "release-control" / "v6" / "status.json"
STATUS_SCHEMA_PATH = REPO_ROOT / "docs" / "release-control" / "v6" / "status.schema.json"
SOURCE_OF_TRUTH_FILE = "docs/release-control/v6/SOURCE_OF_TRUTH.md"
REQUIRED_SOURCE_PRECEDENCE_PREFIX = [
    SOURCE_OF_TRUTH_FILE,
    "docs/release-control/v6/status.json",
    "docs/release-control/v6/status.schema.json",
    "docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md",
    "docs/release-control/v6/subsystems/registry.json",
]
DATE_RE = re.compile(r"^[0-9]{4}-[0-9]{2}-[0-9]{2}$")


def load_status_schema() -> dict[str, Any]:
    return json.loads(STATUS_SCHEMA_PATH.read_text(encoding="utf-8"))


def schema_enum(schema: dict[str, Any], definition: str, property_name: str) -> set[str]:
    properties = schema["$defs"][definition]["properties"]
    return set(properties[property_name]["enum"])


def schema_required(schema: dict[str, Any], definition: str | None = None) -> set[str]:
    target = schema if definition is None else schema["$defs"][definition]
    return set(target["required"])


STATUS_SCHEMA = load_status_schema()
VALID_LANE_STATUSES = schema_enum(STATUS_SCHEMA, "lane", "status")
VALID_OPEN_DECISION_STATUSES = schema_enum(STATUS_SCHEMA, "open_decision", "status")
VALID_RESOLVED_DECISION_KINDS = schema_enum(STATUS_SCHEMA, "resolved_decision", "kind")
REQUIRED_TOP_LEVEL_FIELDS = schema_required(STATUS_SCHEMA)


def env_key_for_repo(repo_name: str) -> str:
    return "PULSE_REPO_ROOT_" + repo_name.upper().replace("-", "_")


def repo_root_for_name(repo_name: str) -> Path:
    raw = os.environ.get(env_key_for_repo(repo_name), "").strip()
    if raw:
        return Path(raw).expanduser().resolve()
    if repo_name == "pulse":
        return REPO_ROOT
    return (REPO_ROOT.parent / repo_name).resolve()


def load_status_payload() -> dict[str, Any]:
    return json.loads(STATUS_PATH.read_text(encoding="utf-8"))


def _require_string(obj: dict[str, Any], key: str, errors: list[str], *, context: str) -> str | None:
    value = obj.get(key)
    if not isinstance(value, str) or not value.strip():
        errors.append(f"{context} missing non-empty string {key}")
        return None
    return value


def _require_bool(obj: dict[str, Any], key: str, errors: list[str], *, context: str) -> bool | None:
    value = obj.get(key)
    if not isinstance(value, bool):
        errors.append(f"{context} missing bool {key}")
        return None
    return value


def _require_number(obj: dict[str, Any], key: str, errors: list[str], *, context: str) -> float | None:
    value = obj.get(key)
    if not isinstance(value, (int, float)):
        errors.append(f"{context} missing numeric {key}")
        return None
    return float(value)


def _require_string_list(obj: dict[str, Any], key: str, errors: list[str], *, context: str) -> list[str]:
    value = obj.get(key)
    if not isinstance(value, list):
        errors.append(f"{context} missing list {key}")
        return []
    if not all(isinstance(item, str) and item.strip() for item in value):
        errors.append(f"{context} {key} must contain only non-empty strings")
        return []
    return [str(item) for item in value]


def _require_object_list(obj: dict[str, Any], key: str, errors: list[str], *, context: str) -> list[dict[str, Any]]:
    value = obj.get(key)
    if not isinstance(value, list):
        errors.append(f"{context} missing list {key}")
        return []
    objects: list[dict[str, Any]] = []
    for index, item in enumerate(value):
        if not isinstance(item, dict):
            errors.append(f"{context}.{key}[{index}] must be an object")
            continue
        objects.append(item)
    return objects


def _validate_date(value: str, errors: list[str], *, context: str) -> None:
    if not DATE_RE.match(value):
        errors.append(f"{context} must use YYYY-MM-DD")


def _validate_clean_relative_path(path: str, errors: list[str], *, context: str) -> None:
    candidate = Path(path)
    if candidate.is_absolute():
        errors.append(f"{context} path must be relative: {path!r}")
        return
    normalized = candidate.as_posix()
    if normalized != path or path.startswith("../") or "/../" in path:
        errors.append(f"{context} path must be a clean relative path: {path!r}")


def _derived_lane_status(*, at_target: bool, all_evidence_present: bool) -> str:
    if not all_evidence_present:
        return "evidence-missing"
    if at_target:
        return "target-met"
    return "behind-target"


def validate_scope(payload: dict[str, Any], errors: list[str]) -> tuple[list[str], list[str]]:
    scope = payload.get("scope")
    if not isinstance(scope, dict):
        errors.append("status.json missing scope object")
        return [], []
    active_repos = _require_string_list(scope, "active_repos", errors, context="scope")
    ignored_repos = _require_string_list(scope, "ignored_repos", errors, context="scope")
    return active_repos, ignored_repos


def validate_source_precedence(payload: dict[str, Any], errors: list[str]) -> None:
    precedence = _require_string_list(payload, "source_precedence", errors, context="status.json")
    if len(precedence) < len(REQUIRED_SOURCE_PRECEDENCE_PREFIX):
        errors.append("status.json source_precedence is shorter than required governance prefix")
        return
    for index, expected in enumerate(REQUIRED_SOURCE_PRECEDENCE_PREFIX):
        if precedence[index] != expected:
            errors.append(
                f"status.json source_precedence[{index}] = {precedence[index]!r}, want {expected!r}"
            )


def validate_priority_engine(payload: dict[str, Any], errors: list[str]) -> None:
    engine = payload.get("priority_engine")
    if not isinstance(engine, dict):
        errors.append("status.json missing priority_engine object")
        return
    _require_string(engine, "formula", errors, context="priority_engine")

    floor_rule = engine.get("floor_rule")
    if not isinstance(floor_rule, dict):
        errors.append("priority_engine missing floor_rule object")
    else:
        _require_string_list(floor_rule, "release_critical_lanes", errors, context="priority_engine.floor_rule")
        minimum_score = floor_rule.get("minimum_score")
        if not isinstance(minimum_score, (int, float)):
            errors.append("priority_engine.floor_rule missing numeric minimum_score")

    weights = engine.get("weights")
    if not isinstance(weights, dict):
        errors.append("priority_engine missing weights object")
    else:
        for key in ("gap_multiplier",):
            if not isinstance(weights.get(key), (int, float)):
                errors.append(f"priority_engine.weights missing numeric {key}")
        for key in ("criticality_range", "staleness_range", "dependency_range"):
            _require_string(weights, key, errors, context="priority_engine.weights")


def validate_evidence_policy(payload: dict[str, Any], errors: list[str]) -> tuple[set[str], str | None]:
    policy = payload.get("evidence_reference_policy")
    if not isinstance(policy, dict):
        errors.append("status.json missing evidence_reference_policy object")
        return set(), None

    format_value = _require_string(policy, "format", errors, context="evidence_reference_policy")
    if format_value and format_value != "repo-qualified-relative-paths":
        errors.append("status.json evidence_reference_policy.format must be repo-qualified-relative-paths")

    local_repo = _require_string(policy, "local_repo", errors, context="evidence_reference_policy")
    absolute_forbidden = _require_bool(
        policy,
        "absolute_paths_forbidden",
        errors,
        context="evidence_reference_policy",
    )
    if absolute_forbidden is False:
        errors.append("status.json evidence_reference_policy.absolute_paths_forbidden must be true")

    kinds = set(
        _require_string_list(
            policy,
            "allowed_kinds",
            errors,
            context="evidence_reference_policy",
        )
    )
    return kinds, local_repo


def audit_lanes(
    payload: dict[str, Any],
    *,
    active_repos: set[str],
    allowed_kinds: set[str],
    errors: list[str],
    warnings: list[str],
) -> tuple[list[dict[str, Any]], set[str]]:
    lanes = payload.get("lanes")
    if not isinstance(lanes, list) or not lanes:
        errors.append("status.json missing non-empty lanes list")
        return [], set()

    lane_reports: list[dict[str, Any]] = []
    seen_lane_ids: set[str] = set()
    lane_ids: set[str] = set()

    for index, raw_lane in enumerate(lanes):
        context = f"lanes[{index}]"
        if not isinstance(raw_lane, dict):
            errors.append(f"{context} must be an object")
            continue

        lane_id = _require_string(raw_lane, "id", errors, context=context) or f"lane-{index}"
        if lane_id in seen_lane_ids:
            errors.append(f"{context} duplicates lane id {lane_id}")
        seen_lane_ids.add(lane_id)
        lane_ids.add(lane_id)

        lane_name = _require_string(raw_lane, "name", errors, context=context) or lane_id
        target = _require_number(raw_lane, "target_score", errors, context=context) or 0.0
        current = _require_number(raw_lane, "current_score", errors, context=context) or 0.0
        status = _require_string(raw_lane, "status", errors, context=context) or "partial"

        if target < 0 or target > 10 or current < 0 or current > 10:
            errors.append(f"{context} score values must stay within 0-10")
        if current > target:
            errors.append(f"{context} current_score {current:g} exceeds target_score {target:g}")
        if status not in VALID_LANE_STATUSES:
            errors.append(f"{context} has invalid status {status!r}")
        subsystems = _require_string_list(raw_lane, "subsystems", errors, context=context)
        if len(subsystems) != len(set(subsystems)):
            errors.append(f"{context}.subsystems must not contain duplicates")
        if subsystems != sorted(subsystems):
            errors.append(f"{context}.subsystems must be sorted lexicographically")

        raw_evidence = raw_lane.get("evidence")
        if not isinstance(raw_evidence, list) or not raw_evidence:
            errors.append(f"{context} missing non-empty evidence list")
            raw_evidence = []

        missing_evidence: list[str] = []
        resolved_evidence: list[str] = []
        for evidence_index, raw_evidence_ref in enumerate(raw_evidence):
            evidence_context = f"{context}.evidence[{evidence_index}]"
            if not isinstance(raw_evidence_ref, dict):
                errors.append(f"{evidence_context} must be an object")
                continue

            repo = _require_string(raw_evidence_ref, "repo", errors, context=evidence_context)
            path = _require_string(raw_evidence_ref, "path", errors, context=evidence_context)
            kind = _require_string(raw_evidence_ref, "kind", errors, context=evidence_context)
            if repo is None or path is None or kind is None:
                continue
            if repo not in active_repos:
                errors.append(f"{evidence_context} repo {repo!r} is not in active_repos")
                continue
            if kind not in allowed_kinds:
                errors.append(f"{evidence_context} kind {kind!r} is not allowed")
                continue

            _validate_clean_relative_path(path, errors, context=evidence_context)

            repo_root = repo_root_for_name(repo)
            if not repo_root.exists() or not repo_root.is_dir():
                errors.append(
                    f"{evidence_context} repo root for {repo!r} missing at {repo_root} "
                    f"(override with {env_key_for_repo(repo)})"
                )
                continue

            resolved = repo_root / path
            if not resolved.exists():
                missing_ref = f"{repo}:{path}"
                missing_evidence.append(missing_ref)
                errors.append(f"{evidence_context} missing evidence {missing_ref}")
                continue
            if kind == "file" and not resolved.is_file():
                errors.append(f"{evidence_context} expected file at {repo}:{path}")
                continue
            if kind == "dir" and not resolved.is_dir():
                errors.append(f"{evidence_context} expected dir at {repo}:{path}")
                continue
            resolved_evidence.append(f"{repo}:{path}")

        at_target = current >= target
        all_evidence_present = len(missing_evidence) == 0
        derived_status = _derived_lane_status(
            at_target=at_target,
            all_evidence_present=all_evidence_present,
        )
        if status == "target-met" and not at_target:
            errors.append(f"{context} cannot be target-met when current_score is below target_score")
        if status == "target-met" and not all_evidence_present:
            errors.append(f"{context} cannot be target-met while evidence is missing")
        if status == "not-started" and current > 0:
            warnings.append(f"{context} is not-started but current_score is already {current:g}")

        lane_reports.append(
            {
                "id": lane_id,
                "name": lane_name,
                "target_score": target,
                "current_score": current,
                "gap": max(0.0, target - current),
                "at_target": at_target,
                "status": status,
                "subsystems": subsystems,
                "derived_status": derived_status,
                "all_evidence_present": all_evidence_present,
                "evidence_count": len(resolved_evidence),
                "missing_evidence": missing_evidence,
            }
        )

    return lane_reports, lane_ids


def validate_lane_subsystem_bindings(lane_reports: list[dict[str, Any]], errors: list[str]) -> None:
    rules = load_subsystem_rules()
    expected_by_lane: dict[str, list[str]] = {}
    for rule in rules:
        lane_id = str(rule.get("lane", "")).strip()
        subsystem_id = str(rule.get("id", "")).strip()
        if not lane_id or not subsystem_id:
            continue
        expected_by_lane.setdefault(lane_id, []).append(subsystem_id)

    for lane_id in list(expected_by_lane):
        expected_by_lane[lane_id] = sorted(expected_by_lane[lane_id])

    lane_ids = {lane["id"] for lane in lane_reports}
    for lane_id in expected_by_lane:
        if lane_id not in lane_ids:
            errors.append(f"status.json missing lane {lane_id} required by subsystem registry")

    known_subsystems = {subsystem_id for subsystem_ids in expected_by_lane.values() for subsystem_id in subsystem_ids}

    for lane in lane_reports:
        lane_id = lane["id"]
        declared = list(lane.get("subsystems", []))
        for subsystem_id in declared:
            if subsystem_id not in known_subsystems:
                errors.append(f"lanes[{lane_id}].subsystems references unknown subsystem {subsystem_id!r}")
        expected = expected_by_lane.get(lane_id, [])
        if declared != expected:
            errors.append(
                f"lanes[{lane_id}].subsystems = {declared!r}, want {expected!r} from subsystem registry"
            )


def validate_open_decisions(
    payload: dict[str, Any],
    *,
    lane_ids: set[str],
    errors: list[str],
) -> list[dict[str, Any]]:
    decisions = _require_object_list(payload, "open_decisions", errors, context="status.json")
    seen_ids: set[str] = set()
    records: list[dict[str, Any]] = []

    for index, raw in enumerate(decisions):
        context = f"open_decisions[{index}]"
        decision_id = _require_string(raw, "id", errors, context=context)
        if decision_id:
            if decision_id in seen_ids:
                errors.append(f"{context} duplicates id {decision_id}")
            seen_ids.add(decision_id)
        summary = _require_string(raw, "summary", errors, context=context)
        owner = _require_string(raw, "owner", errors, context=context)
        opened_at = _require_string(raw, "opened_at", errors, context=context)
        status = _require_string(raw, "status", errors, context=context)
        lane_refs = _require_string_list(raw, "lane_ids", errors, context=context)

        if opened_at:
            _validate_date(opened_at, errors, context=f"{context}.opened_at")
        if status and status not in VALID_OPEN_DECISION_STATUSES:
            errors.append(f"{context} has invalid status {status!r}")
        for lane_id in lane_refs:
            if lane_id not in lane_ids:
                errors.append(f"{context} references unknown lane_id {lane_id!r}")

        if decision_id and summary and owner and opened_at and status:
            records.append(
                {
                    "id": decision_id,
                    "summary": summary,
                    "owner": owner,
                    "status": status,
                    "opened_at": opened_at,
                    "lane_ids": lane_refs,
                }
            )

    return records


def validate_resolved_decisions(
    payload: dict[str, Any],
    *,
    lane_ids: set[str],
    errors: list[str],
) -> list[dict[str, Any]]:
    decisions = _require_object_list(payload, "resolved_decisions", errors, context="status.json")
    seen_ids: set[str] = set()
    records: list[dict[str, Any]] = []

    for index, raw in enumerate(decisions):
        context = f"resolved_decisions[{index}]"
        decision_id = _require_string(raw, "id", errors, context=context)
        if decision_id:
            if decision_id in seen_ids:
                errors.append(f"{context} duplicates id {decision_id}")
            seen_ids.add(decision_id)
        summary = _require_string(raw, "summary", errors, context=context)
        kind = _require_string(raw, "kind", errors, context=context)
        decided_at = _require_string(raw, "decided_at", errors, context=context)
        lane_refs = _require_string_list(raw, "lane_ids", errors, context=context)

        if kind and kind not in VALID_RESOLVED_DECISION_KINDS:
            errors.append(f"{context} has invalid kind {kind!r}")
        if decided_at:
            _validate_date(decided_at, errors, context=f"{context}.decided_at")
        for lane_id in lane_refs:
            if lane_id not in lane_ids:
                errors.append(f"{context} references unknown lane_id {lane_id!r}")

        if decision_id and summary and kind and decided_at:
            records.append(
                {
                    "id": decision_id,
                    "summary": summary,
                    "kind": kind,
                    "decided_at": decided_at,
                    "lane_ids": lane_refs,
                }
            )

    return records


def audit_status_payload(payload: dict[str, Any]) -> dict[str, Any]:
    errors: list[str] = []
    warnings: list[str] = []

    for field in sorted(REQUIRED_TOP_LEVEL_FIELDS):
        if field not in payload:
            errors.append(f"status.json missing required field {field}")

    version = _require_string(payload, "version", errors, context="status.json")
    if version and version != "6.0":
        errors.append(f"status.json version must be '6.0', got {version!r}")

    updated_at = _require_string(payload, "updated_at", errors, context="status.json")
    if updated_at:
        _validate_date(updated_at, errors, context="status.json.updated_at")

    execution_model = _require_string(payload, "execution_model", errors, context="status.json")
    if execution_model and execution_model != "direct-repo-sessions":
        errors.append("status.json execution_model must be direct-repo-sessions")

    source_of_truth_file = _require_string(payload, "source_of_truth_file", errors, context="status.json")
    if source_of_truth_file and source_of_truth_file != SOURCE_OF_TRUTH_FILE:
        errors.append(
            f"status.json source_of_truth_file must be {SOURCE_OF_TRUTH_FILE!r}, got {source_of_truth_file!r}"
        )

    active_repos, ignored_repos = validate_scope(payload, errors)
    active_repo_set = set(active_repos)
    ignored_repo_set = set(ignored_repos)
    if active_repo_set & ignored_repo_set:
        errors.append("status.json scope must not list the same repo as both active and ignored")

    validate_source_precedence(payload, errors)
    validate_priority_engine(payload, errors)
    allowed_kinds, local_repo = validate_evidence_policy(payload, errors)
    if local_repo and local_repo not in active_repo_set:
        errors.append("status.json evidence_reference_policy.local_repo must be present in active_repos")

    lane_reports, lane_ids = audit_lanes(
        payload,
        active_repos=active_repo_set,
        allowed_kinds=allowed_kinds,
        errors=errors,
        warnings=warnings,
    )
    validate_lane_subsystem_bindings(lane_reports, errors)
    open_decisions = validate_open_decisions(payload, lane_ids=lane_ids, errors=errors)
    resolved_decisions = validate_resolved_decisions(payload, lane_ids=lane_ids, errors=errors)

    return {
        "errors": errors,
        "warnings": warnings,
        "summary": {
            "lane_count": len(lane_reports),
            "lanes_at_target": sum(1 for lane in lane_reports if lane["at_target"]),
            "lanes_missing_evidence": sum(1 for lane in lane_reports if not lane["all_evidence_present"]),
            "all_evidence_present": all(lane["all_evidence_present"] for lane in lane_reports),
            "open_decision_count": len(open_decisions),
            "resolved_decision_count": len(resolved_decisions),
        },
        "lanes": lane_reports,
        "open_decisions": open_decisions,
        "resolved_decisions": resolved_decisions,
    }


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Audit docs/release-control/v6/status.json.")
    parser.add_argument(
        "--check",
        action="store_true",
        help="Exit non-zero if the status audit finds any errors.",
    )
    parser.add_argument(
        "--pretty",
        action="store_true",
        help="Print a concise human-readable summary instead of JSON.",
    )
    return parser.parse_args(argv)


def render_pretty(report: dict[str, Any]) -> str:
    lines: list[str] = []
    summary = report.get("summary", {})
    if summary:
        lines.append(
            "summary: "
            f"lanes={summary['lane_count']} "
            f"at_target={summary['lanes_at_target']} "
            f"missing_evidence={summary['lanes_missing_evidence']} "
            f"open_decisions={summary['open_decision_count']} "
            f"resolved_decisions={summary['resolved_decision_count']}"
        )
    for lane in report.get("lanes", []):
        lines.append(
            f"{lane['id']}: gap={lane['gap']:.0f} status={lane['status']} "
            f"derived={lane['derived_status']} evidence_count={lane['evidence_count']} "
            f"subsystems={','.join(lane['subsystems']) or '-'}"
        )
        for missing in lane["missing_evidence"]:
            lines.append(f"  missing {missing}")
    if report.get("warnings"):
        lines.append("warnings:")
        for warning in report["warnings"]:
            lines.append(f"  - {warning}")
    if report.get("errors"):
        lines.append("errors:")
        for err in report["errors"]:
            lines.append(f"  - {err}")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(list(argv or []))
    report = audit_status_payload(load_status_payload())
    output = render_pretty(report) if args.pretty else json.dumps(report, indent=2, sort_keys=True)
    print(output)
    if args.check and report["errors"]:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
