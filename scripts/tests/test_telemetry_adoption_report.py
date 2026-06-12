#!/usr/bin/env python3
"""Unit tests for telemetry_adoption_report."""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from pathlib import Path
import sys
import unittest


SCRIPT_DIR = Path(__file__).resolve().parents[1]
REPO_ROOT = SCRIPT_DIR.parent
sys.path.insert(0, str(SCRIPT_DIR))

import telemetry_adoption_report as report


class TelemetryAdoptionReportTest(unittest.TestCase):
    def test_normalize_reported_version_strips_v_prefix(self) -> None:
        self.assertEqual(report.normalize_reported_version("v6.0.0-rc.1"), "6.0.0-rc.1")

    def test_normalize_reported_version_converts_git_describe(self) -> None:
        self.assertEqual(
            report.normalize_reported_version("v6.0.0-rc.1-45-gABCDEF"),
            "6.0.0-rc.1+git.45.gabcdef",
        )

    def test_classify_reported_version_requires_real_published_tag(self) -> None:
        identity = report.classify_reported_version(
            "v6.0.0-rc.2",
            published_versions={"6.0.0-rc.1"},
        )
        self.assertEqual(identity.version, "6.0.0-rc.2")
        self.assertEqual(identity.channel, "rc")
        self.assertFalse(identity.is_published_release)

    def test_classify_row_version_uses_stored_identity_fields(self) -> None:
        identity = report.classify_row_version(
            {
                "version": "6.0.0-rc.1",
                "version_raw": "v6.0.0-rc.1-45-gABCDEF",
                "version_channel": "rc",
                "version_build": "git.45.gabcdef",
                "version_is_development": 0,
                "version_is_published_release": 1,
            },
            published_versions={"6.0.0-rc.1"},
        )
        self.assertEqual(identity.version, "6.0.0-rc.1")
        self.assertEqual(identity.raw_version, "v6.0.0-rc.1-45-gABCDEF")
        self.assertEqual(identity.channel, "rc")
        self.assertEqual(identity.build, "git.45.gabcdef")
        self.assertFalse(identity.is_development)
        self.assertTrue(identity.is_published_release)

    def test_classify_row_version_still_requires_real_published_release(self) -> None:
        identity = report.classify_row_version(
            {
                "version": "6.0.0-rc.2",
                "version_channel": "rc",
                "version_is_published_release": 1,
            },
            published_versions={"6.0.0-rc.1"},
        )
        self.assertEqual(identity.version, "6.0.0-rc.2")
        self.assertFalse(identity.is_published_release)

    def test_latest_rc_version_uses_published_order_and_ignores_non_app_prereleases(self) -> None:
        self.assertEqual(
            report.latest_rc_version(
                [
                    {
                        "version": "6.0.0-rc.1",
                        "is_prerelease": True,
                        "published_at": "2026-05-20T10:00:00Z",
                    },
                    {
                        "version": "6.0.0",
                        "is_prerelease": False,
                        "published_at": "2026-05-21T10:00:00Z",
                    },
                    {
                        "version": "helm-chart-5.1.33",
                        "is_prerelease": True,
                        "published_at": "2026-05-23T10:00:00Z",
                    },
                    {
                        "version": "6.0.0-rc.2",
                        "is_prerelease": True,
                        "published_at": "2026-05-22T10:00:00Z",
                    },
                ]
            ),
            "6.0.0-rc.2",
        )

    def test_security_docs_use_current_agent_install_surface(self) -> None:
        security_docs = (
            REPO_ROOT / "SECURITY.md",
            REPO_ROOT / "frontend-modern/public/docs/SECURITY.md",
        )

        for path in security_docs:
            with self.subTest(path=path.relative_to(REPO_ROOT)):
                content = path.read_text(encoding="utf-8")
                self.assertIn("Settings → Infrastructure → Install on a host", content)
                self.assertIn("Proxmox or Machines page", content)
                self.assertNotIn("Settings → Agents → Installation commands", content)
                self.assertNotIn("Settings -> Agents -> Installation commands", content)

    def test_summarize_rows_uses_latest_install_state_and_splits_release_validation(self) -> None:
        now = datetime.now(timezone.utc).replace(microsecond=0)
        rows = [
            {
                "install_id": "install-a",
                "version": "v6.0.0-rc.1",
                "version_channel": "rc",
                "version_is_published_release": 1,
                "platform": "binary",
                "received_at": (now - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
            },
            {
                "install_id": "install-b",
                "version": "v6.0.0-rc.2",
                "version_channel": "rc",
                "version_is_published_release": 1,
                "platform": "docker",
                "received_at": (now - timedelta(hours=5)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "agent_hosts": 3,
            },
            {
                "install_id": "install-b",
                "version": "v6.0.0-rc.1",
                "platform": "docker",
                "received_at": (now - timedelta(days=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "startup",
            },
            {
                "install_id": "install-c",
                "version": "feature/new-metric",
                "platform": "binary",
                "received_at": (now - timedelta(days=2, hours=1)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "agent_hosts": 2,
                "patrol_enabled": 1,
            },
        ]

        summary = report.summarize_rows(
            {
                "latest_ping": rows[0]["received_at"],
                "total_rows": 4,
                "total_distinct_installs": 3,
            },
            rows,
            published_versions={"6.0.0-rc.1"},
            target_version="v6.0.0-rc.2",
        )

        self.assertEqual(summary["active_latest"]["active_24h"], 2)
        self.assertEqual(summary["active_latest"]["active_72h"], 3)
        self.assertEqual(summary["active_latest"]["active_7d"], 3)
        self.assertEqual(summary["latest_install_windows"]["24h"]["active_installs"], 2)
        self.assertEqual(summary["latest_install_windows"]["72h"]["active_installs"], 3)
        self.assertEqual(summary["latest_install_windows"]["7d"]["active_installs"], 3)
        self.assertEqual(
            summary["published_version_split_72h"],
            [{"version": "6.0.0-rc.1", "installs": 1}],
        )
        self.assertEqual(
            summary["non_release_version_split_72h"],
            [
                {"version": "0.0.0-feature-new-metric", "installs": 1},
                {"version": "6.0.0-rc.2", "installs": 1},
            ],
        )
        self.assertEqual(
            summary["latest_install_windows"]["24h"]["published_versions"],
            [{"version": "6.0.0-rc.1", "installs": 1}],
        )
        self.assertEqual(
            summary["latest_install_windows"]["72h"]["non_release_versions"],
            [
                {"version": "0.0.0-feature-new-metric", "installs": 1},
                {"version": "6.0.0-rc.2", "installs": 1},
            ],
        )
        self.assertEqual(
            summary["latest_install_windows"]["7d"]["platforms"],
            [
                {"platform": "binary", "installs": 2},
                {"platform": "docker", "installs": 1},
            ],
        )
        deep_sources = {entry["field"]: entry for entry in summary["deep_signal_sources_7d"]}
        self.assertEqual(
            deep_sources["agent_hosts"]["versions"],
            [
                {
                    "version": "0.0.0-feature-new-metric",
                    "installs": 1,
                    "total": 2,
                    "is_published_release": False,
                },
                {
                    "version": "6.0.0-rc.2",
                    "installs": 1,
                    "total": 3,
                    "is_published_release": False,
                },
            ],
        )
        self.assertEqual(
            deep_sources["patrol_enabled"]["versions"],
            [
                {
                    "version": "0.0.0-feature-new-metric",
                    "installs": 1,
                    "total": 1,
                    "is_published_release": False,
                },
            ],
        )
        target_coverage = summary["target_release_coverage_7d"]
        self.assertEqual(target_coverage["version"], "6.0.0-rc.2")
        self.assertEqual(target_coverage["active_installs"], 1)
        self.assertEqual(target_coverage["platforms"], [{"platform": "docker", "installs": 1}])
        signals = {entry["field"]: entry for entry in target_coverage["signals"]}
        self.assertEqual(signals["agent_hosts"]["nonzero_installs"], 1)
        self.assertEqual(signals["agent_hosts"]["total"], 3)
        self.assertEqual(signals["agent_hosts"]["group"], "deep")
        self.assertEqual(signals["pve_nodes"]["group"], "core")

    def test_format_text_includes_latest_install_windows(self) -> None:
        summary = {
            "db_stats": {
                "latest_ping": "2026-04-14 10:04:08",
                "total_rows": 3228,
                "total_distinct_installs": 229,
            },
            "latest_install_windows": {
                "24h": {
                    "active_installs": 127,
                    "published_versions": [{"version": "6.0.0-rc.1", "installs": 91}],
                    "non_release_versions": [{"version": "6.0.0-rc.2", "installs": 32}],
                    "platforms": [{"platform": "docker", "installs": 66}],
                },
                "72h": {
                    "active_installs": 153,
                    "published_versions": [{"version": "6.0.0-rc.1", "installs": 117}],
                    "non_release_versions": [{"version": "6.0.0-rc.2", "installs": 32}],
                    "platforms": [{"platform": "binary", "installs": 83}],
                },
                "7d": {
                    "active_installs": 157,
                    "published_versions": [{"version": "6.0.0-rc.1", "installs": 118}],
                    "non_release_versions": [{"version": "6.0.0-rc.2", "installs": 32}],
                    "platforms": [{"platform": "binary", "installs": 87}],
                },
            },
            "deep_signal_sources_7d": [
                {
                    "field": "agent_hosts",
                    "label": "Agent hosts",
                    "type": "count",
                    "versions": [
                        {
                            "version": "6.0.0-rc.2",
                            "installs": 4,
                            "total": 18,
                            "is_published_release": False,
                        },
                    ],
                },
                {
                    "field": "patrol_enabled",
                    "label": "Patrol enabled",
                    "type": "bool",
                    "versions": [
                        {
                            "version": "6.0.0-rc.2",
                            "installs": 2,
                            "total": 2,
                            "is_published_release": False,
                        },
                    ],
                },
            ],
            "target_release_coverage_7d": {
                "version": "6.0.0-rc.6",
                "active_installs": 74,
                "platforms": [{"platform": "binary", "installs": 54}],
                "signals": [
                    {
                        "field": "pve_nodes",
                        "label": "PVE nodes",
                        "type": "count",
                        "group": "core",
                        "nonzero_installs": 55,
                        "total": 131,
                    },
                    {
                        "field": "ai_enabled",
                        "label": "AI enabled",
                        "type": "bool",
                        "group": "core",
                        "nonzero_installs": 19,
                        "total": 19,
                    },
                    {
                        "field": "agent_hosts",
                        "label": "Agent hosts",
                        "type": "count",
                        "group": "deep",
                        "nonzero_installs": 0,
                        "total": 0,
                    },
                    {
                        "field": "patrol_enabled",
                        "label": "Patrol enabled",
                        "type": "bool",
                        "group": "deep",
                        "nonzero_installs": 0,
                        "total": 0,
                    },
                ],
            },
        }

        rendered = report.format_text(summary, "rcourtman/Pulse", 7)

        self.assertIn("Latest install state (24h):", rendered)
        self.assertIn("Latest install state (72h):", rendered)
        self.assertIn("Latest install state (7d):", rendered)
        self.assertIn("  - 6.0.0-rc.1: 118", rendered)
        self.assertIn("  - 6.0.0-rc.2: 32", rendered)
        self.assertIn("Deep telemetry signal sources (7d):", rendered)
        self.assertIn("- Agent hosts: 6.0.0-rc.2: 4 installs, total 18", rendered)
        self.assertIn("- Patrol enabled: 6.0.0-rc.2: 2 installs", rendered)
        self.assertIn("Target release signal coverage (7d, 6.0.0-rc.6):", rendered)
        self.assertIn("  - PVE nodes: 55 installs, total 131", rendered)
        self.assertIn("  - AI enabled: 19 installs", rendered)
        self.assertIn("  - Agent hosts, Patrol enabled", rendered)

    def test_privacy_docs_keep_relay_mobile_handoff_copy_aligned(self) -> None:
        repo_root = Path(__file__).resolve().parents[2]
        canonical = (repo_root / "docs" / "PRIVACY.md").read_text(encoding="utf-8")
        bundled = (
            repo_root / "frontend-modern" / "public" / "docs" / "PRIVACY.md"
        ).read_text(encoding="utf-8")

        expected = "Pulse Mobile pairing for handoff"
        self.assertIn(expected, canonical)
        self.assertIn(expected, bundled)
        self.assertNotIn("mobile app pairing", canonical)
        self.assertNotIn("mobile app pairing", bundled)


if __name__ == "__main__":
    unittest.main()
