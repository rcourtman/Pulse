#!/usr/bin/env python3
"""Validate the explicit mobile-release decision for governed release dispatch."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path


VALID_DECISIONS = (
    "no-mobile-impact",
    "existing-mobile-build-compatible",
    "mobile-candidate-uploaded",
    "mobile-candidate-required",
)

EVIDENCE_REQUIRED_DECISIONS = {
    "existing-mobile-build-compatible",
    "mobile-candidate-uploaded",
}

BLOCKING_DECISION = "mobile-candidate-required"


def normalize_decision(value: str) -> str:
    return value.strip().lower().replace("_", "-")


def validate_mobile_release_decision(
    decision: str,
    evidence: str,
) -> list[str]:
    normalized = normalize_decision(decision)
    trimmed_evidence = evidence.strip()
    errors: list[str] = []

    if not normalized:
        return [
            "mobile_release_decision is required. Choose one of: "
            + ", ".join(VALID_DECISIONS)
        ]

    if normalized not in VALID_DECISIONS:
        return [
            f"unknown mobile_release_decision {decision!r}. Choose one of: "
            + ", ".join(VALID_DECISIONS)
        ]

    if normalized == BLOCKING_DECISION:
        errors.append(
            "mobile-candidate-required is a blocking release state. "
            "Build and submit the required mobile candidate first, then rerun "
            "with mobile-candidate-uploaded and evidence."
        )

    if normalized in EVIDENCE_REQUIRED_DECISIONS and not trimmed_evidence:
        errors.append(
            f"mobile_release_evidence is required for {normalized}. "
            "Record the compatible build proof or the uploaded candidate build."
        )

    return errors


def _run_git(repo: Path, args: list[str]) -> str:
    try:
        completed = subprocess.run(
            ["git", "-C", str(repo), *args],
            check=True,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
    except (OSError, subprocess.CalledProcessError):
        return ""
    return completed.stdout.strip()


def _read_json(path: Path) -> dict[str, object]:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}


def summarize_mobile_repo(repo: Path) -> str:
    lines: list[str] = [f"Pulse Mobile release context: {repo}"]

    if not (repo / ".git").exists():
        lines.append("  pulse-mobile repo not found; make an explicit mobile release decision manually.")
        return "\n".join(lines)

    app = _read_json(repo / "app.json")
    expo = app.get("expo") if isinstance(app.get("expo"), dict) else {}
    ios = expo.get("ios") if isinstance(expo.get("ios"), dict) else {}
    android = expo.get("android") if isinstance(expo.get("android"), dict) else {}
    readiness = _read_json(repo / "store" / "release-readiness.json")
    current = readiness.get("currentCandidate") if isinstance(readiness.get("currentCandidate"), dict) else {}

    lines.append(
        "  app.json: "
        f"version={expo.get('version', '-')}, "
        f"iosBuildNumber={ios.get('buildNumber', '-')}, "
        f"androidVersionCode={android.get('versionCode', '-')}"
    )
    lines.append(
        "  release-readiness currentCandidate: "
        f"marketingVersion={current.get('marketingVersion', '-')}, "
        f"iosBuildNumber={current.get('iosBuildNumber', '-')}, "
        f"androidVersionCode={current.get('androidVersionCode', '-')}, "
        f"recordedOn={current.get('recordedOn', '-')}"
    )

    recorded_on = str(current.get("recordedOn") or "").strip()
    if recorded_on:
        recent = _run_git(
            repo,
            ["log", f"--since={recorded_on}T00:00:00", "--oneline", "--max-count=8"],
        )
        if recent:
            lines.append("  commits since recorded candidate:")
            lines.extend(f"    {line}" for line in recent.splitlines())
        else:
            lines.append("  commits since recorded candidate: none")
    else:
        recent = _run_git(repo, ["log", "--oneline", "--max-count=5"])
        if recent:
            lines.append("  recent commits:")
            lines.extend(f"    {line}" for line in recent.splitlines())

    return "\n".join(lines)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Validate the mobile impact decision required by release dispatch."
    )
    parser.add_argument("--decision", default="")
    parser.add_argument("--evidence", default="")
    parser.add_argument("--version", default="")
    parser.add_argument("--mobile-repo", default="")
    parser.add_argument("--summary-only", action="store_true")
    parser.add_argument("--github-annotations", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()

    if args.mobile_repo:
        print(summarize_mobile_repo(Path(args.mobile_repo)))
        if args.summary_only:
            return 0

    errors = validate_mobile_release_decision(args.decision, args.evidence)
    if errors:
        for error in errors:
            if args.github_annotations:
                print(f"::error::{error}", file=sys.stderr)
            else:
                print(f"ERROR: {error}", file=sys.stderr)
        return 1

    normalized = normalize_decision(args.decision)
    version_context = f" for {args.version}" if args.version else ""
    print(f"[OK] Mobile release decision{version_context}: {normalized}")
    if args.evidence.strip():
        print(f"[OK] Mobile release evidence: {args.evidence.strip()}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
