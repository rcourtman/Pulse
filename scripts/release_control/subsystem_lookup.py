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
    subsystem_matches_path,
)


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


def lookup_paths(paths: list[str]) -> dict[str, Any]:
    normalized = [normalize_input_path(path) for path in paths if path.strip()]
    rules = load_subsystem_rules()
    impacted = infer_impacted_subsystems(normalized)
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
                "contract_update_required": classification == "runtime" and bool(matches),
                "proof_update_required": classification == "runtime" and bool(matches),
            }
        )

    impacted_summary = []
    for subsystem_id, data in sorted(impacted.items()):
        impacted_summary.append(
            {
                "subsystem": subsystem_id,
                "contract": data["contract"],
                "touched_runtime_files": data["touched_runtime_files"],
                "verification_requirements": data["verification_requirements"],
            }
        )

    return {
        "files": path_entries,
        "impacted_subsystems": impacted_summary,
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
            lines.append(
                f"  - {match['subsystem']} -> {match['contract']} "
                f"[{requirement['id']}: {requirement['label']}]"
            )
        if entry["classification"] == "runtime" and not entry["matches"]:
            lines.append("  - no owning subsystem rule matched")
    for path in result["unowned_runtime_files"]:
        lines.append(f"unowned: {path}")
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
