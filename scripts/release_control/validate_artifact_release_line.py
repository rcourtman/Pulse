#!/usr/bin/env python3
"""Validate public artifact publication against the governed release line."""

from __future__ import annotations

import argparse
import subprocess
from pathlib import Path
from typing import Callable

from control_plane import release_branch_for_version
from repo_file_io import REPO_ROOT, git_env
from resolve_release_promotion import SEMVER_STABLE_RE, is_prerelease_version, normalize_tag


def run_git(args: list[str], *, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=REPO_ROOT,
        env=git_env(),
        check=check,
        capture_output=True,
        text=True,
    )


def ensure_release_refs(required_branch: str) -> None:
    shallow = run_git(["rev-parse", "--is-shallow-repository"]).stdout.strip()
    if shallow == "true":
        run_git(["fetch", "--prune", "--unshallow", "origin"])
    run_git(["fetch", "--prune", "origin", required_branch])


def tag_exists(tag: str) -> bool:
    result = run_git(["rev-parse", "-q", "--verify", f"refs/tags/{tag}"], check=False)
    return result.returncode == 0


def tag_commit(tag: str) -> str:
    return run_git(["rev-list", "-n1", f"refs/tags/{tag}"]).stdout.strip()


def ref_commit(ref: str) -> str:
    return run_git(["rev-list", "-n1", ref]).stdout.strip()


def ref_is_ancestor(ancestor: str, descendant: str) -> bool:
    result = run_git(["merge-base", "--is-ancestor", ancestor, descendant], check=False)
    return result.returncode == 0


def matching_prerelease_tags(version: str) -> tuple[str, ...]:
    output = run_git(["tag", "-l", f"v{version}-rc.*", "--sort=-version:refname"]).stdout
    return tuple(line.strip() for line in output.splitlines() if line.strip())


def previous_stable_tag(version: str) -> str:
    match = SEMVER_STABLE_RE.match(version)
    if not match:
        return ""
    major, minor, patch = (int(part) for part in match.groups())
    if patch <= 0:
        return ""
    return f"v{major}.{minor}.{patch - 1}"


def validate_artifact_release_line(
    *,
    tag: str,
    purpose: str,
    branch_for_version_fn: Callable[[str], str] = release_branch_for_version,
    fetch_refs_fn: Callable[[str], None] = ensure_release_refs,
    tag_exists_fn: Callable[[str], bool] = tag_exists,
    tag_commit_fn: Callable[[str], str] = tag_commit,
    ref_commit_fn: Callable[[str], str] = ref_commit,
    ref_is_ancestor_fn: Callable[[str, str], bool] = ref_is_ancestor,
    prerelease_tags_fn: Callable[[str], tuple[str, ...]] = matching_prerelease_tags,
) -> dict[str, str]:
    normalized_tag = normalize_tag(tag)
    if not normalized_tag:
        raise ValueError("release tag is required")

    version = normalized_tag.removeprefix("v")
    required_branch = branch_for_version_fn(version)
    fetch_refs_fn(required_branch)

    if not tag_exists_fn(normalized_tag):
        raise ValueError(f"Tag {normalized_tag} does not exist in repository tags.")

    tag_sha = tag_commit_fn(normalized_tag)
    branch_ref = f"origin/{required_branch}"
    branch_sha = ref_commit_fn(branch_ref)
    if not ref_is_ancestor_fn(tag_sha, branch_sha):
        raise ValueError(
            f"Tag {normalized_tag} is not reachable from {branch_ref}. Refusing {purpose}."
        )

    lineage = "prerelease"
    lineage_tag = ""
    if not is_prerelease_version(version):
        lineage = ""
        for rc_tag in prerelease_tags_fn(version):
            rc_sha = tag_commit_fn(rc_tag)
            if ref_is_ancestor_fn(rc_sha, tag_sha):
                lineage = "promoted_prerelease"
                lineage_tag = rc_tag
                break

        if not lineage:
            previous_tag = previous_stable_tag(version)
            if previous_tag and tag_exists_fn(previous_tag):
                previous_sha = tag_commit_fn(previous_tag)
                if ref_is_ancestor_fn(previous_sha, tag_sha):
                    lineage = "stable_patch"
                    lineage_tag = previous_tag

        if not lineage:
            previous_tag = previous_stable_tag(version)
            if previous_tag:
                raise ValueError(
                    f"Stable patch tag {normalized_tag} must descend from a matching prerelease tag "
                    f"or previous stable tag {previous_tag}."
                )
            raise ValueError(
                f"Stable tag {normalized_tag} does not descend from any matching prerelease tag "
                f"for base version {version}."
            )

    return {
        "tag": normalized_tag,
        "version": version,
        "required_branch": required_branch,
        "lineage": lineage,
        "lineage_tag": lineage_tag,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--tag", required=True)
    parser.add_argument("--purpose", default="artifact publication")
    parser.add_argument("--github-output", default="")
    return parser.parse_args()


def write_github_output(path: str, values: dict[str, str]) -> None:
    if not path:
        return
    with Path(path).open("a", encoding="utf-8") as output:
        for key, value in values.items():
            output.write(f"{key}={value}\n")


def main() -> int:
    args = parse_args()
    result = validate_artifact_release_line(tag=args.tag, purpose=args.purpose)

    if result["lineage"] == "promoted_prerelease":
        print(f"[OK] {result['tag']} descends from prerelease {result['lineage_tag']}")
    elif result["lineage"] == "stable_patch":
        print(f"[OK] {result['tag']} descends from previous stable {result['lineage_tag']}")
    print(f"[OK] {result['tag']} validated against release line {result['required_branch']}")

    write_github_output(
        args.github_output,
        {
            "required_branch": result["required_branch"],
            "lineage": result["lineage"],
            "lineage_tag": result["lineage_tag"],
        },
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
