#!/usr/bin/env python3
"""Validate exact, reviewed canonical-completion pairs without rewriting history."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile


REPO_ROOT = Path(__file__).resolve().parents[2]
REGISTRY = Path(__file__).with_name("canonical_completion_history.json")
SHA_PATTERN = re.compile(r"[0-9a-f]{40}")


def git(*args: str, cwd: Path = REPO_ROOT) -> str:
    return subprocess.run(
        ["git", *args],
        cwd=cwd,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.rstrip("\n")


def load_completions(path: Path = REGISTRY) -> dict[str, dict[str, str]]:
    document = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(document, dict) or set(document) != {"version", "completions"}:
        raise ValueError("completion registry must contain only version and completions")
    if document["version"] != 1 or isinstance(document["version"], bool):
        raise ValueError("unsupported completion registry version")
    if not isinstance(document["completions"], list):
        raise ValueError("completion registry entries must be a list")

    result: dict[str, dict[str, str]] = {}
    required = {"incomplete_commit", "completion_commit", "reason"}
    for entry in document["completions"]:
        if not isinstance(entry, dict) or set(entry) != required:
            raise ValueError("completion entry has unexpected fields")
        if not all(isinstance(entry[field], str) and entry[field].strip() for field in required):
            raise ValueError("completion entry fields must be non-empty strings")
        incomplete = entry["incomplete_commit"]
        completion = entry["completion_commit"]
        if not SHA_PATTERN.fullmatch(incomplete) or not SHA_PATTERN.fullmatch(completion):
            raise ValueError("completion entry revisions must be full lowercase commit IDs")
        if incomplete == completion or incomplete in result:
            raise ValueError("completion entries must identify distinct, unique commits")
        result[incomplete] = entry
    return result


def changed_files(commit: str) -> list[str]:
    return [
        line
        for line in git("diff-tree", "--no-commit-id", "--name-only", "-r", commit).splitlines()
        if line
    ]


def validate_completion(incomplete: str, head: str) -> bool:
    """Return False for an ordinary commit; validate and return True for a registered pair."""
    entry = load_completions().get(incomplete)
    if entry is None:
        return False
    if not SHA_PATTERN.fullmatch(head):
        raise ValueError("head must be a full lowercase commit ID")

    completion = entry["completion_commit"]
    git("cat-file", "-e", f"{incomplete}^{{commit}}")
    git("cat-file", "-e", f"{completion}^{{commit}}")
    git("cat-file", "-e", f"{head}^{{commit}}")
    subprocess.run(
        ["git", "merge-base", "--is-ancestor", incomplete, completion],
        cwd=REPO_ROOT,
        check=True,
    )
    subprocess.run(
        ["git", "merge-base", "--is-ancestor", completion, head],
        cwd=REPO_ROOT,
        check=True,
    )

    files = sorted(set(changed_files(incomplete) + changed_files(completion)))
    with tempfile.TemporaryDirectory(prefix="pulse-canonical-completion-") as temp:
        worktree = Path(temp) / "pulse"
        git("worktree", "add", "--detach", str(worktree), completion)
        try:
            guard = worktree / "scripts/release_control/canonical_completion_guard.py"
            result = subprocess.run(
                [
                    sys.executable,
                    str(guard),
                    "--files-from-stdin",
                    "--diff-base",
                    f"{incomplete}^",
                    "--commit",
                    completion,
                ],
                cwd=worktree,
                input="".join(f"{path}\n" for path in files),
                text=True,
            )
            if result.returncode != 0:
                raise ValueError(
                    f"registered completion {completion} does not complete {incomplete}"
                )
        finally:
            git("worktree", "remove", "--force", str(worktree))

    print(
        "Canonical completion history passed "
        f"({incomplete} completed by {completion}: {entry['reason']})"
    )
    return True


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--head", required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        return 0 if validate_completion(args.commit, args.head) else 3
    except (OSError, ValueError, subprocess.CalledProcessError, json.JSONDecodeError) as error:
        print(f"Canonical completion history failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
