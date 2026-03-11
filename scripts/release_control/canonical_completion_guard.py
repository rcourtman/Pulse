#!/usr/bin/env python3
"""Canonical subsystem completion guard.

Blocks commits when staged source changes touch a canonical subsystem but the
matching subsystem contract file is not staged in the same commit.
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


def infer_required_contracts(staged_files: Sequence[str]) -> Dict[str, List[str]]:
    required: Dict[str, List[str]] = {}
    rules = load_subsystem_rules()

    for path in staged_files:
        if is_ignored_runtime_file(path) or is_test_or_fixture(path):
            continue

        for rule in rules:
            prefixes = tuple(rule.get("owned_prefixes", []))
            exact_files = tuple(rule.get("owned_files", []))
            contract = str(rule["contract"])
            matches_prefix = any(path.startswith(prefix) for prefix in prefixes)
            matches_exact = path in exact_files
            if not matches_prefix and not matches_exact:
                continue
            required.setdefault(contract, []).append(path)

    return required


def format_missing_contracts(missing: Dict[str, List[str]]) -> str:
    lines = [
        "BLOCKED: canonical subsystem changes require matching contract updates.",
        "",
        "Stage the required subsystem contract file(s) in the same commit:",
    ]

    for contract, files in sorted(missing.items()):
        lines.append(f"- missing {contract}")
        for path in sorted(files):
            lines.append(f"  touched by {path}")

    lines.extend(
        [
            "",
            "Rule:",
            "If a canonical subsystem changes, its contract under",
            "`docs/release-control/v6/subsystems/` must be updated in the same commit.",
            "If the change truly does not affect the contract, add a brief no-op update",
            "to the contract's Current State or Completion Obligations section.",
        ]
    )
    return "\n".join(lines)


def check_staged_contracts(staged_files: Sequence[str]) -> int:
    staged_set: Set[str] = set(staged_files)
    required = infer_required_contracts(staged_files)
    missing = {
        contract: files
        for contract, files in required.items()
        if contract not in staged_set
    }
    if not missing:
        return 0

    print(format_missing_contracts(missing), file=sys.stderr)
    return 1


def main(argv: Sequence[str] | None = None) -> int:
    _ = argv
    return check_staged_contracts(git_staged_files())


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
