#!/usr/bin/env python3
"""Validate the repository's public documentation surface."""

from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import unquote


ROOT = Path(__file__).resolve().parents[1]

ROOT_DOCS = (
    "README.md",
    "CONTRIBUTING.md",
    "CHANGELOG.md",
    "ARCHITECTURE.md",
    "SECURITY.md",
    "TERMS.md",
)

CURRENT_PRODUCT_DOCS = (
    "README.md",
    "ARCHITECTURE.md",
    "docs/README.md",
    "docs/FAQ.md",
    "docs/SCREENSHOTS.md",
)

MARKDOWN_LINK = re.compile(r"!?\[[^\]]*\]\(([^)]+)\)")
HTML_ASSET = re.compile(r"(?:src|href)=[\"']([^\"']+)[\"']")
HEADING = re.compile(r"^#{1,6}\s+(.+?)\s*#*\s*$", re.MULTILINE)
CODE_SPAN = re.compile(r"(?<!`)(`+)(?!`)(.*?)(?<!`)\1(?!`)")
FENCE_OPEN = re.compile(r"^ {0,3}(`{3,}|~{3,})(.*)$")
HTML_TAG = re.compile(r"<[^>]+>")
HTML_ENTITY = re.compile(r"&(?:#\d+|#x[0-9a-fA-F]+|[A-Za-z][A-Za-z0-9]+);")
BARE_URL = re.compile(r"https?://\S+")
MARKDOWN_LINK_WITH_TEXT = re.compile(r"(!?\[[^\]]*\])\([^)]*\)")
DIFF_HUNK = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@")

MISLEADING_PUBLIC_CLAIMS = {
    r"dead-man checks can notify when an expected external signal stops arriving": (
        "dead-man direction is reversed; Pulse sends its own health signal to an "
        "external watchdog"
    ),
    r"api/v2\.0/system/info": (
        "TrueNAS 26 removed the legacy REST system-info endpoint; current "
        "troubleshooting must use Pulse's JSON-RPC connection test"
    ),
}


def public_markdown_files() -> list[Path]:
    files = [ROOT / name for name in ROOT_DOCS if (ROOT / name).exists()]
    files.extend(sorted((ROOT / "docs").glob("*.md")))
    files.extend(sorted((ROOT / "docs" / "releases").glob("*.md")))
    files.extend(sorted((ROOT / "docs" / "i18n").glob("**/*.md")))
    files.extend(sorted((ROOT / ".github" / "ISSUE_TEMPLATE").glob("*.md")))
    return files


def split_target(raw_target: str) -> tuple[str, str]:
    target = raw_target.strip().strip("<>")
    if " " in target and not target.startswith("#"):
        target = target.split(" ", 1)[0]
    path, separator, anchor = target.partition("#")
    return unquote(path), unquote(anchor) if separator else ""


def heading_slug(value: str) -> str:
    def prose(text: str) -> str:
        return re.sub(r"[`*_~]", "", re.sub(r"<[^>]+>", "", text))

    # Underscores in code names are visible text, not emphasis markers.
    # GitHub and the shipped Help keep them in the rendered heading ID.
    parts: list[str] = []
    offset = 0
    for span in CODE_SPAN.finditer(value):
        parts.extend((prose(value[offset:span.start()]), span.group(2)))
        offset = span.end()
    parts.append(prose(value[offset:]))
    value = "".join(parts).lower()
    value = re.sub(r"[^\w\s-]", "", value, flags=re.UNICODE)
    return re.sub(r"\s", "-", value).strip("-")


