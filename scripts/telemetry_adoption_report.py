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
DEFAULT_LATEST_INSTALL_WINDOWS = (
    ("24h", timedelta(hours=24)),
    ("72h", timedelta(hours=72)),
    ("7d", timedelta(days=7)),
)
ADOPTION_COUNT_FIELDS = (
    ("pve_nodes", "PVE nodes"),
    ("pbs_instances", "PBS instances"),
    ("pmg_instances", "PMG instances"),
    ("vms", "VMs"),
    ("containers", "LXC containers"),
    ("agent_hosts", "Agent hosts"),
    ("docker_hosts", "Docker hosts"),
    ("docker_containers", "Docker containers"),
    ("kubernetes_clusters", "Kubernetes clusters"),
    ("kubernetes_nodes", "Kubernetes nodes"),
    ("kubernetes_pods", "Kubernetes pods"),
    ("kubernetes_deployments", "Kubernetes deployments"),
    ("storage_pools", "Storage pools"),
    ("physical_disks", "Physical disks"),
    ("ceph_clusters", "Ceph clusters"),
    ("network_shares", "Network shares"),
    ("truenas_systems", "TrueNAS systems"),
    ("truenas_vms", "TrueNAS VMs"),
    ("truenas_apps", "TrueNAS apps"),
    ("vmware_hosts", "VMware hosts"),
    ("vmware_vms", "VMware VMs"),
    ("vmware_datastores", "VMware datastores"),
    ("availability_targets", "Availability targets"),
    ("active_alerts", "Active alerts"),
)
FEATURE_BOOL_FIELDS = (
    ("ai_enabled", "AI enabled"),
    ("patrol_enabled", "Patrol enabled"),
    ("discovery_enabled", "Discovery enabled"),
    ("notifications_enabled", "Notifications enabled"),
    ("ai_actions_enabled", "AI actions enabled"),
    ("relay_enabled", "Relay enabled"),
    ("sso_enabled", "SSO enabled"),
    ("multi_tenant", "Multi-tenant"),
    ("paid_license", "Paid license"),
    ("has_api_tokens", "Has API tokens"),
)
DEEP_SIGNAL_FIELDS = (
    ("agent_hosts", "Agent hosts", "count"),
    ("docker_containers", "Docker containers", "count"),
    ("kubernetes_nodes", "Kubernetes nodes", "count"),
    ("kubernetes_pods", "Kubernetes pods", "count"),
    ("kubernetes_deployments", "Kubernetes deployments", "count"),
    ("storage_pools", "Storage pools", "count"),
    ("physical_disks", "Physical disks", "count"),
    ("ceph_clusters", "Ceph clusters", "count"),
    ("network_shares", "Network shares", "count"),
    ("truenas_systems", "TrueNAS systems", "count"),
    ("truenas_vms", "TrueNAS VMs", "count"),
    ("truenas_apps", "TrueNAS apps", "count"),
    ("vmware_hosts", "VMware hosts", "count"),
    ("vmware_vms", "VMware VMs", "count"),
    ("vmware_datastores", "VMware datastores", "count"),
    ("availability_targets", "Availability targets", "count"),
    ("patrol_enabled", "Patrol enabled", "bool"),
    ("discovery_enabled", "Discovery enabled", "bool"),
    ("notifications_enabled", "Notifications enabled", "bool"),
    ("ai_actions_enabled", "AI actions enabled", "bool"),
)
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


def parse_optional_bool(value: Any) -> bool | None:
    if value is None:
        return None
    if isinstance(value, bool):
        return value
    if isinstance(value, (int, float)):
        return value != 0
    normalized = str(value).strip().lower()
    if normalized == "":
        return None
    if normalized in {"1", "true", "t", "yes", "y"}:
        return True
    if normalized in {"0", "false", "f", "no", "n"}:
        return False
    return None


def parse_optional_nonnegative_int(value: Any) -> int:
    if value is None:
        return 0
    try:
        parsed = int(value)
    except (TypeError, ValueError):
        return 0
    return max(parsed, 0)


