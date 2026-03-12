#!/usr/bin/env python3
"""Machine audit for the active release profile subsystem registry."""

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
from control_plane import DEFAULT_CONTROL_PLANE
from repo_file_io import load_repo_json
from status_audit import load_status_payload


REGISTRY_PATH = DEFAULT_CONTROL_PLANE["registry_path"]
REGISTRY_SCHEMA_PATH = DEFAULT_CONTROL_PLANE["registry_schema_path"]
LANE_RE = re.compile(r"^L[0-9]+$")


def load_registry_payload(*, staged: bool = False) -> dict[str, Any]:
    return load_repo_json(REGISTRY_PATH, staged=staged)


def load_registry_schema(*, staged: bool = False) -> dict[str, Any]:
    return load_repo_json(REGISTRY_SCHEMA_PATH, staged=staged)


def schema_required(schema: dict[str, Any], definition: str | None = None) -> set[str]:
    target = schema if definition is None else schema["$defs"][definition]
    return set(target["required"])


def registry_schema_contract(*, staged: bool = False) -> dict[str, Any]:
    schema = load_registry_schema(staged=staged)
    return {
        "schema": schema,
        "required_top_level_fields": schema_required(schema),
        "required_subsystem_fields": schema_required(schema, "subsystem"),
        "required_verification_fields": schema_required(schema, "verification"),
        "required_path_policy_fields": schema_required(schema, "path_policy"),
        "required_shared_ownership_fields": schema_required(schema, "shared_ownership"),
    }


DEFAULT_REGISTRY_SCHEMA_CONTRACT = registry_schema_contract()