def outside_fenced_code(text: str) -> str:
    """Exclude fenced examples from heading and navigation-link checks.

    A shell comment is not a heading, and a Markdown example is not a live
    link. Respect the fence's character and minimum length, including an
    unclosed fence; blank lines keep adjacent prose from being joined.
    """

    content: list[str] = []
    closing: re.Pattern[str] | None = None
    for line in text.splitlines(keepends=True):
        value = line.rstrip("\r\n")
        if closing is not None:
            if closing.fullmatch(value):
                closing = None
            content.append("\n")
            continue

        opening = FENCE_OPEN.fullmatch(value)
        if opening is not None:
            fence, info = opening.groups()
            # Backticks in a backtick fence's info string make it ordinary
            # text, not the start of a code block.
            if fence[0] != "`" or "`" not in info:
                closing = re.compile(
                    r" {0,3}" + re.escape(fence[0]) + "{" + str(len(fence)) + r",}[ \t]*"
                )
                content.append("\n")
                continue
        content.append(line)
    return "".join(content)


def anchors_for(path: Path) -> set[str]:
    text = outside_fenced_code(path.read_text(encoding="utf-8", errors="replace"))
    anchors: set[str] = set()
    counts: dict[str, int] = {}
    for heading in HEADING.findall(text):
        base = heading_slug(heading)
        if not base:
            continue
        count = counts.get(base, 0)
        anchors.add(base if count == 0 else f"{base}-{count}")
        counts[base] = count + 1
    return anchors


def check_links(files: list[Path]) -> list[str]:
    errors: list[str] = []
    anchor_cache: dict[Path, set[str]] = {}

    for source in files:
        text = outside_fenced_code(source.read_text(encoding="utf-8", errors="replace"))
        targets = [match.group(1) for match in MARKDOWN_LINK.finditer(text)]
        targets.extend(match.group(1) for match in HTML_ASSET.finditer(text))

        for raw_target in targets:
            if raw_target.startswith(("http://", "https://", "mailto:", "data:")):
                continue
            target_path, anchor = split_target(raw_target)
            resolved = source if not target_path else (source.parent / target_path).resolve()
            display_source = source.relative_to(ROOT)

            if not resolved.exists():
                errors.append(f"{display_source}: missing local target {raw_target}")
                continue

            # GitHub retains some emoji variation selectors in generated
            # anchors. Those anchors are stable in the rendered document but
            # are not practical to reproduce with the standard library, so
            # validate ordinary text anchors and still verify the target file
            # for emoji-prefixed anchors.
            if (
                anchor
                and resolved.suffix.lower() == ".md"
                and not anchor.startswith(("-", "\ufe0f"))
            ):
                # setdefault would parse the file even on a cache hit. Keep
                # this cache local so the next check sees changed headings.
                if resolved not in anchor_cache:
                    anchor_cache[resolved] = anchors_for(resolved)
                anchors = anchor_cache[resolved]
                if anchor.lower() not in anchors:
                    errors.append(
                        f"{display_source}: missing anchor #{anchor} in "
                        f"{resolved.relative_to(ROOT)}"
                    )

    return errors


def check_public_claims(files: list[Path]) -> list[str]:
    """Reject product claims whose direction changes the advertised capability."""

    errors: list[str] = []
    for path in files:
        text = path.read_text(encoding="utf-8", errors="replace")
        display_path = path.relative_to(ROOT)
        for pattern, label in MISLEADING_PUBLIC_CLAIMS.items():
            if re.search(pattern, text, flags=re.IGNORECASE):
                errors.append(f"{display_path}: {label}")
    return errors


