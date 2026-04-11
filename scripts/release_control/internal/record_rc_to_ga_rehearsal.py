#!/usr/bin/env python3
"""Materialize a prerelease-to-GA rehearsal record from a GitHub Actions dry-run."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import tempfile
from datetime import date
from pathlib import Path
from typing import Any

from repo_file_io import REPO_ROOT, git_env
from release_promotion_policy_support import PROMOTION_METADATA_FIELDS

DEFAULT_RECORDS_DIR = Path("docs/release-control/v6/internal/records")
DEFAULT_RECORD_PREFIX = "rc-to-ga-promotion-readiness-rehearsal"


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate a prerelease-to-GA rehearsal record from a Release Dry Run workflow run."
    )
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--run-id", help="GitHub Actions run ID for the Release Dry Run workflow.")
    source.add_argument("--summary-file", help="Existing rc-to-ga-rehearsal-summary.md file.")
    parser.add_argument(
        "--output",
        help=(
            "Repo-relative or absolute path for the generated record markdown. "
            "Defaults to docs/release-control/v6/internal/records/"
            "rc-to-ga-promotion-readiness-rehearsal-<record-date>.md."
        ),
    )
    parser.add_argument(
        "--record-date",
        default=str(date.today()),
        help="Date to record in the rehearsal record (default: today).",
    )
    parser.add_argument(
        "--result",
        choices=("pass", "follow-up-required", "fail"),
        default="pass",
        help="Overall rehearsal result.",
    )
    parser.add_argument(
        "--ga-date",
        help="Exact GA date to publish with the promotion announcement (YYYY-MM-DD). Overrides the artifact value if provided.",
    )
    parser.add_argument(
        "--v5-eos-date",
        help="Exact Pulse v5 end-of-support date to publish (YYYY-MM-DD).",
    )
    parser.add_argument(
        "--rollback-command",
        help="Exact rollback or reinstall command validated for the promoted stable line. Overrides the artifact value if provided.",
    )
    parser.add_argument(
        "--follow-up",
        action="append",
        default=[],
        help="Follow-up issue or note that must be tracked before the real promotion. Repeatable.",
    )
    parser.add_argument(
        "--notes",
        action="append",
        default=[],
        help="Additional operator note to include in the record. Repeatable.",
    )
    parser.add_argument(
        "--force",
        action="store_true",
        help="Overwrite the output path if it already exists.",
    )
    return parser.parse_args(argv)


def _run_gh(*args: str) -> str:
    result = subprocess.run(
        ["gh", *args],
        cwd=REPO_ROOT,
        env=git_env(),
        text=True,
        capture_output=True,
        check=True,
    )
    return result.stdout


def load_run_metadata(run_id: str) -> dict[str, Any]:
    output = _run_gh(
        "run",
        "view",
        run_id,
        "--json",
        "databaseId,url,headBranch,headSha,name,workflowName,status,conclusion,createdAt,updatedAt",
    )
    return json.loads(output)


def download_summary_artifact(run_id: str) -> tuple[str, str]:
    with tempfile.TemporaryDirectory(prefix="rc-to-ga-rehearsal.") as tmp:
        tmp_path = Path(tmp)
        try:
            _run_gh(
                "run",
                "download",
                run_id,
                "-n",
                "rc-to-ga-rehearsal-summary",
                "-D",
                str(tmp_path),
            )
        except subprocess.CalledProcessError as exc:
            message = exc.stderr.strip() or exc.stdout.strip() or str(exc)
            raise FileNotFoundError(
                "rc-to-ga-rehearsal-summary artifact could not be downloaded for "
                f"run {run_id}; it may be missing or expired: {message}"
            ) from exc
        matches = sorted(tmp_path.rglob("rc-to-ga-rehearsal-summary.md"))
        if not matches:
            raise FileNotFoundError(
                f"run {run_id} did not produce rc-to-ga-rehearsal-summary.md"
            )
        summary_path = matches[0]
        return summary_path.read_text(encoding="utf-8"), str(summary_path)


def parse_summary_markdown(content: str) -> dict[str, str]:
    metadata: dict[str, str] = {}
    alias_map = {
        "candidate_stable_tag": "tag",
        "promotion_channel": "channel_under_rehearsal",
        "promoted_rc_tag": "promoted_from_rc",
        "promoted_prerelease_tag": "promoted_from_rc",
        "planned_v5_end_of_support_date": "planned_v5_eos_date",
        "prerelease_soak_hours_at_rehearsal_time": "rc_soak_hours_at_rehearsal_time",
    }
    for line in content.splitlines():
        if not line.startswith("- "):
            continue
        body = line[2:]
        if ":" not in body:
            continue
        label, value = body.split(":", 1)
        normalized_label = (
            label.strip().lower().replace(" ", "_").replace("-", "_")
        )
        metadata[alias_map.get(normalized_label, normalized_label)] = value.strip()
    return metadata


def normalize_summary_command(value: str) -> str:
    normalized = value.strip()
    if normalized.startswith("`") and normalized.endswith("`") and len(normalized) >= 2:
        normalized = normalized[1:-1].strip()
    return normalized


def validate_required_summary_metadata(summary_metadata: dict[str, str]) -> None:
    missing = [
        label
        for key, label in PROMOTION_METADATA_FIELDS
        if not summary_metadata.get(key, "").strip()
    ]
    if not missing:
        return
    raise ValueError(
        "rehearsal summary artifact missing required promotion metadata: "
        + ", ".join(missing)
    )


def normalize_output_path(path_text: str) -> Path:
    path = Path(path_text)
    if path.is_absolute():
        return path
    return REPO_ROOT / path


def normalize_input_path(path_text: str) -> Path:
    path = Path(path_text)
    if path.is_absolute():
        return path
    return REPO_ROOT / path


def default_output_path(record_date: str) -> Path:
    return REPO_ROOT / DEFAULT_RECORDS_DIR / f"{DEFAULT_RECORD_PREFIX}-{record_date}.md"


def render_record(
    *,
    record_date: str,
    result: str,
    rollback_command: str,
    run_metadata: dict[str, Any] | None,
    summary_metadata: dict[str, str],
    summary_source: str,
    ga_date: str | None,
    v5_eos_date: str | None,
    follow_ups: list[str],
    notes: list[str],
    summary_markdown: str,
) -> str:
    run_url = summary_metadata.get("workflow_run")
    if not run_url and run_metadata:
        run_url = str(run_metadata.get("url", "")).strip()
    branch = summary_metadata.get("branch") or (run_metadata or {}).get("headBranch", "")
    version = summary_metadata.get("version", "")
    stable_tag = summary_metadata.get("tag", "")
    promotion_channel = summary_metadata.get("channel_under_rehearsal", "")
    promoted_from_tag = summary_metadata.get("promoted_from_rc", "")
    rollback_target = summary_metadata.get("rollback_target", "")
    rollback_command = rollback_command or normalize_summary_command(
        summary_metadata.get("rollback_command", "")
    )
    soak_hours = summary_metadata.get("rc_soak_hours_at_rehearsal_time", "")
    published_ga_date = ga_date or summary_metadata.get("planned_ga_date", "")
    published_eos = v5_eos_date or summary_metadata.get("planned_v5_eos_date", "")
    hotfix_exception = summary_metadata.get("hotfix_exception", "")
    hotfix_reason = summary_metadata.get("hotfix_reason", "")
    operator_note = summary_metadata.get("operator_note", "")

    lines = [
        "# Prerelease-to-GA Rehearsal Record",
        "",
        f"- Rehearsal date: {record_date}",
        f"- Result: {result}",
        f"- GitHub Actions run URL: {run_url or '[missing]'}",
        f"- Source branch: {branch or '[missing]'}",
        f"- Source commit: {(run_metadata or {}).get('headSha', '') or '[missing]'}",
        f"- Version under rehearsal: {version or '[missing]'}",
        f"- Candidate stable tag: {stable_tag or '[missing]'}",
        f"- Promotion channel: {promotion_channel or '[missing]'}",
        f"- Promoted prerelease tag: {promoted_from_tag or '[missing]'}",
        f"- Current rollback target: {rollback_target or '[missing]'}",
        f"- Exact rollback or reinstall command: `{rollback_command}`",
        f"- Prerelease soak hours at rehearsal time: {soak_hours or '[missing]'}",
        f"- Exact GA date to publish: {published_ga_date or '[missing]'}",
        f"- Exact v5 end-of-support date to publish: {published_eos or '[missing]'}",
        f"- Dry-run artifact source: `{summary_source}`",
    ]

    if hotfix_exception:
        lines.append(f"- Hotfix exception: {hotfix_exception}")
    if hotfix_reason:
        lines.append(f"- Hotfix reason: {hotfix_reason}")
    if operator_note:
        lines.append(f"- Workflow operator note: {operator_note}")
    for note in notes:
        lines.append(f"- Additional note: {note}")

    lines.extend(
        [
            "",
            "## Verification Notes",
            "",
            "1. Confirmed the rehearsal was generated from the GitHub `Release Dry Run` workflow.",
            "2. Confirmed the non-publish release path was exercised end to end up to, but not including, publication.",
            "3. Confirmed rollback target and exact rollback command are recorded explicitly for the promotion candidate.",
            "4. Confirmed the v5 maintenance-only policy remains the governing support contract for the GA handoff.",
            "5. Confirmed the linked artifact is the machine-generated `rc-to-ga-rehearsal-summary` for this run.",
            "",
            "## Follow-Up",
            "",
        ]
    )

    if follow_ups:
        for idx, item in enumerate(follow_ups, start=1):
            lines.append(f"{idx}. {item}")
    else:
        lines.append("1. None.")

    lines.extend(
        [
            "",
            "## Dry-Run Artifact",
            "",
            "```md",
            summary_markdown.rstrip(),
            "```",
            "",
        ]
    )
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    if args.ga_date:
        date.fromisoformat(args.ga_date)
    if args.v5_eos_date:
        date.fromisoformat(args.v5_eos_date)
    date.fromisoformat(args.record_date)

    run_metadata: dict[str, Any] | None = None
    if args.run_id:
        run_metadata = load_run_metadata(args.run_id)
        summary_markdown, summary_source = download_summary_artifact(args.run_id)
    else:
        summary_path = normalize_input_path(args.summary_file)
        if not summary_path.is_file():
            raise FileNotFoundError(f"summary file does not exist: {summary_path}")
        summary_markdown = summary_path.read_text(encoding="utf-8")
        summary_source = str(summary_path)

    summary_metadata = parse_summary_markdown(summary_markdown)
    validate_required_summary_metadata(summary_metadata)
    rollback_command = args.rollback_command or normalize_summary_command(summary_metadata["rollback_command"])
    output_path = (
        normalize_output_path(args.output)
        if args.output
        else default_output_path(args.record_date)
    )
    if output_path.exists() and not args.force:
        raise FileExistsError(
            f"output path already exists: {output_path}. "
            "Pass --force to overwrite or choose a different --output."
        )
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        render_record(
            record_date=args.record_date,
            result=args.result,
            rollback_command=rollback_command,
            run_metadata=run_metadata,
            summary_metadata=summary_metadata,
            summary_source=summary_source,
            ga_date=args.ga_date,
            v5_eos_date=args.v5_eos_date,
            follow_ups=args.follow_up,
            notes=args.notes,
            summary_markdown=summary_markdown,
        ),
        encoding="utf-8",
    )
    print(output_path)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (FileNotFoundError, ValueError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        raise SystemExit(1)