def classify_row_version(row: dict[str, Any], published_versions: set[str]) -> ClassifiedVersion:
    raw_version = str(row.get("version") or "")
    identity = classify_reported_version(raw_version, published_versions)

    stored_raw = str(row.get("version_raw") or "").strip()
    stored_channel = str(row.get("version_channel") or "").strip().lower()
    stored_build = str(row.get("version_build") or "").strip()
    stored_is_development = parse_optional_bool(row.get("version_is_development"))
    stored_is_published = parse_optional_bool(row.get("version_is_published_release"))

    if stored_raw:
        identity = ClassifiedVersion(
            raw_version=stored_raw,
            version=identity.version,
            channel=identity.channel,
            build=identity.build,
            is_development=identity.is_development,
            is_published_release=identity.is_published_release,
        )
    if stored_channel:
        identity = ClassifiedVersion(
            raw_version=identity.raw_version,
            version=identity.version,
            channel=stored_channel,
            build=identity.build,
            is_development=identity.is_development,
            is_published_release=identity.is_published_release,
        )
    if stored_build:
        identity = ClassifiedVersion(
            raw_version=identity.raw_version,
            version=identity.version,
            channel=identity.channel,
            build=stored_build,
            is_development=identity.is_development,
            is_published_release=identity.is_published_release,
        )
    if stored_is_development is not None:
        identity = ClassifiedVersion(
            raw_version=identity.raw_version,
            version=identity.version,
            channel=identity.channel,
            build=identity.build,
            is_development=stored_is_development,
            is_published_release=identity.is_published_release,
        )

    if published_versions:
        is_published_release = identity.version in published_versions
    elif stored_is_published is not None:
        is_published_release = stored_is_published
    else:
        is_published_release = identity.is_published_release

    return ClassifiedVersion(
        raw_version=identity.raw_version,
        version=identity.version,
        channel=identity.channel,
        build=identity.build,
        is_development=identity.is_development,
        is_published_release=is_published_release,
    )


def parse_received_at(raw: str) -> datetime:
    return datetime.strptime(raw, "%Y-%m-%d %H:%M:%S").replace(tzinfo=timezone.utc)


def normalize_release_tag(tag: str) -> str:
    version = tag.strip()
    if version.startswith("v"):
        version = version[1:]
    return version


def fetch_published_releases(repo: str) -> list[dict[str, Any]]:
    releases: list[dict[str, Any]] = []
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
            raw_tag = str(release.get("tag_name", "")).strip()
            version = normalize_release_tag(raw_tag)
            if version:
                releases.append(
                    {
                        "version": version,
                        "tag_name": raw_tag,
                        "is_prerelease": bool(release.get("prerelease")),
                        "published_at": str(release.get("published_at") or ""),
                    }
                )
        page += 1
    return releases


def fetch_published_versions(repo: str) -> set[str]:
    return {release["version"] for release in fetch_published_releases(repo)}


def latest_rc_version(releases: Iterable[dict[str, Any]]) -> str | None:
    rc_releases = [
        release
        for release in releases
        if release.get("is_prerelease") and version_channel(str(release.get("version") or "")) == "rc"
    ]
    if not rc_releases:
        return None
    latest = max(rc_releases, key=lambda release: str(release.get("published_at") or ""))
    return str(latest["version"])


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
                SELECT *
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
    "SELECT * "
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


def counter_entries(counter: Counter[str], key_name: str) -> list[dict[str, Any]]:
    return [
        {key_name: value, "installs": installs}
        for value, installs in sorted(counter.items(), key=lambda item: (-item[1], item[0]))
    ]


