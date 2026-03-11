#!/usr/bin/env python3
"""Canonical subsystem completion guard.

Blocks commits when staged runtime changes touch a canonical subsystem but the
matching subsystem contract file and verification artifact are not staged in the
same commit.
"""

from __future__ import annotations

import json
from pathlib import Path
import subprocess
import sys
from typing import Dict, List, Sequence, Set


REPO_ROOT = Path(__file__).resolve().parents[2]
SUBSYSTEM_REGISTRY = REPO_ROOT / "docs" / "release-control" / "v6" / "subsystems" / "registry.json"

def load_subsystem_rules() -> List[dict]:
    with SUBSYSTEM_REGISTRY.open("r", encoding="utf-8") as handle:
        payload = json.load(handle)
    return list(payload.get("subsystems", []))

IGNORED_PREFIXES: tuple[str, ...] = (
    "docs/release-control/v6/",
    "internal/repoctl/",
    ".husky/",
    "scripts/release_control/",
)


def git_staged_files() -> List[str]:
    result = subprocess.run(
        ["git", "diff", "--cached", "--name-only", "--diff-filter=ACMR"],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    return [line.strip() for line in result.stdout.splitlines() if line.strip()]


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


def infer_impacted_subsystems(staged_files: Sequence[str]) -> Dict[str, dict]:
    impacted: Dict[str, dict] = {}
    rules = load_subsystem_rules()

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
                    "guardrail_files": list(rule.get("guardrail_files", [])),
                },
            )["touched_runtime_files"].append(path)

    return impacted


def staged_verification_files_for_subsystem(rule: dict, staged_files: Sequence[str]) -> List[str]:
    guardrail_files = set(rule.get("guardrail_files", []))
    matches: List[str] = []
    for path in staged_files:
        if path in guardrail_files:
            matches.append(path)
            continue
        if not is_test_or_fixture(path):
            continue
        if subsystem_matches_path(rule, path):
            matches.append(path)
    return sorted(set(matches))


def format_missing_requirements(
    missing_contracts: Dict[str, dict], missing_verification: Dict[str, dict]
) -> str:
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
        lines.append(f"- subsystem {subsystem_id}: missing verification artifact")
        for path in sorted(data["touched_runtime_files"]):
            lines.append(f"  touched by {path}")
        if data["guardrail_files"]:
            lines.append("  acceptable guardrail files include:")
            for path in sorted(data["guardrail_files"]):
                lines.append(f"    {path}")

    lines.extend(
        [
            "",
            "Rule:",
            "If a canonical subsystem changes, its contract under",
            "`docs/release-control/v6/subsystems/` must be updated in the same commit.",
            "Runtime subsystem changes must also stage at least one matching",
            "verification artifact: a subsystem guardrail, contract test,",
            "benchmark/SLO test, or subsystem-owned test/spec file.",
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
    missing_verification = {
        subsystem_id: data
        for subsystem_id, data in impacted.items()
        if not staged_verification_files_for_subsystem(data, staged_files)
    }
    if not missing_contracts and not missing_verification:
        return 0

    print(format_missing_requirements(missing_contracts, missing_verification), file=sys.stderr)
    return 1


def main(argv: Sequence[str] | None = None) -> int:
    _ = argv
    return check_staged_contracts(git_staged_files())


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
