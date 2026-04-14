#!/usr/bin/env python3
"""Unit tests for telemetry_adoption_report."""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from pathlib import Path
import sys
import unittest


SCRIPT_DIR = Path(__file__).resolve().parents[1]
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

    def test_summarize_rows_uses_latest_install_state_and_splits_release_validation(self) -> None:
        now = datetime.now(timezone.utc).replace(microsecond=0)
        rows = [
            {
                "install_id": "install-a",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
            },
            {
                "install_id": "install-b",
                "version": "v6.0.0-rc.2",
                "platform": "docker",
                "received_at": (now - timedelta(hours=5)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
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
        )

        self.assertEqual(summary["active_latest"]["active_24h"], 2)
        self.assertEqual(summary["active_latest"]["active_72h"], 3)
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
        }

        rendered = report.format_text(summary, "rcourtman/Pulse", 7)

        self.assertIn("Latest install state (24h):", rendered)
        self.assertIn("Latest install state (72h):", rendered)
        self.assertIn("Latest install state (7d):", rendered)
        self.assertIn("  - 6.0.0-rc.1: 118", rendered)
        self.assertIn("  - 6.0.0-rc.2: 32", rendered)


if __name__ == "__main__":
    unittest.main()
