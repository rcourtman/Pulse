#!/usr/bin/env python3
"""Resolve and validate shared release-promotion metadata for governed workflows."""

from __future__ import annotations

import argparse
import re
import subprocess
import time
from pathlib import Path
from typing import Callable

from repo_file_io import REPO_ROOT, git_env


SEMVER_PRERELEASE_RE = re.compile(r"-(?:rc|alpha|beta)\.\d+$")


def normalize_tag(value: str) -> str:
    value = (value or "").strip()
    if not value:
        return ""
    if value.startswith("v"):
        return value
    return f"v{value}"


def is_prerelease_version(version: str) -> bool:
    return bool(SEMVER_PRERELEASE_RE.search(version))


def tag_exists(tag: str) -> bool:
    result = subprocess.run(
        ["git", "rev-parse", "-q", "--verify", f"refs/tags/{tag}"],
        cwd=REPO_ROOT,
        env=git_env(),
        capture_output=True,
        text=True,
    )
    return result.returncode == 0


def tag_commit(tag: str) -> str:
    result = subprocess.run(
        ["git", "rev-list", "-n1", f"refs/tags/{tag}"],
        cwd=REPO_ROOT,
        env=git_env(),
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def head_descends_from(commit: str) -> bool:
    result = subprocess.run(
        ["git", "merge-base", "--is-ancestor", commit, "HEAD"],
        cwd=REPO_ROOT,
        env=git_env(),
    )
    return result.returncode == 0


def tag_created_unix(tag: str) -> int:
    result = subprocess.run(
        ["git", "for-each-ref", "--format=%(creatordate:unix)", f"refs/tags/{tag}"],
        cwd=REPO_ROOT,
        env=git_env(),
        check=True,
        capture_output=True,
        text=True,
    )
    value = result.stdout.strip().splitlines()
    if not value or not value[0].strip():
        raise ValueError(f"Could not determine creation time for promoted RC tag {tag}.")
    return int(value[0].strip())


def normalize_whitespace(value: str) -> str:
    return " ".join((value or "").split())


def resolve_metadata(
    *,
    version: str,
    promoted_from_tag_input: str,
    rollback_version_input: str,
    ga_date_input: str,
    v5_eos_date_input: str,
    hotfix_exception: bool,
    hotfix_reason_input: str,
    release_notes_input: str,
    tag_exists_fn: Callable[[str], bool] = tag_exists,
    tag_commit_fn: Callable[[str], str] = tag_commit,
    head_descends_from_fn: Callable[[str], bool] = head_descends_from,
    tag_created_unix_fn: Callable[[str], int] = tag_created_unix,
    now_unix_fn: Callable[[], int] = lambda: int(time.time()),
) -> dict[str, str]:
    tag = normalize_tag(version)
    rollback_tag = normalize_tag(rollback_version_input)
    ga_date = (ga_date_input or "").strip()
    v5_eos_date = (v5_eos_date_input or "").strip()
    hotfix_reason = normalize_whitespace(hotfix_reason_input)
    release_notes = release_notes_input or ""
    is_prerelease = is_prerelease_version(version)

    if not rollback_tag:
        raise ValueError(
            "rollback_version is required for every release rehearsal and promotion so rollback can be executed explicitly."
        )
    if SEMVER_PRERELEASE_RE.search(rollback_tag):
        raise ValueError(
            f"rollback_version must point to a stable release tag, not a prerelease ({rollback_tag})."
        )
    if not tag_exists_fn(rollback_tag):
        raise ValueError(f"rollback_version {rollback_tag} does not exist as a repository tag.")
    rollback_command = f"./scripts/install.sh --version {rollback_tag}"

    promoted_from_tag = ""
    soak_hours = ""
    if is_prerelease:
        if hotfix_exception:
            raise ValueError("hotfix_exception applies only to stable promotions.")
    else:
        promoted_from_tag = normalize_tag(promoted_from_tag_input)
        if not promoted_from_tag:
            raise ValueError(
                "Stable promotion requires promoted_from_tag naming the RC being promoted."
            )
        if not re.match(rf"^v{re.escape(version)}-rc\.\d+$", promoted_from_tag):
            raise ValueError(
                f"promoted_from_tag must reference an RC tag for the same stable version ({version}), got {promoted_from_tag}."
            )
        if not tag_exists_fn(promoted_from_tag):
            raise ValueError(
                f"promoted_from_tag {promoted_from_tag} does not exist as a repository tag."
            )

        promoted_commit = tag_commit_fn(promoted_from_tag)
        if not head_descends_from_fn(promoted_commit):
            raise ValueError(
                f"Stable promotion {tag} must descend from promoted RC tag {promoted_from_tag}."
            )

        promoted_tag_ts = tag_created_unix_fn(promoted_from_tag)
        soak_hours_value = int((now_unix_fn() - promoted_tag_ts) / 3600)
        soak_hours = str(soak_hours_value)

        if hotfix_exception:
            if not hotfix_reason:
                raise ValueError("hotfix_reason is required when hotfix_exception is true.")
        elif soak_hours_value < 72:
            raise ValueError(
                f"Stable promotion {tag} has only {soak_hours_value} hours of RC soak since {promoted_from_tag}; minimum is 72 hours unless hotfix_exception is true."
            )

        if version == "6.0.0":
            if not re.match(r"^\d{4}-\d{2}-\d{2}$", ga_date):
                raise ValueError(
                    "Stable v6.0.0 requires ga_date in YYYY-MM-DD form so the GA publish notice is explicit."
                )
            if not re.match(r"^\d{4}-\d{2}-\d{2}$", v5_eos_date):
                raise ValueError(
                    "Stable v6.0.0 requires v5_eos_date in YYYY-MM-DD form so the support window is published explicitly."
                )
            if release_notes:
                if "maintenance-only support" not in release_notes.lower():
                    raise ValueError(
                        "Stable v6.0.0 release_notes must include the Pulse v5 maintenance-only support notice."
                    )
                if ga_date not in release_notes:
                    raise ValueError(
                        f"Stable v6.0.0 release_notes must include the exact ga_date ({ga_date})."
                    )
                if v5_eos_date not in release_notes:
                    raise ValueError(
                        f"Stable v6.0.0 release_notes must include the exact v5_eos_date ({v5_eos_date})."
                    )

    return {
        "promoted_from_tag": promoted_from_tag,
        "rollback_tag": rollback_tag,
        "rollback_command": rollback_command,
        "ga_date": ga_date,
        "v5_eos_date": v5_eos_date,
        "hotfix_exception": "true" if hotfix_exception else "false",
        "hotfix_reason": hotfix_reason,
        "soak_hours": soak_hours,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True)
    parser.add_argument("--promoted-from-tag", default="")
    parser.add_argument("--rollback-version", default="")
    parser.add_argument("--ga-date", default="")
    parser.add_argument("--v5-eos-date", default="")
    parser.add_argument("--hotfix-exception", action="store_true")
    parser.add_argument("--hotfix-reason", default="")
    parser.add_argument("--release-notes-file", default="")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    release_notes = ""
    if args.release_notes_file:
        release_notes = Path(args.release_notes_file).read_text(encoding="utf-8")

    metadata = resolve_metadata(
        version=args.version,
        promoted_from_tag_input=args.promoted_from_tag,
        rollback_version_input=args.rollback_version,
        ga_date_input=args.ga_date,
        v5_eos_date_input=args.v5_eos_date,
        hotfix_exception=args.hotfix_exception,
        hotfix_reason_input=args.hotfix_reason,
        release_notes_input=release_notes,
    )

    for key, value in metadata.items():
        print(f"{key}={value}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
