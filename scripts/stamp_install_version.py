#!/usr/bin/env python3
"""Point the public install commands at one published stable Pulse release.

The release train runs this after a stable release is published, so the tag in
every command already exists. Other placeholders, such as upgrade guidance that
tells the reader to choose a target, are left alone.

  stamp_install_version.py --version vX.Y.Z   rewrite the commands
  stamp_install_version.py --check            print the version they use
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PAGES = (
    "README.md",
    "docs/INSTALL.md",
    "docs/DOCKER.md",
    "docs/i18n/de/README.md",
    "docs/i18n/es/README.md",
    "docs/i18n/fr/README.md",
)
SHIPPED_DOCS = "frontend-modern/public/docs"
PIN = re.compile(r"(export PULSE_VERSION=|rcourtman/pulse:)(v\d+\.\d+\.\d+)(?![\w.-])")
STABLE_TAG = re.compile(r"v(\d+)\.(\d+)\.(\d+)")


class StampError(Exception):
    """The pages or the requested version cannot be stamped safely."""


def version_key(tag: str) -> tuple[int, int, int]:
    match = STABLE_TAG.fullmatch(tag)
    if not match:
        raise StampError(f"{tag!r} is not a stable release tag")
    major, minor, patch = match.groups()
    return int(major), int(minor), int(patch)


def current_version(root: Path = ROOT) -> str:
    """The one version every install command uses."""
    found: set[str] = set()
    for page in PAGES:
        pins = [match.group(2) for match in PIN.finditer((root / page).read_text(encoding="utf-8"))]
        if not pins:
            raise StampError(f"{page} has no install command to stamp")
        found.update(pins)
    if len(found) != 1:
        raise StampError(f"install commands disagree: {', '.join(sorted(found))}")
    return found.pop()


def stamp(tag: str, root: Path = ROOT) -> list[str]:
    """Rewrite the install commands and return the pages that changed."""
    current = current_version(root)
    if version_key(tag) < version_key(current):
        raise StampError(f"{tag} is older than the current {current}")
    changed = []
    for page in PAGES:
        path = root / page
        before = path.read_text(encoding="utf-8")
        after = PIN.sub(lambda match: match.group(1) + tag, before)
        if after == before:
            continue
        path.write_text(after, encoding="utf-8")
        changed.append(page)
        if page.startswith("docs/"):
            mirror = root / SHIPPED_DOCS / page.removeprefix("docs/")
            mirror.write_text(after, encoding="utf-8")
            changed.append(mirror.relative_to(root).as_posix())
    return changed


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    action = parser.add_mutually_exclusive_group(required=True)
    action.add_argument("--version", help="stable release tag, such as v6.8.0")
    action.add_argument("--check", action="store_true", help="print the version in use")
    parser.add_argument("--root", type=Path, default=ROOT)
    args = parser.parse_args()
    try:
        if args.check:
            print(current_version(args.root))
        else:
            for page in stamp(args.version, args.root):
                print(page)
    except StampError as error:
        print(error, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
