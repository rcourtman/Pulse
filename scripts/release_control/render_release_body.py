#!/usr/bin/env python3
"""Render a publish-safe GitHub release body from the current RC packet."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


class ReleaseBodyIntegrityError(ValueError):
    """Raised when release-note Markdown is not safe to publish."""


_VALIDATION_STATUS_BLOCK_RE = re.compile(
    r"(?:^|\n)<!-- VALIDATION_STATUS_START -->.*?"
    r"<!-- VALIDATION_STATUS_END -->(?:\n{0,2}|$)",
    re.DOTALL,
)


def _normalize_newlines(text: str) -> str:
    return text.replace("\r\n", "\n").replace("\r", "\n")


def _canonical_body(text: str) -> str:
    return _normalize_newlines(text).rstrip("\n") + "\n"


def _find_inline_markdown_markers(text: str) -> list[str]:
    markers: list[str] = []
    for line_number, line in enumerate(_normalize_newlines(text).splitlines(), start=1):
        heading_markers = re.finditer(r"#{2,6}[ \t]+\S", line)
        if any(match.start() != 0 for match in heading_markers):
            markers.append(f"inline heading marker on line {line_number}")
        fence_markers = re.finditer(r"```(?:[A-Za-z0-9_-]+)?", line)
        if any(match.start() != 0 for match in fence_markers):
            markers.append(f"inline code-fence marker on line {line_number}")
    return markers


def validate_release_notes_shape(raw_text: str, version: str) -> None:
    """Fail closed when authored release-note Markdown has lost its structure."""

    text = _normalize_newlines(raw_text).strip()
    if not text:
        raise ReleaseBodyIntegrityError("release notes are empty")

    lines = text.splitlines()
    expected_title = re.compile(
        rf"^# Pulse v{re.escape(version)} (?:Draft )?Release Notes$"
    )
    if not expected_title.fullmatch(lines[0]):
        raise ReleaseBodyIntegrityError(
            "the first line must be the standalone release title "
            f"'# Pulse v{version} Release Notes' (or its Draft form)"
        )

    level_two_headings = [
        line for line in lines if re.fullmatch(r"##[ \t]+\S.*", line)
    ]
    if not level_two_headings:
        raise ReleaseBodyIntegrityError(
            "release notes must contain at least one standalone level-two section"
        )

    inline_markers = _find_inline_markdown_markers(text)
    if inline_markers:
        raise ReleaseBodyIntegrityError(
            "release notes contain flattened Markdown: " + ", ".join(inline_markers)
        )


def strip_validation_status_block(text: str) -> str:
    """Remove the workflow-owned validation annotation from a release body."""

    stripped = _VALIDATION_STATUS_BLOCK_RE.sub("\n", _normalize_newlines(text), count=1)
    return _canonical_body(stripped.lstrip("\n"))


def validate_release_body_shape(
    body: str,
    version: str,
    *,
    expected_body: str | None = None,
) -> str:
    """Validate a stored GitHub release body and return its authored body."""

    clean_body = strip_validation_status_block(body)
    validate_release_notes_shape(clean_body, version)

    if clean_body.count("## Installation\n") != 1:
        raise ReleaseBodyIntegrityError(
            "published release body must contain exactly one Installation section"
        )
    if clean_body.count("## Promotion Metadata\n") != 1:
        raise ReleaseBodyIntegrityError(
            "published release body must contain exactly one Promotion Metadata section"
        )
    if "Draft Release Notes" in clean_body or "_DRAFT.md" in clean_body:
        raise ReleaseBodyIntegrityError(
            "published release body still contains draft-only framing"
        )

    installation_index = clean_body.index("## Installation\n")
    promotion_index = clean_body.index("## Promotion Metadata\n")
    if installation_index >= promotion_index:
        raise ReleaseBodyIntegrityError(
            "Installation must precede Promotion Metadata in the published body"
        )

    authored_prefix = clean_body[:installation_index]
    authored_sections = re.findall(r"(?m)^##[ \t]+\S.*$", authored_prefix)
    if not authored_sections:
        raise ReleaseBodyIntegrityError(
            "published release body has no authored section before Installation"
        )

    if expected_body is not None:
        expected_clean = strip_validation_status_block(expected_body)
        if clean_body != expected_clean:
            raise ReleaseBodyIntegrityError(
                "GitHub's stored release body does not exactly match the expected "
                "rendered Markdown"
            )

    return clean_body


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
    lines.append(f"- Windows Authenticode required: {args.require_windows_signing}")
    lines.append(f"- Unsigned Windows exception: {args.unsigned_windows_exception}")
    if args.unsigned_windows_reason:
        lines.append(f"- Unsigned Windows reason: {args.unsigned_windows_reason}")
    return "\n".join(lines)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    parser.add_argument("--release-notes-file")
    parser.add_argument("--validate-notes-file")
    parser.add_argument("--validate-body-file")
    parser.add_argument("--expected-body-file")
    parser.add_argument("--output")
    parser.add_argument("--promotion-channel")
    parser.add_argument("--candidate-tag")
    parser.add_argument("--promoted-prerelease-tag", default="")
    parser.add_argument("--rollback-target")
    parser.add_argument("--rollback-command")
    parser.add_argument("--planned-ga-date", default="")
    parser.add_argument("--planned-v5-eos-date", default="")
    parser.add_argument("--hotfix-exception")
    parser.add_argument("--hotfix-reason", default="")
    parser.add_argument("--require-windows-signing")
    parser.add_argument("--unsigned-windows-exception")
    parser.add_argument("--unsigned-windows-reason", default="")
    args = parser.parse_args()

    if args.validate_notes_file or args.validate_body_file:
        if args.release_notes_file:
            parser.error(
                "validation modes cannot be combined with --release-notes-file"
            )
        if args.validate_notes_file and args.validate_body_file:
            parser.error(
                "--validate-notes-file cannot be combined with --validate-body-file"
            )
        if args.validate_notes_file and (args.expected_body_file or args.output):
            parser.error(
                "--validate-notes-file cannot use --expected-body-file or --output"
            )
        return args

    required_render_args = {
        "--release-notes-file": args.release_notes_file,
        "--output": args.output,
        "--promotion-channel": args.promotion_channel,
        "--candidate-tag": args.candidate_tag,
        "--rollback-target": args.rollback_target,
        "--rollback-command": args.rollback_command,
        "--hotfix-exception": args.hotfix_exception,
        "--require-windows-signing": args.require_windows_signing,
        "--unsigned-windows-exception": args.unsigned_windows_exception,
    }
    missing = [name for name, value in required_render_args.items() if value is None]
    if missing:
        parser.error("render mode requires " + ", ".join(missing))
    if args.expected_body_file:
        parser.error("--expected-body-file requires --validate-body-file")
    return args


def main() -> int:
    args = parse_args()
    try:
        if args.validate_notes_file:
            raw_text = Path(args.validate_notes_file).read_text(encoding="utf-8")
            validate_release_notes_shape(raw_text, args.version)
            return 0

        if args.validate_body_file:
            body = Path(args.validate_body_file).read_text(encoding="utf-8")
            expected_body = None
            if args.expected_body_file:
                expected_body = Path(args.expected_body_file).read_text(encoding="utf-8")
            clean_body = validate_release_body_shape(
                body,
                args.version,
                expected_body=expected_body,
            )
            if args.output:
                Path(args.output).write_text(clean_body, encoding="utf-8")
            return 0

        raw_text = Path(args.release_notes_file).read_text(encoding="utf-8")
        validate_release_notes_shape(raw_text, args.version)
        sanitized = sanitize_release_notes(raw_text, args.version).rstrip("\n")
        sections = [
            sanitized,
            build_installation_section(args.version),
            build_promotion_metadata_section(args),
        ]
        rendered = "\n\n".join(sections) + "\n"
        validate_release_body_shape(rendered, args.version)
        Path(args.output).write_text(rendered, encoding="utf-8")
        return 0
    except ReleaseBodyIntegrityError as exc:
        print(f"release body integrity check failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
