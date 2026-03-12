#!/usr/bin/env python3
"""Audit the evergreen release control plane and active target freshness."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import subprocess
import sys
from typing import Any

from control_plane import (
    CONTROL_PLANE_DOC_REL,
    CONTROL_PLANE_REL,
    CONTROL_PLANE_SCHEMA_REL,
    load_control_plane,
    validate_control_plane_payload,
)
from repo_file_io import REPO_ROOT
from status_audit import audit_status_payload, load_status_payload, status_schema_contract


PROFILE_PATH_FIELDS = (
    ("root", "dir"),
    ("source_of_truth", "file"),
    ("status", "file"),
    ("status_schema", "file"),
    ("development_protocol", "file"),
    ("high_risk_matrix", "file"),
    ("subsystems_dir", "dir"),
    ("registry", "file"),
    ("registry_schema", "file"),
    ("subsystem_contract_template", "file"),
)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Audit the evergreen release control plane and active target."
    )
    parser.add_argument("--check", action="store_true", help="Exit non-zero when errors are present.")
    parser.add_argument(
        "--pretty",
        action="store_true",
        help="Render a concise human summary instead of JSON.",
    )
    parser.add_argument(
        "--staged",
        action="store_true",
        help="Read control-plane JSON and active status data from the git index when available.",
    )
    return parser.parse_args(argv)


def _path_kind(rel: str, *, expected_kind: str, staged: bool) -> str:
    if expected_kind == "dir":
        target = REPO_ROOT / rel
        if target.is_dir():
            return "dir"
        if target.exists():
            return "file"
        return "missing"

    if staged:
        result = subprocess.run(
            ["git", "cat-file", "-e", f":{rel}"],
            cwd=REPO_ROOT,
            check=False,
            capture_output=True,
            text=True,
        )
        if result.returncode == 0:
            return "file"

    target = REPO_ROOT / rel
    if target.is_file():
        return "file"
    if target.exists():
        return "dir"
    return "missing"


def current_path_kinds(*, staged: bool) -> dict[str, str]:
    payload = load_control_plane(staged=staged)
    resolved = validate_control_plane_payload(payload)
    kinds: dict[str, str] = {
        CONTROL_PLANE_DOC_REL: _path_kind(CONTROL_PLANE_DOC_REL, expected_kind="file", staged=staged),
        CONTROL_PLANE_SCHEMA_REL: _path_kind(
            CONTROL_PLANE_SCHEMA_REL, expected_kind="file", staged=staged
        ),
    }
    for profile in resolved["profiles"]:
        for field, expected_kind in PROFILE_PATH_FIELDS:
            rel = str(profile[field]).strip()
            if rel not in kinds:
                kinds[rel] = _path_kind(rel, expected_kind=expected_kind, staged=staged)
    return kinds


def current_status_report(*, staged: bool) -> dict[str, Any]:
    return audit_status_payload(
        load_status_payload(staged=staged),
        schema_contract=status_schema_contract(staged=staged),
        use_staged_registry=staged,
    )


def audit_control_plane_payload(
    payload: dict[str, Any],
    *,
    path_kinds: dict[str, str],
    status_report: dict[str, Any] | None = None,
) -> dict[str, Any]:
    errors: list[str] = []
    warnings: list[str] = []

    try:
        resolved = validate_control_plane_payload(payload)
    except ValueError as exc:
        return {
            "errors": [str(exc)],
            "warnings": [],
            "summary": {
                "profile_count": 0,
                "target_count": 0,
                "active_target_completion_met": False,
            },
        }

    doc_kind = path_kinds.get(CONTROL_PLANE_DOC_REL, "missing")
    if doc_kind != "file":
        errors.append(
            f"control plane doc {CONTROL_PLANE_DOC_REL!r} must exist as a tracked file, found {doc_kind!r}"
        )
    schema_kind = path_kinds.get(CONTROL_PLANE_SCHEMA_REL, "missing")
    if schema_kind != "file":
        errors.append(
            "control plane schema "
            f"{CONTROL_PLANE_SCHEMA_REL!r} must exist as a tracked file, found {schema_kind!r}"
        )

    profiles: list[dict[str, Any]] = []
    for profile in resolved["profiles"]:
        missing_paths: list[str] = []
        for field, expected_kind in PROFILE_PATH_FIELDS:
            rel = str(profile[field]).strip()
            actual_kind = path_kinds.get(rel, "missing")
            if actual_kind != expected_kind:
                errors.append(
                    f"profile {profile['id']!r} field {field!r} expects {expected_kind} at {rel!r}, "
                    f"found {actual_kind!r}"
                )
                missing_paths.append(rel)
        profiles.append(
            {
                "id": profile["id"],
                "lifecycle": profile["lifecycle"],
                "root": profile["root"],
                "missing_paths": missing_paths,
            }
        )

    active_target_completion_met = False
    if status_report is not None:
        status_errors = status_report.get("errors", [])
        if status_errors:
            warnings.append(
                "status audit reported errors; active target completion could not be trusted for stale-target enforcement"
            )
        else:
            control_plane = status_report.get("control_plane", {})
            active_target = control_plane.get("active_target", {})
            active_target_completion_met = bool(active_target.get("completion_met"))

    if active_target_completion_met:
        errors.append(
            "active target "
            f"{resolved['active_target_id']!r} already satisfies its completion rule; mark it completed "
            f"and promote the next target in {CONTROL_PLANE_REL}"
        )

    return {
        "errors": errors,
        "warnings": warnings,
        "control_plane": {
            "active_profile_id": resolved["active_profile_id"],
            "active_target": {
                **resolved["active_target"],
                "completion_met": active_target_completion_met,
            },
        },
        "profiles": profiles,
        "targets": resolved["targets"],
        "summary": {
            "profile_count": len(resolved["profiles"]),
            "target_count": len(resolved["targets"]),
            "active_target_completion_met": active_target_completion_met,
        },
    }


def render_pretty(report: dict[str, Any]) -> str:
    lines: list[str] = []
    control_plane = report.get("control_plane", {})
    active_target = control_plane.get("active_target", {})
    lines.append(
        "control_plane: "
        f"profile={control_plane.get('active_profile_id', '-') or '-'} "
        f"target={active_target.get('id', '-') or '-'} "
        f"kind={active_target.get('kind', '-') or '-'} "
        f"completion_rule={active_target.get('completion_rule', '-') or '-'} "
        f"completion_met={active_target.get('completion_met')}"
    )
    for profile in report.get("profiles", []):
        lines.append(
            f"profile {profile['id']}: lifecycle={profile['lifecycle']} "
            f"missing_paths={len(profile['missing_paths'])}"
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
    report = audit_control_plane_payload(
        load_control_plane(staged=args.staged),
        path_kinds=current_path_kinds(staged=args.staged),
        status_report=current_status_report(staged=args.staged),
    )
    output = render_pretty(report) if args.pretty else json.dumps(report, indent=2, sort_keys=True)
    print(output)
    if args.check and report["errors"]:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
