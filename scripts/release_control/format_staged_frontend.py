#!/usr/bin/env python3
"""Format staged frontend files with prettier without staging unrelated worktree changes.

Mirrors format_staged_go.py: reads staged blobs, formats them through
prettier's stdin interface, and writes the result back to the git index
directly, so partially staged files do not silently absorb unrelated hunks.
"""

from __future__ import annotations

import os
from pathlib import Path
import subprocess
import sys


REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_REPO_ROOT = REPO_ROOT

FRONTEND_DIR = "frontend-modern"
# Keep in sync with frontend-modern's package.json "format" script scope.
STAGED_PATHSPECS = (
    f":(glob){FRONTEND_DIR}/src/**/*.ts",
    f":(glob){FRONTEND_DIR}/src/**/*.tsx",
    f":(glob){FRONTEND_DIR}/src/**/*.css",
    f":(glob){FRONTEND_DIR}/src/**/*.json",
)
# Prettier is not always idempotent in a single pass; iterate to a fixed
# point with a small cap so a pathological input cannot loop forever.
MAX_FORMAT_PASSES = 5


def git_env() -> dict[str, str]:
    env = os.environ.copy()
    # Unit tests patch REPO_ROOT to a temporary repository. In that case, the
    # inherited hook environment (alternate index, or the absolute GIT_DIR a
    # pre-commit run from a linked worktree exports) should not leak into the
    # temp repo and point git plumbing at a different repository.
    if REPO_ROOT != DEFAULT_REPO_ROOT:
        for name in ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR"):
            env.pop(name, None)
    return env


def git(*args: str, text: bool, input_data: str | bytes | None = None) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["git", *args],
        cwd=REPO_ROOT,
        env=git_env(),
        check=True,
        capture_output=True,
        text=text,
        input=input_data,
    )


def prettier_bin() -> Path | None:
    override = os.environ.get("PULSE_PRETTIER_BIN")
    if override:
        candidate = Path(override)
        return candidate if candidate.exists() else None
    candidate = REPO_ROOT / FRONTEND_DIR / "node_modules" / ".bin" / "prettier"
    return candidate if candidate.exists() else None


def staged_frontend_files() -> list[str]:
    result = git(
        "diff",
        "--cached",
        "--name-only",
        "--diff-filter=ACMR",
        "-z",
        "--",
        *STAGED_PATHSPECS,
        text=False,
    )
    return sorted(
        entry.decode("utf-8")
        for entry in result.stdout.split(b"\x00")
        if entry
    )


def staged_blob(path: str) -> bytes:
    return git("show", f":{path}", text=False).stdout


def staged_mode(path: str) -> str:
    output = git("ls-files", "--stage", "--", path, text=True).stdout.strip()
    if not output:
        raise ValueError(f"missing staged index entry for {path}")
    metadata, _, _ = output.partition("\t")
    parts = metadata.split()
    if len(parts) < 3:
        raise ValueError(f"invalid staged index entry for {path}: {output!r}")
    return parts[0]


def prettier_bytes(prettier: Path, path: str, data: bytes) -> bytes:
    # --stdin-filepath drives parser selection and config resolution, so the
    # staged blob is formatted exactly as `prettier --write <path>` would.
    current = data
    for _ in range(MAX_FORMAT_PASSES):
        result = subprocess.run(
            [str(prettier), "--stdin-filepath", str(REPO_ROOT / path)],
            cwd=REPO_ROOT / FRONTEND_DIR,
            check=True,
            capture_output=True,
            input=current,
        )
        if result.stdout == current:
            break
        current = result.stdout
    return current


def write_blob_to_index(path: str, *, mode: str, data: bytes) -> None:
    blob = (
        git("hash-object", "-w", "--stdin", text=False, input_data=data)
        .stdout.decode("utf-8")
        .strip()
    )
    git("update-index", "--cacheinfo", f"{mode},{blob},{path}", text=True)


def sync_worktree_if_clean(path: str, previous_staged: bytes, formatted: bytes) -> bool:
    absolute = REPO_ROOT / path
    if not absolute.exists():
        return False
    current = absolute.read_bytes()
    if current != previous_staged:
        return False
    absolute.write_bytes(formatted)
    return True


def format_staged_frontend_files() -> int:
    paths = staged_frontend_files()
    if not paths:
        print("Skipping frontend formatter (no staged frontend source files).")
        return 0

    prettier = prettier_bin()
    if prettier is None:
        # Fresh clones and linked worktrees may not have node_modules; skip
        # gracefully like the golangci-lint availability check does. CI's
        # prettier check still catches drift that slips through here.
        print("Skipping frontend formatter (prettier not installed under frontend-modern/node_modules).")
        return 0

    print("Running prettier on staged frontend files...")
    formatted_count = 0
    synced_count = 0
    index_only_count = 0

    for path in paths:
        before = staged_blob(path)
        try:
            after = prettier_bytes(prettier, path, before)
        except subprocess.CalledProcessError as error:
            stderr = error.stderr.decode("utf-8", "replace") if error.stderr else ""
            print(f"BLOCKED: prettier failed on staged {path}:\n{stderr}", file=sys.stderr)
            return 1
        if after == before:
            continue
        write_blob_to_index(path, mode=staged_mode(path), data=after)
        formatted_count += 1
        if sync_worktree_if_clean(path, before, after):
            synced_count += 1
        else:
            index_only_count += 1

    print(
        f"Frontend formatter summary: staged_files={len(paths)} formatted={formatted_count} "
        f"worktree_synced={synced_count} index_only={index_only_count}"
    )
    return 0


def main() -> int:
    return format_staged_frontend_files()


if __name__ == "__main__":
    raise SystemExit(main())
