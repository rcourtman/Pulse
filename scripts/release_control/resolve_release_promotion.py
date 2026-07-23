#!/usr/bin/env python3
"""Resolve and validate shared release-promotion metadata for governed workflows."""

from __future__ import annotations

import argparse
import fnmatch
import re
import subprocess
import time
from pathlib import Path
from typing import Callable

from repo_file_io import REPO_ROOT, git_env


SEMVER_STABLE_RE = re.compile(r"^(\d+)\.(\d+)\.(\d+)$")
SEMVER_STABLE_TAG_RE = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")
SEMVER_PRERELEASE_RE = re.compile(r"-(?:[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)(?:\+[0-9A-Za-z.-]+)?$")

ROUTINE_PATCH_RC_REQUIRED_RULES: tuple[tuple[str, tuple[str, ...]], ...] = (
    (
        "authentication, authorization, or tenant isolation",
        (
            "internal/api/auth*.go",
            "internal/api/security*.go",
            "internal/api/saml*.go",
            "internal/api/sso*.go",
            "internal/auth/**",
            "internal/securityutil/**",
            "pkg/auth/**",
        ),
    ),
    (
        "licensing, entitlement, or billing authority",
        (
            "internal/api/billing*.go",
            "internal/api/license*.go",
            "internal/entitlements/**",
            "internal/licensing/**",
            "pkg/licensing/**",
        ),
    ),
    (
        "persisted data format, schema, or migration",
        (
            "internal/database/**",
            "internal/migrations/**",
            "internal/storage/**",
            "pkg/database/**",
            "pkg/storage/**",
            "**/migrations/**",
            "**/*migration*.go",
        ),
    ),
    (
        "relay or mobile trust protocol",
        (
            "internal/api/cloud_handoff*.go",
            "internal/api/magic_link*.go",
            "internal/api/mobile*.go",
            "internal/relay/**",
            "pkg/relay/**",
        ),
    ),
    (
        "installer, updater, or rollback execution",
        (
            "install.sh",
            "internal/updates/**",
            "scripts/install.ps1",
            "scripts/install.sh",
            "scripts/pulse-auto-update.sh",
        ),
    ),
)


def normalize_tag(value: str) -> str:
    value = (value or "").strip()
    if not value:
        return ""
    if value.startswith("v"):
        return value
    return f"v{value}"


def is_prerelease_version(version: str) -> bool:
    return bool(SEMVER_PRERELEASE_RE.search(version))


def is_stable_patch_version(version: str) -> bool:
    match = SEMVER_STABLE_RE.match(version)
    return bool(match and int(match.group(3)) > 0)


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
        raise ValueError(f"Could not determine creation time for promoted prerelease tag {tag}.")
    return int(value[0].strip())


def normalize_whitespace(value: str) -> str:
    return " ".join((value or "").split())


def list_stable_tags() -> list[str]:
    result = subprocess.run(
        ["git", "tag", "--list", "v*"],
        cwd=REPO_ROOT,
        env=git_env(),
        check=True,
        capture_output=True,
        text=True,
    )
    return [tag for tag in result.stdout.split() if SEMVER_STABLE_TAG_RE.match(tag)]


def list_same_version_rc_tags(version: str) -> list[str]:
    result = subprocess.run(
        ["git", "tag", "--list", f"v{version}-rc.*", "--sort=-version:refname"],
        cwd=REPO_ROOT,
        env=git_env(),
        check=True,
        capture_output=True,
        text=True,
    )
    return [tag for tag in result.stdout.splitlines() if tag.strip()]


def changed_paths_between(base_tag: str) -> list[str]:
    result = subprocess.run(
        ["git", "diff", "--name-only", f"{base_tag}..HEAD"],
        cwd=REPO_ROOT,
        env=git_env(),
        check=True,
        capture_output=True,
        text=True,
    )
    return [path for path in result.stdout.splitlines() if path.strip()]


