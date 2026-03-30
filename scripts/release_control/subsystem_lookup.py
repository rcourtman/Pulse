#!/usr/bin/env python3
"""Lookup canonical subsystem ownership and proof routes for file paths."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import sys
from typing import Any

from canonical_completion_guard import (
    REPO_ROOT,
    build_verification_requirements,
    infer_impacted_subsystems,
    is_ignored_runtime_file,
    is_test_or_fixture,
    load_subsystem_rules,
    normalize_verification_requirement,
    required_contract_updates,
    subsystem_matches_path,
)
from subsystem_contracts import load_contract_index, referenced_contracts_for_path
from status_audit import audit_status_payload, load_status_payload
from registry_audit import load_registry_payload

WORKSPACE_REPOS_ROOT = REPO_ROOT.parent


def normalize_input_path(raw: str) -> str:
    candidate = Path(raw.strip())
    if candidate.is_absolute():
        candidate = candidate.resolve()
        try:
            candidate = candidate.relative_to(REPO_ROOT)
        except ValueError:
            try:
                repo_relative = candidate.relative_to(WORKSPACE_REPOS_ROOT)
            except ValueError:
                return candidate.as_posix()
            parts = repo_relative.parts
            if len(parts) >= 2:
                repo_id = parts[0]
                rel = Path(*parts[1:]).as_posix()
                if repo_id == REPO_ROOT.name:
                    return rel
                return f"{repo_id}:{rel}"
            return candidate.as_posix()
    return candidate.as_posix()


def verification_requirement_for_path(rule: dict[str, Any], path: str) -> dict[str, Any]:
    return build_verification_requirements(rule, [path])[0]


def path_matches_prefixes(path: str, prefixes: list[str]) -> bool:
    return any(path == prefix.rstrip("/") or path.startswith(prefix) for prefix in prefixes)


def verification_requirements_for_test_path(rule: dict[str, Any], path: str) -> list[dict[str, Any]]:
    verification = dict(rule.get("verification", {}))
    requirements: list[dict[str, Any]] = []

    for policy in verification.get("path_policies", []):
        exact_files = [str(item) for item in policy.get("exact_files", [])]
        test_prefixes = [str(item) for item in policy.get("test_prefixes", [])]
        allow_same_subsystem_tests = bool(policy.get("allow_same_subsystem_tests", False))
        matches_exact_file = path in exact_files
        matches_test_prefix = path_matches_prefixes(path, test_prefixes)
        matches_same_subsystem_test = allow_same_subsystem_tests and subsystem_matches_path(rule, path)
        if not (matches_exact_file or matches_test_prefix or matches_same_subsystem_test):
            continue
        policy_id = str(policy.get("id", "default"))
        requirements.append(
            normalize_verification_requirement(
                policy,
                requirement_id=policy_id,
                label=str(policy.get("label", policy_id)),
                touched_runtime_files=[],
            )
        )

    return requirements


def ownership_basis_for_path(rule: dict[str, Any], path: str) -> dict[str, str] | None:
    for exact_file in rule.get("owned_files", []):
        if path == exact_file:
            return {"type": "owned-file", "value": str(exact_file)}
    for prefix in rule.get("owned_prefixes", []):
        normalized = str(prefix).rstrip("/")
        if path == normalized or path.startswith(str(prefix)):
            return {"type": "owned-prefix", "value": str(prefix)}
    return None


def normalized_reference_details(references: list[dict[str, Any]]) -> list[dict[str, Any]]:
    normalized: list[dict[str, Any]] = []
    for reference in references:
        detail = {
            "heading": str(reference.get("heading", "")),
            "path": str(reference.get("path", "")),
        }
        if isinstance(reference.get("line"), int):
            detail["line"] = int(reference["line"])
        if isinstance(reference.get("heading_line"), int):
            detail["heading_line"] = int(reference["heading_line"])
        if detail not in normalized:
            normalized.append(detail)
    return sorted(
        normalized,
        key=lambda item: (
            int(item.get("line", 0) or 0),
            str(item.get("heading", "")).casefold(),
            str(item.get("path", "")).casefold(),
        ),
    )


def lane_context_for_rule(rule: dict[str, Any], status_report: dict[str, Any]) -> dict[str, Any] | None:
    lane_id = str(rule.get("lane", "")).strip()
    subsystem_id = str(rule.get("id", "")).strip()
    if not lane_id:
        return None

    lane = next((entry for entry in status_report.get("lanes", []) if entry.get("id") == lane_id), None)
    open_decisions = [
        decision
        for decision in status_report.get("open_decisions", [])
        if lane_id in decision.get("lane_ids", [])
        if not decision.get("subsystem_ids") or subsystem_id in decision.get("subsystem_ids", [])
    ]
    resolved_decisions = [
        decision
        for decision in status_report.get("resolved_decisions", [])
        if lane_id in decision.get("lane_ids", [])
        if not decision.get("subsystem_ids") or subsystem_id in decision.get("subsystem_ids", [])
    ]
    release_gates = [
        gate
        for gate in status_report.get("release_gates", [])
        if lane_id in gate.get("lane_ids", [])
    ]
    return {
        "lane_id": lane_id,
        "lane": lane,
        "open_decisions": open_decisions,
        "resolved_decisions": resolved_decisions,
        "release_gates": release_gates,
    }


def compact_control_plane(control_plane: dict[str, Any]) -> dict[str, Any]:
    active_target = dict(control_plane.get("active_target", {}))
    return {
        "active_profile_id": control_plane.get("active_profile_id"),
        "active_target": {
            key: active_target[key]
            for key in ("id", "kind", "status", "completion_rule", "completion_met")
            if key in active_target
        },
    }


def compact_lane_context(lane_context: dict[str, Any] | None) -> dict[str, Any] | None:
    if lane_context is None:
        return None
    lane = lane_context.get("lane")
    compact_lane = None
    if isinstance(lane, dict):
        compact_lane = {
            key: lane.get(key)
            for key in ("id", "name", "status", "derived_status", "completion_state", "gap", "repo_ids")
            if key in lane
        }
    return {
        "lane_id": lane_context.get("lane_id"),
        "lane": compact_lane,
        "open_decision_ids": [
            str(decision.get("id"))
            for decision in lane_context.get("open_decisions", [])
            if isinstance(decision, dict) and decision.get("id")
        ],
        "resolved_decision_ids": [
            str(decision.get("id"))
            for decision in lane_context.get("resolved_decisions", [])
            if isinstance(decision, dict) and decision.get("id")
        ],
        "release_gate_ids": [
            str(gate.get("id"))
            for gate in lane_context.get("release_gates", [])
            if isinstance(gate, dict) and gate.get("id")
        ],
    }


def compact_match(match: dict[str, Any]) -> dict[str, Any]:
    compacted = dict(match)
    compacted["lane_context"] = compact_lane_context(match.get("lane_context"))
    return compacted


def compact_contract_update(contract_update: dict[str, Any]) -> dict[str, Any]:
    compacted = dict(contract_update)
    if compacted.get("matched_reference_details"):
        compacted.pop("matched_references", None)
    return compacted


def compact_lookup_result(result: dict[str, Any]) -> dict[str, Any]:
    return {
        "control_plane": compact_control_plane(result.get("control_plane", {})),
        "status_audit_errors": list(result.get("status_audit_errors", [])),
        "files": [
            {
                **entry,
                "matches": [compact_match(match) for match in entry.get("matches", [])],
                "dependent_contract_updates": [
                    compact_contract_update(contract)
                    for contract in entry.get("dependent_contract_updates", [])
                ],
            }
            for entry in result.get("files", [])
        ],
        "impacted_subsystems": [
            {
                **entry,
                "lane_context": compact_lane_context(entry.get("lane_context")),
            }
            for entry in result.get("impacted_subsystems", [])
        ],
        "required_contract_updates": [
            compact_contract_update(contract)
            for contract in result.get("required_contract_updates", [])
        ],
        "unowned_runtime_files": list(result.get("unowned_runtime_files", [])),
    }


def lookup_paths(paths: list[str], *, lean: bool = False) -> dict[str, Any]:
    normalized = [normalize_input_path(path) for path in paths if path.strip()]
    rules = load_subsystem_rules()
    rules_by_id = {str(rule["id"]): rule for rule in rules}
    contract_index = load_contract_index()
    registry_payload = load_registry_payload()
    shared_ownership_by_path = {
        str(entry["path"]): entry
        for entry in registry_payload.get("shared_ownerships", [])
        if isinstance(entry, dict) and isinstance(entry.get("path"), str)
    }
    status_report = audit_status_payload(load_status_payload())
    impacted = infer_impacted_subsystems(normalized)
    contract_updates = required_contract_updates(normalized, impacted)
    path_entries: list[dict[str, Any]] = []
    unowned: list[str] = []

    for path in normalized:
        classification = "runtime"
        if is_ignored_runtime_file(path):
            classification = "ignored"
        elif is_test_or_fixture(path):
            classification = "test-or-fixture"
        reference_matches_by_subsystem = {
            str(contract["subsystem_id"]): normalized_reference_details(
                list(contract.get("matched_references", []))
            )
            for contract in referenced_contracts_for_path(path, contract_index)
        }

        matches = []
        if classification == "runtime":
            for rule in rules:
                if not subsystem_matches_path(rule, path):
                    continue
                requirement = verification_requirement_for_path(rule, path)
                matches.append(
                    {
                        "subsystem": rule["id"],
                        "contract": rule["contract"],
                        "lane_context": lane_context_for_rule(rule, status_report),
                        "contract_update_required": True,
                        "proof_update_required": True,
                        "ownership_basis": ownership_basis_for_path(rule, path),
                        "matched_contract_references": reference_matches_by_subsystem.get(
                            str(rule["id"]),
                            [],
                        ),
                        "verification_requirement": requirement,
                    }
                )
        elif classification == "test-or-fixture":
            for rule in rules:
                for requirement in verification_requirements_for_test_path(rule, path):
                    matches.append(
                        {
                            "subsystem": rule["id"],
                            "contract": rule["contract"],
                            "lane_context": lane_context_for_rule(rule, status_report),
                            "contract_update_required": False,
                            "proof_update_required": False,
                            "ownership_basis": ownership_basis_for_path(rule, path),
                            "verification_requirement": requirement,
                        }
                    )
        if not matches and classification == "runtime":
            unowned.append(path)

        path_entries.append(
            {
                "path": path,
                "classification": classification,
                "matches": matches,
                "shared_ownership": shared_ownership_by_path.get(path),
                "dependent_contract_updates": [
                    contract
                    for contract in contract_updates.values()
                    if contract["reason"] == "dependent-reference"
                    if path in contract["touched_runtime_files"]
                ],
                "contract_update_required": classification == "runtime" and bool(matches),
                "proof_update_required": classification == "runtime" and bool(matches),
            }
        )

    impacted_summary = []
    for subsystem_id, data in sorted(impacted.items()):
        rule = rules_by_id[subsystem_id]
        impacted_summary.append(
            {
                "subsystem": subsystem_id,
                "contract": data["contract"],
                "lane_context": lane_context_for_rule(rule, status_report),
                "touched_runtime_files": data["touched_runtime_files"],
                "verification_requirements": data["verification_requirements"],
            }
        )

    result = {
        "control_plane": status_report.get("control_plane", {}),
        "scope": status_report.get("scope", {}),
        "status_summary": status_report.get("summary", {}),
        "status_audit_errors": status_report.get("errors", []),
        "files": path_entries,
        "impacted_subsystems": impacted_summary,
        "required_contract_updates": list(contract_updates.values()),
        "unowned_runtime_files": unowned,
    }
    return compact_lookup_result(result) if lean else result


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Lookup subsystem ownership for file paths.")
    parser.add_argument("paths", nargs="*", help="Repo-relative or absolute file paths to inspect.")
    parser.add_argument(
        "--files-from-stdin",
        action="store_true",
        help="Read newline-delimited file paths from standard input.",
    )
    parser.add_argument(
        "--pretty",
        action="store_true",
        help="Print a concise human-readable summary instead of JSON.",
    )
    parser.add_argument(
        "--lean",
        action="store_true",
        help="Return compact agent-facing context with exact contract reference lines.",
    )
    return parser.parse_args(argv)


def format_reference_detail(reference: dict[str, Any]) -> str:
    heading = str(reference.get("heading", "")).strip() or "-"
    path = str(reference.get("path", "")).strip()
    line = reference.get("line")
    line_suffix = f" @L{line}" if isinstance(line, int) else ""
    if path:
        return f"{heading}{line_suffix}: {path}"
    return f"{heading}{line_suffix}"


def render_pretty(result: dict[str, Any]) -> str:
    lines: list[str] = []
    control_plane = result.get("control_plane", {})
    if control_plane:
        active_target = control_plane.get("active_target", {})
        lines.append(
            "control_plane: "
            f"profile={control_plane.get('active_profile_id') or '-'} "
            f"target={active_target.get('id') or '-'} "
            f"kind={active_target.get('kind') or '-'}"
        )
    scope = result.get("scope", {})
    if scope:
        lines.append(
            "scope: "
            f"control_plane={scope.get('control_plane_repo') or '-'} "
            f"active_repos={','.join(scope.get('active_repos', [])) or '-'}"
        )
    for entry in result["files"]:
        lines.append(f"{entry['path']}: {entry['classification']}")
        for match in entry["matches"]:
            requirement = match["verification_requirement"]
            lane_context = match["lane_context"]
            lines.append(
                f"  - {match['subsystem']} -> {match['contract']} "
                f"[{requirement['id']}: {requirement['label']}]"
            )
            if lane_context and lane_context.get("lane"):
                lane = lane_context["lane"]
                gap = lane.get("gap")
                gap_text = f"{gap:.0f}" if isinstance(gap, (int, float)) else "-"
                open_decisions = lane_context.get("open_decisions", lane_context.get("open_decision_ids", []))
                release_gates = lane_context.get("release_gates", lane_context.get("release_gate_ids", []))
                lines.append(
                    f"    lane {lane_context['lane_id']} "
                    f"gap={gap_text} derived={lane.get('derived_status') or '-'} "
                    f"repos={','.join(lane.get('repo_ids', [])) or '-'} "
                    f"open_decisions={len(open_decisions)} "
                    f"release_gates={len(release_gates)}"
                )
            if match.get("matched_contract_references"):
                focus = ", ".join(
                    format_reference_detail(reference)
                    for reference in match["matched_contract_references"]
                )
                lines.append(f"    contract focus: {focus}")
            elif match.get("ownership_basis"):
                basis = match["ownership_basis"]
                lines.append(
                    f"    ownership basis: {basis.get('type') or '-'} {basis.get('value') or '-'}"
                )
        if entry.get("shared_ownership"):
            shared = entry["shared_ownership"]
            lines.append(
                f"  - shared ownership: {', '.join(shared['subsystems'])} ({shared['rationale']})"
            )
        for contract in entry.get("dependent_contract_updates", []):
            lines.append(
                f"  - also update {contract['subsystem']} contract -> {contract['contract']}"
            )
            if contract.get("matched_reference_details"):
                for reference in contract["matched_reference_details"]:
                    lines.append(f"    referenced by {format_reference_detail(reference)}")
            else:
                for reference in contract.get("matched_references", []):
                    lines.append(f"    referenced by {reference}")
        if entry["classification"] == "runtime" and not entry["matches"]:
            lines.append("  - no owning subsystem rule matched")
    for path in result["unowned_runtime_files"]:
        lines.append(f"unowned: {path}")
    if result.get("status_audit_errors"):
        lines.append("status audit errors:")
        for err in result["status_audit_errors"]:
            lines.append(f"  - {err}")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(list(argv or []))
    paths = list(args.paths)
    if args.files_from_stdin:
        paths.extend(line.strip() for line in sys.stdin if line.strip())
    if not paths:
        print("no file paths provided", file=sys.stderr)
        return 2

    result = lookup_paths(paths, lean=args.lean)
    output = render_pretty(result) if args.pretty else json.dumps(result, indent=2, sort_keys=True)
    print(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
