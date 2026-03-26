#!/usr/bin/env python3
"""Ensure the canonical clean landing worktree exists for a base branch."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import subprocess
import sys

from repo_file_io import REPO_ROOT
from worktree_claim import WORKTREES_ROOT, list_worktrees


def git(*args: str, cwd: Path, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=cwd,
        check=check,
        capture_output=True,
        text=True,
    )


def base_slug(branch_name: str) -> str:
    return "base__" + branch_name.replace("/", "__")


def canonical_base_worktree_path(*, repo_root: Path, branch_name: str) -> Path:
    return WORKTREES_ROOT / repo_root.name / base_slug(branch_name)


def find_worktree_by_path(*, repo_root: Path, path: Path) -> dict[str, str] | None:
    target = path.resolve()
    for entry in list_worktrees(repo_root=repo_root):
        if Path(entry.get("worktree", "")).resolve() == target:
            return entry
    return None


def is_clean_worktree(*, repo_root: Path) -> bool:
    return not git("status", "--porcelain", cwd=repo_root).stdout.strip()


def create_base_worktree(*, repo_root: Path, branch_name: str, path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run(
        ["git", "worktree", "add", str(path), branch_name],
        cwd=repo_root,
        check=True,
    )


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Ensure the canonical clean landing worktree exists for a base branch."
    )
    parser.add_argument(
        "--base-branch",
        default="pulse/v6",
        help="Base branch that should own the canonical landing worktree. Defaults to pulse/v6.",
    )
    parser.add_argument(
        "--write",
        action="store_true",
        help="Create the canonical landing worktree if it does not exist and validation passes.",
    )
    parser.add_argument(
        "--pretty",
        action="store_true",
        help="Render a concise human summary instead of JSON.",
    )
    return parser.parse_args(argv)


def render_pretty(*, branch_name: str, path: Path, existed: bool, clean: bool, errors: list[str], wrote: bool) -> str:
    lines = [
        "worktree_base:",
        f"  base_branch={branch_name}",
        f"  path={path}",
        f"  existed={'yes' if existed else 'no'}",
        f"  clean={'yes' if clean else 'no'}",
        f"  wrote={'yes' if wrote else 'no'}",
    ]
    if errors:
        lines.append("errors:")
        for error in errors:
            lines.append(f"  - {error}")
    else:
        lines.append("status: ready")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(list(argv or []))
    path = canonical_base_worktree_path(repo_root=REPO_ROOT, branch_name=args.base_branch)
    entry = find_worktree_by_path(repo_root=REPO_ROOT, path=path)
    errors: list[str] = []
    existed = entry is not None
    clean = False

    if entry is not None:
        branch_ref = entry.get("branch", "")
        if branch_ref != f"refs/heads/{args.base_branch}":
            errors.append(f"canonical landing worktree is not on {args.base_branch}: {branch_ref or 'detached'}")
        clean = is_clean_worktree(repo_root=path)
        if not clean:
            errors.append(f"canonical landing worktree is dirty: {path}")
    elif path.exists():
        errors.append(f"canonical landing path exists on disk but is not a registered worktree: {path}")

    wrote = False
    if not errors and not existed and args.write:
        create_base_worktree(repo_root=REPO_ROOT, branch_name=args.base_branch, path=path)
        wrote = True
        existed = True
        clean = is_clean_worktree(repo_root=path)

    if args.pretty:
        print(render_pretty(branch_name=args.base_branch, path=path, existed=existed, clean=clean, errors=errors, wrote=wrote))
    else:
        print(
            json.dumps(
                {
                    "base_branch": args.base_branch,
                    "path": str(path),
                    "existed": existed,
                    "clean": clean,
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
