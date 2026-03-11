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
    required_contract_updates,
    subsystem_matches_path,
)
from status_audit import audit_status_payload, load_status_payload


def normalize_input_path(raw: str) -> str:
    candidate = Path(raw.strip())
    if candidate.is_absolute():
        try:
            candidate = candidate.resolve().relative_to(REPO_ROOT)
        except ValueError:
            return candidate.as_posix()
    return candidate.as_posix()


def verification_requirement_for_path(rule: dict[str, Any], path: str) -> dict[str, Any]:
    return build_verification_requirements(rule, [path])[0]


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
    return {
        "lane_id": lane_id,
        "lane": lane,
        "open_decisions": open_decisions,
        "resolved_decisions": resolved_decisions,
    }


def lookup_paths(paths: list[str]) -> dict[str, Any]:
    normalized = [normalize_input_path(path) for path in paths if path.strip()]
    rules = load_subsystem_rules()
    rules_by_id = {str(rule["id"]): rule for rule in rules}
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

    return {
        "status_summary": status_report.get("summary", {}),
        "status_audit_errors": status_report.get("errors", []),
        "files": path_entries,
        "impacted_subsystems": impacted_summary,
        "required_contract_updates": list(contract_updates.values()),
        "unowned_runtime_files": unowned,
    }


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
    return parser.parse_args(argv)


def render_pretty(result: dict[str, Any]) -> str:
    lines: list[str] = []
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
                lines.append(
                    f"    lane {lane_context['lane_id']} "
                    f"gap={lane['gap']:.0f} derived={lane['derived_status']} "
                    f"open_decisions={len(lane_context['open_decisions'])}"
                )
        for contract in entry.get("dependent_contract_updates", []):
            lines.append(
                f"  - also update {contract['subsystem']} contract -> {contract['contract']}"
            )
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

    result = lookup_paths(paths)
    output = render_pretty(result) if args.pretty else json.dumps(result, indent=2, sort_keys=True)
    print(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
