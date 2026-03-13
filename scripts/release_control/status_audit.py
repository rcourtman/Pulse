#!/usr/bin/env python3
"""Machine audit for the active release profile status.json.

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
from control_plane import DEFAULT_CONTROL_PLANE, active_target_blocking_levels, is_prerelease_version
from repo_file_io import load_repo_json, read_repo_text


REPO_ROOT = Path(__file__).resolve().parents[2]
ACTIVE_PROFILE_ID = DEFAULT_CONTROL_PLANE["active_profile_id"]
ACTIVE_TARGET = dict(DEFAULT_CONTROL_PLANE["active_target"])
STATUS_PATH = DEFAULT_CONTROL_PLANE["status_path"]
STATUS_SCHEMA_PATH = DEFAULT_CONTROL_PLANE["status_schema_path"]
SOURCE_OF_TRUTH_FILE = DEFAULT_CONTROL_PLANE["source_of_truth_rel"]
HIGH_RISK_RELEASE_MATRIX = DEFAULT_CONTROL_PLANE["high_risk_matrix_rel"]
REPO_READY_BLOCKER = "Repo readiness is not yet satisfied; rc_ready and release_ready cannot pass until repo_ready is true."
RC_READY_ASSERTIONS_BLOCKER = (
    "Required rc-ready assertions remain pending or blocked in status.json.readiness_assertions."
)
RC_OPEN_DECISIONS_BLOCKER = (
    "RC-blocking operational decisions remain in status.json.open_decisions."
)
RC_RELEASE_GATES_BLOCKER = (
    "RC-blocking high-risk release gates remain pending or blocked in status.json.release_gates."
)
RELEASE_READY_ASSERTIONS_BLOCKER = (
    "Required release-ready assertions remain pending or blocked in status.json.readiness_assertions."
)
RELEASE_OPEN_DECISIONS_BLOCKER = (
    "Release-blocking operational decisions remain in status.json.open_decisions."
)
RELEASE_GATES_BLOCKER = (
    "Release-blocking high-risk release gates remain pending or blocked in status.json.release_gates."
)
REQUIRED_SOURCE_PRECEDENCE = [
    SOURCE_OF_TRUTH_FILE,
    DEFAULT_CONTROL_PLANE["status_rel"],
    DEFAULT_CONTROL_PLANE["status_schema_rel"],
    DEFAULT_CONTROL_PLANE["development_protocol_rel"],
    DEFAULT_CONTROL_PLANE["registry_rel"],
    DEFAULT_CONTROL_PLANE["registry_schema_rel"],
]
DATE_RE = re.compile(r"^[0-9]{4}-[0-9]{2}-[0-9]{2}$")
EVIDENCE_TIER_ORDER = [
    "test-proof",
    "local-rehearsal",
    "managed-runtime-exercise",
    "real-external-e2e",
    "production-observed",
]
EVIDENCE_TIER_RANK = {tier: index for index, tier in enumerate(EVIDENCE_TIER_ORDER)}


def _lane_sort_key(value: str) -> tuple[int, str]:
    match = re.match(r"^L([0-9]+)$", value)
    if not match:
        return (sys.maxsize, value)
    return (int(match.group(1)), value)


def _readiness_assertion_sort_key(value: str) -> tuple[int, str]:
    match = re.match(r"^RA([0-9]+)$", value)
    if not match:
        return (sys.maxsize, value)
    return (int(match.group(1)), value)


def _evidence_sort_key(value: tuple[str, str, str]) -> tuple[str, str, str]:
    repo, path, kind = value
    return (repo.casefold(), path.casefold(), kind.casefold())


def _is_executable_proof_artifact(path: str) -> bool:
    filename = Path(path).name
    return (
        filename.endswith("_test.go")
        or filename.endswith("_test.py")
        or ".test." in filename
        or ".spec." in filename
    )


def _proof_command_sort_key(value: str) -> str:
    return value.casefold()


def _evidence_tier_rank(value: str | None) -> int:
    if value is None:
        return -1
    return EVIDENCE_TIER_RANK.get(value, -1)


def _highest_evidence_tier(values: list[str]) -> str | None:
    if not values:
        return None
    return max(values, key=_evidence_tier_rank)


def _evidence_tier_meets_minimum(highest: str | None, minimum: str | None) -> bool:
    if minimum is None:
        return True
    return _evidence_tier_rank(highest) >= _evidence_tier_rank(minimum)


def _repo_sort_key(value: str) -> str:
    return value.casefold()


def _decision_sort_key(value: tuple[str, str]) -> tuple[str, str]:
    date_value, decision_id = value
    return (date_value, decision_id.casefold())


def _lane_completion_tracking_sort_key(value: tuple[str, str]) -> tuple[str, str]:
    kind, entry_id = value
    return (kind.casefold(), entry_id.casefold())


def _blocker_detail(
    item: dict[str, Any],
    *,
    summary_key: str,
    status_key: str,
) -> dict[str, Any]:
    detail = {
        "id": item["id"],
        "blocking_level": item["blocking_level"],
        "status": item[status_key],
        "summary": item[summary_key],
        "repo_ids": list(item["repo_ids"]),
        "lane_ids": list(item.get("lane_ids", [])),
    }
    if "effective_status" in item:
        detail["effective_status"] = item["effective_status"]
    if "highest_evidence_tier" in item:
        detail["highest_evidence_tier"] = item["highest_evidence_tier"]
    if "minimum_evidence_tier" in item:
        detail["minimum_evidence_tier"] = item["minimum_evidence_tier"]
    return detail


def _lane_tracking_detail(
    tracking: dict[str, str],
    *,
    lane_followups_by_id: dict[str, dict[str, Any]],
    readiness_assertions_by_id: dict[str, dict[str, Any]],
    release_gates_by_id: dict[str, dict[str, Any]],
    open_decisions_by_id: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    tracking_kind = str(tracking["kind"])
    tracking_id = str(tracking["id"])
    detail: dict[str, Any] = {
        "kind": tracking_kind,
        "id": tracking_id,
        "status": "unknown",
        "resolved": False,
    }
    if tracking_kind == "lane-followup":
        followup = lane_followups_by_id.get(tracking_id)
        if followup:
            detail["status"] = str(followup.get("status", "unknown"))
            detail["resolved"] = detail["status"] == "done"
            detail["summary"] = str(followup.get("summary", ""))
        return detail
    if tracking_kind == "readiness-assertion":
        assertion = readiness_assertions_by_id.get(tracking_id)
        if assertion:
            detail["status"] = str(assertion.get("derived_status", "unknown"))
            detail["resolved"] = bool(assertion.get("derived_pass"))
            detail["summary"] = str(assertion.get("summary", ""))
        return detail
    if tracking_kind == "release-gate":
        gate = release_gates_by_id.get(tracking_id)
        if gate:
            detail["status"] = str(gate.get("effective_status", gate.get("status", "unknown")))
            detail["resolved"] = detail["status"] == "passed"
            detail["summary"] = str(gate.get("summary", ""))
        return detail
    if tracking_kind == "open-decision":
        decision = open_decisions_by_id.get(tracking_id)
        if decision:
            detail["status"] = str(decision.get("status", "unknown"))
            detail["resolved"] = False
            detail["summary"] = str(decision.get("summary", ""))
        return detail
    return detail


def _phase_blocker_details(
    *,
    assertions: list[dict[str, Any]],
    open_decisions: list[dict[str, Any]],
    release_gates: list[dict[str, Any]],
    blocking_levels: set[str],
) -> dict[str, list[dict[str, Any]]]:
    return {
        "assertions": [
            _blocker_detail(assertion, summary_key="summary", status_key="derived_status")
            for assertion in assertions
            if assertion["blocking_level"] in blocking_levels and not assertion["derived_pass"]
        ],
        "open_decisions": [
            _blocker_detail(decision, summary_key="summary", status_key="status")
            for decision in open_decisions
            if decision["blocking_level"] in blocking_levels
        ],
        "release_gates": [
            _blocker_detail(gate, summary_key="summary", status_key="effective_status")
            for gate in release_gates
            if gate["blocking_level"] in blocking_levels and gate["effective_status"] != "passed"
        ],
    }


def load_status_schema(*, staged: bool = False) -> dict[str, Any]:
    return load_repo_json(STATUS_SCHEMA_PATH, staged=staged)


def schema_enum(schema: dict[str, Any], definition: str, property_name: str) -> set[str]:
    properties = schema["$defs"][definition]["properties"]
    return set(properties[property_name]["enum"])


def schema_required(schema: dict[str, Any], definition: str | None = None) -> set[str]:
    target = schema if definition is None else schema["$defs"][definition]
    return set(target["required"])


def status_schema_contract(*, staged: bool = False) -> dict[str, Any]:
    schema = load_status_schema(staged=staged)
    return {
        "schema": schema,
        "valid_lane_statuses": schema_enum(schema, "lane", "status"),
        "valid_evidence_tiers": schema_enum(schema, "evidence_reference", "evidence_tier"),
        "valid_readiness_assertion_blocking_levels": schema_enum(
            schema, "readiness_assertion", "blocking_level"
        ),
        "valid_readiness_assertion_kinds": schema_enum(schema, "readiness_assertion", "kind"),
        "valid_readiness_assertion_proof_types": schema_enum(
            schema, "readiness_assertion", "proof_type"
        ),
        "valid_release_gate_blocking_levels": schema_enum(schema, "release_gate", "blocking_level"),
        "valid_release_gate_statuses": schema_enum(schema, "release_gate", "status"),
        "valid_lane_followup_statuses": schema_enum(schema, "lane_followup", "status"),
        "valid_open_decision_blocking_levels": schema_enum(schema, "open_decision", "blocking_level"),
        "valid_open_decision_statuses": schema_enum(schema, "open_decision", "status"),
        "valid_resolved_decision_kinds": schema_enum(schema, "resolved_decision", "kind"),
        "required_top_level_fields": schema_required(schema),
    }


DEFAULT_STATUS_SCHEMA_CONTRACT = status_schema_contract()


def env_key_for_repo(repo_name: str) -> str:
    return "PULSE_REPO_ROOT_" + repo_name.upper().replace("-", "_")


def repo_root_for_name(repo_name: str) -> Path:
    raw = os.environ.get(env_key_for_repo(repo_name), "").strip()
    if raw:
        return Path(raw).expanduser().resolve()
    if repo_name == "pulse":
        return REPO_ROOT
    return (REPO_ROOT.parent / repo_name).resolve()


def load_status_payload(*, staged: bool = False) -> dict[str, Any]:
    return load_repo_json(STATUS_PATH, staged=staged)


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


def _validate_clean_relative_dir(path: str, errors: list[str], *, context: str) -> None:
    _validate_clean_relative_path(path, errors, context=context)
    resolved = REPO_ROOT / path
    if not resolved.exists():
        errors.append(f"{context} cwd missing directory {path!r}")
        return
    if not resolved.is_dir():
        errors.append(f"{context} cwd must be a directory: {path!r}")


def _derived_lane_status(*, at_target: bool, all_evidence_present: bool) -> str:
    if not all_evidence_present:
        return "evidence-missing"
    if at_target:
        return "target-met"
    return "behind-target"


def _derived_readiness_assertion_status(
    *,
    proof_type: str,
    all_evidence_present: bool,
    linked_release_gates_cleared: bool,
) -> str:
    if not all_evidence_present:
        return "evidence-missing"
    if proof_type in {"manual", "hybrid"} and not linked_release_gates_cleared:
        return "gates-pending"
    return "passed"


def audit_evidence_refs(
    raw_evidence: Any,
    *,
    context: str,
    active_repos: set[str],
    allowed_kinds: set[str],
    valid_evidence_tiers: set[str],
    errors: list[str],
) -> dict[str, Any]:
    if not isinstance(raw_evidence, list) or not raw_evidence:
        errors.append(f"{context} missing non-empty evidence list")
        raw_evidence = []

    missing_evidence: list[str] = []
    resolved_evidence: list[str] = []
    evidence_refs: list[tuple[str, str, str]] = []
    evidence_tiers: list[str] = []

    for evidence_index, raw_evidence_ref in enumerate(raw_evidence):
        evidence_context = f"{context}.evidence[{evidence_index}]"
        if not isinstance(raw_evidence_ref, dict):
            errors.append(f"{evidence_context} must be an object")
            continue

        repo = _require_string(raw_evidence_ref, "repo", errors, context=evidence_context)
        path = _require_string(raw_evidence_ref, "path", errors, context=evidence_context)
        kind = _require_string(raw_evidence_ref, "kind", errors, context=evidence_context)
        evidence_tier = raw_evidence_ref.get("evidence_tier")
        if evidence_tier is not None:
            if not isinstance(evidence_tier, str) or not evidence_tier.strip():
                errors.append(f"{evidence_context}.evidence_tier must be a non-empty string when declared")
                evidence_tier = None
            elif evidence_tier not in valid_evidence_tiers:
                errors.append(f"{evidence_context}.evidence_tier has invalid value {evidence_tier!r}")
                evidence_tier = None
        if repo is None or path is None or kind is None:
            continue
        evidence_refs.append((repo, path, kind))
        if evidence_tier:
            evidence_tiers.append(evidence_tier)
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

    if len(evidence_refs) != len(set(evidence_refs)):
        errors.append(f"{context}.evidence must not contain duplicate repo/path/kind references")
    if evidence_refs != sorted(evidence_refs, key=_evidence_sort_key):
        errors.append(f"{context}.evidence must be sorted by repo, path, then kind")

    return {
        "missing_evidence": missing_evidence,
        "resolved_evidence": resolved_evidence,
        "evidence_refs": evidence_refs,
        "highest_evidence_tier": _highest_evidence_tier(evidence_tiers),
        "all_evidence_present": len(missing_evidence) == 0,
    }


def validate_scope(payload: dict[str, Any], errors: list[str]) -> dict[str, Any]:
    scope = payload.get("scope")
    if not isinstance(scope, dict):
        errors.append("status.json missing scope object")
        return {
            "active_repos": [],
            "control_plane_repo": None,
            "ignored_repos": [],
            "repo_catalog": [],
        }

    active_repos = _require_string_list(scope, "active_repos", errors, context="scope")
    control_plane_repo = _require_string(scope, "control_plane_repo", errors, context="scope")
    ignored_repos = _require_string_list(scope, "ignored_repos", errors, context="scope")
    repo_catalog_raw = _require_object_list(scope, "repo_catalog", errors, context="scope")

    if len(active_repos) != len(set(active_repos)):
        errors.append("scope.active_repos must not contain duplicates")
    if active_repos != sorted(active_repos, key=_repo_sort_key):
        errors.append("scope.active_repos must be sorted lexicographically")
    if len(ignored_repos) != len(set(ignored_repos)):
        errors.append("scope.ignored_repos must not contain duplicates")
    if ignored_repos != sorted(ignored_repos, key=_repo_sort_key):
        errors.append("scope.ignored_repos must be sorted lexicographically")
    if control_plane_repo and control_plane_repo not in active_repos:
        errors.append("scope.control_plane_repo must be present in scope.active_repos")

    repo_catalog: list[dict[str, Any]] = []
    seen_repo_ids: set[str] = set()
    repo_catalog_ids: list[str] = []
    for index, raw_repo in enumerate(repo_catalog_raw):
        context = f"scope.repo_catalog[{index}]"
        repo_id = _require_string(raw_repo, "id", errors, context=context)
        purpose = _require_string(raw_repo, "purpose", errors, context=context)
        visibility = _require_string(raw_repo, "visibility", errors, context=context)
        if repo_id:
            if repo_id in seen_repo_ids:
                errors.append(f"{context} duplicates id {repo_id}")
            seen_repo_ids.add(repo_id)
            repo_catalog_ids.append(repo_id)
        if visibility and visibility not in {"public", "private"}:
            errors.append(f"{context} has invalid visibility {visibility!r}")
        if repo_id and purpose and visibility:
            repo_catalog.append(
                {
                    "id": repo_id,
                    "purpose": purpose,
                    "visibility": visibility,
                }
            )

    if repo_catalog_ids != sorted(repo_catalog_ids, key=_repo_sort_key):
        errors.append("scope.repo_catalog must be sorted by repo id")

    if len(repo_catalog) == len(repo_catalog_raw):
        catalog_repo_ids = [entry["id"] for entry in repo_catalog]
        if catalog_repo_ids != active_repos:
            errors.append("scope.repo_catalog ids must exactly match scope.active_repos in the same order")

    return {
        "active_repos": active_repos,
        "control_plane_repo": control_plane_repo,
        "ignored_repos": ignored_repos,
        "repo_catalog": repo_catalog,
    }


def _derived_repo_ids_for_lane_refs(
    lane_refs: list[str],
    *,
    lane_repo_ids: dict[str, list[str]],
) -> list[str]:
    derived: set[str] = set()
    for lane_id in lane_refs:
        derived.update(lane_repo_ids.get(lane_id, []))
    return sorted(derived, key=_repo_sort_key)


def validate_source_precedence(payload: dict[str, Any], errors: list[str]) -> None:
    precedence = _require_string_list(payload, "source_precedence", errors, context="status.json")
    if precedence != REQUIRED_SOURCE_PRECEDENCE:
        errors.append(
            f"status.json source_precedence = {precedence!r}, want {REQUIRED_SOURCE_PRECEDENCE!r}"
        )
        return


def validate_priority_engine(payload: dict[str, Any], errors: list[str]) -> list[str]:
    engine = payload.get("priority_engine")
    if not isinstance(engine, dict):
        errors.append("status.json missing priority_engine object")
        return []
    _require_string(engine, "formula", errors, context="priority_engine")

    release_critical_lanes: list[str] = []
    floor_rule = engine.get("floor_rule")
    if not isinstance(floor_rule, dict):
        errors.append("priority_engine missing floor_rule object")
    else:
        release_critical_lanes = _require_string_list(
            floor_rule,
            "release_critical_lanes",
            errors,
            context="priority_engine.floor_rule",
        )
        if len(release_critical_lanes) != len(set(release_critical_lanes)):
            errors.append("priority_engine.floor_rule.release_critical_lanes must not contain duplicates")
        if release_critical_lanes != sorted(release_critical_lanes, key=_lane_sort_key):
            errors.append("priority_engine.floor_rule.release_critical_lanes must be sorted by lane id")
        minimum_score = floor_rule.get("minimum_score")
        if not isinstance(minimum_score, (int, float)):
            errors.append("priority_engine.floor_rule missing numeric minimum_score")

    weights = engine.get("weights")
    if not isinstance(weights, dict):
        errors.append("priority_engine missing weights object")
    else:
        for key in ("gap_multiplier", "blocker_bonus"):
            if not isinstance(weights.get(key), (int, float)):
                errors.append(f"priority_engine.weights missing numeric {key}")
        for key in ("criticality_range", "staleness_range", "dependency_range"):
            _require_string(weights, key, errors, context="priority_engine.weights")
    return release_critical_lanes


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


def validate_readiness(payload: dict[str, Any], errors: list[str]) -> dict[str, Any]:
    readiness = payload.get("readiness")
    if not isinstance(readiness, dict):
        errors.append("status.json missing readiness object")
        return {}

    repo_ready_rule = _require_string(readiness, "repo_ready_rule", errors, context="readiness")
    if (
        repo_ready_rule
        and repo_ready_rule
        != "all lanes target-met and evidence-present plus all repo-ready assertions passed"
    ):
        errors.append(
            "status.json readiness.repo_ready_rule must be "
            "'all lanes target-met and evidence-present plus all repo-ready assertions passed'"
        )

    rc_ready_rule = _require_string(readiness, "rc_ready_rule", errors, context="readiness")
    if (
        rc_ready_rule
        and rc_ready_rule
        != "repo_ready plus all rc-ready assertions passed plus zero rc-ready open_decisions plus all rc-ready release_gates passed"
    ):
        errors.append(
            "status.json readiness.rc_ready_rule must be "
            "'repo_ready plus all rc-ready assertions passed plus zero rc-ready open_decisions plus all rc-ready release_gates passed'"
        )

    release_ready_rule = _require_string(readiness, "release_ready_rule", errors, context="readiness")
    if (
        release_ready_rule
        and release_ready_rule
        != "rc_ready plus all release-ready assertions passed plus zero release-ready open_decisions plus all release-ready release_gates passed"
    ):
        errors.append(
            "status.json readiness.release_ready_rule must be "
            "'rc_ready plus all release-ready assertions passed plus zero release-ready open_decisions plus all release-ready release_gates passed'"
        )

    return {
        "repo_ready_rule": repo_ready_rule,
        "rc_ready_rule": rc_ready_rule,
        "release_ready_rule": release_ready_rule,
    }


def audit_lanes(
    payload: dict[str, Any],
    *,
    active_repos: set[str],
    allowed_kinds: set[str],
    valid_evidence_tiers: set[str],
    valid_lane_statuses: set[str],
    errors: list[str],
    warnings: list[str],
) -> tuple[list[dict[str, Any]], set[str], dict[str, list[str]]]:
    lanes = payload.get("lanes")
    if not isinstance(lanes, list) or not lanes:
        errors.append("status.json missing non-empty lanes list")
        return [], set()

    lane_reports: list[dict[str, Any]] = []
    seen_lane_ids: set[str] = set()
    lane_ids: set[str] = set()
    lane_order: list[str] = []
    lane_repo_ids: dict[str, list[str]] = {}

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
        lane_order.append(lane_id)

        lane_name = _require_string(raw_lane, "name", errors, context=context) or lane_id
        target = _require_number(raw_lane, "target_score", errors, context=context) or 0.0
        current = _require_number(raw_lane, "current_score", errors, context=context) or 0.0
        status = _require_string(raw_lane, "status", errors, context=context) or "partial"
        raw_completion = raw_lane.get("completion")

        if target < 0 or target > 10 or current < 0 or current > 10:
            errors.append(f"{context} score values must stay within 0-10")
        if current > target:
            errors.append(f"{context} current_score {current:g} exceeds target_score {target:g}")
        if status not in valid_lane_statuses:
            errors.append(f"{context} has invalid status {status!r}")
        completion_state = "open"
        completion_summary = ""
        completion_tracking: list[dict[str, str]] = []
        completion_tracking_keys: list[tuple[str, str]] = []
        if not isinstance(raw_completion, dict):
            errors.append(f"{context}.completion must be an object")
        else:
            completion_state = (
                _require_string(raw_completion, "state", errors, context=f"{context}.completion") or "open"
            )
            completion_summary = (
                _require_string(raw_completion, "summary", errors, context=f"{context}.completion")
                or ""
            )
            raw_tracking = raw_completion.get("tracking")
            if not isinstance(raw_tracking, list):
                errors.append(f"{context}.completion.tracking must be a list")
                raw_tracking = []
            for tracking_index, raw_tracking_ref in enumerate(raw_tracking):
                tracking_context = f"{context}.completion.tracking[{tracking_index}]"
                if not isinstance(raw_tracking_ref, dict):
                    errors.append(f"{tracking_context} must be an object")
                    continue
                tracking_kind = _require_string(raw_tracking_ref, "kind", errors, context=tracking_context)
                tracking_id = _require_string(raw_tracking_ref, "id", errors, context=tracking_context)
                if tracking_kind is None or tracking_id is None:
                    continue
                completion_tracking.append({"kind": tracking_kind, "id": tracking_id})
                completion_tracking_keys.append((tracking_kind, tracking_id))
        valid_completion_states = {"open", "bounded-residual", "complete"}
        valid_tracking_kinds = {
            "lane-followup",
            "readiness-assertion",
            "release-gate",
            "open-decision",
        }
        if completion_state not in valid_completion_states:
            errors.append(f"{context}.completion.state has invalid value {completion_state!r}")
        for tracking in completion_tracking:
            if tracking["kind"] not in valid_tracking_kinds:
                errors.append(
                    f"{context}.completion.tracking kind {tracking['kind']!r} is not supported"
                )
        if len(completion_tracking_keys) != len(set(completion_tracking_keys)):
            errors.append(f"{context}.completion.tracking must not contain duplicate kind/id pairs")
        if completion_tracking_keys != sorted(
            completion_tracking_keys,
            key=_lane_completion_tracking_sort_key,
        ):
            errors.append(f"{context}.completion.tracking must be sorted by kind then id")
        subsystems = _require_string_list(raw_lane, "subsystems", errors, context=context)
        if len(subsystems) != len(set(subsystems)):
            errors.append(f"{context}.subsystems must not contain duplicates")
        if subsystems != sorted(subsystems):
            errors.append(f"{context}.subsystems must be sorted lexicographically")

        raw_evidence = raw_lane.get("evidence")
        evidence_report = audit_evidence_refs(
            raw_evidence,
            context=context,
            active_repos=active_repos,
            allowed_kinds=allowed_kinds,
            valid_evidence_tiers=valid_evidence_tiers,
            errors=errors,
        )

        at_target = current >= target
        all_evidence_present = evidence_report["all_evidence_present"]
        repo_ids = sorted({repo for repo, _, _ in evidence_report["evidence_refs"]}, key=_repo_sort_key)
        lane_repo_ids[lane_id] = repo_ids
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
        if status == "target-met" and completion_state != "complete":
            errors.append(f"{context} target-met lanes must use completion.state='complete'")
        if at_target and status == "partial" and completion_state != "bounded-residual":
            errors.append(
                f"{context} partial lanes that already meet target_score must use completion.state='bounded-residual'"
            )
        if not at_target and completion_state != "open":
            errors.append(f"{context} lanes below target_score must use completion.state='open'")
        if completion_state == "bounded-residual" and not completion_tracking:
            errors.append(f"{context} bounded-residual completion must declare at least one tracking reference")
        if completion_state == "complete" and completion_tracking:
            errors.append(f"{context} complete lanes must not declare residual tracking references")

        lane_reports.append(
            {
                "id": lane_id,
                "name": lane_name,
                "target_score": target,
                "current_score": current,
                "gap": max(0.0, target - current),
                "at_target": at_target,
                "status": status,
                "completion_state": completion_state,
                "completion_summary": completion_summary,
                "completion_tracking": completion_tracking,
                "subsystems": subsystems,
                "repo_ids": repo_ids,
                "cross_repo": len(repo_ids) > 1,
                "derived_status": derived_status,
                "all_evidence_present": all_evidence_present,
                "evidence_count": len(evidence_report["resolved_evidence"]),
                "missing_evidence": evidence_report["missing_evidence"],
            }
        )

    if lane_order != sorted(lane_order, key=_lane_sort_key):
        errors.append("status.json lanes must be sorted by lane id")

    return lane_reports, lane_ids, lane_repo_ids


def validate_lane_subsystem_bindings(
    lane_reports: list[dict[str, Any]],
    errors: list[str],
    *,
    use_staged_registry: bool = False,
    expected_by_lane: dict[str, list[str]] | None = None,
) -> None:
    expected_by_lane = expected_by_lane or lane_subsystem_map(use_staged_registry=use_staged_registry)

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


def validate_lane_completion_tracking(
    lane_reports: list[dict[str, Any]],
    *,
    lane_followups: list[dict[str, Any]],
    readiness_assertions: list[dict[str, Any]],
    release_gates: list[dict[str, Any]],
    open_decisions: list[dict[str, Any]],
    errors: list[str],
) -> None:
    lane_followups_by_id = {
        str(followup["id"]): followup for followup in lane_followups
    }
    readiness_assertions_by_id = {
        str(assertion["id"]): assertion for assertion in readiness_assertions
    }
    release_gates_by_id = {
        str(gate["id"]): gate for gate in release_gates
    }
    open_decisions_by_id = {
        str(decision["id"]): decision for decision in open_decisions
    }
    readiness_assertion_lane_ids = {
        assertion_id: set(str(lane_id) for lane_id in assertion.get("lane_ids", []))
        for assertion_id, assertion in readiness_assertions_by_id.items()
    }
    release_gate_lane_ids = {
        gate_id: set(str(lane_id) for lane_id in gate.get("lane_ids", []))
        for gate_id, gate in release_gates_by_id.items()
    }
    open_decision_lane_ids = {
        decision_id: set(str(lane_id) for lane_id in decision.get("lane_ids", []))
        for decision_id, decision in open_decisions_by_id.items()
    }
    lane_followup_lane_ids = {
        followup_id: set(str(lane_id) for lane_id in followup.get("lane_ids", []))
        for followup_id, followup in lane_followups_by_id.items()
    }
    lane_followup_refs: dict[str, set[str]] = {
        followup_id: set() for followup_id in lane_followups_by_id
    }
    known_by_kind = {
        "lane-followup": set(lane_followups_by_id),
        "readiness-assertion": set(readiness_assertions_by_id),
        "release-gate": set(release_gates_by_id),
        "open-decision": set(open_decisions_by_id),
    }
    labels = {
        "lane-followup": "lane followup",
        "readiness-assertion": "readiness assertion",
        "release-gate": "release gate",
        "open-decision": "open decision",
    }

    for lane in lane_reports:
        lane_id = lane["id"]
        if lane.get("completion_state") != "bounded-residual":
            continue
        for tracking in lane.get("completion_tracking", []):
            tracking_kind = str(tracking["kind"])
            tracking_id = str(tracking["id"])
            if tracking_id not in known_by_kind.get(tracking_kind, set()):
                errors.append(
                    f"lanes[{lane_id}].completion.tracking references unknown "
                    f"{labels.get(tracking_kind, tracking_kind)} {tracking_id!r}"
                )
                continue
            if tracking_kind == "lane-followup" and lane_id not in lane_followup_lane_ids[tracking_id]:
                errors.append(
                    f"lanes[{lane_id}].completion.tracking lane followup {tracking_id!r} "
                    "does not reference that lane"
                )
                continue
            if tracking_kind == "lane-followup":
                lane_followup_refs[tracking_id].add(lane_id)
            if tracking_kind == "readiness-assertion" and lane_id not in readiness_assertion_lane_ids[tracking_id]:
                errors.append(
                    f"lanes[{lane_id}].completion.tracking readiness assertion {tracking_id!r} "
                    "does not reference that lane"
                )
                continue
            if tracking_kind == "release-gate" and lane_id not in release_gate_lane_ids[tracking_id]:
                errors.append(
                    f"lanes[{lane_id}].completion.tracking release gate {tracking_id!r} "
                    "does not reference that lane"
                )
                continue
            if tracking_kind == "open-decision" and lane_id not in open_decision_lane_ids[tracking_id]:
                errors.append(
                    f"lanes[{lane_id}].completion.tracking open decision {tracking_id!r} "
                    "does not reference that lane"
                )
                continue
            detail = _lane_tracking_detail(
                tracking,
                lane_followups_by_id=lane_followups_by_id,
                readiness_assertions_by_id=readiness_assertions_by_id,
                release_gates_by_id=release_gates_by_id,
                open_decisions_by_id=open_decisions_by_id,
            )
            if detail["resolved"]:
                errors.append(
                    f"lanes[{lane_id}].completion.tracking {labels.get(tracking_kind, tracking_kind)} "
                    f"{tracking_id!r} is already resolved and cannot keep a bounded residual open"
                )
    for followup_id, expected_lane_ids in lane_followup_lane_ids.items():
        referenced_lane_ids = lane_followup_refs.get(followup_id, set())
        if not referenced_lane_ids:
            errors.append(
                f"lane_followups[{followup_id}] is not referenced by any bounded-residual lane completion.tracking"
            )
            continue
        if referenced_lane_ids != expected_lane_ids:
            errors.append(
                f"lane_followups[{followup_id}] is referenced by lanes {sorted(referenced_lane_ids)!r} "
                f"but declares lane_ids {sorted(expected_lane_ids)!r}"
            )


def subsystem_lane_map(*, use_staged_registry: bool = False) -> dict[str, str]:
    return {
        str(rule["id"]).strip(): str(rule["lane"]).strip()
        for rule in load_subsystem_rules(staged=use_staged_registry)
        if str(rule.get("id", "")).strip() and str(rule.get("lane", "")).strip()
    }


def lane_subsystem_map(*, use_staged_registry: bool = False) -> dict[str, list[str]]:
    expected_by_lane: dict[str, list[str]] = {}
    for subsystem_id, lane_id in subsystem_lane_map(use_staged_registry=use_staged_registry).items():
        expected_by_lane.setdefault(lane_id, []).append(subsystem_id)
    for lane_id in list(expected_by_lane):
        expected_by_lane[lane_id] = sorted(expected_by_lane[lane_id])
    return expected_by_lane


def subsystem_contract_map(*, use_staged_registry: bool = False) -> dict[str, str]:
    return {
        str(rule["id"]).strip(): str(rule["contract"]).strip()
        for rule in load_subsystem_rules(staged=use_staged_registry)
        if str(rule.get("id", "")).strip() and str(rule.get("contract", "")).strip()
    }


def contract_paths_for_subsystems(
    subsystem_ids: list[str] | set[str],
    *,
    subsystem_contracts: dict[str, str],
) -> list[str]:
    ordered_subsystems = sorted({str(subsystem_id).strip() for subsystem_id in subsystem_ids if str(subsystem_id).strip()})
    return [
        subsystem_contracts[subsystem_id]
        for subsystem_id in ordered_subsystems
        if subsystem_id in subsystem_contracts
    ]


def validate_decision_subsystems(
    *,
    subsystem_ids: list[str],
    lane_refs: list[str],
    subsystem_to_lane: dict[str, str],
    errors: list[str],
    context: str,
) -> None:
    if len(subsystem_ids) != len(set(subsystem_ids)):
        errors.append(f"{context}.subsystem_ids must not contain duplicates")
    if subsystem_ids != sorted(subsystem_ids):
        errors.append(f"{context}.subsystem_ids must be sorted lexicographically")
    for subsystem_id in subsystem_ids:
        subsystem_lane = subsystem_to_lane.get(subsystem_id)
        if subsystem_lane is None:
            errors.append(f"{context} references unknown subsystem_id {subsystem_id!r}")
            continue
        if subsystem_lane not in lane_refs:
            errors.append(
                f"{context} subsystem_id {subsystem_id!r} belongs to lane {subsystem_lane!r}, "
                f"which is not present in lane_ids {lane_refs!r}"
            )


def validate_release_gates(
    payload: dict[str, Any],
    *,
    lane_ids: set[str],
    lane_repo_ids: dict[str, list[str]],
    active_repos: set[str],
    allowed_kinds: set[str],
    valid_evidence_tiers: set[str],
    valid_release_gate_blocking_levels: set[str],
    valid_release_gate_statuses: set[str],
    errors: list[str],
) -> list[dict[str, Any]]:
    gates = _require_object_list(payload, "release_gates", errors, context="status.json")
    seen_ids: set[str] = set()
    records: list[dict[str, Any]] = []

    for index, raw in enumerate(gates):
        context = f"release_gates[{index}]"
        gate_id = _require_string(raw, "id", errors, context=context)
        if gate_id:
            if gate_id in seen_ids:
                errors.append(f"{context} duplicates id {gate_id}")
            seen_ids.add(gate_id)
        summary = _require_string(raw, "summary", errors, context=context)
        owner = _require_string(raw, "owner", errors, context=context)
        blocking_level = _require_string(raw, "blocking_level", errors, context=context)
        minimum_evidence_tier = _require_string(raw, "minimum_evidence_tier", errors, context=context)
        status = _require_string(raw, "status", errors, context=context)
        verification_doc = _require_string(raw, "verification_doc", errors, context=context)
        lane_refs = _require_string_list(raw, "lane_ids", errors, context=context)

        if blocking_level and blocking_level not in valid_release_gate_blocking_levels:
            errors.append(f"{context} has invalid blocking_level {blocking_level!r}")
        if minimum_evidence_tier and minimum_evidence_tier not in valid_evidence_tiers:
            errors.append(f"{context} has invalid minimum_evidence_tier {minimum_evidence_tier!r}")
        if status and status not in valid_release_gate_statuses:
            errors.append(f"{context} has invalid status {status!r}")
        if verification_doc and verification_doc != HIGH_RISK_RELEASE_MATRIX:
            errors.append(
                f"{context}.verification_doc must be {HIGH_RISK_RELEASE_MATRIX!r}, got {verification_doc!r}"
            )
        if len(lane_refs) != len(set(lane_refs)):
            errors.append(f"{context}.lane_ids must not contain duplicates")
        if lane_refs != sorted(lane_refs, key=_lane_sort_key):
            errors.append(f"{context}.lane_ids must be sorted by lane id")
        for lane_id in lane_refs:
            if lane_id not in lane_ids:
                errors.append(f"{context} references unknown lane_id {lane_id!r}")

        evidence_report = audit_evidence_refs(
            raw.get("evidence"),
            context=context,
            active_repos=active_repos,
            allowed_kinds=allowed_kinds,
            valid_evidence_tiers=valid_evidence_tiers,
            errors=errors,
        )
        evidence_threshold_met = _evidence_tier_meets_minimum(
            evidence_report["highest_evidence_tier"],
            minimum_evidence_tier,
        )
        if status == "passed":
            if not evidence_report["all_evidence_present"]:
                effective_status = "evidence-missing"
            elif not evidence_threshold_met:
                effective_status = "threshold-unmet"
            else:
                effective_status = "passed"
        else:
            effective_status = status

        if gate_id and summary and owner and blocking_level and minimum_evidence_tier and status and verification_doc:
            repo_ids = _derived_repo_ids_for_lane_refs(lane_refs, lane_repo_ids=lane_repo_ids)
            repo_ids = sorted(
                set(repo_ids) | {repo for repo, _, _ in evidence_report["evidence_refs"]},
                key=_repo_sort_key,
            )
            records.append(
                {
                    "id": gate_id,
                    "summary": summary,
                    "owner": owner,
                    "blocking_level": blocking_level,
                    "minimum_evidence_tier": minimum_evidence_tier,
                    "status": status,
                    "effective_status": effective_status,
                    "verification_doc": verification_doc,
                    "lane_ids": lane_refs,
                    "all_evidence_present": evidence_report["all_evidence_present"],
                    "evidence_count": len(evidence_report["resolved_evidence"]),
                    "highest_evidence_tier": evidence_report["highest_evidence_tier"],
                    "evidence_threshold_met": evidence_threshold_met,
                    "missing_evidence": evidence_report["missing_evidence"],
                    "repo_ids": repo_ids,
                    "cross_repo": len(repo_ids) > 1,
                }
            )

    if len(records) == len(gates):
        gate_ids = [record["id"] for record in records]
        if gate_ids != sorted(gate_ids):
            errors.append("status.json release_gates must be sorted by id")

    return records


def validate_proof_commands(raw_commands: Any, *, context: str, errors: list[str]) -> list[dict[str, Any]]:
    if raw_commands is None:
        return []
    if not isinstance(raw_commands, list) or not raw_commands:
        errors.append(f"{context}.proof_commands must be a non-empty list when declared")
        return []

    seen_ids: set[str] = set()
    records: list[dict[str, Any]] = []

    for index, raw in enumerate(raw_commands):
        command_context = f"{context}.proof_commands[{index}]"
        if not isinstance(raw, dict):
            errors.append(f"{command_context} must be an object")
            continue

        command_id = _require_string(raw, "id", errors, context=command_context)
        if command_id:
            if command_id in seen_ids:
                errors.append(f"{command_context} duplicates id {command_id}")
            seen_ids.add(command_id)

        cwd = raw.get("cwd")
        if cwd is not None and not isinstance(cwd, str):
            errors.append(f"{command_context}.cwd must be a string when declared")
            cwd = None
        if isinstance(cwd, str):
            if not cwd.strip():
                errors.append(f"{command_context}.cwd must be a non-empty string when declared")
                cwd = None
            else:
                _validate_clean_relative_dir(cwd, errors, context=f"{command_context}.cwd")

        run = raw.get("run")
        if not isinstance(run, list) or not run or any(
            not isinstance(entry, str) or not entry.strip() for entry in run
        ):
            errors.append(f"{command_context}.run must be a non-empty list of non-empty strings")
            run = []

        if command_id and run:
            record: dict[str, Any] = {
                "id": command_id,
                "run": [str(entry) for entry in run],
            }
            if cwd:
                record["cwd"] = cwd
            records.append(record)

    if len(records) == len(raw_commands):
        command_ids = [record["id"] for record in records]
        if command_ids != sorted(command_ids, key=_proof_command_sort_key):
            errors.append(f"{context}.proof_commands must be sorted by command id")

    return records


def validate_readiness_assertions(
    payload: dict[str, Any],
    *,
    lane_ids: set[str],
    release_gates: list[dict[str, Any]],
    subsystem_to_lane: dict[str, str],
    active_repos: set[str],
    allowed_kinds: set[str],
    valid_evidence_tiers: set[str],
    valid_readiness_assertion_kinds: set[str],
    valid_readiness_assertion_blocking_levels: set[str],
    valid_readiness_assertion_proof_types: set[str],
    errors: list[str],
) -> list[dict[str, Any]]:
    assertions = _require_object_list(payload, "readiness_assertions", errors, context="status.json")
    seen_ids: set[str] = set()
    records: list[dict[str, Any]] = []
    release_gate_statuses = {
        str(gate["id"]).strip(): str(gate["effective_status"]).strip()
        for gate in release_gates
        if str(gate.get("id", "")).strip()
    }
    release_gate_blocking_levels = {
        str(gate["id"]).strip(): str(gate["blocking_level"]).strip()
        for gate in release_gates
        if str(gate.get("id", "")).strip()
    }
    release_gate_repo_ids = {
        str(gate["id"]).strip(): list(gate.get("repo_ids", []))
        for gate in release_gates
        if str(gate.get("id", "")).strip()
    }

    for index, raw in enumerate(assertions):
        context = f"readiness_assertions[{index}]"
        assertion_id = _require_string(raw, "id", errors, context=context)
        if assertion_id:
            if assertion_id in seen_ids:
                errors.append(f"{context} duplicates id {assertion_id}")
            seen_ids.add(assertion_id)
        summary = _require_string(raw, "summary", errors, context=context)
        kind = _require_string(raw, "kind", errors, context=context)
        blocking_level = _require_string(raw, "blocking_level", errors, context=context)
        proof_type = _require_string(raw, "proof_type", errors, context=context)
        lane_refs = _require_string_list(raw, "lane_ids", errors, context=context)
        subsystem_refs = _require_string_list(raw, "subsystem_ids", errors, context=context)
        gate_refs = _require_string_list(raw, "release_gate_ids", errors, context=context)

        if kind and kind not in valid_readiness_assertion_kinds:
            errors.append(f"{context} has invalid kind {kind!r}")
        if blocking_level and blocking_level not in valid_readiness_assertion_blocking_levels:
            errors.append(f"{context} has invalid blocking_level {blocking_level!r}")
        if proof_type and proof_type not in valid_readiness_assertion_proof_types:
            errors.append(f"{context} has invalid proof_type {proof_type!r}")

        if len(lane_refs) != len(set(lane_refs)):
            errors.append(f"{context}.lane_ids must not contain duplicates")
        if lane_refs != sorted(lane_refs, key=_lane_sort_key):
            errors.append(f"{context}.lane_ids must be sorted by lane id")
        for lane_id in lane_refs:
            if lane_id not in lane_ids:
                errors.append(f"{context} references unknown lane_id {lane_id!r}")

        validate_decision_subsystems(
            subsystem_ids=subsystem_refs,
            lane_refs=lane_refs,
            subsystem_to_lane=subsystem_to_lane,
            errors=errors,
            context=context,
        )

        if len(gate_refs) != len(set(gate_refs)):
            errors.append(f"{context}.release_gate_ids must not contain duplicates")
        if gate_refs != sorted(gate_refs):
            errors.append(f"{context}.release_gate_ids must be sorted lexicographically")
        for gate_id in gate_refs:
            if gate_id not in release_gate_statuses:
                errors.append(f"{context} references unknown release_gate_id {gate_id!r}")
            elif blocking_level and release_gate_blocking_levels.get(gate_id) != blocking_level:
                errors.append(
                    f"{context} links release_gate_id {gate_id!r} with blocking_level "
                    f"{release_gate_blocking_levels.get(gate_id)!r}, want {blocking_level!r}"
                )

        if proof_type == "automated" and gate_refs:
            errors.append(f"{context} proof_type automated must not declare release_gate_ids")
        if proof_type in {"manual", "hybrid"} and not gate_refs:
            errors.append(f"{context} proof_type {proof_type!r} must declare release_gate_ids")

        proof_commands = validate_proof_commands(
            raw.get("proof_commands"),
            context=context,
            errors=errors,
        )
        if proof_type == "automated" and not proof_commands:
            errors.append(f"{context} proof_type 'automated' must declare proof_commands")

        evidence_report = audit_evidence_refs(
            raw.get("evidence"),
            context=context,
            active_repos=active_repos,
            allowed_kinds=allowed_kinds,
            valid_evidence_tiers=valid_evidence_tiers,
            errors=errors,
        )
        has_executable_proof = any(
            kind == "file" and _is_executable_proof_artifact(path)
            for _, path, kind in evidence_report["evidence_refs"]
        )
        if proof_type in {"automated", "hybrid"} and not has_executable_proof:
            errors.append(
                f"{context} proof_type {proof_type!r} must include at least one executable proof artifact"
            )
        linked_release_gates_cleared = all(
            release_gate_statuses.get(gate_id) == "passed" for gate_id in gate_refs
        )
        derived_status = _derived_readiness_assertion_status(
            proof_type=proof_type or "",
            all_evidence_present=evidence_report["all_evidence_present"],
            linked_release_gates_cleared=linked_release_gates_cleared,
        )
        derived_pass = derived_status == "passed"

        if (
            assertion_id
            and summary
            and kind
            and blocking_level
            and proof_type
        ):
            repo_ids: set[str] = {repo for repo, _, _ in evidence_report["evidence_refs"]}
            for gate_id in gate_refs:
                repo_ids.update(release_gate_repo_ids.get(gate_id, []))
            records.append(
                {
                    "id": assertion_id,
                    "summary": summary,
                    "kind": kind,
                    "blocking_level": blocking_level,
                    "proof_type": proof_type,
                    "lane_ids": lane_refs,
                    "subsystem_ids": subsystem_refs,
                    "release_gate_ids": gate_refs,
                    "proof_commands": proof_commands,
                    "derived_status": derived_status,
                    "derived_pass": derived_pass,
                    "all_evidence_present": evidence_report["all_evidence_present"],
                    "evidence_count": len(evidence_report["resolved_evidence"]),
                    "proof_command_count": len(proof_commands),
                    "has_executable_proof": has_executable_proof,
                    "missing_evidence": evidence_report["missing_evidence"],
                    "linked_release_gates_cleared": linked_release_gates_cleared,
                    "repo_ids": sorted(repo_ids, key=_repo_sort_key),
                    "cross_repo": len(repo_ids) > 1,
                }
            )

    if len(records) == len(assertions):
        assertion_ids = [record["id"] for record in records]
        if assertion_ids != sorted(assertion_ids, key=_readiness_assertion_sort_key):
            errors.append("status.json readiness_assertions must be sorted by assertion id")

    return records


def validate_open_decisions(
    payload: dict[str, Any],
    *,
    lane_ids: set[str],
    lane_repo_ids: dict[str, list[str]],
    subsystem_to_lane: dict[str, str],
    valid_open_decision_blocking_levels: set[str],
    valid_open_decision_statuses: set[str],
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
        blocking_level = _require_string(raw, "blocking_level", errors, context=context)
        opened_at = _require_string(raw, "opened_at", errors, context=context)
        status = _require_string(raw, "status", errors, context=context)
        lane_refs = _require_string_list(raw, "lane_ids", errors, context=context)
        subsystem_refs = _require_string_list(raw, "subsystem_ids", errors, context=context)

        if opened_at:
            _validate_date(opened_at, errors, context=f"{context}.opened_at")
        if blocking_level and blocking_level not in valid_open_decision_blocking_levels:
            errors.append(f"{context} has invalid blocking_level {blocking_level!r}")
        if status and status not in valid_open_decision_statuses:
            errors.append(f"{context} has invalid status {status!r}")
        if len(lane_refs) != len(set(lane_refs)):
            errors.append(f"{context}.lane_ids must not contain duplicates")
        if lane_refs != sorted(lane_refs, key=_lane_sort_key):
            errors.append(f"{context}.lane_ids must be sorted by lane id")
        for lane_id in lane_refs:
            if lane_id not in lane_ids:
                errors.append(f"{context} references unknown lane_id {lane_id!r}")
        validate_decision_subsystems(
            subsystem_ids=subsystem_refs,
            lane_refs=lane_refs,
            subsystem_to_lane=subsystem_to_lane,
            errors=errors,
            context=context,
        )

        if decision_id and summary and owner and blocking_level and opened_at and status:
            repo_ids = _derived_repo_ids_for_lane_refs(lane_refs, lane_repo_ids=lane_repo_ids)
            records.append(
                {
                    "id": decision_id,
                    "summary": summary,
                    "owner": owner,
                    "blocking_level": blocking_level,
                    "status": status,
                    "opened_at": opened_at,
                    "lane_ids": lane_refs,
                    "subsystem_ids": subsystem_refs,
                    "repo_ids": repo_ids,
                    "cross_repo": len(repo_ids) > 1,
                }
            )

    order = [(record["opened_at"], record["id"]) for record in records]
    if len(records) == len(decisions) and order != sorted(order, key=_decision_sort_key):
        errors.append("status.json open_decisions must be sorted by opened_at then id")

    return records


def validate_lane_followups(
    payload: dict[str, Any],
    *,
    lane_ids: set[str],
    lane_repo_ids: dict[str, list[str]],
    subsystem_to_lane: dict[str, str],
    valid_lane_followup_statuses: set[str],
    errors: list[str],
) -> list[dict[str, Any]]:
    followups = _require_object_list(payload, "lane_followups", errors, context="status.json")
    seen_ids: set[str] = set()
    records: list[dict[str, Any]] = []

    for index, raw in enumerate(followups):
        context = f"lane_followups[{index}]"
        followup_id = _require_string(raw, "id", errors, context=context)
        if followup_id:
            if followup_id in seen_ids:
                errors.append(f"{context} duplicates id {followup_id}")
            seen_ids.add(followup_id)
        summary = _require_string(raw, "summary", errors, context=context)
        owner = _require_string(raw, "owner", errors, context=context)
        status = _require_string(raw, "status", errors, context=context)
        recorded_at = _require_string(raw, "recorded_at", errors, context=context)
        lane_refs = _require_string_list(raw, "lane_ids", errors, context=context)
        subsystem_refs = _require_string_list(raw, "subsystem_ids", errors, context=context)

        if recorded_at:
            _validate_date(recorded_at, errors, context=f"{context}.recorded_at")
        if status and status not in valid_lane_followup_statuses:
            errors.append(f"{context} has invalid status {status!r}")
        if len(lane_refs) != len(set(lane_refs)):
            errors.append(f"{context}.lane_ids must not contain duplicates")
        if lane_refs != sorted(lane_refs, key=_lane_sort_key):
            errors.append(f"{context}.lane_ids must be sorted by lane id")
        if len(lane_refs) != 1:
            errors.append(f"{context}.lane_ids must contain exactly one lane id")
        for lane_id in lane_refs:
            if lane_id not in lane_ids:
                errors.append(f"{context} references unknown lane_id {lane_id!r}")
        validate_decision_subsystems(
            subsystem_ids=subsystem_refs,
            lane_refs=lane_refs,
            subsystem_to_lane=subsystem_to_lane,
            errors=errors,
            context=context,
        )

        if followup_id and summary and owner and status and recorded_at and len(lane_refs) == 1:
            repo_ids = _derived_repo_ids_for_lane_refs(lane_refs, lane_repo_ids=lane_repo_ids)
            records.append(
                {
                    "id": followup_id,
                    "summary": summary,
                    "owner": owner,
                    "status": status,
                    "recorded_at": recorded_at,
                    "lane_ids": lane_refs,
                    "subsystem_ids": subsystem_refs,
                    "repo_ids": repo_ids,
                    "cross_repo": len(repo_ids) > 1,
                }
            )

    order = [(record["recorded_at"], record["id"]) for record in records]
    if len(records) == len(followups) and order != sorted(order, key=_decision_sort_key):
        errors.append("status.json lane_followups must be sorted by recorded_at then id")

    return records


def validate_resolved_decisions(
    payload: dict[str, Any],
    *,
    lane_ids: set[str],
    lane_repo_ids: dict[str, list[str]],
    subsystem_to_lane: dict[str, str],
    valid_resolved_decision_kinds: set[str],
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
        subsystem_refs = _require_string_list(raw, "subsystem_ids", errors, context=context)

        if kind and kind not in valid_resolved_decision_kinds:
            errors.append(f"{context} has invalid kind {kind!r}")
        if decided_at:
            _validate_date(decided_at, errors, context=f"{context}.decided_at")
        if len(lane_refs) != len(set(lane_refs)):
            errors.append(f"{context}.lane_ids must not contain duplicates")
        if lane_refs != sorted(lane_refs, key=_lane_sort_key):
            errors.append(f"{context}.lane_ids must be sorted by lane id")
        for lane_id in lane_refs:
            if lane_id not in lane_ids:
                errors.append(f"{context} references unknown lane_id {lane_id!r}")
        validate_decision_subsystems(
            subsystem_ids=subsystem_refs,
            lane_refs=lane_refs,
            subsystem_to_lane=subsystem_to_lane,
            errors=errors,
            context=context,
        )

        if decision_id and summary and kind and decided_at:
            repo_ids = _derived_repo_ids_for_lane_refs(lane_refs, lane_repo_ids=lane_repo_ids)
            records.append(
                {
                    "id": decision_id,
                    "summary": summary,
                    "kind": kind,
                    "decided_at": decided_at,
                    "lane_ids": lane_refs,
                    "subsystem_ids": subsystem_refs,
                    "repo_ids": repo_ids,
                    "cross_repo": len(repo_ids) > 1,
                }
            )

    order = [(record["decided_at"], record["id"]) for record in records]
    if len(records) == len(decisions) and order != sorted(order, key=_decision_sort_key):
        errors.append("status.json resolved_decisions must be sorted by decided_at then id")

    return records


def audit_status_payload(
    payload: dict[str, Any],
    *,
    schema_contract: dict[str, Any] | None = None,
    use_staged_registry: bool = False,
    current_version: str | None = None,
) -> dict[str, Any]:
    contract = schema_contract or DEFAULT_STATUS_SCHEMA_CONTRACT
    required_top_level_fields = set(contract["required_top_level_fields"])
    valid_lane_statuses = set(contract["valid_lane_statuses"])
    valid_evidence_tiers = set(contract["valid_evidence_tiers"])
    valid_readiness_assertion_blocking_levels = set(contract["valid_readiness_assertion_blocking_levels"])
    valid_readiness_assertion_kinds = set(contract["valid_readiness_assertion_kinds"])
    valid_readiness_assertion_proof_types = set(contract["valid_readiness_assertion_proof_types"])
    valid_release_gate_blocking_levels = set(contract["valid_release_gate_blocking_levels"])
    valid_release_gate_statuses = set(contract["valid_release_gate_statuses"])
    valid_lane_followup_statuses = set(contract["valid_lane_followup_statuses"])
    valid_open_decision_blocking_levels = set(contract["valid_open_decision_blocking_levels"])
    valid_open_decision_statuses = set(contract["valid_open_decision_statuses"])
    valid_resolved_decision_kinds = set(contract["valid_resolved_decision_kinds"])
    errors: list[str] = []
    warnings: list[str] = []

    for field in sorted(required_top_level_fields):
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

    scope_report = validate_scope(payload, errors)
    active_repos = list(scope_report["active_repos"])
    ignored_repos = list(scope_report["ignored_repos"])
    control_plane_repo = scope_report["control_plane_repo"]
    active_repo_set = set(active_repos)
    ignored_repo_set = set(ignored_repos)
    if active_repo_set & ignored_repo_set:
        errors.append("status.json scope must not list the same repo as both active and ignored")

    validate_source_precedence(payload, errors)
    release_critical_lanes = validate_priority_engine(payload, errors)
    readiness = validate_readiness(payload, errors)
    allowed_kinds, local_repo = validate_evidence_policy(payload, errors)
    if local_repo and local_repo not in active_repo_set:
        errors.append("status.json evidence_reference_policy.local_repo must be present in active_repos")
    if local_repo and control_plane_repo and local_repo != control_plane_repo:
        errors.append("status.json evidence_reference_policy.local_repo must match scope.control_plane_repo")

    lane_reports, lane_ids, lane_repo_ids = audit_lanes(
        payload,
        active_repos=active_repo_set,
        allowed_kinds=allowed_kinds,
        valid_evidence_tiers=valid_evidence_tiers,
        valid_lane_statuses=valid_lane_statuses,
        errors=errors,
        warnings=warnings,
    )
    subsystem_to_lane = subsystem_lane_map(use_staged_registry=use_staged_registry)
    lane_to_subsystems = lane_subsystem_map(use_staged_registry=use_staged_registry)
    subsystem_contracts = subsystem_contract_map(use_staged_registry=use_staged_registry)
    validate_lane_subsystem_bindings(
        lane_reports,
        errors,
        expected_by_lane=lane_to_subsystems,
    )
    for lane_id in release_critical_lanes:
        if lane_id not in lane_ids:
            errors.append(f"priority_engine.floor_rule.release_critical_lanes references unknown lane_id {lane_id!r}")
    release_gates = validate_release_gates(
        payload,
        lane_ids=lane_ids,
        lane_repo_ids=lane_repo_ids,
        active_repos=active_repo_set,
        allowed_kinds=allowed_kinds,
        valid_evidence_tiers=valid_evidence_tiers,
        valid_release_gate_blocking_levels=valid_release_gate_blocking_levels,
        valid_release_gate_statuses=valid_release_gate_statuses,
        errors=errors,
    )
    readiness_assertions = validate_readiness_assertions(
        payload,
        lane_ids=lane_ids,
        release_gates=release_gates,
        subsystem_to_lane=subsystem_to_lane,
        active_repos=active_repo_set,
        allowed_kinds=allowed_kinds,
        valid_evidence_tiers=valid_evidence_tiers,
        valid_readiness_assertion_kinds=valid_readiness_assertion_kinds,
        valid_readiness_assertion_blocking_levels=valid_readiness_assertion_blocking_levels,
        valid_readiness_assertion_proof_types=valid_readiness_assertion_proof_types,
        errors=errors,
    )
    open_decisions = validate_open_decisions(
        payload,
        lane_ids=lane_ids,
        lane_repo_ids=lane_repo_ids,
        subsystem_to_lane=subsystem_to_lane,
        valid_open_decision_blocking_levels=valid_open_decision_blocking_levels,
        valid_open_decision_statuses=valid_open_decision_statuses,
        errors=errors,
    )
    lane_followups = validate_lane_followups(
        payload,
        lane_ids=lane_ids,
        lane_repo_ids=lane_repo_ids,
        subsystem_to_lane=subsystem_to_lane,
        valid_lane_followup_statuses=valid_lane_followup_statuses,
        errors=errors,
    )
    validate_lane_completion_tracking(
        lane_reports,
        lane_followups=lane_followups,
        readiness_assertions=readiness_assertions,
        release_gates=release_gates,
        open_decisions=open_decisions,
        errors=errors,
    )
    resolved_decisions = validate_resolved_decisions(
        payload,
        lane_ids=lane_ids,
        lane_repo_ids=lane_repo_ids,
        subsystem_to_lane=subsystem_to_lane,
        valid_resolved_decision_kinds=valid_resolved_decision_kinds,
        errors=errors,
    )

    repo_ready_assertions_cleared = all(
        assertion["derived_pass"]
        for assertion in readiness_assertions
        if assertion["blocking_level"] == "repo-ready"
    )
    rc_ready_assertions_cleared = all(
        assertion["derived_pass"]
        for assertion in readiness_assertions
        if assertion["blocking_level"] == "rc-ready"
    )
    release_ready_assertions_cleared = all(
        assertion["derived_pass"]
        for assertion in readiness_assertions
        if assertion["blocking_level"] == "release-ready"
    )
    repo_ready_derived = (
        all(lane["at_target"] and lane["all_evidence_present"] for lane in lane_reports)
        and repo_ready_assertions_cleared
    )
    rc_ready_release_gates_cleared = all(
        gate["effective_status"] == "passed"
        for gate in release_gates
        if gate["blocking_level"] == "rc-ready"
    )
    release_ready_release_gates_cleared = all(
        gate["effective_status"] == "passed"
        for gate in release_gates
        if gate["blocking_level"] == "release-ready"
    )
    rc_ready_open_decisions_cleared = all(
        decision["blocking_level"] != "rc-ready" for decision in open_decisions
    )
    release_ready_open_decisions_cleared = all(
        decision["blocking_level"] != "release-ready" for decision in open_decisions
    )
    rc_ready_derived = (
        repo_ready_derived
        and rc_ready_assertions_cleared
        and rc_ready_open_decisions_cleared
        and rc_ready_release_gates_cleared
    )
    release_ready_derived = (
        rc_ready_derived
        and release_ready_assertions_cleared
        and release_ready_open_decisions_cleared
        and release_ready_release_gates_cleared
    )
    try:
        active_target_levels = list(active_target_blocking_levels())
    except ValueError as exc:
        active_target_levels = []
        warnings.append(f"active target proof scope could not be derived: {exc}")
    active_target_level_set = set(active_target_levels)
    current_target_assertions = [
        assertion
        for assertion in readiness_assertions
        if assertion["blocking_level"] in active_target_level_set
    ]
    assertion_subsystems_by_id = {
        str(assertion["id"]): list(assertion["subsystem_ids"])
        for assertion in current_target_assertions
    }
    gate_to_current_target_assertions: dict[str, list[str]] = {}
    for assertion in current_target_assertions:
        assertion_id = str(assertion["id"])
        for gate_id in assertion["release_gate_ids"]:
            linked = gate_to_current_target_assertions.setdefault(str(gate_id), [])
            if assertion_id not in linked:
                linked.append(assertion_id)
    current_target_assertion_blockers = [
        {
            "id": assertion["id"],
            "blocking_level": assertion["blocking_level"],
            "derived_status": assertion["derived_status"],
            "summary": assertion["summary"],
            "repo_ids": list(assertion["repo_ids"]),
            "lane_ids": list(assertion["lane_ids"]),
            "subsystem_ids": list(assertion["subsystem_ids"]),
            "contract_paths": contract_paths_for_subsystems(
                assertion["subsystem_ids"],
                subsystem_contracts=subsystem_contracts,
            ),
            "release_gate_ids": list(assertion["release_gate_ids"]),
            "proof_command_ids": [command["id"] for command in assertion["proof_commands"]],
        }
        for assertion in current_target_assertions
        if assertion["blocking_level"] in active_target_level_set and not assertion["derived_pass"]
    ]
    current_target_release_gate_blockers = [
        {
            **_blocker_detail(gate, summary_key="summary", status_key="effective_status"),
            "linked_assertion_ids": list(gate_to_current_target_assertions.get(gate["id"], [])),
            "subsystem_ids": sorted(
                {
                    subsystem_id
                    for lane_id in gate["lane_ids"]
                    for subsystem_id in lane_to_subsystems.get(lane_id, [])
                }
                | {
                    subsystem_id
                    for assertion_id in gate_to_current_target_assertions.get(gate["id"], [])
                    for subsystem_id in assertion_subsystems_by_id.get(assertion_id, [])
                }
            ),
            "contract_paths": contract_paths_for_subsystems(
                {
                    subsystem_id
                    for lane_id in gate["lane_ids"]
                    for subsystem_id in lane_to_subsystems.get(lane_id, [])
                }
                | {
                    subsystem_id
                    for assertion_id in gate_to_current_target_assertions.get(gate["id"], [])
                    for subsystem_id in assertion_subsystems_by_id.get(assertion_id, [])
                },
                subsystem_contracts=subsystem_contracts,
            ),
        }
        for gate in release_gates
        if gate["blocking_level"] in active_target_level_set and gate["effective_status"] != "passed"
    ]
    current_target_open_decision_blockers = [
        {
            **_blocker_detail(decision, summary_key="summary", status_key="status"),
            "subsystem_ids": list(decision["subsystem_ids"]),
            "contract_paths": contract_paths_for_subsystems(
                decision["subsystem_ids"],
                subsystem_contracts=subsystem_contracts,
            ),
        }
        for decision in open_decisions
        if decision["blocking_level"] in active_target_level_set
    ]
    current_target_workstreams_by_subsystem: dict[str, dict[str, Any]] = {}

    def ensure_current_target_workstream(subsystem_id: str) -> dict[str, Any]:
        workstream = current_target_workstreams_by_subsystem.get(subsystem_id)
        if workstream is None:
            workstream = {
                "subsystem_id": subsystem_id,
                "contract_path": subsystem_contracts.get(subsystem_id),
                "assertion_ids": [],
                "release_gate_ids": [],
                "open_decision_ids": [],
                "proof_command_ids": [],
                "lane_ids": [],
                "repo_ids": [],
            }
            current_target_workstreams_by_subsystem[subsystem_id] = workstream
        return workstream

    for assertion in current_target_assertion_blockers:
        for subsystem_id in assertion["subsystem_ids"]:
            workstream = ensure_current_target_workstream(subsystem_id)
            if assertion["id"] not in workstream["assertion_ids"]:
                workstream["assertion_ids"].append(assertion["id"])
            for gate_id in assertion["release_gate_ids"]:
                if gate_id not in workstream["release_gate_ids"]:
                    workstream["release_gate_ids"].append(gate_id)
            for proof_id in assertion["proof_command_ids"]:
                if proof_id not in workstream["proof_command_ids"]:
                    workstream["proof_command_ids"].append(proof_id)
            for lane_id in assertion["lane_ids"]:
                if lane_id not in workstream["lane_ids"]:
                    workstream["lane_ids"].append(lane_id)
            for repo_id in assertion["repo_ids"]:
                if repo_id not in workstream["repo_ids"]:
                    workstream["repo_ids"].append(repo_id)

    for gate in current_target_release_gate_blockers:
        for subsystem_id in gate["subsystem_ids"]:
            workstream = ensure_current_target_workstream(subsystem_id)
            if gate["id"] not in workstream["release_gate_ids"]:
                workstream["release_gate_ids"].append(gate["id"])
            for lane_id in gate["lane_ids"]:
                if lane_id not in workstream["lane_ids"]:
                    workstream["lane_ids"].append(lane_id)
            for repo_id in gate["repo_ids"]:
                if repo_id not in workstream["repo_ids"]:
                    workstream["repo_ids"].append(repo_id)

    for decision in current_target_open_decision_blockers:
        for subsystem_id in decision["subsystem_ids"]:
            workstream = ensure_current_target_workstream(subsystem_id)
            if decision["id"] not in workstream["open_decision_ids"]:
                workstream["open_decision_ids"].append(decision["id"])
            for lane_id in decision["lane_ids"]:
                if lane_id not in workstream["lane_ids"]:
                    workstream["lane_ids"].append(lane_id)
            for repo_id in decision["repo_ids"]:
                if repo_id not in workstream["repo_ids"]:
                    workstream["repo_ids"].append(repo_id)

    current_target_workstreams = sorted(
        (
            {
                **workstream,
                "blocker_count": len(workstream["assertion_ids"])
                + len(workstream["release_gate_ids"])
                + len(workstream["open_decision_ids"]),
            }
            for workstream in current_target_workstreams_by_subsystem.values()
        ),
        key=lambda entry: (-entry["blocker_count"], str(entry["subsystem_id"]).casefold()),
    )
    readiness_assertions_by_id = {str(assertion["id"]): assertion for assertion in readiness_assertions}
    release_gates_by_id = {str(gate["id"]): gate for gate in release_gates}
    lane_followups_by_id = {str(followup["id"]): followup for followup in lane_followups}
    open_decisions_by_id = {str(decision["id"]): decision for decision in open_decisions}
    lane_residuals = sorted(
        (
            {
                "lane_id": lane["id"],
                "lane_name": lane["name"],
                "summary": lane["completion_summary"],
                "tracking": [
                    f"{detail['kind']}:{detail['id']}[{detail['status']}]"
                    for detail in (
                        _lane_tracking_detail(
                            tracking,
                            lane_followups_by_id=lane_followups_by_id,
                            readiness_assertions_by_id=readiness_assertions_by_id,
                            release_gates_by_id=release_gates_by_id,
                            open_decisions_by_id=open_decisions_by_id,
                        )
                        for tracking in lane["completion_tracking"]
                    )
                ],
                "tracking_details": [
                    _lane_tracking_detail(
                        tracking,
                        lane_followups_by_id=lane_followups_by_id,
                        readiness_assertions_by_id=readiness_assertions_by_id,
                        release_gates_by_id=release_gates_by_id,
                        open_decisions_by_id=open_decisions_by_id,
                    )
                    for tracking in lane["completion_tracking"]
                ],
                "unresolved_tracking_count": sum(
                    1
                    for tracking in lane["completion_tracking"]
                    if not _lane_tracking_detail(
                        tracking,
                        lane_followups_by_id=lane_followups_by_id,
                        readiness_assertions_by_id=readiness_assertions_by_id,
                        release_gates_by_id=release_gates_by_id,
                        open_decisions_by_id=open_decisions_by_id,
                    )["resolved"]
                ),
                "repo_ids": list(lane["repo_ids"]),
                "subsystem_ids": list(lane["subsystems"]),
            }
            for lane in lane_reports
            if lane["completion_state"] == "bounded-residual"
        ),
        key=lambda item: _lane_sort_key(str(item["lane_id"])),
    )
    rc_blockers: list[str] = []
    if not repo_ready_derived:
        rc_blockers.append(REPO_READY_BLOCKER)
    if not rc_ready_assertions_cleared:
        rc_blockers.append(RC_READY_ASSERTIONS_BLOCKER)
    if not rc_ready_open_decisions_cleared:
        rc_blockers.append(RC_OPEN_DECISIONS_BLOCKER)
    if not rc_ready_release_gates_cleared:
        rc_blockers.append(RC_RELEASE_GATES_BLOCKER)
    release_blockers: list[str] = []
    if rc_blockers:
        release_blockers.extend(rc_blockers)
    if not release_ready_assertions_cleared:
        release_blockers.append(RELEASE_READY_ASSERTIONS_BLOCKER)
    if not release_ready_open_decisions_cleared:
        release_blockers.append(RELEASE_OPEN_DECISIONS_BLOCKER)
    if not release_ready_release_gates_cleared:
        release_blockers.append(RELEASE_GATES_BLOCKER)
    rc_blocker_details = _phase_blocker_details(
        assertions=readiness_assertions,
        open_decisions=open_decisions,
        release_gates=release_gates,
        blocking_levels={"rc-ready"},
    )
    release_blocker_details = _phase_blocker_details(
        assertions=readiness_assertions,
        open_decisions=open_decisions,
        release_gates=release_gates,
        blocking_levels={"rc-ready", "release-ready"},
    )

    active_target_completion_met = False
    completion_rule = str(ACTIVE_TARGET.get("completion_rule", "")).strip()
    if completion_rule == "rc_ready":
        active_target_completion_met = rc_ready_derived
    elif completion_rule == "release_ready":
        active_target_completion_met = release_ready_derived
    elif completion_rule == "repo_ready":
        active_target_completion_met = repo_ready_derived
    elif completion_rule == "manual":
        active_target_completion_met = False

    if active_target_completion_met:
        warnings.append(
            "active target completion rule is already satisfied; "
            "control_plane_audit.py --check will fail until the next target is promoted in "
            "docs/release-control/control_plane.json"
        )

    normalized_current_version = (current_version or "").strip()
    if normalized_current_version:
        active_target_id = str(ACTIVE_TARGET.get("id", "")).strip()
        active_target_kind = str(ACTIVE_TARGET.get("kind", "")).strip()
        if (
            not is_prerelease_version(normalized_current_version)
            and active_target_id != "v6-ga-promotion"
            and active_target_kind in {"release", "stabilization"}
            and not release_ready_derived
        ):
            warnings.append(
                "VERSION is a stable release string while the active target is still a non-GA "
                f"{active_target_id or 'target'} and release_ready is false; "
                "the repo is carrying a GA candidate on an RC-held line."
            )

    overclosed_release_gates = [
        {
            **_blocker_detail(gate, summary_key="summary", status_key="effective_status"),
            "raw_status": gate["status"],
            "effective_status": gate["effective_status"],
            "highest_evidence_tier": gate["highest_evidence_tier"],
            "minimum_evidence_tier": gate["minimum_evidence_tier"],
        }
        for gate in release_gates
        if gate["status"] == "passed" and gate["effective_status"] != "passed"
    ]

    return {
        "errors": errors,
        "warnings": warnings,
        "control_plane": {
            "active_profile_id": ACTIVE_PROFILE_ID,
            "active_target": {
                **ACTIVE_TARGET,
                "completion_met": active_target_completion_met,
                "blocking_levels": active_target_levels,
            },
            "current_version": normalized_current_version,
        },
        "summary": {
            "lane_count": len(lane_reports),
            "lanes_at_target": sum(1 for lane in lane_reports if lane["at_target"]),
            "lanes_complete": sum(1 for lane in lane_reports if lane["completion_state"] == "complete"),
            "lanes_bounded_residual": sum(
                1 for lane in lane_reports if lane["completion_state"] == "bounded-residual"
            ),
            "lanes_open": sum(1 for lane in lane_reports if lane["completion_state"] == "open"),
            "lanes_missing_evidence": sum(1 for lane in lane_reports if not lane["all_evidence_present"]),
            "all_evidence_present": all(lane["all_evidence_present"] for lane in lane_reports),
            "readiness_assertion_count": len(readiness_assertions),
            "readiness_assertions_passed": sum(1 for assertion in readiness_assertions if assertion["derived_pass"]),
            "repo_ready_assertion_count": sum(
                1 for assertion in readiness_assertions if assertion["blocking_level"] == "repo-ready"
            ),
            "repo_ready_assertions_passed": sum(
                1
                for assertion in readiness_assertions
                if assertion["blocking_level"] == "repo-ready" and assertion["derived_pass"]
            ),
            "rc_ready_assertion_count": sum(
                1 for assertion in readiness_assertions if assertion["blocking_level"] == "rc-ready"
            ),
            "rc_ready_assertions_passed": sum(
                1
                for assertion in readiness_assertions
                if assertion["blocking_level"] == "rc-ready" and assertion["derived_pass"]
            ),
            "release_ready_assertion_count": sum(
                1 for assertion in readiness_assertions if assertion["blocking_level"] == "release-ready"
            ),
            "release_ready_assertions_passed": sum(
                1
                for assertion in readiness_assertions
                if assertion["blocking_level"] == "release-ready" and assertion["derived_pass"]
            ),
            "release_gate_count": len(release_gates),
            "release_gates_passed": sum(1 for gate in release_gates if gate["effective_status"] == "passed"),
            "overclosed_release_gate_count": len(overclosed_release_gates),
            "rc_ready_release_gate_count": sum(
                1 for gate in release_gates if gate["blocking_level"] == "rc-ready"
            ),
            "rc_ready_release_gates_passed": sum(
                1
                for gate in release_gates
                if gate["blocking_level"] == "rc-ready" and gate["effective_status"] == "passed"
            ),
            "release_ready_release_gate_count": sum(
                1 for gate in release_gates if gate["blocking_level"] == "release-ready"
            ),
            "release_ready_release_gates_passed": sum(
                1
                for gate in release_gates
                if gate["blocking_level"] == "release-ready" and gate["effective_status"] == "passed"
            ),
            "open_decision_count": len(open_decisions),
            "rc_ready_open_decision_count": sum(
                1 for decision in open_decisions if decision["blocking_level"] == "rc-ready"
            ),
            "release_ready_open_decision_count": sum(
                1 for decision in open_decisions if decision["blocking_level"] == "release-ready"
            ),
            "resolved_decision_count": len(resolved_decisions),
            "repo_ready": repo_ready_derived,
            "rc_ready": rc_ready_derived,
            "release_ready": release_ready_derived,
        },
        "readiness": {
            "repo_ready": repo_ready_derived,
            "rc_ready": rc_ready_derived,
            "release_ready": release_ready_derived,
            "repo_ready_rule": readiness.get("repo_ready_rule") if readiness else None,
            "rc_ready_rule": readiness.get("rc_ready_rule") if readiness else None,
            "release_ready_rule": readiness.get("release_ready_rule") if readiness else None,
            "repo_ready_assertions_cleared": repo_ready_assertions_cleared,
            "rc_ready_assertions_cleared": rc_ready_assertions_cleared,
            "release_ready_assertions_cleared": release_ready_assertions_cleared,
            "rc_ready_open_decisions_cleared": rc_ready_open_decisions_cleared,
            "release_ready_open_decisions_cleared": release_ready_open_decisions_cleared,
            "rc_ready_release_gates_cleared": rc_ready_release_gates_cleared,
            "release_ready_release_gates_cleared": release_ready_release_gates_cleared,
            "current_target_blockers": {
                "assertions": current_target_assertion_blockers,
                "open_decisions": current_target_open_decision_blockers,
                "release_gates": current_target_release_gate_blockers,
            },
            "current_target_workstreams": current_target_workstreams,
            "lane_residuals": lane_residuals,
            "rc_blockers": rc_blockers,
            "rc_blocker_details": rc_blocker_details,
            "release_blockers": release_blockers,
            "release_blocker_details": release_blocker_details,
            "overclosed_release_gates": overclosed_release_gates,
        },
        "scope": {
            "active_repos": active_repos,
            "control_plane_repo": control_plane_repo,
            "ignored_repos": ignored_repos,
            "repo_catalog": scope_report["repo_catalog"],
        },
        "lanes": lane_reports,
        "readiness_assertions": readiness_assertions,
        "release_gates": release_gates,
        "lane_followups": lane_followups,
        "open_decisions": open_decisions,
        "resolved_decisions": resolved_decisions,
    }


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Audit the active release profile status.json.")
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
    parser.add_argument(
        "--staged",
        action="store_true",
        help="Read status control files from the git index instead of the working tree.",
    )
    return parser.parse_args(argv)


def render_pretty(report: dict[str, Any]) -> str:
    lines: list[str] = []
    control_plane = report.get("control_plane", {})
    if control_plane:
        active_target = control_plane.get("active_target", {})
        lines.append(
            "control_plane: "
            f"profile={control_plane.get('active_profile_id') or '-'} "
            f"target={active_target.get('id') or '-'} "
            f"kind={active_target.get('kind') or '-'} "
            f"completion_rule={active_target.get('completion_rule') or '-'} "
            f"completion_met={active_target.get('completion_met')}"
        )
        if control_plane.get("current_version"):
            lines.append(f"  current_version={control_plane['current_version']}")
        if active_target.get("summary"):
            lines.append(f"  target_summary={active_target['summary']}")
        blocking_levels = active_target.get("blocking_levels", [])
        if blocking_levels:
            lines.append(f"  target_blocking_levels={','.join(blocking_levels)}")

    scope = report.get("scope", {})
    repo_catalog = scope.get("repo_catalog", [])
    if scope:
        lines.append(
            "scope: "
            f"control_plane={scope.get('control_plane_repo') or '-'} "
            f"active_repos={','.join(scope.get('active_repos', [])) or '-'}"
        )
        for repo in repo_catalog:
            marker = " control-plane" if repo.get("id") == scope.get("control_plane_repo") else ""
            lines.append(
                f"  repo {repo['id']} visibility={repo['visibility']}{marker} "
                f"purpose={repo['purpose']}"
            )
        ignored_repos = scope.get("ignored_repos", [])
        if ignored_repos:
            lines.append(f"  ignored_repos={','.join(ignored_repos)}")

    summary = report.get("summary", {})
    if summary:
        lines.append(
            "summary: "
            f"lanes={summary['lane_count']} "
            f"at_target={summary['lanes_at_target']} "
            f"complete={summary['lanes_complete']} "
            f"bounded_residual={summary['lanes_bounded_residual']} "
            f"open={summary['lanes_open']} "
            f"missing_evidence={summary['lanes_missing_evidence']} "
            f"assertions={summary['readiness_assertion_count']} "
            f"assertions_passed={summary['readiness_assertions_passed']} "
            f"release_gates={summary['release_gate_count']} "
            f"release_gates_passed={summary['release_gates_passed']} "
            f"overclosed_release_gates={summary['overclosed_release_gate_count']} "
            f"open_decisions={summary['open_decision_count']} "
            f"resolved_decisions={summary['resolved_decision_count']} "
            f"repo_ready={summary['repo_ready']} "
            f"rc_ready={summary['rc_ready']} "
            f"release_ready={summary['release_ready']}"
        )
    for lane in report.get("lanes", []):
        lines.append(
            f"{lane['id']}: gap={lane['gap']:.0f} status={lane['status']} "
            f"completion={lane['completion_state']} "
            f"derived={lane['derived_status']} evidence_count={lane['evidence_count']} "
            f"repos={','.join(lane['repo_ids']) or '-'} "
            f"subsystems={','.join(lane['subsystems']) or '-'}"
        )
        if lane["completion_summary"]:
            lines.append(f"  completion_summary: {lane['completion_summary']}")
        for tracking in lane["completion_tracking"]:
            lines.append(f"  tracking: {tracking['kind']}:{tracking['id']}")
        for missing in lane["missing_evidence"]:
            lines.append(f"  missing {missing}")
    for assertion in report.get("readiness_assertions", []):
        lines.append(
            f"{assertion['id']}: derived={assertion['derived_status']} "
            f"blocking={assertion['blocking_level']} evidence_count={assertion['evidence_count']} "
            f"proof_commands={assertion['proof_command_count']} "
            f"repos={','.join(assertion['repo_ids']) or '-'} "
            f"subsystems={','.join(assertion['subsystem_ids']) or '-'}"
        )
        for missing in assertion["missing_evidence"]:
            lines.append(f"  missing {missing}")
    if report.get("lane_followups"):
        lines.append("lane_followups:")
        for followup in report["lane_followups"]:
            lines.append(
                f"  - {followup['id']} status={followup['status']} "
                f"repos={','.join(followup['repo_ids']) or '-'} "
                f"lanes={','.join(followup['lane_ids']) or '-'} "
                f"subsystems={','.join(followup['subsystem_ids']) or '-'}"
            )
    if report.get("open_decisions"):
        lines.append("open_decisions:")
        for decision in report["open_decisions"]:
            lines.append(
                f"  - {decision['id']} blocking={decision['blocking_level']} status={decision['status']} "
                f"repos={','.join(decision['repo_ids']) or '-'} "
                f"lanes={','.join(decision['lane_ids']) or '-'}"
            )
    if report.get("release_gates"):
        lines.append("release_gates:")
        for gate in report["release_gates"]:
            lines.append(
                f"  - {gate['id']} blocking={gate['blocking_level']} status={gate['status']} "
                f"effective={gate['effective_status']} "
                f"tier={gate['highest_evidence_tier'] or '-'} "
                f"min_tier={gate['minimum_evidence_tier']} "
                f"repos={','.join(gate['repo_ids']) or '-'} "
                f"lanes={','.join(gate['lane_ids']) or '-'}"
            )
    readiness = report.get("readiness", {})
    overclosed_release_gates = readiness.get("overclosed_release_gates", [])
    if overclosed_release_gates:
        lines.append("overclosed_release_gates:")
        for gate in overclosed_release_gates:
            lines.append(
                f"  - {gate['id']} raw_status={gate['raw_status']} "
                f"effective={gate['effective_status']} "
                f"tier={gate['highest_evidence_tier'] or '-'} "
                f"min_tier={gate['minimum_evidence_tier']} "
                f"repos={','.join(gate['repo_ids']) or '-'} "
                f"lanes={','.join(gate['lane_ids']) or '-'}"
            )
    current_target_blockers = readiness.get("current_target_blockers", {})
    if current_target_blockers:
        lines.append("current_target_blockers:")
        if current_target_blockers.get("assertions"):
            lines.append("  assertions:")
            for assertion in current_target_blockers["assertions"]:
                gate_ids = assertion.get("release_gate_ids", [])
                proof_ids = assertion.get("proof_command_ids", [])
                subsystem_ids = assertion.get("subsystem_ids", [])
                lines.append(
                    f"    - {assertion['id']} blocking={assertion['blocking_level']} "
                    f"derived={assertion['derived_status']} "
                    f"gates={','.join(gate_ids) or '-'} "
                    f"proofs={','.join(proof_ids) or '-'} "
                    f"subsystems={','.join(subsystem_ids) or '-'} "
                    f"repos={','.join(assertion['repo_ids']) or '-'}"
                )
        if current_target_blockers.get("open_decisions"):
            lines.append("  open_decisions:")
            for decision in current_target_blockers["open_decisions"]:
                subsystem_ids = decision.get("subsystem_ids", [])
                lines.append(
                    f"    - {decision['id']} blocking={decision['blocking_level']} "
                    f"status={decision['status']} "
                    f"subsystems={','.join(subsystem_ids) or '-'} "
                    f"repos={','.join(decision['repo_ids']) or '-'}"
                )
        if current_target_blockers.get("release_gates"):
            lines.append("  release_gates:")
            for gate in current_target_blockers["release_gates"]:
                linked_assertions = gate.get("linked_assertion_ids", [])
                subsystem_ids = gate.get("subsystem_ids", [])
                lines.append(
                    f"    - {gate['id']} blocking={gate['blocking_level']} "
                    f"status={gate['effective_status']} "
                    f"tier={gate['highest_evidence_tier'] or '-'} "
                    f"min_tier={gate['minimum_evidence_tier']} "
                    f"assertions={','.join(linked_assertions) or '-'} "
                    f"subsystems={','.join(subsystem_ids) or '-'} "
                    f"repos={','.join(gate['repo_ids']) or '-'}"
                )
    current_target_workstreams = readiness.get("current_target_workstreams", [])
    if current_target_workstreams:
        lines.append("current_target_workstreams:")
        for workstream in current_target_workstreams:
            lines.append(
                f"  - {workstream['subsystem_id']} blockers={workstream['blocker_count']} "
                f"assertions={','.join(workstream['assertion_ids']) or '-'} "
                f"gates={','.join(workstream['release_gate_ids']) or '-'} "
                f"decisions={','.join(workstream['open_decision_ids']) or '-'} "
                f"proofs={','.join(workstream['proof_command_ids']) or '-'} "
                f"repos={','.join(workstream['repo_ids']) or '-'}"
            )
    lane_residuals = readiness.get("lane_residuals", [])
    if lane_residuals:
        lines.append("lane_residuals:")
        for residual in lane_residuals:
            lines.append(
                f"  - {residual['lane_id']} "
                f"unresolved={residual['unresolved_tracking_count']} "
                f"tracking={','.join(residual['tracking']) or '-'} "
                f"repos={','.join(residual['repo_ids']) or '-'} "
                f"subsystems={','.join(residual['subsystem_ids']) or '-'}"
            )
            if residual.get("summary"):
                lines.append(f"    {residual['summary']}")
    if readiness.get("rc_blockers"):
        lines.append("rc_blockers:")
        for blocker in readiness["rc_blockers"]:
            lines.append(f"  - {blocker}")
    rc_blocker_details = readiness.get("rc_blocker_details", {})
    if rc_blocker_details and any(rc_blocker_details.values()):
        lines.append("rc_blocker_details:")
        if rc_blocker_details.get("assertions"):
            lines.append("  assertions:")
            for assertion in rc_blocker_details["assertions"]:
                lines.append(
                    f"    - {assertion['id']} status={assertion['status']} "
                    f"repos={','.join(assertion['repo_ids']) or '-'} "
                    f"lanes={','.join(assertion['lane_ids']) or '-'}"
                )
        if rc_blocker_details.get("open_decisions"):
            lines.append("  open_decisions:")
            for decision in rc_blocker_details["open_decisions"]:
                lines.append(
                    f"    - {decision['id']} status={decision['status']} "
                    f"repos={','.join(decision['repo_ids']) or '-'} "
                    f"lanes={','.join(decision['lane_ids']) or '-'}"
                )
        if rc_blocker_details.get("release_gates"):
            lines.append("  release_gates:")
            for gate in rc_blocker_details["release_gates"]:
                lines.append(
                    f"    - {gate['id']} status={gate['status']} "
                    f"tier={gate.get('highest_evidence_tier') or '-'} "
                    f"min_tier={gate.get('minimum_evidence_tier') or '-'} "
                    f"repos={','.join(gate['repo_ids']) or '-'} "
                    f"lanes={','.join(gate['lane_ids']) or '-'}"
                )
    if readiness.get("release_blockers"):
        lines.append("release_blockers:")
        for blocker in readiness["release_blockers"]:
            lines.append(f"  - {blocker}")
    release_blocker_details = readiness.get("release_blocker_details", {})
    if release_blocker_details and any(release_blocker_details.values()):
        lines.append("release_blocker_details:")
        if release_blocker_details.get("assertions"):
            lines.append("  assertions:")
            for assertion in release_blocker_details["assertions"]:
                lines.append(
                    f"    - {assertion['id']} status={assertion['status']} "
                    f"repos={','.join(assertion['repo_ids']) or '-'} "
                    f"lanes={','.join(assertion['lane_ids']) or '-'}"
                )
        if release_blocker_details.get("open_decisions"):
            lines.append("  open_decisions:")
            for decision in release_blocker_details["open_decisions"]:
                lines.append(
                    f"    - {decision['id']} status={decision['status']} "
                    f"repos={','.join(decision['repo_ids']) or '-'} "
                    f"lanes={','.join(decision['lane_ids']) or '-'}"
                )
        if release_blocker_details.get("release_gates"):
            lines.append("  release_gates:")
            for gate in release_blocker_details["release_gates"]:
                lines.append(
                    f"    - {gate['id']} status={gate['status']} "
                    f"tier={gate.get('highest_evidence_tier') or '-'} "
                    f"min_tier={gate.get('minimum_evidence_tier') or '-'} "
                    f"repos={','.join(gate['repo_ids']) or '-'} "
                    f"lanes={','.join(gate['lane_ids']) or '-'}"
                )
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
    report = audit_status_payload(
        load_status_payload(staged=args.staged),
        schema_contract=status_schema_contract(staged=args.staged),
        use_staged_registry=args.staged,
        current_version=read_repo_text("VERSION", staged=args.staged).strip(),
    )
    output = render_pretty(report) if args.pretty else json.dumps(report, indent=2, sort_keys=True)
    print(output)
    if args.check and report["errors"]:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
