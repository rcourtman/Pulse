#!/usr/bin/env python3
"""Run pre-commit against a proposed commit slice in an isolated git index."""

from __future__ import annotations

import argparse
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from typing import Iterable


REPO_ROOT = Path(__file__).resolve().parents[2]
HOOK_PATH = REPO_ROOT / ".husky" / "pre-commit"


def git_env(index_path: Path) -> dict[str, str]:
    env = os.environ.copy()
    env["GIT_INDEX_FILE"] = str(index_path)
    return env


def repo_relative_path(path: str | Path) -> str:
    candidate = Path(path)
    if candidate.is_absolute():
        candidate = candidate.relative_to(REPO_ROOT)
    return candidate.as_posix()


def git(index_path: Path, *args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=REPO_ROOT,
        check=check,
        capture_output=True,
        text=True,
        env=git_env(index_path),
    )


def stage_paths(index_path: Path, paths: Iterable[str]) -> None:
    normalized = [repo_relative_path(path) for path in paths]
    if not normalized:
        return
    git(index_path, "add", "-f", "--", *normalized)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Verify a proposed commit slice by running .husky/pre-commit in a copied index."
    )
    parser.add_argument(
        "--add-updated",
        action="store_true",
        help="Stage all tracked modifications (`git add -u`) into the copied index before explicit paths.",
    )
    parser.add_argument(
        "--show-staged",
        action="store_true",
        help="Print the staged file list in the copied index before running the hook.",
    )
    parser.add_argument(
        "paths",
        nargs="*",
        help="Explicit repo-relative or absolute paths to stage into the copied index.",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(list(argv or sys.argv[1:]))
    with tempfile.NamedTemporaryFile(prefix="pulse-index.", delete=True) as tmp_index:
        index_path = Path(tmp_index.name)
        git(index_path, "read-tree", "HEAD")
        if args.add_updated:
            git(index_path, "add", "-u")
        stage_paths(index_path, args.paths)

        if args.show_staged:
            staged = git(index_path, "diff", "--cached", "--name-only").stdout.strip()
            if staged:
                print(staged)

        result = subprocess.run(
            [str(HOOK_PATH)],
            cwd=REPO_ROOT,
            env=git_env(index_path),
            text=True,
        )
        return result.returncode


if __name__ == "__main__":
    sys.exit(main())
