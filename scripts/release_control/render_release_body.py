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

_HIGHLIGHTS_HEADING_RE = re.compile(r"^(#{2,6})[ \t]+Highlights[ \t]*$", re.IGNORECASE)
_HIGHLIGHT_BULLET_RE = re.compile(r"^-[ \t]+(.+)$")
_CUSTOMER_SECTION_HEADINGS = {
    "what's improved",
    "what’s improved",
    "fixes",
    "before you upgrade",
    "known issues",
}
_INTERNAL_RELEASE_LANGUAGE_RE = re.compile(
    r"\b(?:"
    r"readiness assertions?"
    r"|release gates?"
    r"|candidate cutoff"
    r"|exact-sha"
    r"|immutable (?:release )?candidate"
    r"|promotion channel"
    r"|completion state"
    r"|lane follow-?ups?"
    r"|artifact identity"
    r"|self-contained checks"
    r")\b",
    re.IGNORECASE,
)
_CUSTOMER_FORMAT_MINIMUM = (6, 4, 0)
_CUSTOMER_FORMAT_EXEMPTIONS = {"6.4.0-rc.1"}
_ISSUE_REFERENCE_RE = re.compile(
    r"(?:"
    r"(?<![A-Za-z0-9])#[0-9]+\b"
    r"|\b[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+#[0-9]+\b"
    r"|\bGH-[0-9]+\b"
    r"|https?://(?:www\.)?github\.com/[^/\s]+/[^/\s]+/(?:issues|pull)/[0-9]+\b"
    r")",
    re.IGNORECASE,
)
_MAX_HIGHLIGHT_ITEMS = 3
_MAX_HIGHLIGHT_LENGTH = 140
_MAX_CUSTOMER_FIX_ITEMS = 12
_MAX_CUSTOMER_ITEM_LENGTH = 260
_MAX_CUSTOMER_SUMMARY_LENGTH = 600


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


def _highlight_items(text: str) -> list[str] | None:
    """Return the optional in-app Highlights list after validating its shape."""

    lines = _normalize_newlines(text).splitlines()
    headings: list[tuple[int, int]] = []
    for index, line in enumerate(lines):
        match = _HIGHLIGHTS_HEADING_RE.fullmatch(line.strip())
        if match:
            headings.append((index, len(match.group(1))))

    if not headings:
        return None
    if len(headings) > 1:
        raise ReleaseBodyIntegrityError(
            "release notes must contain at most one Highlights section"
        )

    start_index, start_level = headings[0]
    section_lines: list[str] = []
    for line in lines[start_index + 1 :]:
        heading = re.fullmatch(r"(#{1,6})[ \t]+\S.*", line.strip())
        if heading and len(heading.group(1)) <= start_level:
            break
        section_lines.append(line)

    items: list[str] = []
    for line in section_lines:
        if not line.strip():
            continue
        bullet = _HIGHLIGHT_BULLET_RE.fullmatch(line)
        if bullet:
            items.append(bullet.group(1).strip())
            continue
        if line[:1].isspace() and items and not line.lstrip().startswith(("- ", "* ", "+ ")):
            items[-1] = f"{items[-1]} {line.strip()}"
            continue
        raise ReleaseBodyIntegrityError(
            "Highlights must be a flat list of short plain-text bullets"
        )

    if not items:
        raise ReleaseBodyIntegrityError("Highlights must contain at least one bullet")
    if len(items) > _MAX_HIGHLIGHT_ITEMS:
        raise ReleaseBodyIntegrityError(
            f"Highlights may contain at most {_MAX_HIGHLIGHT_ITEMS} bullets"
        )

    for item in items:
        if len(item) > _MAX_HIGHLIGHT_LENGTH:
            raise ReleaseBodyIntegrityError(
                "each Highlights bullet must be "
                f"{_MAX_HIGHLIGHT_LENGTH} characters or fewer"
            )
        if (
            "`" in item
            or "*" in item
            or "_" in item
            or re.search(r"!?\[[^\]]+\]\([^\)]+\)", item)
            or re.search(r"<[^>]+>", item)
            or _ISSUE_REFERENCE_RE.search(item)
        ):
            raise ReleaseBodyIntegrityError(
                "Highlights bullets must use plain text without links, code, HTML, "
                "or issue references"
            )

    return items


def _release_core(version: str) -> tuple[int, int, int] | None:
    match = re.fullmatch(
        r"v?(\d+)\.(\d+)\.(\d+)(?:-(?:rc|alpha|beta)\.\d+)?",
        version,
        re.IGNORECASE,
    )
    if not match:
        return None
    return tuple(int(match.group(index)) for index in range(1, 4))


def _requires_customer_facing_standard(version: str) -> bool:
    normalized = version.lower().removeprefix("v")
    if normalized in _CUSTOMER_FORMAT_EXEMPTIONS:
        return False
    core = _release_core(normalized)
    return core is not None and core >= _CUSTOMER_FORMAT_MINIMUM


