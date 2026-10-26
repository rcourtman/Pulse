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
import subprocess
import sys
import time
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


def _github_release_json(repository: str, *, latest: bool = False) -> object:
    """Read only release metadata; never retry a refusal or an unknown error."""
    endpoint = f"repos/{repository}/releases"
    command = ["gh", "api", "--method", "GET"]
    if latest:
        command.append(f"{endpoint}/latest")
    else:
        command.extend(["--paginate", "--slurp", f"{endpoint}?per_page=100"])
    for attempt in range(1, 4):
        try:
            result = subprocess.run(
                command, capture_output=True, text=True, check=False, timeout=60,
            )
        except subprocess.TimeoutExpired as error:
            raise ResolutionError("GitHub release metadata read timed out; not retrying") from error
        if result.stderr:
            print(result.stderr, end="", file=sys.stderr)
        if result.returncode == 0:
            # A successful but malformed/incomplete response is not a
            # transport-only failure and must not select an older fallback.
            return json.loads(result.stdout)
        # Require the complete diagnostic to match the observed peer stream
        # cancellation. Any additional error (including 401/403/429), unknown
        # failure, timeout or signal is terminal. Discard partial stdout.
        if (
            result.returncode != 1
            or attempt == 3
            or re.fullmatch(
                r"stream error: stream ID [0-9]+; CANCEL; received from peer\n?",
                result.stderr,
            ) is None
        ):
            raise ResolutionError(
                f"GitHub release metadata read failed (exit {result.returncode}, attempt {attempt}/3)"
            )
        print(
            f"release lifecycle rehearsal: metadata stream cancelled by peer; "
            f"retrying the same GET ({attempt}/2)", file=sys.stderr,
        )
        time.sleep(attempt * 2)
    raise AssertionError("unreachable")


def discover(repository: str) -> tuple[list[dict], str]:
    # No URL, host, endpoint, method, token or read-filter overrides. The
    # workflow supplies github.repository, not a dispatch input.
    if re.fullmatch(r"[A-Za-z0-9_-][A-Za-z0-9_.-]*/[A-Za-z0-9_-][A-Za-z0-9_.-]*", repository) is None:
        raise ResolutionError("invalid GitHub repository name")
    pages = _github_release_json(repository)
    if not isinstance(pages, list) or not pages or any(not isinstance(page, list) for page in pages):
        raise ResolutionError("GitHub release inventory must contain complete array pages")
    releases = [release for page in pages for release in page]
    if any(not isinstance(release, dict) for release in releases):
        raise ResolutionError("GitHub release inventory contains a non-object")
    latest = _github_release_json(repository, latest=True)
    if not isinstance(latest, dict) or not isinstance(latest.get("tag_name"), str):
        raise ResolutionError("GitHub latest release must contain a tag_name")
    return releases, latest["tag_name"]


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
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--releases-json", type=Path,
                        help="offline JSON array of GitHub release objects (all pages)")
    source.add_argument("--repository", help="read published release metadata from this GitHub repository")
    parser.add_argument("--latest-tag", default="",
                        help="tag_name of GET /repos/{repo}/releases/latest (offline mode only)")
    parser.add_argument("--from", dest="from_version", default="")
    parser.add_argument("--to", dest="to_version", default="")
    args = parser.parse_args(argv)
    if args.repository and args.latest_tag:
        parser.error("--latest-tag cannot override live --repository discovery")

    try:
        if args.repository:
            releases, latest_tag = discover(args.repository)
        else:
            releases = json.loads(args.releases_json.read_text(encoding="utf-8"))
            latest_tag = args.latest_tag.strip()
        if not isinstance(releases, list):
            raise ResolutionError("releases JSON must be an array")
        selection = resolve(
            releases,
            latest_tag,
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