def summarize_latest_install_windows(
    latest_by_install: dict[str, dict[str, Any]],
    published_versions: set[str],
    *,
    now: datetime | None = None,
    windows: tuple[tuple[str, timedelta], ...] = DEFAULT_LATEST_INSTALL_WINDOWS,
) -> dict[str, Any]:
    current_time = now or datetime.now(timezone.utc)
    summary: dict[str, Any] = {}

    for label, limit in windows:
        version_split: Counter[str] = Counter()
        published_split: Counter[str] = Counter()
        non_release_split: Counter[str] = Counter()
        platform_split: Counter[str] = Counter()
        adoption_counts: Counter[str] = Counter()
        feature_counts: Counter[str] = Counter()

        for row in latest_by_install.values():
            received_at = parse_received_at(str(row["received_at"]))
            if current_time - received_at > limit:
                continue
            platform = str(row.get("platform") or "unknown").strip() or "unknown"
            identity = classify_row_version(row, published_versions)
            version_split[identity.version] += 1
            platform_split[platform] += 1
            target = published_split if identity.is_published_release else non_release_split
            target[identity.version] += 1
            for key, _ in ADOPTION_COUNT_FIELDS:
                adoption_counts[key] += parse_optional_nonnegative_int(row.get(key))
            for key, _ in FEATURE_BOOL_FIELDS:
                if parse_optional_bool(row.get(key)):
                    feature_counts[key] += 1

        summary[label] = {
            "active_installs": sum(version_split.values()),
            "latest_versions": counter_entries(version_split, "version"),
            "published_versions": counter_entries(published_split, "version"),
            "non_release_versions": counter_entries(non_release_split, "version"),
            "platforms": counter_entries(platform_split, "platform"),
            "adoption_counts": [
                {"field": key, "label": label, "total": adoption_counts[key]}
                for key, label in ADOPTION_COUNT_FIELDS
            ],
            "feature_enabled_installs": [
                {"field": key, "label": label, "installs": feature_counts[key]}
                for key, label in FEATURE_BOOL_FIELDS
            ],
        }

    return summary


def summarize_deep_signal_sources(
    latest_by_install: dict[str, dict[str, Any]],
    published_versions: set[str],
    *,
    now: datetime | None = None,
    window: timedelta = timedelta(days=7),
) -> list[dict[str, Any]]:
    current_time = now or datetime.now(timezone.utc)
    by_field: dict[str, dict[str, dict[str, Any]]] = {key: {} for key, _, _ in DEEP_SIGNAL_FIELDS}

    for row in latest_by_install.values():
        received_at = parse_received_at(str(row["received_at"]))
        if current_time - received_at > window:
            continue
        identity = classify_row_version(row, published_versions)

        for key, _, kind in DEEP_SIGNAL_FIELDS:
            if kind == "bool":
                value = 1 if parse_optional_bool(row.get(key)) else 0
            else:
                value = parse_optional_nonnegative_int(row.get(key))
            if value <= 0:
                continue

            source = by_field[key].setdefault(
                identity.version,
                {
                    "version": identity.version,
                    "installs": 0,
                    "total": 0,
                    "is_published_release": identity.is_published_release,
                },
            )
            source["installs"] += 1
            source["total"] += value
            source["is_published_release"] = source["is_published_release"] or identity.is_published_release

    result: list[dict[str, Any]] = []
    for key, label, kind in DEEP_SIGNAL_FIELDS:
        versions = list(by_field[key].values())
        if not versions:
            continue
        versions.sort(key=lambda source: (-int(source["installs"]), str(source["version"])))
        result.append(
            {
                "field": key,
                "label": label,
                "type": kind,
                "versions": versions,
            }
        )
    return result


def telemetry_signal_specs() -> list[dict[str, str]]:
    deep_fields = {key for key, _, _ in DEEP_SIGNAL_FIELDS}
    specs: list[dict[str, str]] = []
    for key, label in ADOPTION_COUNT_FIELDS:
        specs.append(
            {
                "field": key,
                "label": label,
                "type": "count",
                "group": "deep" if key in deep_fields else "core",
            }
        )
    for key, label in FEATURE_BOOL_FIELDS:
        specs.append(
            {
                "field": key,
                "label": label,
                "type": "bool",
                "group": "deep" if key in deep_fields else "core",
            }
        )
    return specs