def classify_routine_patch_risks(paths: list[str]) -> list[str]:
    risks: list[str] = []
    for path in sorted(set(paths)):
        for reason, patterns in ROUTINE_PATCH_RC_REQUIRED_RULES:
            if any(fnmatch.fnmatchcase(path, pattern) for pattern in patterns):
                risks.append(f"{path} ({reason})")
                break
    return risks


def derive_latest_stable_rollback_tag(version: str, stable_tags: list[str]) -> str:
    base_match = re.match(r"^(\d+)\.(\d+)\.(\d+)", (version or "").strip())
    if not base_match:
        raise ValueError(f"Cannot derive a rollback target from unparseable version {version!r}.")
    base = tuple(int(part) for part in base_match.groups())
    candidates: list[tuple[tuple[int, int, int], str]] = []
    for tag in stable_tags:
        match = SEMVER_STABLE_TAG_RE.match(tag)
        if not match:
            continue
        numbers = tuple(int(part) for part in match.groups())
        if numbers < base:
            candidates.append((numbers, tag))
    if not candidates:
        raise ValueError(
            f"Cannot derive a rollback target for {version}: no stable release tag precedes it."
        )
    return max(candidates)[1]


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
    unsigned_windows_exception: bool = False,
    unsigned_windows_reason_input: str = "",
    derive_rollback_when_missing: bool = False,
    list_stable_tags_fn: Callable[[], list[str]] = list_stable_tags,
    list_same_version_rc_tags_fn: Callable[[str], list[str]] = list_same_version_rc_tags,
    changed_paths_fn: Callable[[str], list[str]] = changed_paths_between,
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
    unsigned_windows_reason = normalize_whitespace(unsigned_windows_reason_input)
    release_notes = release_notes_input or ""
    is_prerelease = is_prerelease_version(version)
    stable_patch = is_stable_patch_version(version)
    promotion_mode = "prerelease" if is_prerelease else "stable-rc-promotion"

    if unsigned_windows_exception:
        if version not in {"6.1.0", "6.1.1"}:
            raise ValueError(
                "unsigned_windows_exception is approved only for stable v6.1.0 or v6.1.1. "
                "Stable v6.1.2 and later must restore Windows Authenticode signing."
            )
        if not unsigned_windows_reason:
            raise ValueError(
                "unsigned_windows_reason is required when unsigned_windows_exception is true."
            )
        if release_notes and "not authenticode-signed" not in release_notes.lower():
            raise ValueError(
                f"Stable v{version} release_notes must disclose that Windows binaries are not Authenticode-signed."
            )
    elif unsigned_windows_reason:
        raise ValueError(
            "unsigned_windows_reason is allowed only when unsigned_windows_exception is true."
        )

    require_windows_signing = not is_prerelease and not unsigned_windows_exception

    if not rollback_tag and derive_rollback_when_missing:
        rollback_tag = derive_latest_stable_rollback_tag(version, list_stable_tags_fn())
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
            if not stable_patch:
                raise ValueError(
                    "Stable promotion requires promoted_from_tag naming the prerelease being promoted. "
                    "Only governed stable patch releases may use the routine no-RC path."
                )

            expected_rollback_tag = derive_latest_stable_rollback_tag(
                version,
                list_stable_tags_fn(),
            )
            if rollback_tag != expected_rollback_tag:
                raise ValueError(
                    f"Routine stable patch {tag} must roll back to the latest preceding stable tag "
                    f"{expected_rollback_tag}, got {rollback_tag}."
                )

            rollback_commit = tag_commit_fn(rollback_tag)
            if not head_descends_from_fn(rollback_commit):
                raise ValueError(
                    f"Routine stable patch {tag} must descend from rollback target {rollback_tag}."
                )

            same_version_rc_tags = list_same_version_rc_tags_fn(version)
            routine_patch_risks = classify_routine_patch_risks(
                changed_paths_fn(rollback_tag)
            )
            if (same_version_rc_tags or routine_patch_risks) and not hotfix_exception:
                reasons: list[str] = []
                if same_version_rc_tags:
                    reasons.append(
                        "same-version release candidates already exist: "
                        + ", ".join(same_version_rc_tags)
                    )
                if routine_patch_risks:
                    reasons.append(
                        "RC-required runtime changes: "
                        + "; ".join(routine_patch_risks)
                    )
                raise ValueError(
                    "Routine stable patch mode is not allowed because "
                    + " | ".join(reasons)
                    + ". Promote the exercised RC, or use hotfix_exception with a concrete emergency reason."
                )

            if hotfix_exception:
                if not hotfix_reason:
                    raise ValueError("hotfix_reason is required when hotfix_exception is true.")
                promotion_mode = "emergency-stable-patch"
            else:
                promotion_mode = "routine-stable-patch"
        else:
            if not re.match(rf"^v{re.escape(version)}-rc\.\d+$", promoted_from_tag):
                raise ValueError(
                    f"promoted_from_tag must reference a prerelease tag for the same stable version ({version}), got {promoted_from_tag}."
                )
            if not tag_exists_fn(promoted_from_tag):
                raise ValueError(
                    f"promoted_from_tag {promoted_from_tag} does not exist as a repository tag."
                )

            promoted_commit = tag_commit_fn(promoted_from_tag)
            if not head_descends_from_fn(promoted_commit):
                raise ValueError(
                    f"Stable promotion {tag} must descend from promoted prerelease tag {promoted_from_tag}."
                )

            promoted_tag_ts = tag_created_unix_fn(promoted_from_tag)
            soak_hours_value = int((now_unix_fn() - promoted_tag_ts) / 3600)
            soak_hours = str(soak_hours_value)

            if hotfix_exception:
                if not hotfix_reason:
                    raise ValueError("hotfix_reason is required when hotfix_exception is true.")
            elif soak_hours_value < 72:
                raise ValueError(
                    f"Stable promotion {tag} has only {soak_hours_value} hours of prerelease soak since {promoted_from_tag}; minimum is 72 hours unless hotfix_exception is true."
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
        "promotion_mode": promotion_mode,
        "is_stable_patch": "true" if stable_patch else "false",
        "promoted_from_tag": promoted_from_tag,
        "rollback_tag": rollback_tag,
        "rollback_command": rollback_command,
        "ga_date": ga_date,
        "v5_eos_date": v5_eos_date,
        "hotfix_exception": "true" if hotfix_exception else "false",
        "hotfix_reason": hotfix_reason,
        "unsigned_windows_exception": "true" if unsigned_windows_exception else "false",
        "unsigned_windows_reason": unsigned_windows_reason,
        "require_windows_signing": "true" if require_windows_signing else "false",
        "soak_hours": soak_hours,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True)
    parser.add_argument("--promoted-from-tag", default="")
    parser.add_argument("--rollback-version", default="")
    parser.add_argument(
        "--derive-rollback-latest-stable",
        action="store_true",
        help=(
            "Scheduled rehearsals only: when --rollback-version is empty, derive the rollback "
            "target as the latest stable tag preceding the rehearsal version instead of failing."
        ),
    )
    parser.add_argument("--ga-date", default="")
    parser.add_argument("--v5-eos-date", default="")
    parser.add_argument("--hotfix-exception", action="store_true")
    parser.add_argument("--hotfix-reason", default="")
    parser.add_argument("--unsigned-windows-exception", action="store_true")
    parser.add_argument("--unsigned-windows-reason", default="")
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
        derive_rollback_when_missing=args.derive_rollback_latest_stable,
        ga_date_input=args.ga_date,
        v5_eos_date_input=args.v5_eos_date,
        hotfix_exception=args.hotfix_exception,
        hotfix_reason_input=args.hotfix_reason,
        release_notes_input=release_notes,
        unsigned_windows_exception=args.unsigned_windows_exception,
        unsigned_windows_reason_input=args.unsigned_windows_reason,
    )

    for key, value in metadata.items():
        print(f"{key}={value}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
