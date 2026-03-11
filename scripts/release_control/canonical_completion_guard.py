#!/usr/bin/env python3
"""Canonical subsystem completion guard.

Blocks commits when staged source changes touch a canonical subsystem but the
matching subsystem contract file is not staged in the same commit.
"""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
import subprocess
import sys
from typing import Dict, Iterable, List, Sequence, Set


REPO_ROOT = Path(__file__).resolve().parents[2]


@dataclass(frozen=True)
class SubsystemRule:
    name: str
    contract: str
    prefixes: tuple[str, ...] = ()
    exact_files: tuple[str, ...] = ()


SUBSYSTEM_RULES: tuple[SubsystemRule, ...] = (
    SubsystemRule(
        name="alerts",
        contract="docs/release-control/v6/subsystems/alerts.md",
        prefixes=(
            "internal/alerts/",
            "frontend-modern/src/features/alerts/",
            "frontend-modern/src/components/Alerts/",
        ),
        exact_files=("frontend-modern/src/pages/Alerts.tsx",),
    ),
    SubsystemRule(
        name="monitoring",
        contract="docs/release-control/v6/subsystems/monitoring.md",
        prefixes=("internal/monitoring/",),
    ),
    SubsystemRule(
        name="unified-resources",
        contract="docs/release-control/v6/subsystems/unified-resources.md",
        prefixes=("internal/unifiedresources/",),
        exact_files=(
            "internal/api/resources.go",
            "frontend-modern/src/types/resource.ts",
            "frontend-modern/src/hooks/useUnifiedResources.ts",
            "frontend-modern/src/pages/Infrastructure.tsx",
            "frontend-modern/src/hooks/useDashboardTrends.ts",
            "frontend-modern/src/routing/resourceLinks.ts",
        ),
    ),
    SubsystemRule(
        name="cloud-paid",
        contract="docs/release-control/v6/subsystems/cloud-paid.md",
        prefixes=(
            "pkg/licensing/",
            "internal/api/licensing_",
            "internal/api/payments_",
            "internal/api/stripe_",
        ),
        exact_files=(
            "frontend-modern/src/pages/CloudPricing.tsx",
            "frontend-modern/src/pages/HostedSignup.tsx",
        ),
    ),
    SubsystemRule(
        name="api-contracts",
        contract="docs/release-control/v6/subsystems/api-contracts.md",
        prefixes=(
            "internal/api/",
            "frontend-modern/src/api/",
        ),
        exact_files=("frontend-modern/src/types/api.ts",),
    ),
    SubsystemRule(
        name="frontend-primitives",
        contract="docs/release-control/v6/subsystems/frontend-primitives.md",
        prefixes=("frontend-modern/src/components/shared/",),
    ),
    SubsystemRule(
        name="performance-and-scalability",
        contract="docs/release-control/v6/subsystems/performance-and-scalability.md",
        prefixes=("pkg/metrics/",),
        exact_files=(
            "internal/api/slo.go",
            "internal/api/slo_bench_test.go",
            "internal/api/router_bench_test.go",
        ),
    ),
)


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

    for path in staged_files:
        if is_ignored_runtime_file(path) or is_test_or_fixture(path):
            continue

        for rule in SUBSYSTEM_RULES:
            matches_prefix = any(path.startswith(prefix) for prefix in rule.prefixes)
            matches_exact = path in rule.exact_files
            if not matches_prefix and not matches_exact:
                continue
            required.setdefault(rule.contract, []).append(path)

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
