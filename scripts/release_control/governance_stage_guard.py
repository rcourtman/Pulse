#!/usr/bin/env python3
"""Block partially staged governance files that local pre-commit reads from the working tree."""

from __future__ import annotations

import subprocess
import sys

from repo_file_io import REPO_ROOT


WORKTREE_SENSITIVE_PREFIXES = (
    "docs/release-control/v6/",
    "internal/repoctl/",
    "scripts/release_control/",
)
WORKTREE_SENSITIVE_EXACT_FILES = (
    ".github/workflows/canonical-governance.yml",
    ".husky/pre-commit",
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


def blocked_partially_staged_paths(staged_paths: list[str], unstaged_paths: list[str]) -> list[str]:
    partially_staged = set(staged_paths) & set(unstaged_paths)
    return sorted(
        (path for path in partially_staged if is_worktree_sensitive_governance_path(path)),
        key=str.casefold,
    )


def partially_staged_governance_paths() -> list[str]:
    staged_paths = git_diff_name_only("--cached", "--name-only", "-z", "--diff-filter=ACMRT")
    unstaged_paths = git_diff_name_only("--name-only", "-z", "--diff-filter=ACMRT")
    return blocked_partially_staged_paths(staged_paths, unstaged_paths)


def main() -> int:
    blocked_paths = partially_staged_governance_paths()
    if not blocked_paths:
        print("Governance stage guard passed.")
        return 0

    print("BLOCKED: partially staged governance files detected.")
    print(
        "Local pre-commit executes or structurally validates these files from the working tree,"
    )
    print("so partial staging can make local validation disagree with the commit.")
    print("Stage or unstage each affected file completely before committing:")
    for path in blocked_paths:
        print(f"  - {path}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
