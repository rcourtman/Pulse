#!/usr/bin/env python3
"""Materialize the blocked RC-to-GA promotion record from current repo state."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
from datetime import date
from pathlib import Path

from repo_file_io import REPO_ROOT, git_env, read_repo_text


GA_DATE_RE = re.compile(r"Pulse v5 entered maintenance-only support on `(\d{4}-\d{2}-\d{2})`\.")
V5_EOS_RE = re.compile(r"existing v5 users until `(\d{4}-\d{2}-\d{2})`\.")


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate the blocked RC-to-GA promotion record from current repo facts."
    )
    parser.add_argument(
        "--output",
        required=True,
        help="Repo-relative or absolute path for the generated blocked record markdown.",
    )
    parser.add_argument(
        "--record-date",
        default=str(date.today()),
        help="Date to record in the blocked record (default: today).",
    )
    return parser.parse_args(argv)


def read(rel: str) -> str:
    return read_repo_text(rel)


def read_json(rel: str) -> dict:
    return json.loads(read(rel))


def _run_git(*args: str) -> str:
    result = subprocess.run(
        ["git", *args],
        cwd=REPO_ROOT,
        env=git_env(),
        text=True,
        capture_output=True,
        check=True,
    )
    return result.stdout.strip()


def normalize_output_path(path_text: str) -> Path:
    path = Path(path_text)
    if path.is_absolute():
        return path
    return REPO_ROOT / path


def latest_matching_rc_tag(version: str) -> str:
    tags = [
        line.strip()
        for line in _run_git("tag", "--list", f"v{version}-rc.*", "--sort=version:refname").splitlines()
        if line.strip()
    ]
    if not tags:
        raise ValueError(f"no matching RC tag found for stable version {version!r}")
    return tags[-1]


def parse_release_dates() -> tuple[str, str]:
    content = read("docs/releases/RELEASE_NOTES_v6.md")
    ga_match = GA_DATE_RE.search(content)
    eos_match = V5_EOS_RE.search(content)
    if not ga_match or not eos_match:
        raise ValueError("could not derive GA/EOS dates from docs/releases/RELEASE_NOTES_v6.md")
    return ga_match.group(1), eos_match.group(1)


def build_blocked_record(*, record_date: str) -> str:
    control_plane = read_json("docs/release-control/control_plane.json")
    version = read("VERSION").strip()
    active_profile = next(
        profile for profile in control_plane["profiles"] if profile["id"] == control_plane["active_profile_id"]
    )
    active_target_id = str(control_plane["active_target_id"])
    rc_tag = latest_matching_rc_tag(version)
    rc_commit = _run_git("rev-parse", rc_tag)
    ga_date, v5_eos_date = parse_release_dates()

    lines = [
        "# RC-to-GA Promotion Readiness Blocked Record",
        "",
        f"- Date: `{record_date}`",
        "- Gate: `rc-to-ga-promotion-readiness`",
        "- Result: `blocked`",
        "",
        "## Blocking Facts",
        "",
        f"1. The only shipped Pulse v6 RC tag is `{rc_tag}`.",
        f"2. That RC tag resolves to commit `{rc_commit}`.",
        "3. The governed release profile in `docs/release-control/control_plane.json`",
        f"   currently declares both `prerelease_branch` and `stable_branch` as",
        f"   `{active_profile['stable_branch']}`.",
        f"4. The active control-plane target is still `{active_target_id}`, not",
        "   `v6-ga-promotion`.",
        f"5. The active local `pulse/v6` branch currently reports `VERSION={version}`, so a",
        "   local GA candidate exists even though the control plane is still explicitly",
        "   treating v6 as an RC-stabilization line.",
        f"6. There is still no governed `RC-to-GA Rehearsal Record` proving a successful",
        f"   non-publish `Release Dry Run` for the current `{version}` candidate.",
        "7. `docs/releases/RELEASE_NOTES_v6.md` and",
        "   `docs/release-control/v6/V5_MAINTENANCE_SUPPORT_POLICY.md` already carry the",
        "   exact governed dates:",
        f"   - `v6` GA date: `{ga_date}`",
        f"   - `v5` end-of-support date: `{v5_eos_date}`",
        "8. There is still no governed `Release Dry Run` artifact or rehearsal record",
        "   exercising stable inputs for:",
        f"   - `version={version}`",
        f"   - `promoted_from_tag={rc_tag}`",
        "   - an explicit `rollback_version`",
        f"   - `v5_eos_date={v5_eos_date}`",
        "",
        "## Why The Gate Cannot Be Cleared Yet",
        "",
        "The blocker is no longer missing governance text. The remaining problem is that",
        "the control plane still treats v6 as an RC-stabilization line, and there is",
        f"still no exercised `Release Dry Run` record proving the exact `{version}`",
        "candidate is ready for GA-style promotion. Until that rehearsal exists, stable",
        "users would still be the first real cohort for the final promotion path.",
        "",
        "## Required Unblock Steps",
        "",
        f"1. Promote the active target from `{active_target_id}` to",
        "   `v6-ga-promotion` only when that change is actually intended.",
        "2. Push the governed `pulse/v6` branch state, including the current",
        f"   `VERSION={version}` candidate and release-control records, to `origin/pulse/v6`.",
        "3. Run `Release Dry Run` from `pulse/v6` with:",
        f"   - `version={version}`",
        f"   - `promoted_from_tag={rc_tag}`",
        "   - an explicit stable `rollback_version`",
        f"   - `v5_eos_date={v5_eos_date}`",
        "4. Capture the `rc-to-ga-rehearsal-summary` artifact and run URL.",
        "5. Materialize the final rehearsal record from that artifact.",
        "6. Change the gate from `blocked` only if the rehearsal passes and the rollout",
        "   inputs remain explicit.",
        "",
    ]
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    date.fromisoformat(args.record_date)
    output_path = normalize_output_path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(build_blocked_record(record_date=args.record_date), encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
