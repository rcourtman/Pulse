#!/usr/bin/env python3
"""Render a publish-safe GitHub release body from the current RC packet."""

from __future__ import annotations

import argparse
import re
from pathlib import Path


def _normalize_newlines(text: str) -> str:
    return text.replace("\r\n", "\n").replace("\r", "\n")


def _replace_draft_heading(text: str, version: str) -> str:
    lines = text.splitlines()
    for index, line in enumerate(lines):
        if line.startswith("# ") and "Draft Release Notes" in line:
            lines[index] = f"# Pulse v{version} Release Notes"
            return "\n".join(lines)
    return text


def _drop_draft_disclaimer(text: str) -> str:
    pattern = re.compile(
        r"(?:^|\n)_?Draft only\. Do not treat this as published.*?(?:\n{2,}|$)",
        re.DOTALL,
    )
    return pattern.sub("\n\n", text, count=1)


def _drop_level_two_sections(text: str, headings: set[str]) -> str:
    lines = text.splitlines()
    kept: list[str] = []
    skip = False

    for line in lines:
        stripped = line.strip()
        if stripped in headings:
            skip = True
            continue
        if skip and stripped.startswith("## "):
            skip = False
        if not skip:
            kept.append(line)

    return "\n".join(kept)


def _drop_draft_packet_links(text: str) -> str:
    return "\n".join(line for line in text.splitlines() if "_DRAFT.md" not in line)


def _collapse_blank_lines(text: str) -> str:
    text = _normalize_newlines(text).strip()
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text + "\n"


def sanitize_release_notes(raw_text: str, version: str) -> str:
    text = _normalize_newlines(raw_text)
    text = _replace_draft_heading(text, version)
    text = _drop_draft_disclaimer(text)
    text = _drop_level_two_sections(text, {"## Installation", "## Promotion Metadata"})
    text = _drop_draft_packet_links(text)
    return _collapse_blank_lines(text)


def build_installation_section(version: str) -> str:
    return "\n".join(
        [
            "## Installation",
            "",
            "**Docker (recommended):**",
            "```bash",
            f"docker pull rcourtman/pulse:{version}",
            "```",
            "",
            "**Docker Compose:**",
            f"Update your `docker-compose.yml` to use `rcourtman/pulse:{version}`",
            "",
            "See the [Installation Guide](https://github.com/rcourtman/Pulse#installation) for complete setup instructions.",
            "",
            "Paid Pulse Pro, Relay, and eligible legacy customers: public GitHub release assets and the public `rcourtman/pulse` Docker image are community builds. They do not include the private Pulse Pro runtime hooks. Use https://pulserelay.pro/download.html with your activation key to get the private Pulse Pro Docker image or Linux/LXC archive.",
        ]
    )


def build_promotion_metadata_section(args: argparse.Namespace) -> str:
    lines = [
        "## Promotion Metadata",
        "",
        f"- Promotion channel: {args.promotion_channel}",
        f"- Candidate stable tag: {args.candidate_tag}",
        f"- Promoted prerelease tag: {args.promoted_prerelease_tag or 'n/a'}",
        f"- Rollback target: {args.rollback_target}",
        f"- Rollback command: `{args.rollback_command}`",
    ]
    if args.planned_ga_date:
        lines.append(f"- Planned GA date: {args.planned_ga_date}")
    if args.planned_v5_eos_date:
        lines.append(f"- Planned v5 end-of-support date: {args.planned_v5_eos_date}")
    lines.append(f"- Hotfix exception: {args.hotfix_exception}")
    if args.hotfix_reason:
        lines.append(f"- Hotfix reason: {args.hotfix_reason}")
    return "\n".join(lines)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    parser.add_argument("--release-notes-file", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--promotion-channel", required=True)
    parser.add_argument("--candidate-tag", required=True)
    parser.add_argument("--promoted-prerelease-tag", default="")
    parser.add_argument("--rollback-target", required=True)
    parser.add_argument("--rollback-command", required=True)
    parser.add_argument("--planned-ga-date", default="")
    parser.add_argument("--planned-v5-eos-date", default="")
    parser.add_argument("--hotfix-exception", required=True)
    parser.add_argument("--hotfix-reason", default="")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    raw_text = Path(args.release_notes_file).read_text(encoding="utf-8")
    sanitized = sanitize_release_notes(raw_text, args.version).rstrip("\n")
    sections = [
        sanitized,
        build_installation_section(args.version),
        build_promotion_metadata_section(args),
    ]
    Path(args.output).write_text("\n\n".join(sections) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
