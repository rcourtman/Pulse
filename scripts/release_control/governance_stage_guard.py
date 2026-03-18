#!/usr/bin/env python3
"""Block dirty governance files that local pre-commit reads from the working tree."""

from __future__ import annotations

import os
import subprocess
import sys

from control_plane import CONTROL_PLANE_REL
from repo_file_io import REPO_ROOT


WORKTREE_SENSITIVE_PREFIXES = (
    "internal/repoctl/",
    "scripts/release_control/",
)
WORKTREE_SENSITIVE_EXACT_FILES = (
    ".husky/pre-commit",
    CONTROL_PLANE_REL,
)
STAGED_EXECUTION_EXACT_FILES = (
    "scripts/release_control/release_promotion_policy_test.py",
)


DEFAULT_REPO_ROOT = REPO_ROOT


def git_env() -> dict[str, str]:
    env = os.environ.copy()
    if REPO_ROOT != DEFAULT_REPO_ROOT:
        env.pop("GIT_INDEX_FILE", None)
    return env


def git_diff_name_only(*args: str) -> list[str]:
    result = subprocess.run(
        ["git", "diff", *args],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=False,
        env=git_env(),
    )
    return sorted(
        {
            entry.decode("utf-8")
            for entry in result.stdout.split(b"\x00")
            if entry
        },
        key=str.casefold,
    )


def is_worktree_sensitive_governance_path(path: str) -> bool:
    if path in STAGED_EXECUTION_EXACT_FILES:
        return False
    if path in WORKTREE_SENSITIVE_EXACT_FILES:
        return True
    return any(path.startswith(prefix) for prefix in WORKTREE_SENSITIVE_PREFIXES)


def blocked_unstaged_governance_paths(unstaged_paths: list[str]) -> list[str]:
    return sorted(
        (path for path in unstaged_paths if is_worktree_sensitive_governance_path(path)),
        key=str.casefold,
    )


def unstaged_governance_paths() -> list[str]:
    unstaged_paths = git_diff_name_only("--name-only", "-z", "--diff-filter=ACMRT")
    return blocked_unstaged_governance_paths(unstaged_paths)


def main() -> int:
    blocked_paths = unstaged_governance_paths()
    if not blocked_paths:
        print("Governance stage guard passed.")
        return 0

    print("BLOCKED: unstaged governance file changes detected.")
    print(
        "Local pre-commit executes or structurally validates these files from the working tree,"
    )
    print("so unstaged changes can make local validation disagree with the commit.")
    print("Stage or revert each affected file before committing:")
    for path in blocked_paths:
        print(f"  - {path}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
