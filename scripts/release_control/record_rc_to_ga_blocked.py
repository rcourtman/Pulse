#!/usr/bin/env python3
"""Materialize the blocked RC-to-GA promotion record from current repo state."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
from datetime import date
from pathlib import Path

from control_plane import DEFAULT_CONTROL_PLANE
from repo_file_io import REPO_ROOT, git_env, read_repo_text


GA_DATE_RE = re.compile(
    r"Pulse v5 entered maintenance-only support on `(?P<ga_date>\[v6-ga-date\]|\d{4}-\d{2}-\d{2})`\."
)
V5_EOS_RE = re.compile(
    r"existing v5 users until `(?P<v5_eos_date>\[v5-eos-date\]|\d{4}-\d{2}-\d{2})`\."
)
PRERELEASE_RE = re.compile(r"^(?P<stable>\d+\.\d+\.\d+)-(?:rc|alpha|beta)\.\d+$")
TAG_LITERAL_RE = re.compile(r"`(v\d+\.\d+\.\d+-rc\.\d+)`")
ACCIDENTAL_RC_TAG_DECISION_ID = "accidental-prerelease-tags-do-not-count-as-shipped-rcs"
RELEASE_DRY_RUN_WORKFLOW = ".github/workflows/release-dry-run.yml"
STABLE_REHEARSAL_INPUT_KEYS = (
    "promoted_from_tag:",
    "rollback_version:",
    "ga_date:",
    "v5_eos_date:",
)


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


def _run_git_optional(*args: str) -> str | None:
    try:
        return _run_git(*args)
    except subprocess.CalledProcessError:
        return None


def normalize_output_path(path_text: str) -> Path:
    path = Path(path_text)
    if path.is_absolute():
        return path
    return REPO_ROOT / path


def matching_rc_tags(version: str) -> list[str]:
    return [
        line.strip()
        for line in _run_git("tag", "--list", f"v{version}-rc.*", "--sort=version:refname").splitlines()
        if line.strip()
    ]


def excluded_accidental_rc_tags(status: dict, stable_version: str) -> set[str]:
    excluded: set[str] = set()
    for decision in status.get("resolved_decisions", []):
        if decision.get("id") != ACCIDENTAL_RC_TAG_DECISION_ID:
            continue
        for tag in TAG_LITERAL_RE.findall(str(decision.get("summary", ""))):
            if tag.startswith(f"v{stable_version}-rc."):
                excluded.add(tag)
    return excluded


def latest_matching_rc_tag(version: str, *, excluded_tags: set[str] | None = None) -> str | None:
    excluded = excluded_tags or set()
    tags = [
        tag
        for tag in matching_rc_tags(version)
        if tag not in excluded
    ]
    if not tags:
        return None
    return tags[-1]


def stable_candidate_version(version: str) -> str:
    match = PRERELEASE_RE.match(version)
    if match:
        return str(match.group("stable"))
    return version


def origin_default_branch() -> str:
    symbolic_ref = _run_git_optional("symbolic-ref", "refs/remotes/origin/HEAD")
    if not symbolic_ref:
        return "main"
    prefix = "refs/remotes/origin/"
    if symbolic_ref.startswith(prefix):
        return symbolic_ref[len(prefix):]
    return "main"


def branch_release_dry_run_workflow(branch: str) -> str | None:
    return _run_git_optional("show", f"origin/{branch}:{RELEASE_DRY_RUN_WORKFLOW}")


def workflow_supports_stable_rehearsal_inputs(content: str | None) -> bool:
    if not content:
        return False
    return all(key in content for key in STABLE_REHEARSAL_INPUT_KEYS)


def parse_release_dates() -> tuple[str, str]:
    content = read("docs/releases/RELEASE_NOTES_v6.md")
    ga_match = GA_DATE_RE.search(content)
    eos_match = V5_EOS_RE.search(content)
    if not ga_match or not eos_match:
        raise ValueError("could not derive GA/EOS dates from docs/releases/RELEASE_NOTES_v6.md")
    return ga_match.group("ga_date"), eos_match.group("v5_eos_date")


def render_numbered_blocks(blocks: list[list[str] | tuple[str, ...] | str]) -> list[str]:
    rendered: list[str] = []
    for idx, block in enumerate(blocks, start=1):
        lines = [block] if isinstance(block, str) else list(block)
        first, *rest = lines
        rendered.append(f"{idx}. {first}")
        for line in rest:
            rendered.append(f"   {line}")
    return rendered


def build_blocked_record(*, record_date: str) -> str:
    control_plane = read_json("docs/release-control/control_plane.json")
    status = read_json(str(DEFAULT_CONTROL_PLANE["status_path"].relative_to(REPO_ROOT)))
    version = read("VERSION").strip()
    stable_version = stable_candidate_version(version)
    version_is_prerelease = stable_version != version
    active_profile = next(
        profile for profile in control_plane["profiles"] if profile["id"] == control_plane["active_profile_id"]
    )
    active_target_id = str(control_plane["active_target_id"])
    accidental_tags = excluded_accidental_rc_tags(status, stable_version)
    rc_tag = latest_matching_rc_tag(stable_version, excluded_tags=accidental_tags)
    rc_commit = _run_git("rev-parse", rc_tag) if rc_tag else ""
    ga_date, v5_eos_date = parse_release_dates()
    target_is_ga_promotion = active_target_id == "v6-ga-promotion"
    default_branch = origin_default_branch()
    default_branch_release_dry_run = branch_release_dry_run_workflow(default_branch)
    default_branch_supports_stable_inputs = workflow_supports_stable_rehearsal_inputs(
        default_branch_release_dry_run
    )

    if version_is_prerelease:
        version_fact_lines = [
            [
                f"The active local `pulse/v6` branch currently reports `VERSION={version}`, so the",
                f"working line is still prerelease and there is not yet a governed local stable",
                f"`{stable_version}` candidate.",
            ],
            [
                "There is still no governed `RC-to-GA Rehearsal Record` proving a successful",
                f"non-publish `Release Dry Run` for the eventual stable `{stable_version}` candidate.",
            ],
        ]
    else:
        version_fact_lines = [
            [
                f"The active local `pulse/v6` branch currently reports `VERSION={version}`, so a",
                "local GA candidate exists on the governed stable line.",
            ],
            [
                "There is still no governed `RC-to-GA Rehearsal Record` proving a successful",
                f"non-publish `Release Dry Run` for the current `{version}` candidate.",
            ],
        ]

    if target_is_ga_promotion:
        target_fact_lines = [[
            f"The active control-plane target is `{active_target_id}`, so stable or GA",
            "promotion is now the governed objective for Pulse v6.",
        ]]
        if version_is_prerelease:
            why_lines = [
                "The blocker is no longer missing governance text. The remaining problem is that",
                f"the working version is still prerelease (`{version}`), and there is still no",
                f"exercised `Release Dry Run` record proving the eventual stable `{stable_version}`",
                "candidate is ready for GA-style promotion. Until that rehearsal exists, stable",
                "users would still be the first real cohort for the final promotion path.",
            ]
            required_step_lines = [
                [
                    "Push the governed `pulse/v6` branch state that is intended to become the",
                    f"stable `{stable_version}` candidate, including the eventual `VERSION={stable_version}`",
                    "change and release-control records, to `origin/pulse/v6`.",
                ],
            ]
        else:
            why_lines = [
                "The blocker is no longer missing governance text. The remaining problem is that",
                f"there is still no exercised `Release Dry Run` record proving the exact `{version}`",
                "candidate is ready for GA-style promotion. Until that rehearsal exists, stable",
                "users would still be the first real cohort for the final promotion path.",
            ]
            required_step_lines = [
                [
                    "Push the governed `pulse/v6` branch state, including the current",
                    f"`VERSION={version}` candidate and release-control records, to `origin/pulse/v6`.",
                ],
            ]
    else:
        target_fact_lines = [[
            f"The active control-plane target is still `{active_target_id}`, not",
            "`v6-ga-promotion`.",
        ]]
        if version_is_prerelease:
            why_lines = [
                "The blocker is no longer missing governance text. The remaining problem is that",
                "the control plane still treats v6 as an RC-stabilization line, the working",
                f"version is still prerelease (`{version}`), and there is still no exercised",
                f"`Release Dry Run` record proving the eventual stable `{stable_version}`",
                "candidate is ready for GA-style promotion. Until that rehearsal exists, stable",
                "users would still be the first real cohort for the final promotion path.",
            ]
            required_step_lines = [
                [
                    f"Promote the active target from `{active_target_id}` to",
                    "`v6-ga-promotion` only when that change is actually intended.",
                ],
                [
                    "Push the governed `pulse/v6` branch state that is intended to become the",
                    f"stable `{stable_version}` candidate, including the eventual `VERSION={stable_version}`",
                    "change and release-control records, to `origin/pulse/v6`.",
                ],
            ]
        else:
            why_lines = [
                "The blocker is no longer missing governance text. The remaining problem is that",
                "the control plane still treats v6 as an RC-stabilization line, and there is",
                f"still no exercised `Release Dry Run` record proving the exact `{version}`",
                "candidate is ready for GA-style promotion. Until that rehearsal exists, stable",
                "users would still be the first real cohort for the final promotion path.",
            ]
            required_step_lines = [
                [
                    f"Promote the active target from `{active_target_id}` to",
                    "`v6-ga-promotion` only when that change is actually intended.",
                ],
                [
                    "Push the governed `pulse/v6` branch state, including the current",
                    f"`VERSION={version}` candidate and release-control records, to `origin/pulse/v6`.",
                ],
            ]

    blocking_fact_lines: list[list[str] | tuple[str, ...] | str] = []
    if rc_tag:
        blocking_fact_lines.extend(
            [
                [f"The latest shipped Pulse v6 RC tag is `{rc_tag}`."],
                [f"That shipped RC tag resolves to commit `{rc_commit}`."],
            ]
        )
    else:
        blocking_fact_lines.append(["No Pulse v6 RC has shipped yet."])
        if accidental_tags:
            accidental_list = ", ".join(f"`{tag}`" for tag in sorted(accidental_tags))
            blocking_fact_lines.append(
                [
                    f"The repository contains accidental prerelease git tag history ({accidental_list}),",
                    "but those tags were never published and do not count as shipped RC lineage.",
                ]
            )

    if not default_branch_supports_stable_inputs:
        blocking_fact_lines.append(
            [
                f"GitHub still validates `Release Dry Run` workflow dispatch inputs against the",
                f"default branch `{default_branch}`, whose current workflow contract does not",
                "accept the governed stable rehearsal metadata envelope (`promoted_from_tag`,",
                "`rollback_version`, `ga_date`, and `v5_eos_date`).",
            ]
        )

    dates_are_locked = ga_date != "[v6-ga-date]" and v5_eos_date != "[v5-eos-date]"
    if dates_are_locked:
        release_date_fact_lines = [
            "`docs/releases/RELEASE_NOTES_v6.md` and",
            "`docs/release-control/v6/V5_MAINTENANCE_SUPPORT_POLICY.md` now carry the",
            "currently proposed exact dates for the eventual GA notice:",
            f"- `v6` GA date: `{ga_date}`",
            f"- `v5` end-of-support date: `{v5_eos_date}`",
        ]
        rehearsal_input_lines = [
            f"- `version={stable_version}`",
            "- the artifact-owned candidate stable tag for that rehearsal",
            "- the artifact-owned promotion channel for that rehearsal",
            "- the artifact-owned promoted RC tag for that rehearsal",
            "- the artifact-owned rollback target for that stable candidate",
            f"- `ga_date={ga_date}`",
            "- an explicit `rollback_version`",
            "- the exact derived rollback command that artifact will publish",
            f"- `v5_eos_date={v5_eos_date}`",
        ]
    else:
        release_date_fact_lines = [
            "`docs/releases/RELEASE_NOTES_v6.md` and",
            "`docs/release-control/v6/V5_MAINTENANCE_SUPPORT_POLICY.md` still leave the",
            "GA announcement dates as placeholders because no real RC lineage or GA-ready",
            "rehearsal has locked them yet:",
            f"- `v6` GA date placeholder: `{ga_date}`",
            f"- `v5` end-of-support placeholder: `{v5_eos_date}`",
        ]
        rehearsal_input_lines = [
            f"- `version={stable_version}`",
            "- the artifact-owned candidate stable tag for that rehearsal",
            "- the artifact-owned promotion channel for that rehearsal",
            "- the artifact-owned promoted RC tag for that rehearsal",
            "- the artifact-owned rollback target for that stable candidate",
            "- no exact `ga_date` is locked yet because the GA notice is still pending",
            "- no exact `v5_eos_date` is locked yet because the GA notice is still pending",
            "- an explicit `rollback_version`",
            "- the exact derived rollback command that artifact will publish",
        ]
    if rc_tag:
        rehearsal_input_lines.insert(1, f"- `promoted_from_tag={rc_tag}`")
    else:
        rehearsal_input_lines.insert(
            1,
            "- no governed `promoted_from_tag` exists yet because no RC has shipped",
        )

    required_blocks: list[list[str] | tuple[str, ...] | str] = list(required_step_lines)
    if not default_branch_supports_stable_inputs:
        required_blocks.append(
            [
                f"Land the canonical `{RELEASE_DRY_RUN_WORKFLOW}` workflow-dispatch input",
                f"contract on the default branch `{default_branch}` so GitHub accepts the",
                "governed stable rehearsal metadata envelope when dispatching from `pulse/v6`.",
            ]
        )
    if not rc_tag:
        required_blocks.append(
            [
                "Ship the first real RC through the governed prerelease release path and record",
                "its exact published prerelease tag plus rollback target and exact derived",
                "rollback command.",
            ]
        )
        required_blocks.append(
            [
                "Run `Release Dry Run` from `pulse/v6` using that published RC as",
                "`promoted_from_tag` with:",
                f"- `version={stable_version}`",
                "- an artifact-owned candidate stable tag matching that rehearsal",
                "- an artifact-owned promotion channel matching that rehearsal",
                "- an artifact-owned promoted RC tag matching that rehearsal",
                "- an artifact-owned rollback target for the stable candidate",
                "- the exact planned GA and v5 end-of-support dates for the publish notice",
                f"- `ga_date={ga_date}`" if dates_are_locked else "- an explicit `ga_date` chosen for that rehearsal",
                "- an explicit stable `rollback_version`",
                "- the exact derived rollback command that artifact will publish",
                f"- `v5_eos_date={v5_eos_date}`" if dates_are_locked else "- an explicit `v5_eos_date` chosen for that rehearsal",
            ]
        )
    else:
        required_blocks.append(
            [
                "Run `Release Dry Run` from `pulse/v6` with:",
                f"- `version={stable_version}`",
                f"- `promoted_from_tag={rc_tag}`",
                "- an artifact-owned candidate stable tag matching that rehearsal",
                "- an artifact-owned promotion channel matching that rehearsal",
                "- an artifact-owned promoted RC tag matching that rehearsal",
                "- an artifact-owned rollback target for the stable candidate",
                "- the exact planned GA and v5 end-of-support dates for the publish notice",
                f"- `ga_date={ga_date}`" if dates_are_locked else "- an explicit `ga_date` chosen for that rehearsal",
                "- an explicit stable `rollback_version`",
                "- the exact derived rollback command that artifact will publish",
                f"- `v5_eos_date={v5_eos_date}`" if dates_are_locked else "- an explicit `v5_eos_date` chosen for that rehearsal",
            ]
        )
    required_blocks.extend(
        [
            ["Capture the `rc-to-ga-rehearsal-summary` artifact and run URL."],
            [
                "Materialize the final rehearsal record from that artifact without",
                "hand-repairing any missing candidate tag, promoted RC tag, rollback",
                "target, rollback command, or GA/EOS metadata.",
            ],
            [
                "Change the gate from `blocked` only if the rehearsal passes and the rollout",
                "inputs remain explicit.",
            ],
        ]
    )

    lines = [
        "# RC-to-GA Promotion Readiness Blocked Record",
        "",
        f"- Date: `{record_date}`",
        "- Gate: `rc-to-ga-promotion-readiness`",
        "- Result: `blocked`",
        "",
        "## Blocking Facts",
        "",
        *render_numbered_blocks(
            blocking_fact_lines
            + [[
                "The governed release profile in `docs/release-control/control_plane.json`",
                f"currently declares both `prerelease_branch` and `stable_branch` as",
                f"`{active_profile['stable_branch']}`.",
            ]]
            + target_fact_lines
            + version_fact_lines
            + [release_date_fact_lines]
            + [[
                "There is still no governed `Release Dry Run` artifact or rehearsal record",
                "exercising stable inputs for:",
                *rehearsal_input_lines,
            ]]
        ),
        "",
        "## Why The Gate Cannot Be Cleared Yet",
        "",
        *why_lines,
        "",
        "## Required Unblock Steps",
        "",
        *render_numbered_blocks(required_blocks),
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
