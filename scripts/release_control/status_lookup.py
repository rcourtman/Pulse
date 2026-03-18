#!/usr/bin/env python3
"""Targeted lookup helper for active release-control status surfaces."""

from __future__ import annotations

import argparse
import json
import sys
from typing import Any

from repo_file_io import read_repo_text
from status_audit import audit_status_payload, load_status_payload, status_schema_contract


ENTRY_OPTIONS: tuple[tuple[str, str, str], ...] = (
    ("lane", "lanes", "id"),
    ("assertion", "readiness_assertions", "id"),
    ("release_gate", "release_gates", "id"),
    ("followup", "lane_followups", "id"),
    ("decision", "open_decisions", "id"),
    ("resolved_decision", "resolved_decisions", "id"),
    ("work_claim", "work_claims", "id"),
)


def load_status_report(*, staged: bool = False) -> dict[str, Any]:
    return audit_status_payload(
        load_status_payload(staged=staged),
        schema_contract=status_schema_contract(staged=staged),
        use_staged_registry=staged,
        current_version=read_repo_text("VERSION", staged=staged).strip(),
    )


def _selected_lookup(args: argparse.Namespace) -> tuple[str, str, str]:
    selected: list[tuple[str, str, str]] = []
    for option, collection, key in ENTRY_OPTIONS:
        value = getattr(args, option)
        if value:
            selected.append((option, collection, key))
    if len(selected) != 1:
        raise ValueError("exactly one lookup selector is required")
    return selected[0]


def lookup_status_entry(report: dict[str, Any], *, kind: str, entry_id: str) -> dict[str, Any]:
    for option, collection, key in ENTRY_OPTIONS:
        if option != kind:
            continue
        entries = report.get(collection, [])
        if not isinstance(entries, list):
            raise KeyError(f"{collection} is not a list in status report")
        for entry in entries:
            if isinstance(entry, dict) and str(entry.get(key)) == entry_id:
                return {
                    "kind": kind,
                    "collection": collection,
                    "id": entry_id,
                    "entry": entry,
                    "control_plane": report.get("control_plane", {}),
                    "summary": report.get("summary", {}),
                }
        raise KeyError(f"{kind} {entry_id!r} not found")
    raise KeyError(f"unsupported kind {kind!r}")


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Lookup one release-control status entry by id.")
    parser.add_argument("--lane", help="Lane id, such as L16.")
    parser.add_argument("--assertion", help="Readiness assertion id, such as RA8.")
    parser.add_argument("--release-gate", dest="release_gate", help="Release gate id.")
    parser.add_argument("--followup", help="Lane followup id.")
    parser.add_argument("--decision", help="Open decision id.")
    parser.add_argument("--resolved-decision", dest="resolved_decision", help="Resolved decision id.")
    parser.add_argument("--work-claim", dest="work_claim", help="Work claim id.")
    parser.add_argument(
        "--pretty",
        action="store_true",
        help="Print a concise human-readable summary instead of JSON.",
    )
    parser.add_argument(
        "--staged",
        action="store_true",
        help="Read status data from the git index when available.",
    )
    return parser.parse_args(argv)


def _render_collection_specific(lines: list[str], kind: str, entry: dict[str, Any]) -> None:
    if kind == "lane":
        lines.extend(
            [
                f"gap={entry.get('gap')} status={entry.get('status')} completion={entry.get('completion_state')} derived={entry.get('derived_status')}",
                f"repos={','.join(entry.get('repo_ids', [])) or '-'} subsystems={','.join(entry.get('subsystem_ids', [])) or '-'} evidence_count={entry.get('evidence_count')}",
            ]
        )
        if entry.get("completion_summary"):
            lines.append(f"completion_summary: {entry['completion_summary']}")
        for tracking in entry.get("completion_tracking", []):
            lines.append(
                f"tracking: {tracking.get('kind')}:{tracking.get('id')} status={tracking.get('status')} resolved={tracking.get('resolved')}"
            )
        for blocker in entry.get("blockers", []):
            lines.append(f"blocker: {blocker}")
        return
    if kind == "assertion":
        lines.extend(
            [
                f"derived={entry.get('derived_status')} blocking={entry.get('blocking_level')} proof_type={entry.get('proof_type')}",
                f"lanes={','.join(entry.get('lane_ids', [])) or '-'} subsystems={','.join(entry.get('subsystem_ids', [])) or '-'} gates={','.join(entry.get('release_gate_ids', [])) or '-'}",
                f"evidence_count={entry.get('evidence_count')} proof_commands={entry.get('proof_command_count')}",
            ]
        )
        if entry.get("minimum_evidence_tier"):
            lines.append(
                f"highest_tier={entry.get('highest_evidence_tier')} min_tier={entry.get('minimum_evidence_tier')}"
            )
        return
    if kind == "release_gate":
        lines.extend(
            [
                f"status={entry.get('status')} effective={entry.get('effective_status')} blocking={entry.get('blocking_level')}",
                f"lanes={','.join(entry.get('lane_ids', [])) or '-'} repos={','.join(entry.get('repo_ids', [])) or '-'} subsystems={','.join(entry.get('subsystem_ids', [])) or '-'}",
                f"highest_tier={entry.get('highest_evidence_tier')} min_tier={entry.get('minimum_evidence_tier')}",
            ]
        )
        return
    if kind == "followup":
        lines.extend(
            [
                f"status={entry.get('status')} lanes={','.join(entry.get('lane_ids', [])) or '-'} repos={','.join(entry.get('repo_ids', [])) or '-'}",
                f"subsystems={','.join(entry.get('subsystem_ids', [])) or '-'}",
            ]
        )
        return
    if kind in {"decision", "resolved_decision"}:
        lines.extend(
            [
                f"status={entry.get('status')} blocking={entry.get('blocking_level') or '-'}",
                f"lanes={','.join(entry.get('lane_ids', [])) or '-'} repos={','.join(entry.get('repo_ids', [])) or '-'} subsystems={','.join(entry.get('subsystem_ids', [])) or '-'}",
            ]
        )
        return
    if kind == "work_claim":
        work_item = entry.get("work_item", {})
        lines.extend(
            [
                f"agent={entry.get('agent_id')} target={entry.get('target_id')} claimed_at={entry.get('claimed_at')}",
                f"work={work_item.get('kind')}:{work_item.get('id')} expires_at={entry.get('expires_at')}",
            ]
        )


def render_pretty(result: dict[str, Any]) -> str:
    control_plane = result.get("control_plane", {})
    active_target = control_plane.get("active_target", {})
    entry = result["entry"]
    lines = [
        "control_plane: "
        f"profile={control_plane.get('active_profile_id') or '-'} "
        f"target={active_target.get('id') or '-'} "
        f"kind={active_target.get('kind') or '-'}",
        f"{result['kind']} {result['id']}: {entry.get('summary') or '-'}",
    ]
    _render_collection_specific(lines, result["kind"], entry)
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(list(argv or []))
    try:
        kind, _, _ = _selected_lookup(args)
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 2

    entry_id = str(getattr(args, kind))
    report = load_status_report(staged=args.staged)
    try:
        result = lookup_status_entry(report, kind=kind, entry_id=entry_id)
    except KeyError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    output = render_pretty(result) if args.pretty else json.dumps(result, indent=2, sort_keys=True)
    print(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