def _requires_single_change_list(version: str) -> bool:
    """Return whether fixes must be folded into the one customer outcome list."""

    normalized = version.lower().removeprefix("v")
    core = _release_core(normalized)
    if core is None or core < (6, 4, 0):
        return False
    if core > (6, 4, 0):
        return True

    rc_match = re.fullmatch(r"6\.4\.0-rc\.(\d+)", normalized)
    return rc_match is None or int(rc_match.group(1)) >= 6


def _requires_plain_release_punctuation(version: str) -> bool:
    """Apply the punctuation rule after the already-published v6.4 RC packets."""

    normalized = version.lower().removeprefix("v")
    core = _release_core(normalized)
    if core is None or core < (6, 4, 0):
        return False
    if core > (6, 4, 0):
        return True
    rc_match = re.fullmatch(r"6\.4\.0-rc\.(\d+)", normalized)
    return rc_match is None or int(rc_match.group(1)) >= 7


def _section_lines(text: str, heading_index: int) -> list[str]:
    lines = _normalize_newlines(text).splitlines()
    section: list[str] = []
    for line in lines[heading_index + 1 :]:
        if re.fullmatch(r"##[ \t]+\S.*", line):
            break
        section.append(line)
    return section


def _flat_bullet_items(lines: list[str], section_name: str) -> list[str]:
    items: list[str] = []
    for line in lines:
        if not line.strip():
            continue
        bullet = _HIGHLIGHT_BULLET_RE.fullmatch(line)
        if bullet:
            items.append(bullet.group(1).strip())
            continue
        if line[:1].isspace() and items and not line.lstrip().startswith(("- ", "* ", "+ ")):
            items[-1] = f"{items[-1]} {line.strip()}"
            continue
        raise ReleaseBodyIntegrityError(
            f"{section_name} must be a flat Markdown bullet list"
        )
    return items


def _validate_customer_facing_release_notes(text: str, version: str) -> None:
    """Enforce concise public notes without release-control implementation prose."""

    lines = _normalize_newlines(text).strip().splitlines()
    if _requires_plain_release_punctuation(version) and any(
        character in text for character in (";", "—")
    ):
        raise ReleaseBodyIntegrityError(
            "customer-facing release notes must not contain semicolons or em dashes"
        )
    first_section_index = next(
        (index for index, line in enumerate(lines) if re.fullmatch(r"##[ \t]+\S.*", line)),
        None,
    )
    if first_section_index is None:
        raise ReleaseBodyIntegrityError("customer-facing release notes need sections")

    summary = " ".join(line.strip() for line in lines[1:first_section_index] if line.strip())
    if not summary or summary.startswith(('-', '*', '+')):
        raise ReleaseBodyIntegrityError(
            "customer-facing release notes need one plain-language summary paragraph"
        )
    if len(summary) > _MAX_CUSTOMER_SUMMARY_LENGTH:
        raise ReleaseBodyIntegrityError(
            f"the release summary must be {_MAX_CUSTOMER_SUMMARY_LENGTH} characters or fewer"
        )

    headings: dict[str, int] = {}
    for index, line in enumerate(lines):
        heading = re.fullmatch(r"##[ \t]+(.+?)\s*", line)
        if not heading:
            continue
        normalized = re.sub(r"\s+", " ", heading.group(1)).lower()
        if normalized not in _CUSTOMER_SECTION_HEADINGS:
            raise ReleaseBodyIntegrityError(
                "customer-facing release notes may only use What's improved, "
                "Fixes, Before you upgrade, and Known issues sections"
            )
        if normalized in headings:
            raise ReleaseBodyIntegrityError(
                f"customer-facing release notes contain duplicate {heading.group(1)} sections"
            )
        headings[normalized] = index

    improvements_key = next(
        (key for key in ("what's improved", "what’s improved") if key in headings),
        None,
    )
    if improvements_key is None:
        raise ReleaseBodyIntegrityError(
            "customer-facing release notes must contain a What's improved section"
        )

    improvements = _flat_bullet_items(
        _section_lines(text, headings[improvements_key]),
        "What's improved",
    )
    if not improvements:
        raise ReleaseBodyIntegrityError(
            "What's improved must contain at least one bullet"
        )
    for item in improvements:
        if len(item) > _MAX_CUSTOMER_ITEM_LENGTH:
            raise ReleaseBodyIntegrityError(
                f"customer-facing bullets must be {_MAX_CUSTOMER_ITEM_LENGTH} characters or fewer"
            )
        if not re.match(r"^\*\*[^*]+\*\*[ \t]+(?:—|-)[ \t]+\S", item):
            raise ReleaseBodyIntegrityError(
                "What's improved bullets must start with a short bold outcome followed by a dash"
            )

    if "fixes" in headings:
        if _requires_single_change_list(version):
            raise ReleaseBodyIntegrityError(
                "customer-facing release notes must describe features and fixes once "
                "in What's improved instead of adding a separate Fixes section"
            )
        fixes = _flat_bullet_items(_section_lines(text, headings["fixes"]), "Fixes")
        if not fixes or len(fixes) > _MAX_CUSTOMER_FIX_ITEMS:
            raise ReleaseBodyIntegrityError(
                f"Fixes must contain between 1 and {_MAX_CUSTOMER_FIX_ITEMS} bullets"
            )
        if any(len(item) > _MAX_CUSTOMER_ITEM_LENGTH for item in fixes):
            raise ReleaseBodyIntegrityError(
                f"customer-facing bullets must be {_MAX_CUSTOMER_ITEM_LENGTH} characters or fewer"
            )

    if _INTERNAL_RELEASE_LANGUAGE_RE.search(text):
        raise ReleaseBodyIntegrityError(
            "customer-facing release notes contain internal release-control language"
        )


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

    _highlight_items(text)
    if _requires_customer_facing_standard(version):
        _validate_customer_facing_release_notes(text, version)


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

    if clean_body.count("## Install\n") != 1:
        raise ReleaseBodyIntegrityError(
            "published release body must contain exactly one Install section"
        )
    if clean_body.count("## Roll back\n") != 1:
        raise ReleaseBodyIntegrityError(
            "published release body must contain exactly one Roll back section"
        )
    if "## Promotion Metadata\n" in clean_body:
        raise ReleaseBodyIntegrityError(
            "published release body must keep promotion metadata out of customer notes"
        )
    if "Draft Release Notes" in clean_body or "_DRAFT.md" in clean_body:
        raise ReleaseBodyIntegrityError(
            "published release body still contains draft-only framing"
        )

    installation_index = clean_body.index("## Install\n")
    rollback_index = clean_body.index("## Roll back\n")
    if installation_index >= rollback_index:
        raise ReleaseBodyIntegrityError(
            "Install must precede Roll back in the published body"
        )

    authored_prefix = clean_body[:installation_index]
    inline_markers = _find_inline_markdown_markers(authored_prefix)
    if inline_markers:
        raise ReleaseBodyIntegrityError(
            "release notes contain flattened Markdown: " + ", ".join(inline_markers)
        )
    authored_sections = re.findall(r"(?m)^##[ \t]+\S.*$", authored_prefix)
    if not authored_sections:
        raise ReleaseBodyIntegrityError(
            "published release body has no authored section before Install"
        )
    visual_heading = "\n## See the difference\n"
    if authored_prefix.count(visual_heading) > 1:
        raise ReleaseBodyIntegrityError(
            "published release body must contain at most one See the difference section"
        )
    authored_notes = authored_prefix.split(visual_heading, 1)[0]
    validate_release_notes_shape(authored_notes, version)

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
    text = _drop_level_two_sections(
        text,
        {"## Installation", "## Install", "## Roll back", "## Promotion Metadata"},
    )
    text = _drop_draft_packet_links(text)
    return _collapse_blank_lines(text)


