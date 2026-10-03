#!/usr/bin/env python3
"""Choose the published release pair for the release lifecycle rehearsal.

The rehearsal installs FROM, upgrades to TO and rolls back to FROM. Both tags
must name published (non-draft) Pulse server releases and TO must be strictly
newer than FROM under SemVer precedence, so a shadow run can never report a
same-version "upgrade" as proof.

Defaults:
  FROM = the release GitHub advertises as latest (must be stable).
  TO   = the newest published release or prerelease newer than FROM.
When FROM was defaulted and nothing newer exists yet (the common case right
after a stable release), the pair becomes previous stable -> latest stable,
which is the upgrade users are taking at that moment.
"""

from __future__ import annotations

import argparse
import functools
import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path


TAG_RE = re.compile(
    r"^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$"
)


class ResolutionError(ValueError):
    """The requested or default release pair cannot be rehearsed."""


@dataclass(frozen=True)
class Selection:
    from_tag: str
    to_tag: str
    reason: str


def parse_tag(tag: str) -> tuple[tuple[int, int, int], tuple[str, ...]]:
    match = TAG_RE.fullmatch(tag)
    if not match:
        raise ResolutionError(f"not a Pulse release tag: {tag!r}")
    core = (int(match.group(1)), int(match.group(2)), int(match.group(3)))
    pre = tuple(match.group(4).split(".")) if match.group(4) else ()
    return core, pre


def is_stable(tag: str) -> bool:
    return not parse_tag(tag)[1]


def _compare_identifiers(left: tuple[str, ...], right: tuple[str, ...]) -> int:
    # SemVer 2.0.0 section 11: a release outranks any prerelease of the same
    # core; numeric identifiers compare numerically and rank below
    # alphanumeric ones; a longer identifier list wins when all shared
    # identifiers are equal.
    if not left and not right:
        return 0
    if not left:
        return 1
    if not right:
        return -1
    for a, b in zip(left, right):
        if a == b:
            continue
        a_num, b_num = a.isdigit(), b.isdigit()
        if a_num and b_num:
            return -1 if int(a) < int(b) else 1
        if a_num != b_num:
            return -1 if a_num else 1
        return -1 if a < b else 1
    if len(left) == len(right):
        return 0
    return -1 if len(left) < len(right) else 1


def compare_tags(left: str, right: str) -> int:
    left_core, left_pre = parse_tag(left)
    right_core, right_pre = parse_tag(right)
    if left_core != right_core:
        return -1 if left_core < right_core else 1
    return _compare_identifiers(left_pre, right_pre)


def published_tags(releases: list[dict]) -> list[str]:
    tags: set[str] = set()
    for release in releases:
        if not isinstance(release, dict) or release.get("draft") is not False:
            continue
        tag = release.get("tag_name")
        if isinstance(tag, str) and TAG_RE.fullmatch(tag):
            tags.add(tag)
    return sorted(tags, key=functools.cmp_to_key(compare_tags))


def resolve(
    releases: list[dict],
    latest_tag: str,
    requested_from: str = "",
    requested_to: str = "",
) -> Selection:
    tags = published_tags(releases)
    published = set(tags)

    for label, value in (("from_version", requested_from), ("to_version", requested_to)):
        if value:
            parse_tag(value)
            if value not in published:
                raise ResolutionError(f"{label} {value} is not a published Pulse release")

    if requested_from:
        from_tag = requested_from
    else:
        if not latest_tag or latest_tag not in published or not is_stable(latest_tag):
            raise ResolutionError(
                f"latest advertised release {latest_tag!r} is not a published stable Pulse release"
            )
        from_tag = latest_tag

    if requested_to:
        if compare_tags(requested_to, from_tag) <= 0:
            raise ResolutionError(
                f"to_version {requested_to} must be newer than from_version {from_tag}"
            )
        return Selection(from_tag, requested_to, "requested pair")

    newer = [tag for tag in tags if compare_tags(tag, from_tag) > 0]
    if newer:
        reason = "requested from_version" if requested_from else "latest stable"
        return Selection(from_tag, newer[-1], f"{reason} -> newest published release")

    if requested_from:
        raise ResolutionError(f"no published release is newer than from_version {from_tag}")

    older_stable = [
        tag for tag in tags if is_stable(tag) and compare_tags(tag, from_tag) < 0
    ]
    if not older_stable:
        raise ResolutionError(f"no stable release precedes latest stable {from_tag}")
    return Selection(
        older_stable[-1],
        from_tag,
        "nothing newer than latest stable is published; previous stable -> latest stable",
    )


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--releases-json", required=True, type=Path,
                        help="JSON array of GitHub release objects (all pages)")
    parser.add_argument("--latest-tag", default="",
                        help="tag_name of GET /repos/{repo}/releases/latest")
    parser.add_argument("--from", dest="from_version", default="")
    parser.add_argument("--to", dest="to_version", default="")
    args = parser.parse_args(argv)

    try:
        releases = json.loads(args.releases_json.read_text(encoding="utf-8"))
        if not isinstance(releases, list):
            raise ResolutionError("releases JSON must be an array")
        selection = resolve(
            releases,
            args.latest_tag.strip(),
            args.from_version.strip(),
            args.to_version.strip(),
        )
    except (ResolutionError, json.JSONDecodeError, OSError) as error:
        print(f"release lifecycle rehearsal: {error}", file=sys.stderr)
        return 1

    print(f"from_tag={selection.from_tag}")
    print(f"to_tag={selection.to_tag}")
    print(f"reason={selection.reason}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
