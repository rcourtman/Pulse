#!/usr/bin/env python3
"""Block dirty governance files that local pre-commit reads from the working tree."""

from __future__ import annotations

import subprocess
import sys

from control_plane import CONTROL_PLANE_DOC_REL, CONTROL_PLANE_REL, CONTROL_PLANE_SCHEMA_REL, DEFAULT_CONTROL_PLANE
from repo_file_io import REPO_ROOT


WORKTREE_SENSITIVE_PREFIXES = (
    DEFAULT_CONTROL_PLANE["profile_root_rel"] + "/",
    "internal/repoctl/",
    "scripts/release_control/",
)
WORKTREE_SENSITIVE_EXACT_FILES = (
    ".github/workflows/canonical-governance.yml",
    ".husky/pre-commit",
    CONTROL_PLANE_DOC_REL,
    CONTROL_PLANE_REL,
    CONTROL_PLANE_SCHEMA_REL,
)


def git_diff_name_only(*args: str) -> list[str]:
    result = subprocess.run(
        ["git", "diff", *args],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=False,
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