def build_installation_section(version: str) -> str:
    return "\n".join(
        [
            "## Install",
            "",
            "For systemd and Proxmox LXC installs, use **Settings → System → Updates** or:",
            "",
            "```bash",
            f"sudo /bin/update --version v{version}",
            "```",
            "",
            "For Docker:",
            "",
            "```bash",
            f"docker pull rcourtman/pulse:{version}",
            "```",
            "",
            f"For Docker Compose, update the image to `rcourtman/pulse:{version}` and recreate the container.",
            "",
            "Pulse Pro and Relay customers should continue using the "
            "[private download page](https://pulserelay.pro/download.html) and private "
            "runtime image for paid features.",
        ]
    )


def build_rollback_section(args: argparse.Namespace) -> str:
    return "\n".join(
        [
            "## Roll back",
            "",
            f"The rollback target is `{args.rollback_target}`:",
            "",
            "```bash",
            args.rollback_command,
            "```",
            "",
            "For Docker Compose, set the Pulse image to the rollback target and recreate the container.",
        ]
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    parser.add_argument("--release-notes-file")
    parser.add_argument("--release-visuals-file")
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
        sections = [sanitized]
        if args.release_visuals_file:
            release_visuals = Path(args.release_visuals_file).read_text(
                encoding="utf-8"
            ).strip()
            if release_visuals:
                if not release_visuals.startswith("## See the difference\n"):
                    raise ReleaseBodyIntegrityError(
                        "release visuals must begin with '## See the difference'"
                    )
                sections.append(release_visuals)
        sections.extend(
            [
                build_installation_section(args.version),
                build_rollback_section(args),
            ]
        )
        rendered = "\n\n".join(sections) + "\n"
        validate_release_body_shape(rendered, args.version)
        Path(args.output).write_text(rendered, encoding="utf-8")
        return 0
    except ReleaseBodyIntegrityError as exc:
        print(f"release body integrity check failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