def check_current_claims() -> list[str]:
    errors: list[str] = []
    retired_patterns = {
        r"organis(?:e|z)(?:s|ed)\s+the\s+ui\s+by\s+task":
            "retired task-based navigation claim",
        r"unified,\s*task-based navigation":
            "retired unified-navigation screenshot claim",
        r"canonical v6 task surfaces":
            "retired canonical task-surface claim",
    }

    for relative_path in CURRENT_PRODUCT_DOCS:
        path = ROOT / relative_path
        text = path.read_text(encoding="utf-8", errors="replace")
        for pattern, label in retired_patterns.items():
            if re.search(pattern, text, flags=re.IGNORECASE):
                errors.append(f"{relative_path}: {label}")

    readme_lines = (ROOT / "README.md").read_text(encoding="utf-8").splitlines()
    if len(readme_lines) > 220:
        errors.append(
            f"README.md: {len(readme_lines)} lines exceeds the 220-line landing-page budget"
        )

    issue_config = (ROOT / ".github" / "ISSUE_TEMPLATE" / "config.yml").read_text(
        encoding="utf-8"
    )
    if "github.com/rcourtman/Pulse/wiki" in issue_config:
        errors.append(".github/ISSUE_TEMPLATE/config.yml: documentation link points at the retired wiki")

    artifact_hub = ROOT / "artifacthub-repo.yml"
    if artifact_hub.exists() and "01234567-89ab-cdef-0123-456789abcdef" in artifact_hub.read_text(
        encoding="utf-8"
    ):
        errors.append("artifacthub-repo.yml: placeholder repository ID is not publishable metadata")

    return errors



def strip_non_prose_inline(line: str) -> str:
    """Remove inline code spans, link targets, HTML tags, URLs, and entities."""

    value = CODE_SPAN.sub("", line)
    value = MARKDOWN_LINK_WITH_TEXT.sub(r"\1", value)
    value = HTML_TAG.sub("", value)
    value = BARE_URL.sub("", value)
    value = HTML_ENTITY.sub("", value)
    return value


def prose_lines_by_number(text: str) -> dict[int, str]:
    """Return 1-based line numbers outside fenced code blocks with non-prose stripped."""

    lines: dict[int, str] = {}
    closing: re.Pattern[str] | None = None
    for index, raw in enumerate(text.splitlines(), 1):
        if closing is not None:
            if closing.fullmatch(raw):
                closing = None
            continue
        opening = FENCE_OPEN.fullmatch(raw)
        if opening is not None:
            fence, info = opening.groups()
            if fence[0] != "`" or "`" not in info:
                closing = re.compile(
                    r" {0,3}" + re.escape(fence[0]) + "{" + str(len(fence)) + r",}[ \t]*"
                )
                continue
        lines[index] = strip_non_prose_inline(raw)
    return lines


def check_prose_punctuation_in_text(
    display_path: str,
    text: str,
    line_numbers: set[int] | None = None,
) -> list[str]:
    """Reject em dashes and semicolons in public documentation prose."""

    errors: list[str] = []
    for line_no, prose in prose_lines_by_number(text).items():
        if line_numbers is not None and line_no not in line_numbers:
            continue
        if "\u2014" in prose:
            errors.append(
                f"{display_path}:{line_no}: avoid em dashes in public docs prose"
            )
        if ";" in prose:
            errors.append(
                f"{display_path}:{line_no}: avoid semicolons in public docs prose"
            )
    return errors