def sorted_casefold(values: list[str]) -> list[str]:
    return sorted(values, key=lambda value: value.casefold())


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
    schema_contract: dict[str, Any] | None = None,
) -> dict[str, Any]:
    contract = schema_contract or DEFAULT_REGISTRY_SCHEMA_CONTRACT
    schema = contract["schema"]
    required_top_level_fields = set(contract["required_top_level_fields"])
    required_subsystem_fields = set(contract["required_subsystem_fields"])
    required_verification_fields = set(contract["required_verification_fields"])
    required_path_policy_fields = set(contract["required_path_policy_fields"])
    required_shared_ownership_fields = set(contract["required_shared_ownership_fields"])
    errors: list[str] = []
    warnings: list[str] = []

    for field in sorted(required_top_level_fields):
        if field not in payload:
            errors.append(f"registry.json missing required field {field}")

    version = payload.get("version")
    if version != schema["properties"]["version"]["const"]:
        errors.append(f"registry.json version must be {schema['properties']['version']['const']}")

    raw_subsystems = payload.get("subsystems")
    if not isinstance(raw_subsystems, list) or not raw_subsystems:
        errors.append("registry.json missing non-empty subsystems list")
        return {"errors": errors, "warnings": warnings, "summary": {}}

    raw_shared_ownerships = payload.get("shared_ownerships")
    if not isinstance(raw_shared_ownerships, list):
        errors.append("registry.json missing shared_ownerships list")
        raw_shared_ownerships = []

    seen_ids: set[str] = set()
    seen_contracts: set[str] = set()
    subsystem_summaries: list[dict[str, Any]] = []
    subsystem_order: list[str] = []

    for index, raw_subsystem in enumerate(raw_subsystems):
        context = f"subsystems[{index}]"
        if not isinstance(raw_subsystem, dict):
            errors.append(f"{context} must be an object")
            continue

        for field in sorted(required_subsystem_fields):
            if field not in raw_subsystem:
                errors.append(f"{context} missing required field {field}")

        subsystem_id = raw_subsystem.get("id")
        if not isinstance(subsystem_id, str) or not subsystem_id.strip():
            errors.append(f"{context} missing non-empty string id")
            continue
        if subsystem_id in seen_ids:
            errors.append(f"{context} duplicates subsystem id {subsystem_id!r}")
        seen_ids.add(subsystem_id)
        subsystem_order.append(subsystem_id)

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
        else:
            if len(owned_prefixes) != len(set(owned_prefixes)):
                errors.append(f"{context}.owned_prefixes must not contain duplicates")
            if owned_prefixes != sorted_casefold(owned_prefixes):
                errors.append(f"{context}.owned_prefixes must be sorted lexicographically")
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
        else:
            if len(owned_files) != len(set(owned_files)):
                errors.append(f"{context}.owned_files must not contain duplicates")
            if owned_files != sorted_casefold(owned_files):
                errors.append(f"{context}.owned_files must be sorted lexicographically")
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
        for field in sorted(required_verification_fields):
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
        else:
            if len(test_prefixes) != len(set(test_prefixes)):
                errors.append(f"{context}.verification.test_prefixes must not contain duplicates")
            if test_prefixes != sorted_casefold(test_prefixes):
                errors.append(f"{context}.verification.test_prefixes must be sorted lexicographically")
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
        else:
            if len(exact_files) != len(set(exact_files)):
                errors.append(f"{context}.verification.exact_files must not contain duplicates")
            if exact_files != sorted_casefold(exact_files):
                errors.append(f"{context}.verification.exact_files must be sorted lexicographically")
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
        valid_policies: list[tuple[str, dict[str, Any]]] = []
        for policy_index, raw_policy in enumerate(path_policies):
            policy_context = f"{context}.verification.path_policies[{policy_index}]"
            if not isinstance(raw_policy, dict):
                errors.append(f"{policy_context} must be an object")
                continue

            for field in sorted(required_path_policy_fields):
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
            else:
                if len(match_prefixes) != len(set(match_prefixes)):
                    errors.append(f"{policy_context}.match_prefixes must not contain duplicates")
                if match_prefixes != sorted_casefold(match_prefixes):
                    errors.append(f"{policy_context}.match_prefixes must be sorted lexicographically")
            match_files = raw_policy.get("match_files")
            if not isinstance(match_files, list):
                errors.append(f"{policy_context}.match_files must be a list")
                match_files = []
            else:
                if len(match_files) != len(set(match_files)):
                    errors.append(f"{policy_context}.match_files must not contain duplicates")
                if match_files != sorted_casefold(match_files):
                    errors.append(f"{policy_context}.match_files must be sorted lexicographically")
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
            else:
                if len(policy_test_prefixes) != len(set(policy_test_prefixes)):
                    errors.append(f"{policy_context}.test_prefixes must not contain duplicates")
                if policy_test_prefixes != sorted_casefold(policy_test_prefixes):
                    errors.append(f"{policy_context}.test_prefixes must be sorted lexicographically")
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
            else:
                if len(policy_exact_files) != len(set(policy_exact_files)):
                    errors.append(f"{policy_context}.exact_files must not contain duplicates")
                if policy_exact_files != sorted_casefold(policy_exact_files):
                    errors.append(f"{policy_context}.exact_files must be sorted lexicographically")
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

            valid_policies.append((policy_context, raw_policy))

        owned_runtime = owned_runtime_files(raw_subsystem, tracked_files)
        previous_policies: list[dict[str, Any]] = []
        for policy_context, policy in valid_policies:
            matched_owned_runtime = [path for path in owned_runtime if path_policy_matches(policy, path)]
            if not matched_owned_runtime:
                errors.append(f"{policy_context} does not match any owned runtime files")
                previous_policies.append(policy)
                continue
            if previous_policies and all(
                any(path_policy_matches(previous_policy, path) for previous_policy in previous_policies)
                for path in matched_owned_runtime
            ):
                errors.append(
                    f"{policy_context} is unreachable because earlier path policies already match all owned runtime files"
                )
            previous_policies.append(policy)

        uncovered_owned_runtime = [
            path
            for path in owned_runtime
            if not any(path_policy_matches(policy, path) for _, policy in valid_policies)
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

    if subsystem_order != sorted_casefold(subsystem_order):
        errors.append("registry.json subsystems must be sorted by subsystem id")

    overlap_index: dict[str, list[str]] = {}
    for raw_subsystem in raw_subsystems:
        if not isinstance(raw_subsystem, dict):
            continue
        subsystem_id = raw_subsystem.get("id")
        if not isinstance(subsystem_id, str) or not subsystem_id.strip():
            continue
        for path in owned_runtime_files(raw_subsystem, tracked_files):
            overlap_index.setdefault(path, []).append(subsystem_id)
    actual_shared_ownership = {
        path: sorted_casefold(subsystems)
        for path, subsystems in overlap_index.items()
        if len(subsystems) > 1
    }

    seen_shared_paths: set[str] = set()
    declared_shared_paths: list[str] = []
    for index, raw_shared in enumerate(raw_shared_ownerships):
        context = f"shared_ownerships[{index}]"
        if not isinstance(raw_shared, dict):
            errors.append(f"{context} must be an object")
            continue
        for field in sorted(required_shared_ownership_fields):
            if field not in raw_shared:
                errors.append(f"{context} missing required field {field}")

        path = raw_shared.get("path")
        if not isinstance(path, str) or not path.strip():
            errors.append(f"{context}.path must be a non-empty string")
            continue
        declared_shared_paths.append(path)
        validate_path_reference(
            path,
            context=f"{context}.path",
            errors=errors,
            tracked_files=tracked_files,
        )
        if path in seen_shared_paths:
            errors.append(f"{context}.path duplicates shared ownership entry for {path!r}")
        seen_shared_paths.add(path)

        rationale = raw_shared.get("rationale")
        if not isinstance(rationale, str) or not rationale.strip():
            errors.append(f"{context}.rationale must be a non-empty string")

        subsystems = raw_shared.get("subsystems")
        normalized_subsystems: list[str] = []
        if not isinstance(subsystems, list):
            errors.append(f"{context}.subsystems must be a list")
            subsystems = []
        else:
            if len(subsystems) != len(set(subsystems)):
                errors.append(f"{context}.subsystems must not contain duplicates")
        for subsystem_index, subsystem_id in enumerate(subsystems):
            if not isinstance(subsystem_id, str) or not subsystem_id.strip():
                errors.append(f"{context}.subsystems[{subsystem_index}] must be a non-empty string")
                continue
            normalized_subsystems.append(subsystem_id)
            if subsystem_id not in seen_ids:
                errors.append(f"{context}.subsystems[{subsystem_index}] references unknown subsystem {subsystem_id!r}")
        if normalized_subsystems != sorted_casefold(normalized_subsystems):
            errors.append(f"{context}.subsystems must be sorted lexicographically")

        actual = actual_shared_ownership.get(path)
        if actual is None:
            errors.append(f"{context}.path = {path!r} is not an actual shared-owned runtime file")
        elif normalized_subsystems and normalized_subsystems != actual:
            errors.append(f"{context}.subsystems = {normalized_subsystems!r}, want {actual!r}")

    if declared_shared_paths != sorted_casefold(declared_shared_paths):
        errors.append("registry.json shared_ownerships must be sorted by path")

    declared_shared_ownership = set(declared_shared_paths)
    for path in sorted(set(actual_shared_ownership) - declared_shared_ownership, key=str.casefold):
        errors.append(
            f"registry.json missing shared ownership entry for {path!r} owned by {actual_shared_ownership[path]!r}"
        )
    for path in sorted(declared_shared_ownership - set(actual_shared_ownership), key=str.casefold):
        errors.append(f"registry.json shared ownership entry for {path!r} is stale")

    return {
        "errors": errors,
        "warnings": warnings,
        "summary": {
            "shared_ownership_count": len(actual_shared_ownership),
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
    parser = argparse.ArgumentParser(description="Audit the active release profile subsystem registry.")
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
    parser.add_argument(
        "--staged",
        action="store_true",
        help="Read registry control files from the git index instead of the working tree.",
    )
    return parser.parse_args(argv)


def render_pretty(report: dict[str, Any]) -> str:
    lines: list[str] = []
    summary = report.get("summary", {})
    if summary:
        lines.append(
            "summary: "
            f"subsystems={summary.get('subsystem_count', 0)} "
            f"shared_ownerships={summary.get('shared_ownership_count', 0)} "
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
    status_payload = load_status_payload(staged=args.staged)
    lane_ids = {
        lane.get("id")
        for lane in status_payload.get("lanes", [])
        if isinstance(lane, dict) and isinstance(lane.get("id"), str)
    }
    report = audit_registry_payload(
        load_registry_payload(staged=args.staged),
        tracked_files=tracked_repo_files(),
        status_lane_ids=lane_ids,
        schema_contract=registry_schema_contract(staged=args.staged),
    )
    output = render_pretty(report) if args.pretty else json.dumps(report, indent=2, sort_keys=True)
    print(output)
    if args.check and report["errors"]:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
