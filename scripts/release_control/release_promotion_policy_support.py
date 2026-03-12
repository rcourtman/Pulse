#!/usr/bin/env python3
"""Shared helpers for RC-to-GA promotion proof governance."""

from __future__ import annotations

from typing import Sequence

from repo_file_io import missing_staged_repo_paths, read_repo_text


PROMOTION_PROOF_TRIGGER_PATHS: tuple[str, ...] = (
    ".github/workflows/release-dry-run.yml",
    "docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md",
    "docs/release-control/v6/PRE_RELEASE_CHECKLIST.md",
    "docs/release-control/v6/RC_TO_GA_REHEARSAL_TEMPLATE.md",
    "docs/release-control/v6/SOURCE_OF_TRUTH.md",
    "docs/releases/V6_PRERELEASE_RUNBOOK.md",
    "scripts/trigger-release.sh",
)


REQUIRED_STAGED_GOVERNANCE_INPUTS: tuple[str, ...] = (
    "docs/release-control/v6/PRE_RELEASE_CHECKLIST.md",
    "docs/release-control/v6/RC_TO_GA_REHEARSAL_TEMPLATE.md",
)


def slice_requires_staged_governance_inputs(staged_files: Sequence[str]) -> bool:
    staged_set = set(staged_files)
    return any(path in staged_set for path in PROMOTION_PROOF_TRIGGER_PATHS)


def staged_governance_input_errors(*, use_staged_governance: bool) -> list[str]:
    if not use_staged_governance:
        return []

    errors: list[str] = []
    missing = missing_staged_repo_paths(REQUIRED_STAGED_GOVERNANCE_INPUTS)
    if missing:
        errors.append(
            "stage the canonical promotion proof inputs:\n- " + "\n- ".join(missing)
        )

    checklist_rel = "docs/release-control/v6/PRE_RELEASE_CHECKLIST.md"
    if checklist_rel not in missing:
        checklist = read_repo_text(
            checklist_rel,
            staged=True,
            strict_staged=True,
        )
        if "rc-to-ga-rehearsal-summary" not in checklist:
            errors.append(
                "stage the updated docs/release-control/v6/PRE_RELEASE_CHECKLIST.md "
                "that records the rc-to-ga-rehearsal-summary gate input"
            )
    else:
        errors.append(
            "stage the updated docs/release-control/v6/PRE_RELEASE_CHECKLIST.md "
            "that records the rc-to-ga-rehearsal-summary gate input"
        )

    return errors
