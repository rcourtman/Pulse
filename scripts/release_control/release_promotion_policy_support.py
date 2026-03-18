#!/usr/bin/env python3
"""Shared helpers for RC-to-GA promotion proof governance."""

from __future__ import annotations

import subprocess
from typing import Sequence

from repo_file_io import REPO_ROOT, git_env, missing_staged_repo_paths, read_repo_text


PROMOTION_PROOF_TRIGGER_PATHS: tuple[str, ...] = (
    ".github/workflows/release-dry-run.yml",
    "docs/release-control/v6/internal/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md",
    "docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md",
    "docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md",
    "docs/release-control/v6/internal/SOURCE_OF_TRUTH.md",
    "docs/releases/V6_PRERELEASE_RUNBOOK.md",
    "scripts/check-workflow-dispatch-inputs.py",
    "scripts/trigger-release.sh",
    "scripts/trigger-release-dry-run.sh",
)


REQUIRED_STAGED_GOVERNANCE_INPUTS: tuple[str, ...] = (
    "docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md",
    "docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md",
)


PROMOTION_METADATA_FIELDS: tuple[tuple[str, str], ...] = (
    ("tag", "candidate stable tag"),
    ("channel_under_rehearsal", "promotion channel"),
    ("promoted_from_rc", "promoted RC tag"),
    ("rollback_target", "rollback target"),
    ("rollback_command", "exact rollback command"),
    ("planned_ga_date", "planned GA date"),
    ("planned_v5_eos_date", "planned v5 end-of-support date"),
)


def promotion_metadata_labels() -> tuple[str, ...]:
    return tuple(label for _, label in PROMOTION_METADATA_FIELDS)


def promotion_metadata_envelope() -> str:
    return ", ".join(promotion_metadata_labels()[:-1]) + ", and " + promotion_metadata_labels()[-1]


def _run_git(*args: str) -> str:
    result = subprocess.run(
        ["git", *args],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=True,
        env=git_env(),
    )
    return result.stdout.strip()


def _run_git_optional(*args: str) -> str | None:
    try:
        return _run_git(*args)
    except subprocess.CalledProcessError:
        return None


def origin_default_branch() -> str:
    symbolic_ref = _run_git_optional("symbolic-ref", "refs/remotes/origin/HEAD")
    if not symbolic_ref:
        return "main"
    prefix = "refs/remotes/origin/"
    if symbolic_ref.startswith(prefix):
        return symbolic_ref[len(prefix):]
    return "main"


def branch_workflow_text(branch: str, workflow_path: str) -> str:
    workflow = _run_git_optional("show", f"origin/{branch}:{workflow_path}")
    if not workflow:
        raise ValueError(
            f"default branch '{branch}' does not contain workflow '{workflow_path}'"
        )
    return workflow


def parse_workflow_dispatch_inputs(content: str) -> tuple[str, ...]:
    lines = content.splitlines()
    in_workflow_dispatch = False
    dispatch_indent = -1
    in_inputs = False
    inputs_indent = -1
    inputs: list[str] = []

    for raw_line in lines:
        stripped = raw_line.strip()
        indent = len(raw_line) - len(raw_line.lstrip(" "))

        if not stripped or stripped.startswith("#"):
            continue

        if not in_workflow_dispatch:
            if stripped == "workflow_dispatch:":
                in_workflow_dispatch = True
                dispatch_indent = indent
            continue

        if in_workflow_dispatch and not in_inputs:
            if indent <= dispatch_indent and stripped.endswith(":"):
                in_workflow_dispatch = False
                continue
            if stripped == "inputs:":
                in_inputs = True
                inputs_indent = indent
            continue

        if in_inputs:
            if indent <= inputs_indent:
                break
            if (
                stripped.endswith(":")
                and not stripped.startswith("- ")
                and ":" not in stripped[:-1]
            ):
                inputs.append(stripped[:-1])

    return tuple(inputs)


def missing_workflow_dispatch_inputs(
    *, workflow_path: str, required_inputs: Sequence[str], branch: str | None = None
) -> tuple[str, tuple[str, ...]]:
    resolved_branch = branch or origin_default_branch()
    workflow = branch_workflow_text(resolved_branch, workflow_path)
    actual_inputs = set(parse_workflow_dispatch_inputs(workflow))
    missing = tuple(name for name in required_inputs if name not in actual_inputs)
    return resolved_branch, missing


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

    checklist_rel = "docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md"
    if checklist_rel not in missing:
        checklist = read_repo_text(
            checklist_rel,
            staged=True,
            strict_staged=True,
        )
        if "rc-to-ga-rehearsal-summary" not in checklist:
            errors.append(
                "stage the updated docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md "
                "that records the rc-to-ga-rehearsal-summary gate input"
            )
        if (
            promotion_metadata_envelope() not in checklist
            or "v5 end-of-support date" not in checklist
        ):
            errors.append(
                "stage the updated docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md "
                "that records the canonical promotion metadata envelope"
            )
    else:
        errors.append(
            "stage the updated docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md "
            "that records the rc-to-ga-rehearsal-summary gate input"
        )
        errors.append(
            "stage the updated docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md "
            "that records the canonical promotion metadata envelope"
        )

    template_rel = "docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md"
    if template_rel not in missing:
        template = read_repo_text(
            template_rel,
            staged=True,
            strict_staged=True,
        )
        if (
            "Exact rollback or reinstall command" not in template
            or "rc-to-ga-rehearsal-summary" not in template
            or "promotion channel, promoted RC tag, rollback target, exact rollback command" not in template
        ):
            errors.append(
                "stage the updated docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md "
                "that preserves the artifact-backed promotion metadata record shape"
            )
    else:
        errors.append(
            "stage the updated docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md "
            "that preserves the artifact-backed promotion metadata record shape"
        )

    return errors