def _git_output(*args: str) -> str | None:
    try:
        result = subprocess.run(
            ["git", *args],
            cwd=ROOT,
            text=True,
            capture_output=True,
            timeout=10,
            check=False,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if result.returncode != 0:
        return None
    return result.stdout


def _parse_added_diff_lines(diff_text: str) -> dict[str, set[int]]:
    added: dict[str, set[int]] = {}
    current_path: str | None = None
    for line in diff_text.splitlines():
        if line.startswith("+++ b/"):
            current_path = line[6:]
            continue
        if line.startswith("+++ /dev/null"):
            current_path = None
            continue
        if current_path is None:
            continue
        match = DIFF_HUNK.match(line)
        if match is None:
            continue
        start = int(match.group(1))
        count = 1 if match.group(2) is None else int(match.group(2))
        if count == 0:
            continue
        added.setdefault(current_path, set()).update(range(start, start + count))
    return added


def _resolve_base_commit(base_ref: str | None) -> str | None:
    if base_ref and re.fullmatch(r"[0-9a-fA-F]{7,40}|[\w./-]+", base_ref):
        if _git_output("rev-parse", "--verify", f"{base_ref}^{{commit}}") is not None:
            return base_ref
    if base_ref:
        # Shallow CI push checkouts may omit older base SHAs. Fall back to HEAD~1 when present.
        if _git_output("rev-parse", "--verify", "HEAD~1^{commit}") is not None:
            return "HEAD~1"
    return None


def check_changed_prose_punctuation(
    files: list[Path],
    *,
    staged: bool = False,
    base_ref: str | None = None,
) -> list[str]:
    """Check newly added or edited lines in public docs for em dashes and semicolons."""

    if _git_output("rev-parse", "--is-inside-work-tree") is None:
        return []

    public_by_rel = {path.relative_to(ROOT).as_posix(): path for path in files}
    if not public_by_rel:
        return []

    if staged:
        diff_text = _git_output(
            "diff", "--cached", "-U0", "--no-ext-diff", "--no-color", "--", *sorted(public_by_rel)
        )
        added_lines = _parse_added_diff_lines(diff_text or "")
        errors: list[str] = []
        for rel_path, line_numbers in sorted(added_lines.items()):
            if rel_path not in public_by_rel:
                continue
            blob = _git_output("show", f":{rel_path}")
            if blob is None:
                continue
            errors.extend(check_prose_punctuation_in_text(rel_path, blob, line_numbers))
        return errors

    if base_ref:
        resolved = _resolve_base_commit(base_ref)
        if resolved is None:
            return []
        diff_text = _git_output(
            "diff", "-U0", "--no-ext-diff", "--no-color", resolved, "HEAD", "--", *sorted(public_by_rel)
        )
        added_lines = _parse_added_diff_lines(diff_text or "")
        errors = []
        for rel_path, line_numbers in sorted(added_lines.items()):
            path = public_by_rel.get(rel_path)
            if path is None or not path.exists():
                continue
            text = path.read_text(encoding="utf-8", errors="replace")
            errors.extend(check_prose_punctuation_in_text(rel_path, text, line_numbers))
        return errors

    # Plain local invocation: inspect working tree + staged changes and untracked public docs.
    diff_text = _git_output(
        "diff", "-U0", "--no-ext-diff", "--no-color", "HEAD", "--", *sorted(public_by_rel)
    )
    added_lines = _parse_added_diff_lines(diff_text or "")
    untracked = _git_output("ls-files", "--others", "--exclude-standard", "--", *sorted(public_by_rel))
    errors = []
    seen: set[str] = set()
    for rel_path, line_numbers in sorted(added_lines.items()):
        path = public_by_rel.get(rel_path)
        if path is None or not path.exists():
            continue
        seen.add(rel_path)
        text = path.read_text(encoding="utf-8", errors="replace")
        errors.extend(check_prose_punctuation_in_text(rel_path, text, line_numbers))
    for rel_path in sorted((untracked or "").splitlines()):
        if rel_path in seen or rel_path not in public_by_rel:
            continue
        path = public_by_rel[rel_path]
        if not path.exists():
            continue
        text = path.read_text(encoding="utf-8", errors="replace")
        errors.extend(check_prose_punctuation_in_text(rel_path, text))
    return errors


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Validate public documentation.")
    parser.add_argument(
        "--staged",
        action="store_true",
        help="Check added or modified lines in staged public docs.",
    )
    parser.add_argument(
        "--base-ref",
        default=os.environ.get("PUBLIC_DOCS_BASE_SHA") or None,
        help="Git base commit or ref to diff HEAD against for prose checks.",
    )
    args = parser.parse_args(argv)

    files = public_markdown_files()
    errors = check_links(files)
    errors.extend(check_public_claims(files))
    errors.extend(check_current_claims())
    errors.extend(
        check_changed_prose_punctuation(
            files,
            staged=args.staged,
            base_ref=args.base_ref,
        )
    )

    if errors:
        print("Public documentation check failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1

    print(f"Public documentation check passed ({len(files)} Markdown files).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
