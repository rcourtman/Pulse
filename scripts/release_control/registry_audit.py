#!/usr/bin/env python3
"""Machine audit for the v6 subsystem registry."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import subprocess
import sys
from typing import Any

from canonical_completion_guard import (
    REPO_ROOT,
    is_ignored_runtime_file,
    is_test_or_fixture,
    path_policy_matches,
    subsystem_matches_path,
)
from status_audit import load_status_payload


REGISTRY_PATH = REPO_ROOT / "docs" / "release-control" / "v6" / "subsystems" / "registry.json"
REGISTRY_SCHEMA_PATH = REPO_ROOT / "docs" / "release-control" / "v6" / "subsystems" / "registry.schema.json"
LANE_RE = re.compile(r"^L[0-9]+$")


def load_registry_payload() -> dict[str, Any]:
    return json.loads(REGISTRY_PATH.read_text(encoding="utf-8"))


def load_registry_schema() -> dict[str, Any]:
    return json.loads(REGISTRY_SCHEMA_PATH.read_text(encoding="utf-8"))


def schema_required(schema: dict[str, Any], definition: str | None = None) -> set[str]:
    target = schema if definition is None else schema["$defs"][definition]
    return set(target["required"])


REGISTRY_SCHEMA = load_registry_schema()
REQUIRED_TOP_LEVEL_FIELDS = schema_required(REGISTRY_SCHEMA)
REQUIRED_SUBSYSTEM_FIELDS = schema_required(REGISTRY_SCHEMA, "subsystem")
REQUIRED_VERIFICATION_FIELDS = schema_required(REGISTRY_SCHEMA, "verification")
REQUIRED_PATH_POLICY_FIELDS = schema_required(REGISTRY_SCHEMA, "path_policy")


def tracked_repo_files() -> set[str]:
    result = subprocess.run(
        ["git", "ls-files", "-z"],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=False,
    )
    return {
        entry.decode("utf-8")
        for entry in result.stdout.split(b"\x00")
        if entry
    }


def clean_relative_path(path: str) -> str:
    return Path(path).as_posix()


def validate_path_reference(
    path: str,
    *,
    context: str,
    errors: list[str],
    tracked_files: set[str],
    require_file: bool = True,
) -> None:
    normalized = clean_relative_path(path)
    if normalized != path or path.startswith("../") or "/../" in path or Path(path).is_absolute():
        errors.append(f"{context} must be a clean repo-relative path: {path!r}")
        return
    if require_file and path not in tracked_files:
        errors.append(f"{context} missing tracked file {path!r}")


def validate_prefix(
    prefix: str,
    *,
    context: str,
    errors: list[str],
    tracked_files: set[str],
) -> None:
    normalized = clean_relative_path(prefix.rstrip("/"))
    raw = prefix.rstrip("/")
    if normalized != raw or raw.startswith("../") or "/../" in raw or Path(raw).is_absolute():
        errors.append(f"{context} must be a clean repo-relative prefix: {prefix!r}")
        return
    if not any(path.startswith(prefix) for path in tracked_files):
        errors.append(f"{context} does not match any tracked files: {prefix!r}")


def owned_runtime_files(rule: dict[str, Any], tracked_files: set[str]) -> list[str]:
    return sorted(
        path
        for path in tracked_files
        if subsystem_matches_path(rule, path)
        if not is_test_or_fixture(path)
        if not is_ignored_runtime_file(path)
    )


def audit_registry_payload(
    payload: dict[str, Any],
    *,
    tracked_files: set[str],
    status_lane_ids: set[str],
) -> dict[str, Any]:
    errors: list[str] = []
    warnings: list[str] = []

    for field in sorted(REQUIRED_TOP_LEVEL_FIELDS):
        if field not in payload:
            errors.append(f"registry.json missing required field {field}")

    version = payload.get("version")
    if version != REGISTRY_SCHEMA["properties"]["version"]["const"]:
        errors.append(f"registry.json version must be {REGISTRY_SCHEMA['properties']['version']['const']}")

    raw_subsystems = payload.get("subsystems")
    if not isinstance(raw_subsystems, list) or not raw_subsystems:
        errors.append("registry.json missing non-empty subsystems list")
        return {"errors": errors, "warnings": warnings, "summary": {}}

    seen_ids: set[str] = set()
    seen_contracts: set[str] = set()
    subsystem_summaries: list[dict[str, Any]] = []

    for index, raw_subsystem in enumerate(raw_subsystems):
        context = f"subsystems[{index}]"
        if not isinstance(raw_subsystem, dict):
            errors.append(f"{context} must be an object")
            continue

        for field in sorted(REQUIRED_SUBSYSTEM_FIELDS):
            if field not in raw_subsystem:
                errors.append(f"{context} missing required field {field}")

        subsystem_id = raw_subsystem.get("id")
        if not isinstance(subsystem_id, str) or not subsystem_id.strip():
            errors.append(f"{context} missing non-empty string id")
            continue
        if subsystem_id in seen_ids:
            errors.append(f"{context} duplicates subsystem id {subsystem_id!r}")
        seen_ids.add(subsystem_id)

        lane = raw_subsystem.get("lane")
        if not isinstance(lane, str) or not LANE_RE.match(lane):
            errors.append(f"{context} has invalid lane {lane!r}")
        elif lane not in status_lane_ids:
            errors.append(f"{context} references unknown status lane {lane!r}")

        contract = raw_subsystem.get("contract")
        if not isinstance(contract, str) or not contract.strip():
            errors.append(f"{context} missing non-empty string contract")
        else:
            validate_path_reference(
                contract,
                context=f"{context}.contract",
                errors=errors,
                tracked_files=tracked_files,
            )
            if contract in seen_contracts:
                errors.append(f"{context} duplicates contract path {contract!r}")
            seen_contracts.add(contract)

        owned_prefixes = raw_subsystem.get("owned_prefixes")
        if not isinstance(owned_prefixes, list):
            errors.append(f"{context}.owned_prefixes must be a list")
            owned_prefixes = []
        for prefix_index, prefix in enumerate(owned_prefixes):
            if not isinstance(prefix, str) or not prefix.strip():
                errors.append(f"{context}.owned_prefixes[{prefix_index}] must be a non-empty string")
                continue
            validate_prefix(
                prefix,
                context=f"{context}.owned_prefixes[{prefix_index}]",
                errors=errors,
                tracked_files=tracked_files,
            )

        owned_files = raw_subsystem.get("owned_files")
        if not isinstance(owned_files, list):
            errors.append(f"{context}.owned_files must be a list")
            owned_files = []
        for file_index, path in enumerate(owned_files):
            if not isinstance(path, str) or not path.strip():
                errors.append(f"{context}.owned_files[{file_index}] must be a non-empty string")
                continue
            validate_path_reference(
                path,
                context=f"{context}.owned_files[{file_index}]",
                errors=errors,
                tracked_files=tracked_files,
            )

        verification = raw_subsystem.get("verification")
        if not isinstance(verification, dict):
            errors.append(f"{context}.verification must be an object")
            continue
        for field in sorted(REQUIRED_VERIFICATION_FIELDS):
            if field not in verification:
                errors.append(f"{context}.verification missing required field {field}")

        if not isinstance(verification.get("allow_same_subsystem_tests"), bool):
            errors.append(f"{context}.verification.allow_same_subsystem_tests must be a bool")
        if not isinstance(verification.get("require_explicit_path_policy_coverage"), bool):
            errors.append(f"{context}.verification.require_explicit_path_policy_coverage must be a bool")
        elif verification.get("require_explicit_path_policy_coverage") is not True:
            errors.append(f"{context}.verification.require_explicit_path_policy_coverage must be true")

        test_prefixes = verification.get("test_prefixes")
        if not isinstance(test_prefixes, list):
            errors.append(f"{context}.verification.test_prefixes must be a list")
            test_prefixes = []
        for prefix_index, prefix in enumerate(test_prefixes):
            if not isinstance(prefix, str) or not prefix.strip():
                errors.append(f"{context}.verification.test_prefixes[{prefix_index}] must be a non-empty string")
                continue
            validate_prefix(
                prefix,
                context=f"{context}.verification.test_prefixes[{prefix_index}]",
                errors=errors,
                tracked_files=tracked_files,
            )

        exact_files = verification.get("exact_files")
        if not isinstance(exact_files, list):
            errors.append(f"{context}.verification.exact_files must be a list")
            exact_files = []
        for file_index, path in enumerate(exact_files):
            if not isinstance(path, str) or not path.strip():
                errors.append(f"{context}.verification.exact_files[{file_index}] must be a non-empty string")
                continue
            validate_path_reference(
                path,
                context=f"{context}.verification.exact_files[{file_index}]",
                errors=errors,
                tracked_files=tracked_files,
            )

        path_policies = verification.get("path_policies")
        if not isinstance(path_policies, list):
            errors.append(f"{context}.verification.path_policies must be a list")
            path_policies = []

        seen_policy_ids: set[str] = set()
        for policy_index, raw_policy in enumerate(path_policies):
            policy_context = f"{context}.verification.path_policies[{policy_index}]"
            if not isinstance(raw_policy, dict):
                errors.append(f"{policy_context} must be an object")
                continue

            for field in sorted(REQUIRED_PATH_POLICY_FIELDS):
                if field not in raw_policy:
                    errors.append(f"{policy_context} missing required field {field}")

            policy_id = raw_policy.get("id")
            if not isinstance(policy_id, str) or not policy_id.strip():
                errors.append(f"{policy_context} missing non-empty string id")
            elif policy_id in seen_policy_ids:
                errors.append(f"{policy_context} duplicates policy id {policy_id!r}")
            else:
                seen_policy_ids.add(policy_id)

            label = raw_policy.get("label")
            if not isinstance(label, str) or not label.strip():
                errors.append(f"{policy_context} missing non-empty string label")

            match_prefixes = raw_policy.get("match_prefixes")
            if not isinstance(match_prefixes, list):
                errors.append(f"{policy_context}.match_prefixes must be a list")
                match_prefixes = []
            match_files = raw_policy.get("match_files")
            if not isinstance(match_files, list):
                errors.append(f"{policy_context}.match_files must be a list")
                match_files = []
            if not match_prefixes and not match_files:
                errors.append(f"{policy_context} must define at least one match_prefix or match_file")

            for prefix_index, prefix in enumerate(match_prefixes):
                if not isinstance(prefix, str) or not prefix.strip():
                    errors.append(f"{policy_context}.match_prefixes[{prefix_index}] must be a non-empty string")
                    continue
                validate_prefix(
                    prefix,
                    context=f"{policy_context}.match_prefixes[{prefix_index}]",
                    errors=errors,
                    tracked_files=tracked_files,
                )

            for file_index, path in enumerate(match_files):
                if not isinstance(path, str) or not path.strip():
                    errors.append(f"{policy_context}.match_files[{file_index}] must be a non-empty string")
                    continue
                validate_path_reference(
                    path,
                    context=f"{policy_context}.match_files[{file_index}]",
                    errors=errors,
                    tracked_files=tracked_files,
                )

            if not isinstance(raw_policy.get("allow_same_subsystem_tests"), bool):
                errors.append(f"{policy_context}.allow_same_subsystem_tests must be a bool")

            policy_test_prefixes = raw_policy.get("test_prefixes")
            if not isinstance(policy_test_prefixes, list):
                errors.append(f"{policy_context}.test_prefixes must be a list")
                policy_test_prefixes = []
            for prefix_index, prefix in enumerate(policy_test_prefixes):
                if not isinstance(prefix, str) or not prefix.strip():
                    errors.append(f"{policy_context}.test_prefixes[{prefix_index}] must be a non-empty string")
                    continue
                validate_prefix(
                    prefix,
                    context=f"{policy_context}.test_prefixes[{prefix_index}]",
                    errors=errors,
                    tracked_files=tracked_files,
                )

            policy_exact_files = raw_policy.get("exact_files")
            if not isinstance(policy_exact_files, list):
                errors.append(f"{policy_context}.exact_files must be a list")
                policy_exact_files = []
            for file_index, path in enumerate(policy_exact_files):
                if not isinstance(path, str) or not path.strip():
                    errors.append(f"{policy_context}.exact_files[{file_index}] must be a non-empty string")
                    continue
                validate_path_reference(
                    path,
                    context=f"{policy_context}.exact_files[{file_index}]",
                    errors=errors,
                    tracked_files=tracked_files,
                )

        owned_runtime = owned_runtime_files(raw_subsystem, tracked_files)
        uncovered_owned_runtime = [
            path
            for path in owned_runtime
            if not any(path_policy_matches(policy, path) for policy in path_policies if isinstance(policy, dict))
        ]

        if uncovered_owned_runtime:
            for path in uncovered_owned_runtime:
                errors.append(
                    f"{context} requires explicit path policy coverage but {path!r} falls back to default verification"
                )

        subsystem_summaries.append(
            {
                "id": subsystem_id,
                "lane": lane,
                "owned_runtime_file_count": len(owned_runtime),
                "default_fallback_count": len(uncovered_owned_runtime),
                "path_policy_count": len(path_policies),
            }
        )

    return {
        "errors": errors,
        "warnings": warnings,
        "summary": {
            "subsystem_count": len(subsystem_summaries),
            "explicit_coverage_subsystems": sum(
                1
                for subsystem in raw_subsystems
                if isinstance(subsystem, dict)
                and isinstance(subsystem.get("verification"), dict)
                and subsystem["verification"].get("require_explicit_path_policy_coverage") is True
            ),
        },
        "subsystems": subsystem_summaries,
    }


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Audit docs/release-control/v6/subsystems/registry.json.")
    parser.add_argument(
        "--check",
        action="store_true",
        help="Exit non-zero if the registry audit finds any errors.",
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
            f"subsystems={summary.get('subsystem_count', 0)} "
            f"explicit_coverage={summary.get('explicit_coverage_subsystems', 0)}"
        )
    for subsystem in report.get("subsystems", []):
        lines.append(
            f"{subsystem['id']}: lane={subsystem['lane']} "
            f"owned_runtime={subsystem['owned_runtime_file_count']} "
            f"default_fallback={subsystem['default_fallback_count']} "
            f"path_policies={subsystem['path_policy_count']}"
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
    status_payload = load_status_payload()
    lane_ids = {
        lane.get("id")
        for lane in status_payload.get("lanes", [])
        if isinstance(lane, dict) and isinstance(lane.get("id"), str)
    }
    report = audit_registry_payload(
        load_registry_payload(),
        tracked_files=tracked_repo_files(),
        status_lane_ids=lane_ids,
    )
    output = render_pretty(report) if args.pretty else json.dumps(report, indent=2, sort_keys=True)
    print(output)
    if args.check and report["errors"]:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
