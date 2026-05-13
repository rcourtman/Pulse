#!/usr/bin/env python3
"""Helpers for reading repo files from either the working tree or the git index."""

from __future__ import annotations

import json
import os
from pathlib import Path
import subprocess
from typing import Any, Iterable


REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_REPO_ROOT = REPO_ROOT


def git_env(repo_root: Path | None = None) -> dict[str, str]:
    env = os.environ.copy()
    root = repo_root or REPO_ROOT
    if root != DEFAULT_REPO_ROOT:
        env.pop("GIT_INDEX_FILE", None)
    return env


def git_common_dir(repo_root: Path | None = None) -> Path:
    root = (repo_root or REPO_ROOT).resolve()
    result = subprocess.run(
        ["git", "rev-parse", "--path-format=absolute", "--git-common-dir"],
        cwd=root,
        check=True,
        capture_output=True,
        text=True,
        env=git_env(root),
    )
    common_dir = Path(result.stdout.strip())
    if not common_dir.is_absolute():
        common_dir = root / common_dir
    return common_dir.resolve()


def canonical_repo_root(repo_root: Path | None = None) -> Path:
    common_dir = git_common_dir(repo_root)
    if common_dir.name == ".git":
        return common_dir.parent.resolve()
    return (repo_root or REPO_ROOT).resolve()


def canonical_repo_id(repo_root: Path | None = None) -> str:
    return canonical_repo_root(repo_root).name


def canonical_workspace_repos_root(repo_root: Path | None = None) -> Path:
    return canonical_repo_root(repo_root).parent


def repo_relative_path(path: str | Path) -> str:
    candidate = Path(path)
    if candidate.is_absolute():
        candidate = candidate.relative_to(REPO_ROOT)
    return candidate.as_posix()


def read_repo_text(path: str | Path, *, staged: bool = False, strict_staged: bool = False) -> str:
    rel = repo_relative_path(path)
    if staged:
        try:
            result = subprocess.run(
                ["git", "show", f":{rel}"],
                cwd=REPO_ROOT,
                check=True,
                capture_output=True,
                text=True,
                env=git_env(),
            )
            return result.stdout
        except subprocess.CalledProcessError:
            if strict_staged:
                raise FileNotFoundError(f"missing staged index entry for {rel}") from None
    return (REPO_ROOT / rel).read_text(encoding="utf-8")


def staged_path_exists(path: str | Path) -> bool:
    rel = repo_relative_path(path)
    result = subprocess.run(
        ["git", "cat-file", "-e", f":{rel}"],
        cwd=REPO_ROOT,
        check=False,
        capture_output=True,
        text=True,
        env=git_env(),
    )
    return result.returncode == 0


def missing_staged_repo_paths(paths: Iterable[str | Path]) -> list[str]:
    missing: list[str] = []
    for path in paths:
        rel = repo_relative_path(path)
        if not staged_path_exists(rel):
            missing.append(rel)
    return missing


def load_repo_json(path: str | Path, *, staged: bool = False, strict_staged: bool = False) -> dict[str, Any]:
    return json.loads(read_repo_text(path, staged=staged, strict_staged=strict_staged))
