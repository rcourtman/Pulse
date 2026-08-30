#!/usr/bin/env python3
"""Fail closed when GitHub Actions trust inputs become mutable or implicit."""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path


ACTION_SHA_RE = re.compile(r"^[0-9a-f]{40}$")
CONTAINER_DIGEST_RE = re.compile(r"^docker://.+@sha256:[0-9a-f]{64}$")
HOSTED_LATEST_RE = re.compile(r"\b(?:ubuntu|windows|macos)-latest\b")
USES_RE = re.compile(r"^\s*(?:-\s*)?uses:\s*([^\s#]+)")
CHECKOUT_PREFIX = "actions/checkout@"
WRITE_CREDENTIAL_RATIONALE = "# required: authenticated git writes"


@dataclass(frozen=True)
class Finding:
    path: Path
    line: int
    message: str

    def render(self) -> str:
        return f"{self.path}:{self.line}: {self.message}"


def _indent(line: str) -> int:
    return len(line) - len(line.lstrip())


def _checkout_block(lines: list[str], uses_index: int) -> list[tuple[int, str]]:
    """Return lines belonging to the checkout step after its uses declaration."""
    uses_indent = _indent(lines[uses_index])
    block: list[tuple[int, str]] = []
    for index in range(uses_index + 1, len(lines)):
        line = lines[index]
        stripped = line.strip()
        if stripped and (
            _indent(line) < uses_indent
            or (_indent(line) == uses_indent and stripped.startswith("- "))
        ):
            break
        block.append((index, line))
    return block


def audit_workflow(path: Path) -> list[Finding]:
    lines = path.read_text(encoding="utf-8").splitlines()
    findings: list[Finding] = []

    for index, line in enumerate(lines):
        line_number = index + 1
        code = line.split("#", 1)[0]
        if HOSTED_LATEST_RE.search(code):
            findings.append(
                Finding(
                    path,
                    line_number,
                    "mutable hosted runner label; use an explicit dated image",
                )
            )

        match = USES_RE.search(code)
        if not match:
            continue
        dependency = match.group(1).strip("'\"")
        if dependency.startswith("./"):
            continue
        if dependency.startswith("docker://"):
            if not CONTAINER_DIGEST_RE.fullmatch(dependency):
                findings.append(
                    Finding(
                        path,
                        line_number,
                        "container action is not pinned to a sha256 digest",
                    )
                )
            continue

        owner_and_action, separator, ref = dependency.rpartition("@")
        if not separator or "/" not in owner_and_action or not ACTION_SHA_RE.fullmatch(ref):
            findings.append(
                Finding(
                    path,
                    line_number,
                    "remote action or reusable workflow is not pinned to a full commit SHA",
                )
            )
            continue

        # GitHub repository names are case-insensitive, so normalize before
        # applying checkout-specific credential controls.
        if not dependency.lower().startswith(CHECKOUT_PREFIX):
            continue
        credential_settings = [
            (block_index, block_line)
            for block_index, block_line in _checkout_block(lines, index)
            if re.match(r"^\s*persist-credentials\s*:", block_line)
        ]
        if len(credential_settings) != 1:
            findings.append(
                Finding(
                    path,
                    line_number,
                    "checkout must set persist-credentials explicitly exactly once",
                )
            )
            continue

        setting_index, setting = credential_settings[0]
        value_match = re.match(
            r"^\s*persist-credentials\s*:\s*(true|false)\b", setting
        )
        if not value_match:
            findings.append(
                Finding(
                    path,
                    setting_index + 1,
                    "persist-credentials must be the literal true or false",
                )
            )
        elif value_match.group(1) == "true" and WRITE_CREDENTIAL_RATIONALE not in setting:
            findings.append(
                Finding(
                    path,
                    setting_index + 1,
                    f"persisted checkout credentials require {WRITE_CREDENTIAL_RATIONALE}",
                )
            )

    return findings


def audit_directory(workflow_directory: Path) -> list[Finding]:
    findings: list[Finding] = []
    paths = sorted(workflow_directory.glob("*.yml")) + sorted(
        workflow_directory.glob("*.yaml")
    )
    for path in paths:
        findings.extend(audit_workflow(path))
    return findings


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "workflow_directory",
        nargs="?",
        type=Path,
        default=Path(__file__).resolve().parents[1] / ".github" / "workflows",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if not args.workflow_directory.is_dir():
        print(f"workflow directory not found: {args.workflow_directory}", file=sys.stderr)
        return 2
    findings = audit_directory(args.workflow_directory)
    if findings:
        for finding in findings:
            print(finding.render(), file=sys.stderr)
        print(f"GitHub Actions trust validation failed ({len(findings)} finding(s)).", file=sys.stderr)
        return 1
    print("GitHub Actions trust validation passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
