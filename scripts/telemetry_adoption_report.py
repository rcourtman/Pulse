#!/usr/bin/env python3
"""Summarize Pulse anonymous telemetry for operator-facing adoption reads.

This script intentionally normalizes version strings before aggregation so
manual builds, dev builds, and accidental `v` prefixes do not pollute
published-release reporting.
"""

from __future__ import annotations

import argparse
from collections import Counter
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
import json
import re
import sqlite3
import subprocess
import sys
from typing import Any, Iterable
from urllib.request import Request, urlopen


DEFAULT_DB_PATH = "/var/lib/pulse-license/licenses.sqlite"
DEFAULT_GITHUB_REPO = "rcourtman/Pulse"
GIT_DESCRIBE_RE = re.compile(
    r"^(?P<base>\d+\.\d+\.\d+(?:-[0-9A-Za-z\.-]+)?)-(?P<count>\d+)-g(?P<sha>[0-9a-fA-F]+)(?P<dirty>-dirty)?$"
)
SEMVER_RE = re.compile(
    r"^(?P<major>\d+)\.(?P<minor>\d+)\.(?P<patch>\d+)(?:-(?P<prerelease>[^+]+))?(?:\+(?P<build>.+))?$"
)
TOKEN_RE = re.compile(r"[^0-9A-Za-z.-]+")


@dataclass(frozen=True)
class ClassifiedVersion:
    raw_version: str
    version: str
    channel: str
    build: str
    is_development: bool
    is_published_release: bool


def normalize_reported_version(raw: str) -> str:
    value = raw.strip()
    if value.startswith("v"):
        value = value[1:]
    if not value:
        return "0.0.0-dev"

    match = GIT_DESCRIBE_RE.match(value)
    if match:
        build = f"git.{match.group('count')}.g{match.group('sha').lower()}"
        if match.group("dirty"):
            build += ".dirty"
        return f"{match.group('base')}+{build}"

    if SEMVER_RE.match(value):
        return value

    sanitized = TOKEN_RE.sub("-", value).strip("-.").lower()
    if not sanitized:
        sanitized = "dev"
    return f"0.0.0-{sanitized}"


def parse_semver(version: str) -> dict[str, str] | None:
    match = SEMVER_RE.match(version)
    if not match:
        return None
    return {
        "prerelease": match.group("prerelease") or "",
        "build": match.group("build") or "",
    }


def version_channel(version: str) -> str:
    parsed = parse_semver(version)
    if parsed is None:
        return "unknown"
    prerelease = parsed["prerelease"].lower()
    build = parsed["build"].lower()
    if build:
        return "dev"
    if prerelease.startswith("rc."):
        return "rc"
    if prerelease == "dev" or prerelease.startswith("dev."):
        return "dev"
    if prerelease:
        return "prerelease"
    return "stable"


def classify_reported_version(raw: str, published_versions: set[str]) -> ClassifiedVersion:
    normalized = normalize_reported_version(raw)
    parsed = parse_semver(normalized) or {"build": ""}
    channel = version_channel(normalized)
    published_candidate = channel in {"stable", "rc"} and not parsed["build"]
    is_published_release = normalized in published_versions if published_versions else published_candidate
    return ClassifiedVersion(
        raw_version=raw.strip(),
        version=normalized,
        channel=channel,
        build=parsed["build"],
        is_development=channel == "dev",
        is_published_release=is_published_release,
    )


def parse_received_at(raw: str) -> datetime:
    return datetime.strptime(raw, "%Y-%m-%d %H:%M:%S").replace(tzinfo=timezone.utc)


def fetch_published_versions(repo: str) -> set[str]:
    versions: set[str] = set()
    page = 1
    while True:
        request = Request(
            f"https://api.github.com/repos/{repo}/releases?per_page=100&page={page}",
            headers={
                "Accept": "application/vnd.github+json",
                "User-Agent": "pulse-telemetry-adoption-report",
            },
        )
        with urlopen(request, timeout=15) as response:
            payload = json.loads(response.read().decode("utf-8"))
        if not payload:
            break
        for release in payload:
            if release.get("draft"):
                continue
            tag = str(release.get("tag_name", "")).strip()
            if tag.startswith("v"):
                tag = tag[1:]
            if tag:
                versions.add(tag)
        page += 1
    return versions


def fetch_rows_local(db_path: str, since_days: int) -> dict[str, Any]:
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    try:
        db_stats = dict(
            conn.execute(
                """
                SELECT
                  MAX(received_at) AS latest_ping,
                  COUNT(*) AS total_rows,
                  COUNT(DISTINCT install_id) AS total_distinct_installs
                FROM telemetry_pings
                """
            ).fetchone()
        )
        rows = [
            dict(row)
            for row in conn.execute(
                """
                SELECT install_id, version, platform, received_at, event
                FROM telemetry_pings
                WHERE julianday(received_at) >= julianday('now') - ?
                ORDER BY received_at DESC
                """,
                (since_days,),
            ).fetchall()
        ]
        return {"db_stats": db_stats, "rows": rows}
    finally:
        conn.close()


