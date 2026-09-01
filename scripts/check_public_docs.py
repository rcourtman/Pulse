#!/usr/bin/env python3
"""Validate the repository's public documentation surface."""

from __future__ import annotations

import re
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
    value = re.sub(r"<[^>]+>", "", value)
    value = re.sub(r"[`*_~]", "", value).lower()
    value = re.sub(r"[^\w\s-]", "", value, flags=re.UNICODE)
    return re.sub(r"\s", "-", value).strip("-")


def anchors_for(path: Path) -> set[str]:
    text = path.read_text(encoding="utf-8", errors="replace")
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
        text = source.read_text(encoding="utf-8", errors="replace")
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
                anchors = anchor_cache.setdefault(resolved, anchors_for(resolved))
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


def main() -> int:
    files = public_markdown_files()
    errors = check_links(files)
    errors.extend(check_public_claims(files))
    errors.extend(check_current_claims())

    if errors:
        print("Public documentation check failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1

    print(f"Public documentation check passed ({len(files)} Markdown files).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