def summarize_target_version_coverage(
    latest_by_install: dict[str, dict[str, Any]],
    published_versions: set[str],
    target_version: str,
    *,
    now: datetime | None = None,
    window: timedelta = timedelta(days=7),
) -> dict[str, Any]:
    current_time = now or datetime.now(timezone.utc)
    normalized_target = normalize_release_tag(target_version)
    platform_split: Counter[str] = Counter()
    target_rows: list[dict[str, Any]] = []

    for row in latest_by_install.values():
        received_at = parse_received_at(str(row["received_at"]))
        if current_time - received_at > window:
            continue
        identity = classify_row_version(row, published_versions)
        if identity.version != normalized_target:
            continue
        target_rows.append(row)
        platform = str(row.get("platform") or "unknown").strip() or "unknown"
        platform_split[platform] += 1

    signals: list[dict[str, Any]] = []
    for spec in telemetry_signal_specs():
        values: list[int] = []
        for row in target_rows:
            if spec["type"] == "bool":
                values.append(1 if parse_optional_bool(row.get(spec["field"])) else 0)
            else:
                values.append(parse_optional_nonnegative_int(row.get(spec["field"])))
        signals.append(
            {
                **spec,
                "nonzero_installs": sum(1 for value in values if value > 0),
                "total": sum(values),
            }
        )

    return {
        "version": normalized_target,
        "active_installs": len(target_rows),
        "platforms": counter_entries(platform_split, "platform"),
        "signals": signals,
    }


def summarize_rows(
    db_stats: dict[str, Any],
    rows: Iterable[dict[str, Any]],
    published_versions: set[str],
    target_version: str | None = None,
) -> dict[str, Any]:
    latest_by_install: dict[str, dict[str, Any]] = {}
    for row in rows:
        install_id = str(row["install_id"])
        existing = latest_by_install.get(install_id)
        if existing is None or str(row["received_at"]) > str(existing["received_at"]):
            latest_by_install[install_id] = row

    current_time = datetime.now(timezone.utc)
    latest_install_windows = summarize_latest_install_windows(
        latest_by_install,
        published_versions,
        now=current_time,
    )
    summary_72h = latest_install_windows["72h"]
    summary_7d = latest_install_windows["7d"]

    return {
        "db_stats": db_stats,
        "latest_install_windows": latest_install_windows,
        "deep_signal_sources_7d": summarize_deep_signal_sources(
            latest_by_install,
            published_versions,
            now=current_time,
        ),
        "target_release_coverage_7d": summarize_target_version_coverage(
            latest_by_install,
            published_versions,
            target_version,
            now=current_time,
        )
        if target_version
        else None,
        "active_latest": {
            "active_24h": latest_install_windows["24h"]["active_installs"],
            "active_72h": summary_72h["active_installs"],
            "active_7d": summary_7d["active_installs"],
        },
        "latest_version_split_72h": summary_72h["latest_versions"],
        "published_version_split_72h": summary_72h["published_versions"],
        "non_release_version_split_72h": summary_72h["non_release_versions"],
        "latest_platform_split_72h": summary_72h["platforms"],
    }


def format_target_signal(signal: dict[str, Any]) -> str:
    install_word = "install" if signal["nonzero_installs"] == 1 else "installs"
    text = f"{signal['label']}: {signal['nonzero_installs']} {install_word}"
    if signal["type"] == "count":
        text += f", total {signal['total']}"
    return text


