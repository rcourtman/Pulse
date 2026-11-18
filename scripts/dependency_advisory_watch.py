#!/usr/bin/env python3
"""Helpers for the daily dependency advisory watch workflow.

`summary` reads the output of `scripts/npm-audit-retry.sh` for main and writes
the step summary plus one failure annotation naming the advisories and the
fix. Releases come only from main; retained release branches are not repair
targets. Registry-sourced text never reaches the annotation: only the fixed
branch name, validated package names and GHSA identifiers do.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


GHSA_RE = re.compile(r"GHSA(?:-[23456789cfghjmpqrvwx]{4}){3}")
PACKAGE_RE = re.compile(r"^(?:@[a-z0-9][a-z0-9._~-]*/)?[a-z0-9][a-z0-9._~-]*$")
FINDING_PREFIX = "audit finding "
UNREACHABLE_MARKER = "could not reach the advisory endpoint"
FIX_STEPS = (
    "Start with the packages that report the advisory itself; other rows may "
    "inherit it through dependencies. Confirm patched versions and qualify the "
    "smallest compatible lockfile repair in `frontend-modern`. npm's suggestions "
    "are unverified: do not force an update or downgrade below existing floors. "
    "Raise the matching floors in "
    "`frontend-modern/src/security/__tests__/dependencySecurity.test.ts` "
    "and land both on main. A source fix reaches users only in a containing release."
)


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


def advisory_ids(via: list) -> list[str]:
    found = set()
    for item in via:
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
    rows = ["| Package | Severity | Advisory or affected dependency | npm suggestion (unverified) |",
            "| --- | --- | --- | --- |"]
    origins: dict[str, set[str]] = {}
    advisory_packages: set[str] = set()
    packages: set[str] = set()
    for finding in findings:
        package = safe_package(finding.get("name"))
        packages.add(package)
        via = finding.get("via")
        if not isinstance(via, list):
            via = []
        ids = advisory_ids(via)
        for identifier in ids:
            origins.setdefault(identifier, set()).add(package)
            advisory_packages.add(package)
        affected_dependencies = sorted({
            item for item in via if isinstance(item, str) and PACKAGE_RE.fullmatch(item)
        })
        titles = [str(item.get("title") or "") for item in via
                  if isinstance(item, dict) and item.get("title")]
        details = [", ".join(ids) or "; ".join(titles)]
        if affected_dependencies:
            details.append("via " + ", ".join(affected_dependencies))
        fix = finding.get("fixAvailable")
        if isinstance(fix, dict):
            fix_text = f"{fix.get('name', '')} {fix.get('version', '')}".strip() or "not reported"
            if fix.get("isSemVerMajor") is True:
                fix_text += " (major)"
        else:
            fix_text = "available (version not reported)" if fix is True else "none reported"
        rows.append("| " + " | ".join((
            markdown_cell(package), markdown_cell(finding.get("severity")),
            markdown_cell("; ".join(filter(None, details)) or "see job log"),
            markdown_cell(fix_text))) + " |")
    if origins:
        advisories = "; ".join(
            f"{identifier} in {', '.join(sorted(packages_for_id))}"
            for identifier, packages_for_id in sorted(origins.items())
        )
        other_packages = len(packages - advisory_packages)
        if other_packages:
            advisories += f". Other affected packages: {other_packages} (see step summary)"
    else:
        advisories = f"see job log (affected packages: {', '.join(sorted(packages))})"
    body = (header
            + "The strict daily audit found dependency advisories on main. "
            + "Pull requests that change the graph must pass the required "
            + "*Audit complete frontend dependency graph* check; unchanged graphs only warn.\n\n"
            + "\n".join(rows)
            + "\n\n**Suggested fix.** " + FIX_STEPS + "\n")
    annotation = (f"Frontend dependency advisory on {branch}: {advisories}. The strict daily watch failed. "
                  "Qualify a compatible lockfile repair and matching dependencySecurity.test.ts floors on main.")
    return body, annotation


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    summary = commands.add_parser("summary")
    summary.add_argument("--branch", required=True)
    summary.add_argument("--outcome", required=True)
    summary.add_argument("--log", required=True, type=Path)
    summary.add_argument("--summary-file", required=True, type=Path)
    args = parser.parse_args(argv)

    if args.branch != "main":
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