def fetch_rows_remote(ssh_host: str, db_path: str, since_days: int) -> dict[str, Any]:
    remote_script = """
import json
import sqlite3
import sys

db_path = sys.argv[1]
since_days = int(sys.argv[2])
conn = sqlite3.connect(db_path)
conn.row_factory = sqlite3.Row
db_stats_sql = (
    "SELECT MAX(received_at) AS latest_ping, "
    "COUNT(*) AS total_rows, "
    "COUNT(DISTINCT install_id) AS total_distinct_installs "
    "FROM telemetry_pings"
)
rows_sql = (
    "SELECT install_id, version, platform, received_at, event "
    "FROM telemetry_pings "
    "WHERE julianday(received_at) >= julianday('now') - ? "
    "ORDER BY received_at DESC"
)
try:
    db_stats = dict(conn.execute(db_stats_sql).fetchone())
    rows = [
        dict(row)
        for row in conn.execute(rows_sql, (since_days,)).fetchall()
    ]
    print(json.dumps({"db_stats": db_stats, "rows": rows}))
finally:
    conn.close()
"""
    result = subprocess.run(
        ["ssh", ssh_host, "python3", "-", db_path, str(since_days)],
        input=remote_script,
        text=True,
        capture_output=True,
        check=True,
    )
    return json.loads(result.stdout)


def summarize_rows(
    db_stats: dict[str, Any],
    rows: Iterable[dict[str, Any]],
    published_versions: set[str],
) -> dict[str, Any]:
    latest_by_install: dict[str, dict[str, Any]] = {}
    for row in rows:
        install_id = str(row["install_id"])
        existing = latest_by_install.get(install_id)
        if existing is None or str(row["received_at"]) > str(existing["received_at"]):
            latest_by_install[install_id] = row

    now = datetime.now(timezone.utc)
    active_24h = 0
    active_72h = 0
    platform_split = Counter()
    version_split = Counter()
    published_split = Counter()
    non_release_split = Counter()

    for row in latest_by_install.values():
        received_at = parse_received_at(str(row["received_at"]))
        age = now - received_at
        if age <= timedelta(hours=24):
            active_24h += 1
        if age <= timedelta(hours=72):
            active_72h += 1
            platform = str(row.get("platform") or "unknown").strip() or "unknown"
            platform_split[platform] += 1
            identity = classify_reported_version(str(row.get("version") or ""), published_versions)
            version_split[identity.version] += 1
            target = published_split if identity.is_published_release else non_release_split
            target[identity.version] += 1

    return {
        "db_stats": db_stats,
        "active_latest": {
            "active_24h": active_24h,
            "active_72h": active_72h,
        },
        "latest_version_split_72h": [
            {"version": version, "installs": installs}
            for version, installs in sorted(version_split.items(), key=lambda item: (-item[1], item[0]))
        ],
        "published_version_split_72h": [
            {"version": version, "installs": installs}
            for version, installs in sorted(published_split.items(), key=lambda item: (-item[1], item[0]))
        ],
        "non_release_version_split_72h": [
            {"version": version, "installs": installs}
            for version, installs in sorted(non_release_split.items(), key=lambda item: (-item[1], item[0]))
        ],
        "latest_platform_split_72h": [
            {"platform": platform, "installs": installs}
            for platform, installs in sorted(platform_split.items(), key=lambda item: (-item[1], item[0]))
        ],
    }


def format_text(summary: dict[str, Any], repo: str, since_days: int) -> str:
    lines = [
        "Pulse telemetry adoption report",
        f"source window: last {since_days} day(s)",
        f"published release validation: {repo}",
        f"latest ping: {summary['db_stats'].get('latest_ping') or 'unknown'}",
        f"total rows: {summary['db_stats'].get('total_rows', 0)}",
        f"total distinct installs: {summary['db_stats'].get('total_distinct_installs', 0)}",
        f"active installs (24h): {summary['active_latest']['active_24h']}",
        f"active installs (72h): {summary['active_latest']['active_72h']}",
        "",
        "Published versions (latest install state in last 72h):",
    ]
    published = summary["published_version_split_72h"]
    if published:
        lines.extend(f"- {entry['version']}: {entry['installs']}" for entry in published)
    else:
        lines.append("- none")
    lines.append("")
    lines.append("Non-release or unpublished versions (latest install state in last 72h):")
    non_release = summary["non_release_version_split_72h"]
    if non_release:
        lines.extend(f"- {entry['version']}: {entry['installs']}" for entry in non_release)
    else:
        lines.append("- none")
    lines.append("")
    lines.append("Platforms (latest install state in last 72h):")
    platform_split = summary["latest_platform_split_72h"]
    if platform_split:
        lines.extend(f"- {entry['platform']}: {entry['installs']}" for entry in platform_split)
    else:
        lines.append("- none")
    return "\n".join(lines)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--db-path", default=DEFAULT_DB_PATH, help="path to licenses.sqlite")
    parser.add_argument("--ssh-host", help="optional SSH host to query remotely, e.g. root@pulse-license")
    parser.add_argument("--since-days", type=int, default=7, help="history window to inspect")
    parser.add_argument(
        "--github-repo",
        default=DEFAULT_GITHUB_REPO,
        help="GitHub repo used to validate actually published release tags",
    )
    parser.add_argument(
        "--format",
        choices=("text", "json"),
        default="text",
        help="output format",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    if args.since_days < 3:
        raise SystemExit("--since-days must be at least 3 so the 72h view is meaningful")

    published_versions = fetch_published_versions(args.github_repo)
    source = (
        fetch_rows_remote(args.ssh_host, args.db_path, args.since_days)
        if args.ssh_host
        else fetch_rows_local(args.db_path, args.since_days)
    )
    summary = summarize_rows(source["db_stats"], source["rows"], published_versions)

    if args.format == "json":
        print(json.dumps(summary, indent=2, sort_keys=True))
    else:
        print(format_text(summary, args.github_repo, args.since_days))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