def format_text(summary: dict[str, Any], repo: str, since_days: int) -> str:
    lines = [
        "Pulse telemetry adoption report",
        f"source window: last {since_days} day(s)",
        f"published release validation: {repo}",
        f"latest ping: {summary['db_stats'].get('latest_ping') or 'unknown'}",
        f"total rows: {summary['db_stats'].get('total_rows', 0)}",
        f"total distinct installs: {summary['db_stats'].get('total_distinct_installs', 0)}",
    ]

    for label, _ in DEFAULT_LATEST_INSTALL_WINDOWS:
        window_summary = summary["latest_install_windows"][label]
        lines.extend(
            [
                "",
                f"Latest install state ({label}):",
                f"- active installs: {window_summary['active_installs']}",
                "- published versions:",
            ]
        )
        if window_summary["published_versions"]:
            lines.extend(f"  - {entry['version']}: {entry['installs']}" for entry in window_summary["published_versions"])
        else:
            lines.append("  - none")
        lines.append("- non-release or unpublished versions:")
        if window_summary["non_release_versions"]:
            lines.extend(
                f"  - {entry['version']}: {entry['installs']}" for entry in window_summary["non_release_versions"]
            )
        else:
            lines.append("  - none")
        lines.append("- platforms:")
        if window_summary["platforms"]:
            lines.extend(f"  - {entry['platform']}: {entry['installs']}" for entry in window_summary["platforms"])
        else:
            lines.append("  - none")
        lines.append("- aggregate adoption counts:")
        adoption_counts = [entry for entry in window_summary.get("adoption_counts", []) if entry["total"] > 0]
        if adoption_counts:
            lines.extend(f"  - {entry['label']}: {entry['total']}" for entry in adoption_counts)
        else:
            lines.append("  - none")
        lines.append("- feature-enabled installs:")
        feature_counts = [entry for entry in window_summary.get("feature_enabled_installs", []) if entry["installs"] > 0]
        if feature_counts:
            lines.extend(f"  - {entry['label']}: {entry['installs']}" for entry in feature_counts)
        else:
            lines.append("  - none")

    target_coverage = summary.get("target_release_coverage_7d")
    if target_coverage:
        lines.extend(
            [
                "",
                f"Target release signal coverage (7d, {target_coverage['version']}):",
                f"- active installs: {target_coverage['active_installs']}",
                "- platforms:",
            ]
        )
        if target_coverage["platforms"]:
            lines.extend(f"  - {entry['platform']}: {entry['installs']}" for entry in target_coverage["platforms"])
        else:
            lines.append("  - none")

        for group, heading in (("core", "core signals with data"), ("deep", "deep signals with data")):
            signals = [
                signal
                for signal in target_coverage["signals"]
                if signal["group"] == group and signal["nonzero_installs"] > 0
            ]
            lines.append(f"- {heading}:")
            if signals:
                lines.extend(f"  - {format_target_signal(signal)}" for signal in signals)
            else:
                lines.append("  - none")

        missing_deep = [
            signal["label"]
            for signal in target_coverage["signals"]
            if signal["group"] == "deep" and signal["nonzero_installs"] == 0
        ]
        lines.append("- deep signals with no target-release data:")
        if missing_deep:
            lines.append("  - " + ", ".join(missing_deep))
        else:
            lines.append("  - none")

    lines.extend(["", "Deep telemetry signal sources (7d):"])
    deep_sources = summary.get("deep_signal_sources_7d", [])
    if deep_sources:
        for entry in deep_sources:
            versions = []
            for source in entry["versions"]:
                install_word = "install" if source["installs"] == 1 else "installs"
                source_text = f"{source['version']}: {source['installs']} {install_word}"
                if entry["type"] == "count":
                    source_text += f", total {source['total']}"
                versions.append(source_text)
            lines.append(f"- {entry['label']}: " + "; ".join(versions))
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
        "--target-version",
        help="release version to highlight for per-signal coverage; defaults to the latest published RC",
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

    published_releases = fetch_published_releases(args.github_repo)
    published_versions = {release["version"] for release in published_releases}
    target_version = args.target_version or latest_rc_version(published_releases)
    source = (
        fetch_rows_remote(args.ssh_host, args.db_path, args.since_days)
        if args.ssh_host
        else fetch_rows_local(args.db_path, args.since_days)
    )
    summary = summarize_rows(source["db_stats"], source["rows"], published_versions, target_version=target_version)

    if args.format == "json":
        print(json.dumps(summary, indent=2, sort_keys=True))
    else:
        print(format_text(summary, args.github_repo, args.since_days))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
