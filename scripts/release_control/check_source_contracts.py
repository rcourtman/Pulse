#!/usr/bin/env python3
"""Run the existing per-commit contract admission before CI dependency fan-out.

This is not application acceptance. Browser receipts are checked separately;
all runtime, secret-scan and canonical audits retain their existing authority.
"""

from __future__ import annotations

import argparse
import os
from pathlib import Path
import re
import subprocess
import sys

from browser_verification_guard import frontend_runtime_paths
from repo_file_io import git_env


ROOT = Path(__file__).resolve().parents[2]
SHA = re.compile(r"[0-9a-f]{40}")
NEUTRAL = "PULSE_ALLOW_CONTRACT_NEUTRAL_COMMIT"


def git(*args: str) -> str:
    return subprocess.run(
        ["git", *args], cwd=ROOT, env=git_env(ROOT), text=True,
        capture_output=True, check=True,
    ).stdout.rstrip("\n")


def check(base: str, head: str, *, early: bool = False) -> int:
    if not SHA.fullmatch(base) or not SHA.fullmatch(head):
        raise ValueError("base and head must be exact lowercase commit IDs")
    if git("rev-parse", "HEAD") != head:
        raise ValueError("checked-out source does not match the requested head")
    git("cat-file", "-e", f"{base}^{{commit}}")
    merge_base = git("merge-base", base, head)
    commits = git("rev-list", "--reverse", "--no-merges", f"{base}..{head}").splitlines()
    commit_files: dict[str, str] = {}
    frontend_in_range = False
    if early:
        # The established formatting-only exception needs locked Prettier.
        # Do not misclassify a legitimate formatting commit before npm ci.
        # Include intermediate/reverted edits and merge-only resolutions, not
        # just the net tip delta. Defer only the commits needing that exception,
        # not unrelated backend contract/history checks in the same proposal.
        paths = git("diff", "--name-only", merge_base, head, "--").splitlines()
        for commit in commits:
            commit_files[commit] = git("diff-tree", "--no-commit-id", "--name-only", "-r", commit)
            paths.extend(commit_files[commit].splitlines())
        frontend_in_range = bool(frontend_runtime_paths(paths))
    status = 0
    # Only each commit's final trailer can supply its override. In particular,
    # an ambient override cannot turn a split-history validation green.
    env = {**os.environ, NEUTRAL: ""}
    for commit in commits:
        parents = git("rev-list", "--parents", "-n", "1", commit).split()
        if len(parents) == 1:
            print(f"Skipping root commit {commit} (no parent to diff against).", flush=True)
            continue
        parent = parents[1]
        if early and frontend_runtime_paths(commit_files[commit].splitlines()):
            print(f"Frontend source in commit {commit}; defer early admission for this commit to full Canonical Governance with locked formatting tools.", flush=True)
            continue
        reason = git(
            "log", "-1", "--format=%(trailers:key=Contract-Neutral,valueonly,separator=; )",
            commit,
        ).replace("\n", " ")
        print(f"::group::canonical completion guard @ {commit}", flush=True)
        history = subprocess.run(
            [sys.executable, "scripts/release_control/canonical_completion_history.py",
             "--commit", commit, "--head", head], cwd=ROOT, env=env,
        )
        if history.returncode == 3:
            files = commit_files[commit] if early else git("diff-tree", "--no-commit-id", "--name-only", "-r", commit)
            result = subprocess.run(
                [sys.executable, "scripts/release_control/canonical_completion_guard.py",
                 "--files-from-stdin", "--diff-base", parent, "--commit", commit],
                cwd=ROOT, env={**env, NEUTRAL: reason}, text=True,
                input=files + "\n" if files else "",
            )
            if result.returncode != 0:
                print(f"Canonical completion guard failed for commit {commit}.", flush=True)
                status = 1
        elif history.returncode != 0:
            print(f"Canonical completion history failed for commit {commit}.", flush=True)
            status = 1
        print("::endgroup::", flush=True)
    if early and frontend_in_range:
        # A merge-only frontend resolution also needs final-content evidence.
        # Checked backend commits cannot answer for those pixels or formatting.
        print("Frontend source in range; defer early admission for frontend commits and final browser evidence to full Canonical Governance with locked formatting tools. Other commits were checked.", flush=True)
    elif early and status == 0:
        # Receipt-only changes must not bypass the required evidence rule.
        # There is no frontend source/formatter exception in this range.
        result = subprocess.run(
            [sys.executable, "scripts/release_control/browser_verification_guard.py",
             "--base", base, "--commit", head], cwd=ROOT, env=env,
        )
        if result.returncode != 0:
            status = 1
    return status


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--head", required=True)
    parser.add_argument("--early", action="store_true",
                        help="check non-rendering commits before dependencies; defer frontend commits and their final browser evidence to the full required job")
    args = parser.parse_args()
    try:
        return check(args.base, args.head, early=args.early)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"Source contract admission unavailable: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
