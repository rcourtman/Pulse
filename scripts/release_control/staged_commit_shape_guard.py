#!/usr/bin/env python3
"""Preflight staged commit shape for governed runtime and promotion-proof slices."""

from __future__ import annotations

import sys
from typing import Sequence

from canonical_completion_guard import (
    format_missing_requirements,
    infer_impacted_subsystems,
    required_contract_updates,
    staged_contract_has_substantive_change,
    staged_verification_files_for_requirement,
)
from release_promotion_policy_support import (
    slice_requires_staged_governance_inputs,
    staged_governance_input_errors,
)


def git_staged_files() -> list[str]:
    from canonical_completion_guard import git_staged_files as canonical_git_staged_files

    return canonical_git_staged_files()


def canonical_commit_shape_errors(staged_files: Sequence[str]) -> list[str]:
    staged_set = set(staged_files)
    impacted = infer_impacted_subsystems(staged_files, use_staged_registry=True)
    required_contracts = required_contract_updates(
        staged_files,
        impacted,
        use_staged_contract_graph=True,
    )
    missing_contracts = {
        contract_path: data
        for contract_path, data in required_contracts.items()
        if contract_path not in staged_set
    }
    insufficient_contract_updates = {
        contract_path: data
        for contract_path, data in required_contracts.items()
        if contract_path in staged_set
        if not staged_contract_has_substantive_change(contract_path)
    }
    missing_verification: dict[str, dict] = {}
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

    if not missing_contracts and not insufficient_contract_updates and not missing_verification:
        return []

    return [
        format_missing_requirements(
            missing_contracts,
            insufficient_contract_updates,
            missing_verification,
        )
    ]


def staged_promotion_proof_errors(staged_files: Sequence[str]) -> list[str]:
    if not slice_requires_staged_governance_inputs(staged_files):
        return []
    errors = staged_governance_input_errors(use_staged_governance=True)
    if not errors:
        return []
    lines = ["BLOCKED: staged promotion proof inputs are incomplete.", "", "Required staged promotion inputs:"]
    for error in errors:
        error_lines = error.splitlines()
        lines.append(f"- {error_lines[0]}")
        for extra in error_lines[1:]:
            lines.append(f"  {extra}")
    return ["\n".join(lines)]


def format_combined_errors(errors: Sequence[str]) -> str:
    return "\n\n".join(error.rstrip() for error in errors if error.strip())


def main() -> int:
    staged_files = git_staged_files()
    errors = canonical_commit_shape_errors(staged_files)
    errors.extend(staged_promotion_proof_errors(staged_files))
    if not errors:
        print("Staged commit shape guard passed.")
        return 0

    print(format_combined_errors(errors), file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
