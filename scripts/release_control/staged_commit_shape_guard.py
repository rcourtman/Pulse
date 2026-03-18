#!/usr/bin/env python3
"""Preflight staged commit shape for governed runtime and promotion-proof slices."""

from __future__ import annotations

import json
import subprocess
import sys
from typing import Sequence

from canonical_completion_guard import (
    format_missing_requirements,
    git_staged_files as canonical_git_staged_files,
    infer_impacted_subsystems,
    load_subsystem_rules,
    required_contract_updates,
    staged_contract_has_substantive_change,
    staged_verification_files_for_requirement,
)
from repo_file_io import load_repo_json
from release_promotion_policy_support import (
    slice_requires_staged_governance_inputs,
    staged_governance_input_errors,
)

STATUS_PATH = "docs/release-control/v6/internal/status.json"
REGISTRY_PATH = "docs/release-control/v6/internal/subsystems/registry.json"

LANE_STATUS_RANK = {
    "not-started": 0,
    "blocked": 1,
    "partial": 2,
    "target-met": 3,
}
LANE_COMPLETION_RANK = {
    "open": 0,
    "bounded-residual": 1,
    "complete": 2,
}


def git_staged_files() -> list[str]:
    return canonical_git_staged_files()


def load_head_repo_json(path: str) -> dict | None:
    result = subprocess.run(
        ["git", "show", f"HEAD:{path}"],
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        return None
    return json.loads(result.stdout)


def load_staged_status_payload() -> dict:
    # Internal governance files are not git-tracked; always read from filesystem.
    return load_repo_json(STATUS_PATH, staged=False)


def load_head_status_payload() -> dict | None:
    return load_head_repo_json(STATUS_PATH)


def load_staged_registry_payload() -> dict:
    # Internal governance files are not git-tracked; always read from filesystem.
    return load_repo_json(REGISTRY_PATH, staged=False)


def _is_runtime_surface_path(path: str) -> bool:
    filename = path.rsplit("/", 1)[-1]
    if filename.endswith("_test.go") or filename.endswith("_test.py"):
        return False
    if ".test." in filename or ".spec." in filename:
        return False
    if "/__tests__/" in path:
        return False
    return True


def _lane_by_id(payload: dict | None) -> dict[str, dict]:
    if not payload:
        return {}
    lanes = payload.get("lanes", [])
    if not isinstance(lanes, list):
        return {}
    mapped: dict[str, dict] = {}
    for raw_lane in lanes:
        if not isinstance(raw_lane, dict):
            continue
        lane_id = raw_lane.get("id")
        if isinstance(lane_id, str) and lane_id.strip():
            mapped[lane_id] = raw_lane
    return mapped


def advanced_lane_ids(previous_status: dict | None, staged_status: dict) -> list[str]:
    previous_lanes = _lane_by_id(previous_status)
    staged_lanes = _lane_by_id(staged_status)
    advanced: list[str] = []
    for lane_id, staged_lane in staged_lanes.items():
        previous_lane = previous_lanes.get(lane_id)
        if previous_lane is None:
            continue
        previous_score = float(previous_lane.get("current_score", 0))
        staged_score = float(staged_lane.get("current_score", 0))
        previous_target = float(previous_lane.get("target_score", 0))
        staged_target = float(staged_lane.get("target_score", 0))
        previous_status_rank = LANE_STATUS_RANK.get(str(previous_lane.get("status", "")), -1)
        staged_status_rank = LANE_STATUS_RANK.get(str(staged_lane.get("status", "")), -1)
        previous_completion_rank = LANE_COMPLETION_RANK.get(
            str((previous_lane.get("completion") or {}).get("state", "")),
            -1,
        )
        staged_completion_rank = LANE_COMPLETION_RANK.get(
            str((staged_lane.get("completion") or {}).get("state", "")),
            -1,
        )
        if (
            staged_score > previous_score
            or staged_target < previous_target
            or staged_status_rank > previous_status_rank
            or staged_completion_rank > previous_completion_rank
        ):
            advanced.append(lane_id)
    return sorted(advanced)


def runtime_surface_paths_by_lane(registry_payload: dict, staged_status: dict) -> dict[str, set[str]]:
    lane_to_subsystems = {
        str(lane.get("id")): {
            str(subsystem_id)
            for subsystem_id in lane.get("subsystems", [])
            if isinstance(subsystem_id, str) and subsystem_id.strip()
        }
        for lane in staged_status.get("lanes", [])
        if isinstance(lane, dict) and isinstance(lane.get("id"), str)
    }
    subsystem_rules = {
        str(rule["id"]): rule
        for rule in load_subsystem_rules(staged=True)
        if isinstance(rule, dict) and isinstance(rule.get("id"), str)
    }
    shared_paths = registry_payload.get("shared_ownerships", [])

    runtime_paths: dict[str, set[str]] = {lane_id: set() for lane_id in lane_to_subsystems}
    for lane_id, subsystem_ids in lane_to_subsystems.items():
        for subsystem_id in subsystem_ids:
            rule = subsystem_rules.get(subsystem_id)
            if not rule:
                continue
            for prefix in rule.get("owned_prefixes", []):
                if isinstance(prefix, str) and prefix.strip():
                    runtime_paths[lane_id].add(prefix)
            for path in rule.get("owned_files", []):
                if isinstance(path, str) and path.strip():
                    runtime_paths[lane_id].add(path)
        for shared in shared_paths:
            if not isinstance(shared, dict):
                continue
            path = shared.get("path")
            subsystems = shared.get("subsystems", [])
            if (
                isinstance(path, str)
                and path.strip()
                and isinstance(subsystems, list)
                and subsystem_ids.intersection(
                    {str(subsystem_id) for subsystem_id in subsystems if isinstance(subsystem_id, str)}
                )
            ):
                runtime_paths[lane_id].add(path)
    return runtime_paths


def staged_runtime_paths_for_lane(
    lane_id: str,
    staged_files: Sequence[str],
    lane_runtime_paths: dict[str, set[str]],
) -> list[str]:
    allowed_paths = lane_runtime_paths.get(lane_id, set())
    matches: list[str] = []
    for path in staged_files:
        if not _is_runtime_surface_path(path):
            continue
        if any(path == allowed or path.startswith(allowed) for allowed in allowed_paths):
            matches.append(path)
    return sorted(set(matches))


def lane_progress_shape_errors(staged_files: Sequence[str]) -> list[str]:
    if STATUS_PATH not in staged_files:
        return []

    staged_status = load_staged_status_payload()
    previous_status = load_head_status_payload()
    advanced_lanes = advanced_lane_ids(previous_status, staged_status)
    if not advanced_lanes:
        return []

    registry_payload = load_staged_registry_payload()
    lane_runtime_paths = runtime_surface_paths_by_lane(registry_payload, staged_status)
    lane_by_id = _lane_by_id(staged_status)

    blocked_lines: list[str] = []
    for lane_id in advanced_lanes:
        runtime_matches = staged_runtime_paths_for_lane(lane_id, staged_files, lane_runtime_paths)
        if runtime_matches:
            continue
        lane = lane_by_id.get(lane_id, {})
        blocked_lines.append(
            f"- {lane_id} {lane.get('name', '')}: staged lane advancement has no owned runtime/product path change"
        )
        subsystems = lane.get("subsystems", [])
        if subsystems:
            blocked_lines.append(f"  subsystems: {', '.join(str(item) for item in subsystems)}")
        runtime_examples = sorted(lane_runtime_paths.get(lane_id, set()))
        if runtime_examples:
            blocked_lines.append("  expected at least one owned runtime path such as:")
            for example in runtime_examples[:5]:
                blocked_lines.append(f"  - {example}")

    if not blocked_lines:
        return []

    return [
        "\n".join(
            [
                "BLOCKED: staged status.json advances lane progress using support-only proof or guardrail changes.",
                "",
                "Guardrail-only, proof-routing, contract, and test updates may support a lane,",
                "but they must not count as lane advancement unless the same commit also changes",
                "an owned runtime or product surface for that lane.",
                "",
                *blocked_lines,
            ]
        )
    ]


def canonical_commit_shape_errors(staged_files: Sequence[str]) -> list[str]:
    staged_set = set(staged_files)
    impacted = infer_impacted_subsystems(staged_files, use_staged_registry=True)
    required_contracts = required_contract_updates(
        staged_files,
        impacted,
        use_staged_contract_graph=True,
    )
    missing_contracts = {
        contract_path: data
        for contract_path, data in required_contracts.items()
        if contract_path not in staged_set
    }
    insufficient_contract_updates = {
        contract_path: data
        for contract_path, data in required_contracts.items()
        if contract_path in staged_set
        if not staged_contract_has_substantive_change(contract_path)
    }
    missing_verification: dict[str, dict] = {}
    for subsystem_id, data in impacted.items():
        missing_requirements = [
            requirement
            for requirement in data.get("verification_requirements", [])
            if not staged_verification_files_for_requirement(data, requirement, staged_files)
        ]
        if missing_requirements:
            missing_verification[subsystem_id] = {
                **data,
                "missing_requirements": missing_requirements,
            }

    if not missing_contracts and not insufficient_contract_updates and not missing_verification:
        return []

    return [
        format_missing_requirements(
            missing_contracts,
            insufficient_contract_updates,
            missing_verification,
        )
    ]


def staged_promotion_proof_errors(staged_files: Sequence[str]) -> list[str]:
    if not slice_requires_staged_governance_inputs(staged_files):
        return []
    errors = staged_governance_input_errors(use_staged_governance=True)
    if not errors:
        return []
    lines = ["BLOCKED: staged promotion proof inputs are incomplete.", "", "Required staged promotion inputs:"]
    for error in errors:
        error_lines = error.splitlines()
        lines.append(f"- {error_lines[0]}")
        for extra in error_lines[1:]:
            lines.append(f"  {extra}")
    return ["\n".join(lines)]


def format_combined_errors(errors: Sequence[str]) -> str:
    return "\n\n".join(error.rstrip() for error in errors if error.strip())


def main() -> int:
    staged_files = git_staged_files()
    errors = canonical_commit_shape_errors(staged_files)
    errors.extend(lane_progress_shape_errors(staged_files))
    errors.extend(staged_promotion_proof_errors(staged_files))
    if not errors:
        print("Staged commit shape guard passed.")
        return 0

    print(format_combined_errors(errors), file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
