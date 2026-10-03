#!/usr/bin/env python3
"""Helpers for the daily dependency advisory watch workflow.

`lines` turns `git ls-remote --heads` output and the newest stable release tag
into the JSON list of branches the watch audits: `main` plus every
`release/v<major>.<minor>` line at or newer than the newest stable's line.
Without a usable stable tag every release line is kept, so a lookup failure
widens the watch instead of silently narrowing it.

`summary` reads the output of `scripts/npm-audit-retry.sh` for one branch and
writes the step summary plus one failure annotation naming the branch, the
advisories and the fix. Registry-sourced text never reaches the annotation:
only the validated branch name, package names and GHSA identifiers do.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


LINE_RE = re.compile(r"^release/v(\d+)\.(\d+)$")
STABLE_RE = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")
GHSA_RE = re.compile(r"GHSA(?:-[23456789cfghjmpqrvwx]{4}){3}")
PACKAGE_RE = re.compile(r"^(?:@[a-z0-9][a-z0-9._~-]*/)?[a-z0-9][a-z0-9._~-]*$")
FINDING_PREFIX = "audit finding "
UNREACHABLE_MARKER = "could not reach the advisory endpoint"
FIX_STEPS = (
    "In frontend-modern on that branch run `npm audit fix --package-lock-only`, "
    "then raise the matching floors in "
    "`frontend-modern/src/security/__tests__/dependencySecurity.test.ts` "
    "and land both on the branch."
)


def release_lines(ls_remote: str, latest_stable: str) -> list[str]:
    lines: dict[tuple[int, int], str] = {}
    for row in ls_remote.splitlines():
        parts = row.split()
        if len(parts) != 2 or not parts[1].startswith("refs/heads/"):
            continue
        branch = parts[1][len("refs/heads/"):]
        match = LINE_RE.fullmatch(branch)
        if match:
            lines[(int(match.group(1)), int(match.group(2)))] = branch
    stable = STABLE_RE.fullmatch(latest_stable.strip())
    floor = (int(stable.group(1)), int(stable.group(2))) if stable else (0, 0)
    active = [lines[key] for key in sorted(lines) if key >= floor]
    return ["main", *active]


def parse_findings(log: str) -> list[dict]:
    findings = []
    for row in log.splitlines():
        row = row.strip()
        if not row.startswith(FINDING_PREFIX):
            continue
        try:
            finding = json.loads(row[len(FINDING_PREFIX):])
        except ValueError:
            continue
        if isinstance(finding, dict):
            findings.append(finding)
    return findings


def advisory_ids(finding: dict) -> list[str]:
    found = set()
    for item in finding.get("via") or []:
        if isinstance(item, dict):
            found.update(GHSA_RE.findall(str(item.get("url") or "")))
    return sorted(found)


def safe_package(name: object) -> str:
    text = str(name or "")
    return text if PACKAGE_RE.fullmatch(text) else "unnamed-package"


def markdown_cell(value: object) -> str:
    text = " ".join(str(value or "").split())[:160]
    for character in "\\|`*_[]<>":
        text = text.replace(character, "\\" + character)
    return text


def render(branch: str, outcome: str, log: str) -> tuple[str, str | None]:
    """Return the step summary and the annotation (None when clean)."""
    findings = parse_findings(log)
    header = f"## Frontend dependency audit: `{branch}`\n\n"
    if outcome == "success":
        note = ""
        if UNREACHABLE_MARKER in log:
            note = " The advisory endpoint did not answer, so this run proves nothing new."
        return header + f"No advisories reported.{note}\n", None
    if not findings:
        if UNREACHABLE_MARKER in log:
            text = (f"The npm advisory endpoint could not be reached for {branch}. "
                    "No advisory was observed; rerun the watch once npm recovers.")
        else:
            text = (f"The frontend dependency audit for {branch} failed without an advisory finding. "
                    "Read the job log: install or audit tooling broke before a verdict.")
        return header + text + "\n", text
    rows = ["| Package | Severity | Advisories | Fix available |", "| --- | --- | --- | --- |"]
    identifiers: set[str] = set()
    packages = []
    for finding in findings:
        package = safe_package(finding.get("name"))
        packages.append(package)
        ids = advisory_ids(finding)
        identifiers.update(ids)
        fix = finding.get("fixAvailable")
        if isinstance(fix, dict):
            fix_text = f"{fix.get('name', '')} {fix.get('version', '')}".strip()
            if fix.get("isSemVerMajor"):
                fix_text += " (major)"
        else:
            fix_text = "yes" if fix is True else "no"
        titles = [str(item.get("title") or "") for item in finding.get("via") or [] if isinstance(item, dict)]
        rows.append("| " + " | ".join((
            markdown_cell(package), markdown_cell(finding.get("severity")),
            markdown_cell(", ".join(ids) or "; ".join(titles) or "via dependency"),
            markdown_cell(fix_text))) + " |")
    advisories = ", ".join(sorted(identifiers)) or "see job log"
    body = (header
            + f"New npm advisories block every pull request and release on `{branch}` through the "
            + "required *Audit complete frontend dependency graph* check in build-and-test.yml.\n\n"
            + "\n".join(rows)
            + "\n\n**Suggested fix.** " + FIX_STEPS + "\n")
    annotation = (f"Frontend dependency advisory on {branch}: {advisories} in "
                  f"{', '.join(sorted(set(packages)))}. This blocks every PR and release on {branch}. "
                  "The fix is a lockfile advisory floor bump on that line: npm audit fix --package-lock-only "
                  "in frontend-modern, then raise the floors in dependencySecurity.test.ts.")
    return body, annotation


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    lines = commands.add_parser("lines")
    lines.add_argument("--heads", required=True, type=Path, help="git ls-remote --heads output")
    lines.add_argument("--latest-stable", default="", help="newest stable release tag, if known")
    summary = commands.add_parser("summary")
    summary.add_argument("--branch", required=True)
    summary.add_argument("--outcome", required=True)
    summary.add_argument("--log", required=True, type=Path)
    summary.add_argument("--summary-file", required=True, type=Path)
    args = parser.parse_args(argv)

    if args.command == "lines":
        print(json.dumps(release_lines(args.heads.read_text(encoding="utf-8"), args.latest_stable)))
        return 0

    if args.branch != "main" and not LINE_RE.fullmatch(args.branch):
        print(f"refusing unexpected branch name {args.branch!r}", file=sys.stderr)
        return 2
    try:
        log = args.log.read_text(encoding="utf-8", errors="replace")
    except OSError:
        log = ""
    body, annotation = render(args.branch, args.outcome, log)
    with args.summary_file.open("a", encoding="utf-8") as stream:
        stream.write(body)
    if annotation:
        print(f"::error title=Dependency advisory watch ({args.branch})::{annotation}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
