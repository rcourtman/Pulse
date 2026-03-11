#!/usr/bin/env python3
"""Format staged Go files without staging unrelated worktree changes."""

from __future__ import annotations

from pathlib import Path
import subprocess
import sys


REPO_ROOT = Path(__file__).resolve().parents[2]


def git(*args: str, text: bool, input_data: str | bytes | None = None) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["git", *args],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=text,
        input=input_data,
    )


def staged_go_files() -> list[str]:
    result = git(
        "diff",
        "--cached",
        "--name-only",
        "--diff-filter=ACMR",
        "-z",
        "--",
        "*.go",
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


def gofmt_bytes(data: bytes) -> bytes:
    result = subprocess.run(
        ["gofmt", "-s"],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        input=data,
    )
    return result.stdout


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


def format_staged_go_files() -> int:
    paths = staged_go_files()
    if not paths:
        print("Skipping Go formatter (no staged Go files).")
        return 0

    print("Running Go formatter on staged Go files...")
    formatted_count = 0
    synced_count = 0
    index_only_count = 0

    for path in paths:
        before = staged_blob(path)
        after = gofmt_bytes(before)
        if after == before:
            continue
        write_blob_to_index(path, mode=staged_mode(path), data=after)
        formatted_count += 1
        if sync_worktree_if_clean(path, before, after):
            synced_count += 1
        else:
            index_only_count += 1

    print(
        f"Go formatter summary: staged_files={len(paths)} formatted={formatted_count} "
        f"worktree_synced={synced_count} index_only={index_only_count}"
    )
    return 0


def main() -> int:
    return format_staged_go_files()


if __name__ == "__main__":
    raise SystemExit(main())
