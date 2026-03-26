#!/usr/bin/env python3
"""Land an isolated worktree's finished slice onto the base branch worktree."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import subprocess
import sys
from typing import Any

from repo_file_io import REPO_ROOT
from worktree_base import canonical_base_worktree_path
from worktree_claim import list_worktrees


def git(*args: str, cwd: Path, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=cwd,
        check=check,
        capture_output=True,
        text=True,
    )


def current_branch(*, repo_root: Path) -> str:
    return git("branch", "--show-current", cwd=repo_root).stdout.strip()


def current_head(*, repo_root: Path) -> str:
    return git("rev-parse", "HEAD", cwd=repo_root).stdout.strip()


def is_clean_worktree(*, repo_root: Path) -> bool:
    return not git("status", "--porcelain", cwd=repo_root).stdout.strip()


def branch_worktree_path(*, repo_root: Path, branch_name: str, preferred_path: Path | None = None) -> Path | None:
    preferred = preferred_path.resolve() if preferred_path is not None else None
    fallback: Path | None = None
    for entry in list_worktrees(repo_root=repo_root):
        if entry.get("branch") != f"refs/heads/{branch_name}":
            continue
        candidate = Path(entry["worktree"]).resolve()
        if preferred is not None and candidate == preferred:
            return candidate
        if fallback is None:
            fallback = candidate
    return fallback


def commits_ahead_of_base(*, repo_root: Path, base_branch: str) -> list[str]:
    output = git("rev-list", "--reverse", f"{base_branch}..HEAD", cwd=repo_root).stdout.strip()
    return [line for line in output.splitlines() if line.strip()]


def cherry_pick_commits(*, base_worktree: Path, commits: list[str]) -> None:
    for commit in commits:
        subprocess.run(["git", "cherry-pick", commit], cwd=base_worktree, check=True)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Land an isolated worktree's finished slice onto the base branch worktree."
    )
    parser.add_argument(
        "--base-branch",
        default="pulse/v6",
        help="Base branch that receives the finished slice. Defaults to pulse/v6.",
    )
    parser.add_argument(
        "--write",
        action="store_true",
        help="Cherry-pick the finished commit(s) onto the base branch worktree when validation passes.",
    )
    parser.add_argument(
        "--pretty",
        action="store_true",
        help="Render a concise human summary instead of JSON.",
    )
    return parser.parse_args(argv)


def render_pretty(
    *,
    current_worktree: Path,
    current_branch_name: str,
    base_branch: str,
    base_worktree: Path | None,
    commits: list[str],
    errors: list[str],
    wrote: bool,
) -> str:
    lines = [
        "worktree_finish:",
        f"  current_worktree={current_worktree}",
        f"  current_branch={current_branch_name}",
        f"  base_branch={base_branch}",
        f"  base_worktree={base_worktree if base_worktree is not None else 'missing'}",
        f"  commit_count={len(commits)}",
        f"  wrote={'yes' if wrote else 'no'}",
    ]
    if commits:
        lines.append("commits:")
        for commit in commits:
            lines.append(f"  - {commit}")
    if errors:
        lines.append("errors:")
        for error in errors:
            lines.append(f"  - {error}")
    else:
        lines.append("status: ready")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(list(argv or []))
    errors: list[str] = []
    current_worktree = REPO_ROOT.resolve()
    current_branch_name = current_branch(repo_root=current_worktree)
    preferred_base = canonical_base_worktree_path(repo_root=current_worktree, branch_name=args.base_branch)
    base_worktree = branch_worktree_path(
        repo_root=current_worktree, branch_name=args.base_branch, preferred_path=preferred_base
    )

    if not current_branch_name:
        errors.append("current worktree is not on a branch")
    if current_branch_name == args.base_branch:
        errors.append("current worktree is already on the base branch")
    if base_worktree is None:
        errors.append(
            f"no canonical landing worktree found for base branch {args.base_branch}; run worktree_base.py first"
        )
    elif base_worktree == current_worktree:
        errors.append("base branch worktree resolves to the current worktree")
    elif base_worktree != preferred_base.resolve():
        errors.append(
            f"base branch is checked out at {base_worktree}, not at canonical landing path {preferred_base.resolve()}"
        )

    commits: list[str] = []
    if not errors:
        commits = commits_ahead_of_base(repo_root=current_worktree, base_branch=args.base_branch)
        if not commits:
            errors.append(f"no commits ahead of {args.base_branch} to land")
        if not is_clean_worktree(repo_root=current_worktree):
            errors.append("current worktree has uncommitted changes")
        if base_worktree is not None and not is_clean_worktree(repo_root=base_worktree):
            errors.append(f"base worktree is dirty: {base_worktree}")

    wrote = False
    if not errors and args.write and base_worktree is not None:
        cherry_pick_commits(base_worktree=base_worktree, commits=commits)
        wrote = True

    if args.pretty:
        print(
            render_pretty(
                current_worktree=current_worktree,
                current_branch_name=current_branch_name,
                base_branch=args.base_branch,
                base_worktree=base_worktree,
                commits=commits,
                errors=errors,
                wrote=wrote,
            )
        )
    else:
        print(
            json.dumps(
                {
                    "current_worktree": str(current_worktree),
                    "current_branch": current_branch_name,
                    "base_branch": args.base_branch,
                    "base_worktree": str(base_worktree) if base_worktree is not None else None,
                    "commits": commits,
                    "errors": errors,
                    "wrote": wrote,
                },
                indent=2,
                sort_keys=True,
            )
        )
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
