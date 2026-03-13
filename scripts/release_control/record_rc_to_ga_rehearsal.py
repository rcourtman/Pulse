#!/usr/bin/env python3
"""Materialize an RC-to-GA rehearsal record from a GitHub Actions dry-run."""

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


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate an RC-to-GA rehearsal record from a Release Dry Run workflow run."
    )
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--run-id", help="GitHub Actions run ID for the Release Dry Run workflow.")
    source.add_argument("--summary-file", help="Existing rc-to-ga-rehearsal-summary.md file.")
    parser.add_argument(
        "--output",
        required=True,
        help="Repo-relative or absolute path for the generated record markdown.",
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
        help="Exact GA date to publish with the promotion announcement (YYYY-MM-DD).",
    )
    parser.add_argument(
        "--v5-eos-date",
        help="Exact Pulse v5 end-of-support date to publish (YYYY-MM-DD).",
    )
    parser.add_argument(
        "--rollback-command",
        required=True,
        help="Exact rollback or reinstall command validated for the promoted stable line.",
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
    for line in content.splitlines():
        if not line.startswith("- "):
            continue
        body = line[2:]
        if ":" not in body:
            continue
        label, value = body.split(":", 1)
        metadata[label.strip().lower().replace(" ", "_")] = value.strip()
    return metadata


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
    promoted_from_tag = summary_metadata.get("promoted_from_rc", "")
    rollback_target = summary_metadata.get("rollback_target", "")
    soak_hours = summary_metadata.get("rc_soak_hours_at_rehearsal_time", "")
    published_eos = v5_eos_date or summary_metadata.get("planned_v5_eos_date", "")
    hotfix_exception = summary_metadata.get("hotfix_exception", "")
    hotfix_reason = summary_metadata.get("hotfix_reason", "")
    operator_note = summary_metadata.get("operator_note", "")

    lines = [
        "# RC-to-GA Rehearsal Record",
        "",
        f"- Rehearsal date: {record_date}",
        f"- Result: {result}",
        f"- GitHub Actions run URL: {run_url or '[missing]'}",
        f"- Source branch: {branch or '[missing]'}",
        f"- Source commit: {(run_metadata or {}).get('headSha', '') or '[missing]'}",
        f"- Version under rehearsal: {version or '[missing]'}",
        f"- Candidate stable tag: {stable_tag or '[missing]'}",
        f"- Promoted RC tag: {promoted_from_tag or '[missing]'}",
        f"- Current rollback target: {rollback_target or '[missing]'}",
        f"- Exact rollback or reinstall command: `{rollback_command}`",
        f"- RC soak hours at rehearsal time: {soak_hours or '[missing]'}",
        f"- Exact GA date to publish: {ga_date or '[pending]'}",
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
    output_path = normalize_output_path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        render_record(
            record_date=args.record_date,
            result=args.result,
            rollback_command=args.rollback_command,
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
