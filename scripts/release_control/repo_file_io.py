#!/usr/bin/env python3
"""Helpers for reading repo files from either the working tree or the git index."""

from __future__ import annotations

import json
from pathlib import Path
import subprocess
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[2]


def repo_relative_path(path: str | Path) -> str:
    candidate = Path(path)
    if candidate.is_absolute():
        candidate = candidate.relative_to(REPO_ROOT)
    return candidate.as_posix()


def read_repo_text(path: str | Path, *, staged: bool = False) -> str:
    rel = repo_relative_path(path)
    if staged:
        try:
            result = subprocess.run(
                ["git", "show", f":{rel}"],
                cwd=REPO_ROOT,
                check=True,
                capture_output=True,
                text=True,
            )
            return result.stdout
        except subprocess.CalledProcessError:
            pass
    return (REPO_ROOT / rel).read_text(encoding="utf-8")


def load_repo_json(path: str | Path, *, staged: bool = False) -> dict[str, Any]:
    return json.loads(read_repo_text(path, staged=staged))
