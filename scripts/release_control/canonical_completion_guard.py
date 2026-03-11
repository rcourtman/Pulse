#!/usr/bin/env python3
"""Canonical subsystem completion guard.

Blocks commits when staged runtime changes touch a canonical subsystem but the
matching subsystem contract file and required verification artifact are not
staged in the same commit.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import subprocess
import sys
from typing import Dict, List, Sequence, Set


REPO_ROOT = Path(__file__).resolve().parents[2]
SUBSYSTEM_REGISTRY = REPO_ROOT / "docs" / "release-control" / "v6" / "subsystems" / "registry.json"

REQUIRED_VERIFICATION_FIELDS: tuple[str, ...] = (
    "allow_same_subsystem_tests",
    "test_prefixes",
    "exact_files",
)


def validate_verification_policy(subsystem_id: str, policy: dict, *, context: str) -> None:
    if not isinstance(policy, dict):
        raise ValueError(f"subsystem {subsystem_id} {context} missing verification policy")
    for field in REQUIRED_VERIFICATION_FIELDS:
        if field not in policy:
            raise ValueError(f"subsystem {subsystem_id} {context} missing {field}")


def load_subsystem_rules() -> List[dict]:
    with SUBSYSTEM_REGISTRY.open("r", encoding="utf-8") as handle:
        payload = json.load(handle)
    rules = list(payload.get("subsystems", []))
    for rule in rules:
        subsystem_id = str(rule.get("id"))
        verification = rule.get("verification")
        validate_verification_policy(subsystem_id, verification, context="verification policy")
        path_policies = verification.get("path_policies", [])
        if not isinstance(path_policies, list):
            raise ValueError(f"subsystem {subsystem_id} verification policy path_policies must be a list")
        for index, policy in enumerate(path_policies):
            validate_verification_policy(
                subsystem_id,
                policy,
                context=f"path policy #{index}",
            )
            if "id" not in policy:
                raise ValueError(f"subsystem {subsystem_id} path policy #{index} missing id")
            if "match_prefixes" not in policy:
                raise ValueError(
                    f"subsystem {subsystem_id} path policy {policy['id']} missing match_prefixes"
                )
            if "match_files" not in policy:
                raise ValueError(
                    f"subsystem {subsystem_id} path policy {policy['id']} missing match_files"
                )
    return rules

IGNORED_PREFIXES: tuple[str, ...] = (
    "docs/release-control/v6/",
    "internal/repoctl/",
    ".husky/",
    "scripts/release_control/",
)


def git_staged_files() -> List[str]:
    result = subprocess.run(
        ["git", "diff", "--cached", "--name-only", "--diff-filter=ACMRD"],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    return [line.strip() for line in result.stdout.splitlines() if line.strip()]


def stdin_files(stdin: Sequence[str]) -> List[str]:
    return [line.strip() for line in stdin if line.strip()]


def is_test_or_fixture(path: str) -> bool:
    if path.endswith("_test.go"):
        return True
    if "/__tests__/" in path:
        return True
    if path.endswith(".test.ts") or path.endswith(".test.tsx"):
        return True
    if path.endswith(".spec.ts") or path.endswith(".spec.tsx"):
        return True
    return False


def is_ignored_runtime_file(path: str) -> bool:
    return any(path.startswith(prefix) for prefix in IGNORED_PREFIXES)


def subsystem_matches_path(rule: dict, path: str) -> bool:
    prefixes = tuple(rule.get("owned_prefixes", []))
    exact_files = tuple(rule.get("owned_files", []))
    return any(path.startswith(prefix) for prefix in prefixes) or path in exact_files


def path_policy_matches(policy: dict, path: str) -> bool:
    prefixes = tuple(policy.get("match_prefixes", []))
    exact_files = tuple(policy.get("match_files", []))
    return any(path.startswith(prefix) for prefix in prefixes) or path in exact_files


def normalize_verification_requirement(
    source: dict,
    *,
    requirement_id: str,
    label: str,
    touched_runtime_files: Sequence[str],
) -> dict:
    return {
        "id": requirement_id,
        "label": label,
        "touched_runtime_files": list(touched_runtime_files),
        "allow_same_subsystem_tests": bool(source.get("allow_same_subsystem_tests", False)),
        "test_prefixes": list(source.get("test_prefixes", [])),
        "exact_files": list(source.get("exact_files", [])),
    }


def build_verification_requirements(rule: dict, touched_runtime_files: Sequence[str]) -> List[dict]:
    verification = dict(rule.get("verification", {}))
    path_policies = list(verification.get("path_policies", []))
    requirements: List[dict] = []
    matched_by_policy: Dict[str, dict] = {}
    unmatched_runtime_files: List[str] = []

    for path in touched_runtime_files:
        matching_policy = next((policy for policy in path_policies if path_policy_matches(policy, path)), None)
        if matching_policy is None:
            unmatched_runtime_files.append(path)
            continue

        policy_id = str(matching_policy["id"])
        requirement = matched_by_policy.get(policy_id)
        if requirement is None:
            requirement = normalize_verification_requirement(
                matching_policy,
                requirement_id=policy_id,
                label=str(matching_policy.get("label", policy_id)),
                touched_runtime_files=[],
            )
            matched_by_policy[policy_id] = requirement
            requirements.append(requirement)
        requirement["touched_runtime_files"].append(path)

    if unmatched_runtime_files:
        requirements.insert(
            0,
            normalize_verification_requirement(
                verification,
                requirement_id="default",
                label="default subsystem verification",
                touched_runtime_files=unmatched_runtime_files,
            ),
        )

    return requirements


def infer_impacted_subsystems(staged_files: Sequence[str]) -> Dict[str, dict]:
    impacted: Dict[str, dict] = {}
    rules = load_subsystem_rules()
    rules_by_id = {str(rule["id"]): rule for rule in rules}

    for path in staged_files:
        if is_ignored_runtime_file(path) or is_test_or_fixture(path):
            continue

        for rule in rules:
            if not subsystem_matches_path(rule, path):
                continue
            impacted.setdefault(
                str(rule["id"]),
                {
                    "id": str(rule["id"]),
                    "contract": str(rule["contract"]),
                    "touched_runtime_files": [],
                    "verification": dict(rule.get("verification", {})),
                },
            )["touched_runtime_files"].append(path)

    for subsystem_id, data in impacted.items():
        data["verification_requirements"] = build_verification_requirements(
            rules_by_id[subsystem_id],
            data["touched_runtime_files"],
        )

    return impacted


def staged_verification_files_for_requirement(rule: dict, requirement: dict, staged_files: Sequence[str]) -> List[str]:
    exact_files = set(requirement.get("exact_files", []))
    test_prefixes = tuple(requirement.get("test_prefixes", []))
    allow_same_subsystem_tests = bool(requirement.get("allow_same_subsystem_tests", False))
    matches: List[str] = []
    for path in staged_files:
        if path in exact_files:
            matches.append(path)
            continue
        if not is_test_or_fixture(path):
            continue
        if any(path.startswith(prefix) for prefix in test_prefixes):
            matches.append(path)
            continue
        if allow_same_subsystem_tests and subsystem_matches_path(rule, path):
            matches.append(path)
    return sorted(set(matches))


def format_missing_requirements(missing_contracts: Dict[str, dict], missing_verification: Dict[str, dict]) -> str:
    lines = [
        "BLOCKED: canonical subsystem changes require matching contract and verification updates.",
        "",
        "Stage the required subsystem file(s) in the same commit:",
    ]

    for subsystem_id, data in sorted(missing_contracts.items()):
        lines.append(f"- subsystem {subsystem_id}: missing contract {data['contract']}")
        for path in sorted(data["touched_runtime_files"]):
            lines.append(f"  touched by {path}")

    for subsystem_id, data in sorted(missing_verification.items()):
        for requirement in data["missing_requirements"]:
            lines.append(
                f"- subsystem {subsystem_id}: missing verification artifact for {requirement['label']}"
            )
            for path in sorted(requirement["touched_runtime_files"]):
                lines.append(f"  touched by {path}")
            exact_files = sorted(requirement.get("exact_files", []))
            test_prefixes = sorted(requirement.get("test_prefixes", []))
            allow_same_subsystem_tests = bool(requirement.get("allow_same_subsystem_tests", False))
            if exact_files:
                lines.append("  acceptable exact proof files include:")
                for path in exact_files:
                    lines.append(f"    {path}")
            if test_prefixes:
                lines.append("  acceptable staged test prefixes include:")
                for prefix in test_prefixes:
                    lines.append(f"    {prefix}")
            if allow_same_subsystem_tests:
                lines.append("  same-subsystem test/spec files are also accepted")

    lines.extend(
        [
            "",
            "Rule:",
            "If a canonical subsystem changes, its contract under",
            "`docs/release-control/v6/subsystems/` must be updated in the same commit.",
            "Each touched runtime path must also satisfy the first matching",
            "verification policy from that subsystem's registry entry.",
        ]
    )
    return "\n".join(lines)


def check_staged_contracts(staged_files: Sequence[str]) -> int:
    staged_set: Set[str] = set(staged_files)
    impacted = infer_impacted_subsystems(staged_files)
    missing_contracts = {
        subsystem_id: data
        for subsystem_id, data in impacted.items()
        if data["contract"] not in staged_set
    }
    missing_verification: Dict[str, dict] = {}
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
    if not missing_contracts and not missing_verification:
        return 0

    print(format_missing_requirements(missing_contracts, missing_verification), file=sys.stderr)
    return 1


def parse_args(argv: Sequence[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Enforce canonical subsystem contract and proof-of-change updates for "
            "staged files or an explicit changed-file list."
        )
    )
    parser.add_argument(
        "--files-from-stdin",
        action="store_true",
        help="Read newline-delimited changed files from standard input instead of git staged files.",
    )
    return parser.parse_args(list(argv))


def main(argv: Sequence[str] | None = None) -> int:
    args = parse_args(list(argv or ()))
    if args.files_from_stdin:
        return check_staged_contracts(stdin_files(sys.stdin))
    return check_staged_contracts(git_staged_files())


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
