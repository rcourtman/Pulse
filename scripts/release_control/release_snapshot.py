#!/usr/bin/env python3
"""Bind release execution to a reviewed snapshot, independently of a moving train."""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import subprocess


SHA = re.compile(r"[0-9a-f]{40}")
SNAPSHOT_REF = re.compile(r"release-candidate/[0-9A-Za-z._-]+")
SOURCE_BRANCH = re.compile(r"main|release/v[0-9]+\.[0-9]+")


def source_branch(ref: str, requested: str, pull_request: str) -> str:
    if not ref.startswith("refs/heads/"):
        raise ValueError("release dispatch must name a branch ref")
    branch = ref.removeprefix("refs/heads/")
    if SNAPSHOT_REF.fullmatch(branch):
        if not SOURCE_BRANCH.fullmatch(requested) or not re.fullmatch(r"[1-9][0-9]*", pull_request):
            raise ValueError("snapshot dispatch requires its governed source branch and merged pull request")
        return requested
    if requested or pull_request:
        raise ValueError("snapshot provenance inputs require a reserved release-candidate ref")
    if not SOURCE_BRANCH.fullmatch(branch):
        raise ValueError("release dispatch is outside a governed source branch")
    return branch


def reviewed_merge(pr: dict, *, repository: str, ref: str, branch: str, sha: str) -> str:
    if not SHA.fullmatch(sha):
        raise ValueError("source must be an exact commit")
    head, base = pr.get("head", {}), pr.get("base", {})
    if pr.get("merged") is not True or pr.get("state") != "closed":
        raise ValueError("release snapshot pull request is not merged")
    if head.get("sha") != sha or head.get("ref") != ref.removeprefix("refs/heads/"):
        raise ValueError("pull request does not identify the dispatched snapshot")
    if base.get("ref") != branch:
        raise ValueError("pull request belongs to another release line")
    if any(part.get("repo", {}).get("full_name") != repository for part in (head, base)):
        raise ValueError("release snapshot must come from the canonical repository")
    merge = pr.get("merge_commit_sha", "")
    if not isinstance(merge, str) or not SHA.fullmatch(merge):
        raise ValueError("pull request has no exact merge commit")
    return merge


def check_workflow(path: Path) -> None:
    # BaseLoader preserves the YAML `on` key rather than treating it as a bool.
    import yaml

    workflow = yaml.load(path.read_text(), Loader=yaml.BaseLoader)
    inputs = workflow["on"]["workflow_dispatch"]["inputs"]
    for name in ("expected_source_sha", "release_source_branch", "release_pull_request"):
        if inputs.get(name, {}).get("type") != "string":
            raise ValueError(f"release workflow lacks snapshot input {name}")
    steps = workflow["jobs"]["prepare"]["steps"]
    guard = next((step for step in steps if step.get("id") == "snapshot"), {})
    if "python3 scripts/release_control/release_snapshot.py" not in guard.get("run", ""):
        raise ValueError("release workflow does not execute the snapshot provenance guard")
    expected = {"RELEASE_SOURCE_BRANCH": "${{ inputs.release_source_branch }}", "RELEASE_PULL_REQUEST": "${{ inputs.release_pull_request }}"}
    if any(guard.get("env", {}).get(key) != value for key, value in expected.items()):
        raise ValueError("release workflow does not bind snapshot provenance inputs")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check-workflow", type=Path)
    args = parser.parse_args()
    if args.check_workflow:
        check_workflow(args.check_workflow)
        return

    ref = os.environ["GITHUB_REF"]
    requested = os.environ.get("RELEASE_SOURCE_BRANCH", "")
    pr_number = os.environ.get("RELEASE_PULL_REQUEST", "")
    branch = source_branch(ref, requested, pr_number)
    if requested:
        repository = os.environ["GITHUB_REPOSITORY"]
        if repository != "rcourtman/Pulse":
            raise ValueError("snapshot releases are restricted to the canonical Pulse repository")
        sha = os.environ["GITHUB_SHA"]
        if os.environ["GITHUB_WORKFLOW_SHA"] != sha:
            raise ValueError("workflow and source must be the same immutable snapshot")
        pr = json.loads(subprocess.check_output(["gh", "api", f"repos/{repository}/pulls/{pr_number}"], text=True))
        merge = reviewed_merge(pr, repository=repository, ref=ref, branch=branch, sha=sha)
        subprocess.run(["git", "fetch", "--quiet", "--no-tags", f"https://github.com/{repository}.git", f"refs/heads/{branch}"], check=True)
        # Later merges are allowed. A rewrite that removes the reviewed merge is not.
        subprocess.run(["git", "merge-base", "--is-ancestor", sha, merge], check=True)
        subprocess.run(["git", "merge-base", "--is-ancestor", merge, "FETCH_HEAD"], check=True)
    with Path(os.environ["GITHUB_OUTPUT"]).open("a") as output:
        output.write(f"source_branch={branch}\n")
    print(f"Release source is bound to {os.environ['GITHUB_SHA']} from {branch}")


if __name__ == "__main__":
    main()
