#!/usr/bin/env python3
"""Unit tests for telemetry_adoption_report."""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from pathlib import Path
import gzip
import json
import sqlite3
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock


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

    def test_fetch_rows_remote_parses_json_lines_stream(self) -> None:
        db_stats = {"latest_ping": "2026-07-17 00:00:00", "total_rows": 2, "total_distinct_installs": 2}
        rows = [
            {"install_id": "a", "received_at": "2026-07-17 00:00:00"},
            {"install_id": "b", "received_at": "2026-07-16 00:00:00"},
        ]
        analysis_facts = [
            {
                "install_id": "a",
                "latest_received_at": "2026-07-17 00:00:00",
                "first_free_at": "2026-07-17 00:00:00",
                "first_paid_at": None,
                "signal_fields": [],
                "free_signal_fields": [],
            }
        ]
        first_counts = [0] * len(report.TARGET_RELEASE_ACTIVITY_COUNT_FIELDS)
        increase_totals = list(first_counts)
        increase_totals[0] = 2
        decrease_totals = list(first_counts)
        target_release_facts = [
            {
                "install_id": "a",
                "heartbeat_count": 2,
                "same_version_pair_count": 1,
                "first_received_at": "2026-07-16 23:00:00",
                "latest_received_at": "2026-07-17 00:00:00",
                "first_counts": first_counts,
                "increase_totals": increase_totals,
                "decrease_totals": decrease_totals,
            }
        ]
        stdout = "\n".join(
            [
                json.dumps(
                    {
                        "db_stats": db_stats,
                        "row_columns": list(report.REPORT_ROW_COLUMNS),
                        "unavailable_columns": [],
                    }
                ),
                *(
                    json.dumps(
                        {
                            "a": [
                                fact["install_id"],
                                fact["latest_received_at"],
                                fact["first_free_at"],
                                fact["first_paid_at"],
                                fact["signal_fields"],
                                fact["free_signal_fields"],
                            ]
                        }
                    )
                    for fact in analysis_facts
                ),
                *(
                    json.dumps(
                        {
                            "t": [
                                fact["install_id"],
                                fact["heartbeat_count"],
                                fact["same_version_pair_count"],
                                fact["first_received_at"],
                                fact["latest_received_at"],
                                fact["first_counts"],
                                fact["increase_totals"],
                                fact["decrease_totals"],
                            ]
                        }
                    )
                    for fact in target_release_facts
                ),
                *(
                    json.dumps(
                        {
                            "r": [row.get(column) for column in report.REPORT_ROW_COLUMNS]
                        }
                    )
                    for row in rows
                ),
                "",
            ]
        )
        completed = subprocess.CompletedProcess(
            args=[],
            returncode=0,
            stdout=gzip.compress(stdout.encode("utf-8")),
            stderr=b"",
        )
        with mock.patch.object(report.subprocess, "run", return_value=completed) as run_mock:
            result = report.fetch_rows_remote(
                "pulse-license",
                "/opt/licenses.sqlite",
                30,
                target_version="v6.3.0-rc.3",
            )
        expanded_rows = [
            {column: row.get(column) for column in report.REPORT_ROW_COLUMNS}
            for row in rows
        ]
        self.assertEqual(
            result,
            {
                "db_stats": db_stats,
                "rows": expanded_rows,
                "pulse_intelligence_analysis_facts": analysis_facts,
                "target_release_analysis_facts": target_release_facts,
                "unavailable_columns": [],
            },
        )
        remote_script = run_mock.call_args.kwargs["input"].decode("utf-8")
        self.assertNotIn("fetchall", remote_script)
        self.assertNotIn("SELECT *", remote_script)
        self.assertIn("received_at >= datetime('now', ?)", remote_script)
        self.assertEqual(
            run_mock.call_args.args[0][-4],
            ",".join(report.REPORT_ROW_COLUMNS),
        )
        self.assertEqual(
            run_mock.call_args.args[0][-3],
            ",".join(report.REPORT_HISTORY_SIGNAL_COLUMNS),
        )
        self.assertEqual(run_mock.call_args.args[0][-2], "6.3.0-rc.3")
        self.assertEqual(
            run_mock.call_args.args[0][-1],
            ",".join(report.TARGET_RELEASE_ACTIVITY_COUNT_FIELDS),
        )
        self.assertIn("ROW_NUMBER() OVER (", remote_script)
        self.assertIn("GROUP BY install_id", remote_script)
        self.assertIn("MIN(CASE WHEN paid_license = 0", remote_script)
        self.assertIn("target_analysis_sql", remote_script)
        self.assertIn("PRAGMA table_info(telemetry_pings)", remote_script)
        self.assertIn("0 AS \" + name", remote_script)
        compile(remote_script, "<telemetry-remote-fetch>", "exec")

    def test_report_projection_and_signal_specs_cover_recent_schema_fields(self) -> None:
        recent_count_fields = {
            "availability_probe_targets",
            "availability_probe_agents",
            "rbac_custom_roles",
            "rbac_user_assignments",
            "audit_reads_30d",
            "report_schedules",
            "report_schedules_enabled",
            "report_schedules_run_30d",
            "agent_profiles",
            "update_attempts_30d",
            "update_successes_30d",
            "update_failures_30d",
        }
        projected = set(report.REPORT_ROW_COLUMNS)
        self.assertTrue(recent_count_fields <= projected)
        self.assertIn("alert_ai_enabled", projected)
        self.assertIn("update_last_failure_category", projected)
        self.assertTrue(set(report.SERVICE_HEALTH_ROW_FIELDS) <= projected)
        self.assertNotIn("business_estate", projected)

        specs = {entry["field"]: entry for entry in report.telemetry_signal_specs()}
        for field in recent_count_fields:
            self.assertEqual(specs[field]["type"], "count")
            self.assertEqual(specs[field]["group"], "deep")
        self.assertEqual(specs["alert_ai_enabled"]["type"], "bool")
        self.assertEqual(specs["alert_ai_enabled"]["group"], "deep")

    def test_remote_target_release_query_executes_and_emits_compact_pair_deltas(self) -> None:
        empty_header = gzip.compress(
            (
                json.dumps(
                    {
                        "db_stats": {},
                        "row_columns": list(report.REPORT_ROW_COLUMNS),
                    }
                )
                + "\n"
            ).encode("utf-8")
        )
        completed = subprocess.CompletedProcess(
            args=[],
            returncode=0,
            stdout=empty_header,
            stderr=b"",
        )
        with mock.patch.object(report.subprocess, "run", return_value=completed) as run_mock:
            report.fetch_rows_remote(
                "pulse-license",
                "/unused.sqlite",
                7,
                target_version="6.3.0-rc.3",
            )
        remote_script = run_mock.call_args.kwargs["input"]

        numeric_columns = {
            "schema_version",
            "version_is_development",
            "version_is_published_release",
            "notification_failures_7d",
            *(key for key, _ in report.ADOPTION_COUNT_FIELDS),
            *(key for key, _ in report.FEATURE_BOOL_FIELDS),
            *(key for key, _ in report.USER_BASE_BOOL_FIELDS),
            *(key for key, _ in report.USER_BASE_COUNT_FIELDS),
            *(key for key, _ in report.PULSE_INTELLIGENCE_BOOL_FIELDS),
            *(key for key, _ in report.PULSE_INTELLIGENCE_COUNT_FIELDS),
        }
        column_definitions = ", ".join(
            f"{column} {'INTEGER' if column in numeric_columns else 'TEXT'}"
            for column in report.REPORT_ROW_COLUMNS
        )
        now = datetime.now(timezone.utc).replace(microsecond=0)
        rows = [
            {
                "install_id": "install-a",
                "version": "6.3.0-rc.3",
                "received_at": (now - timedelta(hours=3)).strftime("%Y-%m-%d %H:%M:%S"),
                "alerts_fired_30d": 4,
            },
            {
                "install_id": "install-a",
                "version": "6.3.0-rc.3",
                "received_at": (now - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "alerts_fired_30d": 6,
            },
            {
                "install_id": "install-a",
                "version": "6.2.1",
                "received_at": (now - timedelta(hours=1)).strftime("%Y-%m-%d %H:%M:%S"),
                "alerts_fired_30d": 6,
            },
        ]
        with tempfile.TemporaryDirectory() as temp_dir:
            db_path = str(Path(temp_dir) / "telemetry.sqlite")
            conn = sqlite3.connect(db_path)
            try:
                conn.execute(f"CREATE TABLE telemetry_pings ({column_definitions})")
                for row in rows:
                    columns = ", ".join(row)
                    placeholders = ", ".join("?" for _ in row)
                    conn.execute(
                        f"INSERT INTO telemetry_pings ({columns}) VALUES ({placeholders})",
                        tuple(row.values()),
                    )
                conn.commit()
            finally:
                conn.close()

            result = subprocess.run(
                [
                    sys.executable,
                    "-",
                    db_path,
                    "7",
                    ",".join(report.REPORT_ROW_COLUMNS),
                    ",".join(report.REPORT_HISTORY_SIGNAL_COLUMNS),
                    "6.3.0-rc.3",
                    ",".join(report.TARGET_RELEASE_ACTIVITY_COUNT_FIELDS),
                ],
                input=remote_script,
                capture_output=True,
                check=True,
            )

        records = [
            json.loads(line)
            for line in gzip.decompress(result.stdout).decode("utf-8").splitlines()
        ]
        target_record = next(record["t"] for record in records if "t" in record)
        alerts_index = report.TARGET_RELEASE_ACTIVITY_COUNT_FIELDS.index("alerts_fired_30d")
        self.assertEqual(target_record[1], 2)
        self.assertEqual(target_record[2], 1)
        self.assertEqual(target_record[5][alerts_index], 4)
        self.assertEqual(target_record[6][alerts_index], 2)
        self.assertEqual(target_record[7][alerts_index], 0)

    def test_compact_intelligence_facts_match_full_history_analysis(self) -> None:
        numeric_columns = {
            "schema_version",
            "version_is_development",
            "version_is_published_release",
            *(key for key, _ in report.ADOPTION_COUNT_FIELDS),
            *(key for key, _ in report.FEATURE_BOOL_FIELDS),
            *(key for key, _ in report.USER_BASE_BOOL_FIELDS),
            *(key for key, _ in report.USER_BASE_COUNT_FIELDS),
            *(key for key, _ in report.PULSE_INTELLIGENCE_BOOL_FIELDS),
            *(key for key, _ in report.PULSE_INTELLIGENCE_COUNT_FIELDS),
        }
        column_definitions = ", ".join(
            f"{column} {'INTEGER' if column in numeric_columns else 'TEXT'}"
            for column in report.REPORT_ROW_COLUMNS
        )
        now = datetime.now(timezone.utc).replace(microsecond=0)
        rows = [
            {
                "install_id": "install-a",
                "received_at": (now - timedelta(hours=6)).strftime("%Y-%m-%d %H:%M:%S"),
                "paid_license": 0,
            },
            {
                "install_id": "install-a",
                "received_at": (now - timedelta(hours=5)).strftime("%Y-%m-%d %H:%M:%S"),
                "paid_license": 0,
                "pulse_intelligence_loop_configured": 1,
            },
            {
                "install_id": "install-a",
                "received_at": (now - timedelta(hours=4)).strftime("%Y-%m-%d %H:%M:%S"),
                "paid_license": 0,
            },
            {
                "install_id": "install-a",
                "received_at": (now - timedelta(hours=3)).strftime("%Y-%m-%d %H:%M:%S"),
                "paid_license": 1,
            },
            {
                "install_id": "install-a",
                "received_at": (now - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "paid_license": 1,
                "pulse_intelligence_approved_action_successes_30d": 1,
            },
            {
                "install_id": "install-a",
                "received_at": (now - timedelta(hours=1)).strftime("%Y-%m-%d %H:%M:%S"),
                "paid_license": 1,
            },
        ]

        with tempfile.TemporaryDirectory() as temp_dir:
            db_path = str(Path(temp_dir) / "telemetry.sqlite")
            conn = sqlite3.connect(db_path)
            try:
                conn.execute(f"CREATE TABLE telemetry_pings ({column_definitions})")
                for row in rows:
                    columns = ", ".join(row)
                    placeholders = ", ".join("?" for _ in row)
                    conn.execute(
                        f"INSERT INTO telemetry_pings ({columns}) VALUES ({placeholders})",
                        tuple(row.values()),
                    )
                conn.commit()
            finally:
                conn.close()

            fetched_rows = report.fetch_rows_local(db_path, 1)["rows"]

        self.assertEqual(len(fetched_rows), len(rows))
        facts = [
            {
                "install_id": "install-a",
                "latest_received_at": rows[-1]["received_at"],
                "first_free_at": rows[0]["received_at"],
                "first_paid_at": rows[3]["received_at"],
                "signal_fields": [
                    "pulse_intelligence_loop_configured",
                    "pulse_intelligence_approved_action_successes_30d",
                ],
                "free_signal_fields": ["pulse_intelligence_loop_configured"],
            }
        ]
        self.assertEqual(
            report.analyze_pulse_intelligence_facts(facts),
            report.analyze_pulse_intelligence_rows(rows),
        )
        self.assertEqual(fetched_rows[0]["received_at"], rows[-1]["received_at"])

    def test_fetch_rows_remote_rejects_empty_response(self) -> None:
        completed = subprocess.CompletedProcess(
            args=[],
            returncode=0,
            stdout=gzip.compress(b"\n"),
            stderr=b"",
        )
        with mock.patch.object(report.subprocess, "run", return_value=completed):
            with self.assertRaisesRegex(RuntimeError, "empty response"):
                report.fetch_rows_remote("pulse-license", "/opt/licenses.sqlite", 30)

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

    def test_latest_target_release_version_prefers_latest_stable_release(self) -> None:
        self.assertEqual(
            report.latest_target_release_version(
                [
                    {
                        "version": "6.1.2",
                        "is_prerelease": False,
                        "published_at": "2026-07-26T22:11:46Z",
                    },
                    {
                        "version": "6.2.0-rc.1",
                        "is_prerelease": True,
                        "published_at": "2026-07-27T08:00:00Z",
                    },
                    {
                        "version": "helm-chart-6.1.2",
                        "is_prerelease": False,
                        "published_at": "2026-07-27T09:00:00Z",
                    },
                ]
            ),
            "6.1.2",
        )

    def test_latest_target_release_version_falls_back_to_latest_rc(self) -> None:
        self.assertEqual(
            report.latest_target_release_version(
                [
                    {
                        "version": "6.2.0-rc.1",
                        "is_prerelease": True,
                        "published_at": "2026-07-27T08:00:00Z",
                    }
                ]
            ),
            "6.2.0-rc.1",
        )

    def test_compare_semver_precedence_orders_rc_stable_and_patch_rollbacks(self) -> None:
        self.assertLess(report.compare_semver_precedence("6.3.0-rc.2", "6.3.0-rc.3"), 0)
        self.assertGreater(report.compare_semver_precedence("6.3.0", "6.3.0-rc.3"), 0)
        self.assertLess(report.compare_semver_precedence("6.2.1", "6.3.0-rc.3"), 0)
        self.assertEqual(report.compare_semver_precedence("6.3.0+build.2", "6.3.0"), 0)

    def test_target_release_service_health_uses_direct_version_change_observations(self) -> None:
        now = datetime(2026, 8, 29, 12, tzinfo=timezone.utc)
        rows = {
            "healthy-upgrade": {
                "install_id": "healthy-upgrade",
                "received_at": "2026-08-29 11:00:00",
                "version": "6.5.0",
                "service_health_observed": 1,
                "service_health_healthy": 1,
                "service_health_cohort": "version_change",
                "service_health_previous_version": "6.4.0",
                "service_health_previous_observed": 1,
                "service_health_previous_healthy": 1,
            },
            "broken-upgrade": {
                "install_id": "broken-upgrade",
                "received_at": "2026-08-29 10:00:00",
                "version": "6.5.0",
                "service_health_observed": 1,
                "service_health_healthy": 0,
                "service_health_failure_category": "frontend_assets",
                "service_health_cohort": "version_change",
                "service_health_previous_version": "6.4.0",
                "service_health_previous_observed": 1,
                "service_health_previous_healthy": 1,
                # Rolling counters are deliberately irrelevant to this summary.
                "update_successes_30d": 99,
            },
            "legacy-target": {
                "install_id": "legacy-target",
                "received_at": "2026-08-29 09:00:00",
                "version": "6.5.0",
                "service_health_observed": 0,
            },
            "other-release": {
                "install_id": "other-release",
                "received_at": "2026-08-29 11:30:00",
                "version": "6.4.0",
                "service_health_observed": 1,
                "service_health_healthy": 0,
                "service_health_failure_category": "listener",
            },
        }

        summary = report.summarize_target_release_service_health(
            rows,
            {"6.4.0", "6.5.0"},
            "6.5.0",
            now=now,
        )

        self.assertEqual(summary["target_installs"], 3)
        self.assertEqual(summary["observed_installs"], 2)
        self.assertEqual(summary["unobserved_installs"], 1)
        self.assertEqual(summary["healthy_installs"], 1)
        self.assertEqual(summary["unhealthy_installs"], 1)
        self.assertEqual(summary["comparable_version_change_installs"], 2)
        self.assertEqual(
            {entry["transition"]: entry["installs"] for entry in summary["transitions"]},
            {"healthy_to_healthy": 1, "healthy_to_unhealthy": 1},
        )
        self.assertEqual(
            {entry["category"]: entry["installs"] for entry in summary["failure_categories"]},
            {"healthy": 1, "frontend_assets": 1},
        )

    def test_target_release_followup_excludes_first_heartbeat_baselines_and_flags_rollbacks(self) -> None:
        now = datetime(2026, 8, 19, 12, tzinfo=timezone.utc)

        def row(
            install_id: str,
            version: str,
            hour: int,
            **signals: object,
        ) -> dict[str, object]:
            return {
                "install_id": install_id,
                "version": version,
                "platform": "binary",
                "received_at": now.replace(hour=hour).strftime("%Y-%m-%d %H:%M:%S"),
                **signals,
            }

        rows = [
            row("followup", "6.3.0-rc.2", 7, alerts_fired_30d=38),
            row(
                "followup",
                "6.3.0-rc.3",
                8,
                alerts_fired_30d=40,
                pulse_intelligence_patrol_ai_calls_30d=100,
            ),
            row(
                "followup",
                "6.3.0-rc.3",
                10,
                alerts_fired_30d=43,
                pulse_intelligence_patrol_ai_calls_30d=100,
            ),
            row("baseline-only", "6.3.0-rc.3", 9, alerts_fired_30d=500),
            row("rollback", "6.3.0-rc.3", 7, pulse_intelligence_patrol_runs_30d=10),
            row("rollback", "6.3.0-rc.3", 8, pulse_intelligence_patrol_runs_30d=12),
            row("rollback", "6.2.1", 11, pulse_intelligence_patrol_runs_30d=12),
            row("forward", "6.3.0-rc.3", 8),
            row("forward", "6.3.0", 11),
            row("drift", "6.3.0-rc.3", 8),
            row("drift", "feature/local-build", 11),
        ]
        full_summary = report.summarize_rows(
            {
                "latest_ping": rows[-1]["received_at"],
                "total_rows": len(rows),
                "total_distinct_installs": 5,
            },
            rows,
            published_versions={"6.2.1", "6.3.0-rc.2", "6.3.0-rc.3", "6.3.0"},
            target_version="v6.3.0-rc.3",
            now=now,
        )
        summary = full_summary["target_release_followup"]

        self.assertEqual(summary["installs_seen"], 5)
        self.assertEqual(summary["current_target_installs"], 2)
        self.assertEqual(summary["same_version_followup_installs"], 2)
        self.assertEqual(summary["current_target_followup_installs"], 1)
        self.assertEqual(summary["departed_followup_installs"], 1)
        self.assertEqual(summary["without_later_same_version_heartbeat"], 3)
        self.assertEqual(summary["rollback_installs"], 1)
        self.assertEqual(
            summary["rollback_transitions"],
            [{"destination_version": "6.2.1", "installs": 1}],
        )
        self.assertEqual(
            summary["forward_transitions"],
            [{"destination_version": "6.3.0", "installs": 1}],
        )
        self.assertEqual(
            summary["unclassified_transitions"],
            [{"destination_version": "0.0.0-feature-local-build", "installs": 1}],
        )

        signals = {entry["field"]: entry for entry in summary["activity_signals"]}
        alerts = signals["alerts_fired_30d"]
        self.assertEqual(alerts["first_heartbeat_baseline_total"], 540)
        self.assertEqual(alerts["same_version_total_increase"], 3)
        self.assertEqual(alerts["same_version_increased_installs"], 1)
        self.assertEqual(alerts["current_target_total_increase"], 3)
        self.assertEqual(alerts["departed_total_increase"], 0)
        patrol_calls = signals["pulse_intelligence_patrol_ai_calls_30d"]
        self.assertEqual(patrol_calls["first_heartbeat_baseline_total"], 100)
        self.assertEqual(patrol_calls["same_version_total_increase"], 0)
        self.assertEqual(patrol_calls["same_version_unchanged_installs"], 2)
        patrol_runs = signals["pulse_intelligence_patrol_runs_30d"]
        self.assertEqual(patrol_runs["same_version_total_increase"], 2)
        self.assertEqual(patrol_runs["current_target_total_increase"], 0)
        self.assertEqual(patrol_runs["departed_total_increase"], 2)

        rendered = report.format_text(full_summary, "rcourtman/Pulse", 7)
        self.assertIn("Target release follow-up (6.3.0-rc.3):", rendered)
        self.assertIn("first target-version heartbeat in the source window is baseline only", rendered)
        self.assertIn("Alerts fired (30d): +3 across 1 install(s)", rendered)
        self.assertIn("rollback transitions:", rendered)
        self.assertIn("6.2.1: 1 install(s)", rendered)

    def test_compact_target_release_facts_match_local_one_pass_analysis(self) -> None:
        rows = [
            {
                "install_id": "install-a",
                "version": "6.3.0-rc.3",
                "received_at": "2026-08-19 08:00:00",
                "alerts_fired_30d": 7,
            },
            {
                "install_id": "install-a",
                "version": "6.3.0-rc.3",
                "received_at": "2026-08-19 09:00:00",
                "alerts_fired_30d": 9,
            },
            {
                "install_id": "install-a",
                "version": "6.2.1",
                "received_at": "2026-08-19 10:00:00",
                "alerts_fired_30d": 9,
            },
        ]
        original_parse_received_at = report.parse_received_at
        with mock.patch.object(
            report,
            "parse_received_at",
            wraps=original_parse_received_at,
        ) as parse_received_at:
            local = report.analyze_target_release_rows(
                rows,
                {"6.3.0-rc.3", "6.2.1"},
                "6.3.0-rc.3",
            )
        self.assertEqual(parse_received_at.call_count, 3)

        analysis = local["install-a"]
        facts = [{
            "install_id": "install-a",
            "heartbeat_count": analysis.heartbeat_count,
            "same_version_pair_count": analysis.same_version_pair_count,
            "first_received_at": analysis.first_received_at.strftime("%Y-%m-%d %H:%M:%S"),
            "latest_received_at": analysis.latest_received_at.strftime("%Y-%m-%d %H:%M:%S"),
            "first_counts": list(analysis.first_counts),
            "increase_totals": list(analysis.increase_totals),
            "decrease_totals": list(analysis.decrease_totals),
        }]
        self.assertEqual(report.analyze_target_release_facts(facts), local)

    def test_value_loop_reconciles_schema_v4_action_outcomes_and_refusals(self) -> None:
        now = datetime.now(timezone.utc).replace(microsecond=0)
        summary = report.summarize_pulse_intelligence_value_loop(
            {
                "install-a": {
                    "install_id": "install-a",
                    "received_at": now.strftime("%Y-%m-%d %H:%M:%S"),
                    "schema_version": 4,
                    "pulse_intelligence_approved_action_attempts_30d": 7,
                    "pulse_intelligence_approved_action_successes_30d": 1,
                    "pulse_intelligence_approved_action_failures_pre_dispatch_30d": 2,
                    "pulse_intelligence_approved_action_failures_execution_30d": 1,
                    "pulse_intelligence_approved_action_failures_unverified_30d": 1,
                    "pulse_intelligence_approved_action_stuck_executing_30d": 1,
                    "pulse_intelligence_approved_action_in_flight_30d": 1,
                    "pulse_intelligence_approved_action_unclassified_30d": 0,
                    "pulse_intelligence_approved_action_refusals_plan_stale_30d": 1,
                    "pulse_intelligence_approved_action_refusals_policy_30d": 1,
                    "pulse_intelligence_verified_finding_resolutions_30d": 1,
                }
            },
            now=now,
        )
        accounting = summary["approved_action_outcome_accounting"]
        self.assertEqual(accounting["attempts"], 7)
        self.assertEqual(accounting["accounted"], 7)
        self.assertEqual(accounting["gap"], 0)
        self.assertEqual(accounting["overflow"], 0)
        self.assertEqual(accounting["pre_dispatch_refusals"], 2)
        self.assertEqual(accounting["refusal_categories_accounted"], 2)
        self.assertEqual(accounting["refusal_category_gap"], 0)

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
                "paid_license": 1,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_loop_active_30d": 1,
                "pulse_intelligence_assistant_ai_calls_30d": 9,
                "pulse_intelligence_assistant_context_ai_calls_30d": 4,
                "pulse_intelligence_external_agent_enabled": 1,
                "pulse_intelligence_external_agent_used_30d": 1,
                "pulse_intelligence_mcp_adapter_used_30d": 1,
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
                "paid_license": 0,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_loop_active_30d": 1,
                "pulse_intelligence_governed_action_active_30d": 1,
                "pulse_intelligence_patrol_runs_30d": 2,
                "pulse_intelligence_action_plans_30d": 2,
                "pulse_intelligence_approval_requests_30d": 1,
                "pulse_intelligence_approved_action_attempts_30d": 1,
                "pulse_intelligence_approved_action_successes_30d": 1,
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
                "paid_license": 1,
                "patrol_enabled": 1,
                "pulse_intelligence_loop_configured": 1,
            },
            {
                "install_id": "install-d",
                "version": "v6.0.0-rc.1",
                "version_channel": "rc",
                "version_is_published_release": 1,
                "platform": "binary",
                "received_at": (now - timedelta(days=20)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_loop_active_30d": 1,
                "pulse_intelligence_complete_operations_loop_30d": 1,
                "pulse_intelligence_approved_execution_loop_30d": 1,
                "pulse_intelligence_external_agent_used_30d": 1,
                "pulse_intelligence_patrol_runs_30d": 1,
                "pulse_intelligence_action_plans_30d": 1,
                "pulse_intelligence_approved_action_attempts_30d": 1,
                "pulse_intelligence_approved_action_successes_30d": 1,
            },
        ]

        summary = report.summarize_rows(
            {
                "latest_ping": rows[0]["received_at"],
                "total_rows": 5,
                "total_distinct_installs": 4,
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
        pulse_loop = summary["pulse_intelligence_value_loop_7d"]
        self.assertEqual(pulse_loop["active_installs"], 3)
        self.assertEqual(pulse_loop["paid_installs"], 2)
        self.assertEqual(pulse_loop["free_installs"], 1)
        loop_flags = {entry["field"]: entry for entry in pulse_loop["boolean_signals"]}
        self.assertEqual(loop_flags["pulse_intelligence_loop_configured"]["installs"], 3)
        self.assertEqual(loop_flags["pulse_intelligence_loop_configured"]["paid_installs"], 2)
        self.assertEqual(loop_flags["pulse_intelligence_loop_active_30d"]["installs"], 2)
        self.assertEqual(loop_flags["pulse_intelligence_governed_action_active_30d"]["free_installs"], 1)
        loop_counts = {entry["field"]: entry for entry in pulse_loop["count_signals"]}
        self.assertEqual(loop_counts["pulse_intelligence_assistant_ai_calls_30d"]["total"], 9)
        self.assertEqual(loop_counts["pulse_intelligence_assistant_ai_calls_30d"]["paid_total"], 9)
        self.assertEqual(loop_counts["pulse_intelligence_assistant_context_ai_calls_30d"]["total"], 4)
        self.assertEqual(loop_counts["pulse_intelligence_assistant_context_ai_calls_30d"]["paid_total"], 4)
        self.assertEqual(loop_counts["pulse_intelligence_action_plans_30d"]["free_total"], 2)
        self.assertEqual(loop_counts["pulse_intelligence_approved_action_successes_30d"]["free_total"], 1)
        cohorts = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_outcome_cohorts"]["cohorts"]
        }
        self.assertEqual(cohorts["loop_configured"]["installs"], 4)
        self.assertEqual(cohorts["loop_configured"]["retained_7d"], 3)
        self.assertEqual(cohorts["loop_configured"]["retained_7d_rate_pct"], 75)
        self.assertEqual(cohorts["loop_configured"]["paid_latest"], 2)
        self.assertEqual(cohorts["loop_configured"]["paid_latest_rate_pct"], 50)
        self.assertEqual(cohorts["loop_configured"]["free_latest"], 2)
        self.assertEqual(cohorts["loop_active_30d"]["installs"], 3)
        self.assertEqual(cohorts["loop_active_30d"]["retained_7d"], 2)
        self.assertEqual(cohorts["loop_active_30d"]["retained_7d_rate_pct"], 66.67)
        self.assertEqual(cohorts["complete_operations_loop_30d"]["installs"], 1)
        self.assertEqual(cohorts["complete_operations_loop_30d"]["free_latest"], 1)
        self.assertEqual(cohorts["complete_operations_loop_30d"]["retained_7d"], 0)
        self.assertEqual(cohorts["complete_operations_loop_30d"]["retained_7d_rate_pct"], 0)
        self.assertEqual(cohorts["approved_execution_loop_30d"]["installs"], 1)
        self.assertEqual(cohorts["approved_execution_loop_30d"]["free_latest"], 1)
        self.assertEqual(cohorts["assistant_activity"]["installs"], 1)
        self.assertEqual(cohorts["assistant_context_activity"]["installs"], 1)
        self.assertEqual(cohorts["patrol_activity"]["installs"], 2)
        self.assertEqual(cohorts["patrol_activity"]["retained_7d"], 1)
        self.assertEqual(cohorts["external_agent_used_30d"]["installs"], 2)
        self.assertEqual(cohorts["external_agent_used_30d"]["retained_7d"], 1)
        self.assertEqual(cohorts["mcp_adapter_used_30d"]["installs"], 1)
        self.assertEqual(cohorts["mcp_adapter_used_30d"]["retained_7d"], 1)
        self.assertEqual(cohorts["governed_action_active_30d"]["installs"], 2)
        self.assertEqual(cohorts["governed_action_active_30d"]["retained_7d"], 1)
        self.assertEqual(cohorts["approved_action_execution_30d"]["installs"], 2)
        self.assertEqual(cohorts["approved_action_execution_30d"]["retained_7d"], 1)
        self.assertEqual(cohorts["approved_action_success_30d"]["installs"], 2)
        self.assertEqual(cohorts["approved_action_success_30d"]["retained_7d"], 1)
        funnel = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_operations_loop_funnel"]["stages"]
        }
        self.assertEqual(funnel["configured"]["installs"], 4)
        self.assertEqual(funnel["configured"]["retained_7d"], 3)
        self.assertEqual(funnel["configured"]["retained_7d_rate_pct"], 75)
        self.assertEqual(funnel["patrol_activity"]["installs"], 2)
        self.assertEqual(funnel["patrol_activity"]["retained_7d"], 1)
        self.assertEqual(funnel["patrol_activity"]["retained_7d_rate_pct"], 50)
        self.assertEqual(funnel["assistant_mcp_collaboration"]["installs"], 2)
        self.assertEqual(funnel["assistant_mcp_collaboration"]["retained_7d"], 1)
        self.assertEqual(funnel["governed_action"]["installs"], 2)
        self.assertEqual(funnel["governed_action"]["retained_7d"], 1)
        self.assertEqual(funnel["approved_action_execution"]["installs"], 2)
        self.assertEqual(funnel["approved_action_execution"]["retained_7d"], 1)
        self.assertEqual(funnel["approved_action_success"]["installs"], 2)
        self.assertEqual(funnel["approved_action_success"]["retained_7d"], 1)
        self.assertEqual(funnel["complete_operations_loop"]["installs"], 1)
        self.assertEqual(funnel["complete_operations_loop"]["retained_7d"], 0)
        self.assertEqual(funnel["complete_operations_loop"]["free_latest"], 1)
        self.assertEqual(funnel["approved_execution_loop"]["installs"], 1)
        self.assertEqual(funnel["approved_execution_loop"]["retained_7d"], 0)
        self.assertEqual(funnel["approved_execution_loop"]["free_latest"], 1)

    def test_pulse_intelligence_outcome_cohorts_record_observed_conversion(self) -> None:
        now = datetime.now(timezone.utc).replace(microsecond=0)
        rows = [
            {
                "install_id": "converted",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_assistant_ai_calls_30d": 1,
                "pulse_intelligence_assistant_context_ai_calls_30d": 1,
                "pulse_intelligence_mcp_adapter_used_30d": 1,
            },
            {
                "install_id": "converted",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=1)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_loop_active_30d": 1,
                "pulse_intelligence_assistant_ai_calls_30d": 3,
                "pulse_intelligence_assistant_context_ai_calls_30d": 3,
                "pulse_intelligence_mcp_adapter_used_30d": 1,
            },
            {
                "install_id": "still-free",
                "version": "v6.0.0-rc.1",
                "platform": "docker",
                "received_at": (now - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_patrol_runs_30d": 1,
            },
            {
                "install_id": "paid-first",
                "version": "v6.0.0-rc.1",
                "platform": "docker",
                "received_at": (now - timedelta(hours=3)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_loop_active_30d": 1,
            },
            {
                "install_id": "unknown-first",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=4)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "pulse_intelligence_loop_configured": 1,
            },
        ]

        summary = report.summarize_rows(
            {
                "latest_ping": rows[1]["received_at"],
                "total_rows": len(rows),
                "total_distinct_installs": 4,
            },
            rows,
            published_versions={"6.0.0-rc.1"},
        )

        cohorts = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_outcome_cohorts"]["cohorts"]
        }
        self.assertEqual(cohorts["loop_configured"]["installs"], 3)
        self.assertEqual(cohorts["loop_configured"]["paid_latest"], 1)
        self.assertEqual(cohorts["loop_configured"]["free_latest"], 2)
        self.assertEqual(cohorts["loop_configured"]["observed_free_starts"], 2)
        self.assertEqual(cohorts["loop_configured"]["observed_free_to_paid"], 1)
        self.assertEqual(cohorts["loop_configured"]["observed_free_to_paid_rate_pct"], 50)
        self.assertEqual(cohorts["loop_configured"]["observed_signal_free_starts"], 2)
        self.assertEqual(cohorts["loop_configured"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(cohorts["loop_configured"]["observed_signal_free_to_paid_rate_pct"], 50)
        self.assertEqual(cohorts["loop_active_30d"]["installs"], 3)
        self.assertEqual(cohorts["loop_active_30d"]["observed_free_starts"], 2)
        self.assertEqual(cohorts["loop_active_30d"]["observed_free_to_paid"], 1)
        self.assertEqual(cohorts["loop_active_30d"]["observed_free_to_paid_rate_pct"], 50)
        self.assertEqual(cohorts["loop_active_30d"]["observed_signal_free_starts"], 2)
        self.assertEqual(cohorts["loop_active_30d"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(cohorts["loop_active_30d"]["observed_signal_free_to_paid_rate_pct"], 50)
        self.assertEqual(cohorts["assistant_activity"]["installs"], 1)
        self.assertEqual(cohorts["assistant_activity"]["observed_free_starts"], 1)
        self.assertEqual(cohorts["assistant_activity"]["observed_free_to_paid"], 1)
        self.assertEqual(cohorts["assistant_activity"]["observed_free_to_paid_rate_pct"], 100)
        self.assertEqual(cohorts["assistant_activity"]["observed_signal_free_starts"], 1)
        self.assertEqual(cohorts["assistant_activity"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(cohorts["assistant_activity"]["observed_signal_free_to_paid_rate_pct"], 100)
        self.assertEqual(cohorts["assistant_context_activity"]["installs"], 1)
        self.assertEqual(cohorts["assistant_context_activity"]["observed_free_starts"], 1)
        self.assertEqual(cohorts["assistant_context_activity"]["observed_free_to_paid"], 1)
        self.assertEqual(cohorts["assistant_context_activity"]["observed_signal_free_starts"], 1)
        self.assertEqual(cohorts["assistant_context_activity"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(cohorts["mcp_adapter_used_30d"]["installs"], 1)
        self.assertEqual(cohorts["mcp_adapter_used_30d"]["observed_free_starts"], 1)
        self.assertEqual(cohorts["mcp_adapter_used_30d"]["observed_free_to_paid"], 1)
        self.assertEqual(cohorts["mcp_adapter_used_30d"]["observed_signal_free_starts"], 1)
        self.assertEqual(cohorts["mcp_adapter_used_30d"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(cohorts["patrol_activity"]["installs"], 1)
        self.assertEqual(cohorts["patrol_activity"]["observed_free_starts"], 1)
        self.assertEqual(cohorts["patrol_activity"]["observed_free_to_paid"], 0)
        self.assertEqual(cohorts["patrol_activity"]["observed_signal_free_starts"], 1)
        self.assertEqual(cohorts["patrol_activity"]["observed_signal_free_to_paid"], 0)

    def test_pulse_intelligence_mcp_adapter_counts_as_external_agent_outcome(self) -> None:
        now = datetime.now(timezone.utc).replace(microsecond=0)
        rows = [
            {
                "install_id": "adapter-only",
                "version": "v6.0.0-rc.1",
                "platform": "docker",
                "received_at": (now - timedelta(days=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_mcp_adapter_used_30d": 1,
            },
            {
                "install_id": "adapter-only",
                "version": "v6.0.0-rc.1",
                "platform": "docker",
                "received_at": (now - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
            },
        ]

        summary = report.summarize_rows(
            {
                "latest_ping": rows[1]["received_at"],
                "total_rows": len(rows),
                "total_distinct_installs": 1,
            },
            rows,
            published_versions={"6.0.0-rc.1"},
        )

        cohorts = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_outcome_cohorts"]["cohorts"]
        }
        self.assertEqual(
            cohorts["external_agent_used_30d"]["label"],
            "Capability API/MCP adapter used 30d",
        )
        self.assertEqual(cohorts["external_agent_used_30d"]["installs"], 1)
        self.assertEqual(cohorts["external_agent_used_30d"]["paid_latest"], 1)
        self.assertEqual(cohorts["external_agent_used_30d"]["observed_signal_free_starts"], 1)
        self.assertEqual(cohorts["external_agent_used_30d"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(cohorts["mcp_adapter_used_30d"]["installs"], 1)

        stages = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_operations_loop_funnel"]["stages"]
        }
        self.assertEqual(stages["assistant_mcp_collaboration"]["installs"], 1)
        self.assertEqual(stages["assistant_mcp_collaboration"]["observed_signal_free_to_paid"], 1)

    def test_pulse_intelligence_operations_funnel_requires_all_loop_parts(self) -> None:
        now = datetime.now(timezone.utc).replace(microsecond=0)
        rows = [
            {
                "install_id": "full-loop-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=3)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_patrol_runs_30d": 1,
                "pulse_intelligence_patrol_new_findings_30d": 1,
                "pulse_intelligence_external_agent_used_30d": 1,
                "pulse_intelligence_governed_action_active_30d": 1,
                "pulse_intelligence_approved_action_attempts_30d": 1,
                "pulse_intelligence_approved_action_successes_30d": 1,
            },
            {
                "install_id": "full-loop-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=6)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
            },
            {
                "install_id": "paid-first-full-loop",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=7)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_patrol_runs_30d": 1,
                "pulse_intelligence_patrol_new_findings_30d": 1,
                "pulse_intelligence_external_agent_used_30d": 1,
                "pulse_intelligence_governed_action_active_30d": 1,
                "pulse_intelligence_approved_action_attempts_30d": 1,
                "pulse_intelligence_approved_action_successes_30d": 1,
            },
            {
                "install_id": "full-loop",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_patrol_runs_30d": 1,
                "pulse_intelligence_patrol_new_findings_30d": 1,
            },
            {
                "install_id": "full-loop",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=1)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_external_agent_used_30d": 1,
                "pulse_intelligence_assistant_ai_calls_30d": 2,
                "pulse_intelligence_governed_action_active_30d": 1,
                "pulse_intelligence_action_plans_30d": 1,
                "pulse_intelligence_approved_action_attempts_30d": 1,
                "pulse_intelligence_approved_action_successes_30d": 1,
            },
            {
                "install_id": "patrol-only",
                "version": "v6.0.0-rc.1",
                "platform": "docker",
                "received_at": (now - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_patrol_new_findings_30d": 1,
            },
            {
                "install_id": "collaboration-action",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=3)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_assistant_ai_calls_30d": 4,
                "pulse_intelligence_assistant_context_ai_calls_30d": 4,
                "pulse_intelligence_action_plans_30d": 1,
            },
            {
                "install_id": "patrol-collaboration",
                "version": "v6.0.0-rc.1",
                "platform": "docker",
                "received_at": (now - timedelta(hours=4)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_patrol_runs_30d": 2,
                "pulse_intelligence_external_agent_used_30d": 1,
            },
            {
                "install_id": "generic-chat-action",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=5)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_assistant_ai_calls_30d": 5,
                "pulse_intelligence_action_plans_30d": 1,
            },
        ]

        summary = report.summarize_rows(
            {
                "latest_ping": rows[1]["received_at"],
                "total_rows": len(rows),
                "total_distinct_installs": 7,
            },
            rows,
            published_versions={"6.0.0-rc.1"},
        )

        stages = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_operations_loop_funnel"]["stages"]
        }
        self.assertEqual(stages["configured"]["installs"], 7)
        self.assertEqual(stages["patrol_activity"]["installs"], 5)
        self.assertEqual(stages["assistant_mcp_collaboration"]["installs"], 5)
        self.assertEqual(stages["governed_action"]["installs"], 5)
        self.assertEqual(stages["approved_action_execution"]["installs"], 3)
        self.assertEqual(stages["approved_action_success"]["installs"], 3)
        self.assertEqual(stages["approved_action_success"]["observed_signal_free_starts"], 1)
        self.assertEqual(stages["approved_action_success"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(stages["complete_operations_loop"]["installs"], 3)
        self.assertEqual(stages["complete_operations_loop"]["paid_latest"], 3)
        self.assertEqual(stages["complete_operations_loop"]["retained_7d"], 3)
        self.assertEqual(stages["complete_operations_loop"]["observed_free_starts"], 2)
        self.assertEqual(stages["complete_operations_loop"]["observed_free_to_paid"], 2)
        self.assertEqual(stages["complete_operations_loop"]["observed_signal_free_starts"], 1)
        self.assertEqual(stages["complete_operations_loop"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(stages["approved_execution_loop"]["installs"], 3)
        self.assertEqual(stages["approved_execution_loop"]["paid_latest"], 3)
        self.assertEqual(stages["approved_execution_loop"]["observed_signal_free_starts"], 1)
        self.assertEqual(stages["approved_execution_loop"]["observed_signal_free_to_paid"], 1)

    def test_pulse_intelligence_operations_funnel_reports_mcp_adapter_loop_value(self) -> None:
        now = datetime.now(timezone.utc).replace(microsecond=0)
        rows = [
            {
                "install_id": "adapter-loop-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=3)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_patrol_runs_30d": 1,
                "pulse_intelligence_patrol_new_findings_30d": 1,
                "pulse_intelligence_mcp_adapter_used_30d": 1,
                "pulse_intelligence_governed_action_active_30d": 1,
                "pulse_intelligence_action_plans_30d": 1,
                "pulse_intelligence_approved_action_attempts_30d": 1,
                "pulse_intelligence_approved_action_successes_30d": 1,
            },
            {
                "install_id": "adapter-loop-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=1)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
            },
            {
                "install_id": "direct-agent-loop",
                "version": "v6.0.0-rc.1",
                "platform": "docker",
                "received_at": (now - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_patrol_runs_30d": 1,
                "pulse_intelligence_patrol_new_findings_30d": 1,
                "pulse_intelligence_external_agent_used_30d": 1,
                "pulse_intelligence_governed_action_active_30d": 1,
                "pulse_intelligence_approved_action_attempts_30d": 1,
                "pulse_intelligence_approved_action_successes_30d": 1,
            },
            {
                "install_id": "adapter-without-patrol",
                "version": "v6.0.0-rc.1",
                "platform": "docker",
                "received_at": (now - timedelta(hours=3)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_mcp_adapter_used_30d": 1,
                "pulse_intelligence_governed_action_active_30d": 1,
                "pulse_intelligence_action_plans_30d": 1,
            },
            {
                "install_id": "adapter-without-action",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=4)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_patrol_runs_30d": 1,
                "pulse_intelligence_mcp_adapter_used_30d": 1,
            },
        ]

        summary = report.summarize_rows(
            {
                "latest_ping": rows[1]["received_at"],
                "total_rows": len(rows),
                "total_distinct_installs": 4,
            },
            rows,
            published_versions={"6.0.0-rc.1"},
        )

        stages = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_operations_loop_funnel"]["stages"]
        }
        self.assertEqual(stages["complete_operations_loop"]["installs"], 2)
        self.assertEqual(stages["approved_execution_loop"]["installs"], 2)
        self.assertEqual(stages["approved_action_success"]["installs"], 2)

        adapter_loop = stages["mcp_adapter_operations_loop"]
        self.assertEqual(adapter_loop["label"], "Pulse MCP adapter operations loop")
        self.assertEqual(
            adapter_loop["required_signal_groups"],
            ["mcp_adapter_operations_loop"],
        )
        self.assertEqual(adapter_loop["installs"], 1)
        self.assertEqual(adapter_loop["retained_7d"], 1)
        self.assertEqual(adapter_loop["retained_7d_rate_pct"], 100)
        self.assertEqual(adapter_loop["paid_latest"], 1)
        self.assertEqual(adapter_loop["paid_latest_rate_pct"], 100)
        self.assertEqual(adapter_loop["observed_free_starts"], 1)
        self.assertEqual(adapter_loop["observed_free_to_paid"], 1)
        self.assertEqual(adapter_loop["observed_free_to_paid_rate_pct"], 100)
        self.assertEqual(adapter_loop["observed_signal_free_starts"], 1)
        self.assertEqual(adapter_loop["observed_signal_free_to_paid"], 1)
        self.assertEqual(adapter_loop["observed_signal_free_to_paid_rate_pct"], 100)

        adapter_approved = stages["mcp_adapter_approved_execution_loop"]
        self.assertEqual(
            adapter_approved["required_signal_groups"],
            ["mcp_adapter_approved_execution_loop"],
        )
        self.assertEqual(adapter_approved["installs"], 1)
        self.assertEqual(adapter_approved["retained_7d_rate_pct"], 100)
        self.assertEqual(adapter_approved["paid_latest_rate_pct"], 100)
        self.assertEqual(adapter_approved["observed_signal_free_to_paid_rate_pct"], 100)

        adapter_success = stages["mcp_adapter_approved_success_loop"]
        self.assertEqual(
            adapter_success["required_signal_groups"],
            ["mcp_adapter_approved_success_loop"],
        )
        self.assertEqual(adapter_success["installs"], 1)
        self.assertEqual(adapter_success["retained_7d_rate_pct"], 100)
        self.assertEqual(adapter_success["paid_latest_rate_pct"], 100)
        self.assertEqual(adapter_success["observed_signal_free_to_paid_rate_pct"], 100)

    def test_pulse_intelligence_reports_source_specific_loop_value(self) -> None:
        now = datetime.now(timezone.utc).replace(microsecond=0)
        rows = [
            {
                "install_id": "assistant-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=3)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_operations_loop_starter_requests_30d": 2,
                "pulse_intelligence_assistant_operations_loop_starter_requests_30d": 2,
                "pulse_intelligence_assistant_operations_loop_30d": 1,
                "pulse_intelligence_assistant_approved_execution_loop_30d": 1,
                "pulse_intelligence_assistant_approved_action_success_loop_30d": 1,
                "pulse_intelligence_assistant_resolved_operations_loop_30d": 1,
                "pulse_intelligence_rejected_action_decisions_30d": 1,
            },
            {
                "install_id": "assistant-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=1)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_operations_loop_starter_requests_30d": 2,
                "pulse_intelligence_assistant_operations_loop_starter_requests_30d": 2,
                "pulse_intelligence_assistant_operations_loop_30d": 1,
                "pulse_intelligence_assistant_approved_execution_loop_30d": 1,
                "pulse_intelligence_assistant_approved_action_success_loop_30d": 1,
                "pulse_intelligence_assistant_resolved_operations_loop_30d": 1,
                "pulse_intelligence_rejected_action_decisions_30d": 1,
            },
            {
                "install_id": "direct-agent-loop",
                "version": "v6.0.0-rc.1",
                "platform": "docker",
                "received_at": (now - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_external_agent_operations_loop_30d": 1,
                "pulse_intelligence_external_agent_approved_execution_loop_30d": 1,
                "pulse_intelligence_external_agent_approved_action_success_loop_30d": 1,
                "pulse_intelligence_external_agent_resolved_operations_loop_30d": 1,
            },
            {
                "install_id": "patrol-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=4)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_patrol_operations_loop_starter_requests_30d": 1,
            },
            {
                "install_id": "patrol-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(minutes=40)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_patrol_operations_loop_starter_requests_30d": 1,
            },
            {
                "install_id": "legacy-pro-entry-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=5)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_patrol_control_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_pro_activation_operations_loop_starter_requests_30d": 1,
            },
            {
                "install_id": "legacy-pro-entry-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(minutes=20)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_patrol_control_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_pro_activation_operations_loop_starter_requests_30d": 1,
            },
            {
                "install_id": "mcp-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_external_agent_operations_loop_30d": 1,
                "pulse_intelligence_external_agent_approved_execution_loop_30d": 1,
                "pulse_intelligence_external_agent_approved_action_success_loop_30d": 1,
                "pulse_intelligence_external_agent_resolved_operations_loop_30d": 1,
                "pulse_intelligence_mcp_adapter_operations_loop_30d": 1,
                "pulse_intelligence_mcp_adapter_approved_execution_loop_30d": 1,
                "pulse_intelligence_mcp_adapter_approved_action_success_loop_30d": 1,
                "pulse_intelligence_mcp_adapter_resolved_operations_loop_30d": 1,
                "pulse_intelligence_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_mcp_operations_loop_starter_requests_30d": 1,
            },
            {
                "install_id": "mcp-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(minutes=30)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_external_agent_operations_loop_30d": 1,
                "pulse_intelligence_external_agent_approved_execution_loop_30d": 1,
                "pulse_intelligence_external_agent_approved_action_success_loop_30d": 1,
                "pulse_intelligence_external_agent_resolved_operations_loop_30d": 1,
                "pulse_intelligence_mcp_adapter_operations_loop_30d": 1,
                "pulse_intelligence_mcp_adapter_approved_execution_loop_30d": 1,
                "pulse_intelligence_mcp_adapter_approved_action_success_loop_30d": 1,
                "pulse_intelligence_mcp_adapter_resolved_operations_loop_30d": 1,
                "pulse_intelligence_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_mcp_operations_loop_starter_requests_30d": 1,
            },
        ]

        summary = report.summarize_rows(
            {
                "latest_ping": (now - timedelta(minutes=20)).strftime("%Y-%m-%d %H:%M:%S"),
                "total_rows": len(rows),
                "total_distinct_installs": 5,
            },
            rows,
            published_versions={"6.0.0-rc.1"},
        )

        loop_flags = {
            entry["field"]: entry
            for entry in summary["pulse_intelligence_value_loop_7d"]["boolean_signals"]
        }
        self.assertEqual(loop_flags["pulse_intelligence_assistant_operations_loop_30d"]["installs"], 1)
        self.assertEqual(loop_flags["pulse_intelligence_external_agent_operations_loop_30d"]["installs"], 2)
        self.assertEqual(loop_flags["pulse_intelligence_mcp_adapter_operations_loop_30d"]["installs"], 1)
        self.assertEqual(
            loop_flags["pulse_intelligence_mcp_adapter_resolved_operations_loop_30d"]["paid_installs"],
            1,
        )

        loop_counts = {
            entry["field"]: entry
            for entry in summary["pulse_intelligence_value_loop_7d"]["count_signals"]
        }
        self.assertEqual(loop_counts["pulse_intelligence_operations_loop_starter_requests_30d"]["total"], 5)
        self.assertEqual(
            loop_counts["pulse_intelligence_assistant_operations_loop_starter_requests_30d"]["total"],
            2,
        )
        self.assertEqual(
            loop_counts["pulse_intelligence_patrol_operations_loop_starter_requests_30d"]["total"],
            1,
        )
        self.assertEqual(
            loop_counts["pulse_intelligence_patrol_control_operations_loop_starter_requests_30d"]["total"],
            1,
        )
        self.assertEqual(
            loop_counts["pulse_intelligence_pro_activation_operations_loop_starter_requests_30d"]["total"],
            1,
        )
        self.assertEqual(
            loop_counts["pulse_intelligence_mcp_operations_loop_starter_requests_30d"]["total"],
            1,
        )
        self.assertEqual(loop_counts["pulse_intelligence_rejected_action_decisions_30d"]["total"], 1)

        cohorts = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_outcome_cohorts"]["cohorts"]
        }
        self.assertEqual(cohorts["assistant_operations_loop_30d"]["installs"], 1)
        self.assertEqual(cohorts["assistant_operations_loop_30d"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(cohorts["assistant_resolved_operations_loop_30d"]["installs"], 1)
        self.assertEqual(cohorts["external_agent_operations_loop_30d"]["installs"], 2)
        self.assertEqual(cohorts["external_agent_operations_loop_30d"]["paid_latest"], 2)
        self.assertEqual(cohorts["external_agent_resolved_operations_loop_30d"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(cohorts["mcp_adapter_operations_loop_30d"]["installs"], 1)
        self.assertEqual(cohorts["mcp_adapter_resolved_operations_loop_30d"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(cohorts["operations_loop_starter_requests"]["installs"], 4)
        self.assertEqual(cohorts["patrol_operations_loop_starter_requests"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(
            cohorts["patrol_control_operations_loop_starter_requests"]["observed_signal_free_to_paid"],
            2,
        )
        self.assertEqual(
            cohorts["pro_activation_operations_loop_starter_requests"]["observed_signal_free_to_paid"],
            1,
        )
        self.assertEqual(cohorts["mcp_operations_loop_starter_requests"]["observed_signal_free_to_paid"], 1)

        stages = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_operations_loop_funnel"]["stages"]
        }
        self.assertEqual(stages["complete_operations_loop"]["installs"], 3)
        self.assertEqual(stages["approved_execution_loop"]["installs"], 3)
        self.assertEqual(stages["resolved_operations_loop"]["installs"], 3)
        self.assertEqual(stages["assistant_operations_loop"]["installs"], 1)
        self.assertEqual(stages["assistant_operations_loop"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(stages["assistant_approved_success_loop"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(stages["assistant_resolved_operations_loop"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(stages["external_agent_operations_loop"]["installs"], 2)
        self.assertEqual(stages["external_agent_resolved_operations_loop"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(stages["mcp_adapter_operations_loop"]["installs"], 1)
        self.assertEqual(stages["mcp_adapter_resolved_operations_loop"]["observed_signal_free_to_paid"], 1)

    def test_pulse_intelligence_reports_patrol_control_resolved_loop_as_first_class_signal(self) -> None:
        now = datetime.now(timezone.utc).replace(microsecond=0)
        rows = [
            {
                "install_id": "legacy-pro-completed-loop",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=4)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_pro_activation_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_pro_activation_completed_operations_loop_30d": 1,
            },
            {
                "install_id": "legacy-pro-completed-loop",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_pro_activation_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_pro_activation_completed_operations_loop_30d": 1,
                "pulse_intelligence_pro_activation_paid_completed_operations_loop_30d": 1,
            },
            {
                "install_id": "explicit-patrol-control-completed-loop",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_patrol_control_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_patrol_control_completed_operations_loop_30d": 1,
            },
            {
                "install_id": "explicit-patrol-control-completed-loop",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(minutes=90)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_patrol_control_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_patrol_control_completed_operations_loop_30d": 1,
                "pulse_intelligence_patrol_control_paid_completed_operations_loop_30d": 1,
            },
            {
                "install_id": "explicit-patrol-control-resolved-loop",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=3)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_patrol_control_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_patrol_control_resolved_operations_loop_30d": 1,
            },
            {
                "install_id": "explicit-patrol-control-resolved-loop",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=1)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_patrol_control_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_patrol_control_resolved_operations_loop_30d": 1,
                "pulse_intelligence_patrol_control_paid_resolved_operations_loop_30d": 1,
            },
            {
                "install_id": "legacy-pro-resolved-loop",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=3)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_pro_activation_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_pro_activation_resolved_operations_loop_30d": 1,
            },
            {
                "install_id": "legacy-pro-resolved-loop",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(minutes=45)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_pro_activation_operations_loop_starter_requests_30d": 1,
                "pulse_intelligence_pro_activation_resolved_operations_loop_30d": 1,
                "pulse_intelligence_pro_activation_paid_resolved_operations_loop_30d": 1,
            },
        ]

        summary = report.summarize_rows(
            {
                "latest_ping": rows[-1]["received_at"],
                "total_rows": len(rows),
                "total_distinct_installs": 4,
            },
            rows,
            published_versions={"6.0.0-rc.1"},
        )

        loop_flags = {
            entry["field"]: entry
            for entry in summary["pulse_intelligence_value_loop_7d"]["boolean_signals"]
        }
        patrol_control_completed = loop_flags[
            "pulse_intelligence_patrol_control_completed_operations_loop_30d"
        ]
        self.assertEqual(patrol_control_completed["installs"], 1)
        self.assertEqual(patrol_control_completed["paid_installs"], 1)
        patrol_control_resolved = loop_flags[
            "pulse_intelligence_patrol_control_resolved_operations_loop_30d"
        ]
        self.assertEqual(patrol_control_resolved["installs"], 1)
        self.assertEqual(patrol_control_resolved["paid_installs"], 1)
        legacy_completed = loop_flags["pulse_intelligence_pro_activation_completed_operations_loop_30d"]
        self.assertEqual(legacy_completed["installs"], 1)
        self.assertEqual(legacy_completed["paid_installs"], 1)
        legacy_resolved = loop_flags["pulse_intelligence_pro_activation_resolved_operations_loop_30d"]
        self.assertEqual(legacy_resolved["installs"], 1)
        self.assertEqual(legacy_resolved["paid_installs"], 1)

        cohorts = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_outcome_cohorts"]["cohorts"]
        }
        self.assertEqual(cohorts["patrol_control_operations_loop_starter_requests"]["installs"], 4)
        self.assertEqual(cohorts["pro_activation_operations_loop_starter_requests"]["installs"], 2)
        self.assertEqual(cohorts["patrol_control_completed_operations_loop_30d"]["installs"], 2)
        self.assertEqual(
            cohorts["patrol_control_completed_operations_loop_30d"]["observed_signal_free_to_paid"],
            2,
        )
        self.assertEqual(cohorts["patrol_control_resolved_operations_loop_30d"]["installs"], 2)
        self.assertEqual(
            cohorts["patrol_control_resolved_operations_loop_30d"]["observed_signal_free_to_paid"],
            2,
        )
        self.assertEqual(cohorts["patrol_control_paid_completed_operations_loop_30d"]["paid_latest"], 2)
        self.assertEqual(cohorts["patrol_control_paid_resolved_operations_loop_30d"]["paid_latest"], 2)

        stages = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_operations_loop_funnel"]["stages"]
        }
        self.assertNotIn("pro_activation_completed_operations_loop", stages)
        self.assertNotIn("pro_activation_resolved_operations_loop", stages)
        self.assertEqual(stages["complete_operations_loop"]["installs"], 2)
        self.assertEqual(stages["resolved_operations_loop"]["installs"], 2)
        self.assertEqual(stages["patrol_control_completed_operations_loop"]["installs"], 2)
        self.assertEqual(
            stages["patrol_control_completed_operations_loop"]["observed_signal_free_to_paid"],
            2,
        )
        self.assertEqual(stages["patrol_control_resolved_operations_loop"]["installs"], 2)
        self.assertEqual(
            stages["patrol_control_resolved_operations_loop"]["observed_signal_free_to_paid"],
            2,
        )

    def test_pulse_intelligence_reports_external_agent_capability_activity(self) -> None:
        now = datetime.now(timezone.utc).replace(microsecond=0)
        rows = [
            {
                "install_id": "capability-loop-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(days=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_patrol_runs_30d": 1,
                "pulse_intelligence_patrol_new_findings_30d": 1,
                "pulse_intelligence_external_agent_context_requests_30d": 2,
                "pulse_intelligence_external_agent_action_requests_30d": 1,
                "pulse_intelligence_governed_action_active_30d": 1,
                "pulse_intelligence_approved_action_attempts_30d": 1,
                "pulse_intelligence_approved_action_successes_30d": 1,
            },
            {
                "install_id": "capability-loop-before-paid",
                "version": "v6.0.0-rc.1",
                "platform": "binary",
                "received_at": (now - timedelta(hours=1)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 1,
                "pulse_intelligence_loop_configured": 1,
                "pulse_intelligence_patrol_runs_30d": 1,
                "pulse_intelligence_patrol_new_findings_30d": 1,
                "pulse_intelligence_external_agent_context_requests_30d": 3,
                "pulse_intelligence_external_agent_action_requests_30d": 2,
                "pulse_intelligence_governed_action_active_30d": 1,
                "pulse_intelligence_approved_action_attempts_30d": 1,
                "pulse_intelligence_approved_action_successes_30d": 1,
            },
            {
                "install_id": "assistant-only",
                "version": "v6.0.0-rc.1",
                "platform": "docker",
                "received_at": (now - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
                "event": "heartbeat",
                "paid_license": 0,
                "pulse_intelligence_assistant_ai_calls_30d": 4,
            },
        ]

        summary = report.summarize_rows(
            {
                "latest_ping": rows[1]["received_at"],
                "total_rows": len(rows),
                "total_distinct_installs": 2,
            },
            rows,
            published_versions={"6.0.0-rc.1"},
        )

        loop_counts = {
            entry["field"]: entry
            for entry in summary["pulse_intelligence_value_loop_7d"]["count_signals"]
        }
        self.assertEqual(
            loop_counts["pulse_intelligence_external_agent_context_requests_30d"]["installs"],
            1,
        )
        self.assertEqual(
            loop_counts["pulse_intelligence_external_agent_context_requests_30d"]["total"],
            3,
        )
        self.assertEqual(
            loop_counts["pulse_intelligence_external_agent_context_requests_30d"]["paid_total"],
            3,
        )
        self.assertEqual(loop_counts["pulse_intelligence_approved_action_successes_30d"]["installs"], 1)

        cohorts = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_outcome_cohorts"]["cohorts"]
        }
        self.assertEqual(cohorts["external_agent_used_30d"]["installs"], 1)
        self.assertEqual(cohorts["external_agent_used_30d"]["paid_latest"], 1)
        self.assertEqual(cohorts["external_agent_used_30d"]["observed_signal_free_starts"], 1)
        self.assertEqual(cohorts["external_agent_used_30d"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(cohorts["external_agent_context_requests"]["installs"], 1)
        self.assertEqual(cohorts["external_agent_context_requests"]["paid_latest"], 1)
        self.assertEqual(cohorts["external_agent_context_requests"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(cohorts["external_agent_action_requests"]["installs"], 1)
        self.assertEqual(cohorts["external_agent_action_requests"]["paid_latest"], 1)
        self.assertEqual(cohorts["approved_action_success_30d"]["installs"], 1)
        self.assertEqual(cohorts["approved_action_success_30d"]["paid_latest"], 1)

        stages = {
            entry["key"]: entry
            for entry in summary["pulse_intelligence_operations_loop_funnel"]["stages"]
        }
        self.assertEqual(stages["assistant_mcp_collaboration"]["installs"], 1)
        self.assertEqual(stages["assistant_mcp_collaboration"]["paid_latest"], 1)
        self.assertEqual(stages["complete_operations_loop"]["installs"], 1)
        self.assertEqual(stages["complete_operations_loop"]["paid_latest"], 1)
        self.assertEqual(stages["complete_operations_loop"]["observed_signal_free_starts"], 1)
        self.assertEqual(stages["complete_operations_loop"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(stages["approved_execution_loop"]["installs"], 1)
        self.assertEqual(stages["approved_execution_loop"]["paid_latest"], 1)
        self.assertEqual(stages["approved_execution_loop"]["observed_signal_free_starts"], 1)
        self.assertEqual(stages["approved_execution_loop"]["observed_signal_free_to_paid"], 1)
        self.assertEqual(stages["approved_action_success"]["installs"], 1)
        self.assertEqual(stages["approved_action_success"]["paid_latest"], 1)
        self.assertEqual(stages["approved_action_success"]["observed_signal_free_starts"], 1)
        self.assertEqual(stages["approved_action_success"]["observed_signal_free_to_paid"], 1)

    def test_precomputed_pulse_intelligence_analysis_matches_reference_helpers(self) -> None:
        rows = [
            {
                "install_id": "mixed-posture",
                "received_at": "2026-07-20 09:00:00",
                "paid_license": "yes",
                "pulse_intelligence_patrol_resolved_findings_30d": 1,
                "pulse_intelligence_approved_action_successes_30d": 1,
            },
            {
                "install_id": "mixed-posture",
                "received_at": "2026-07-18 09:00:00",
                "paid_license": 0,
                "pulse_intelligence_patrol_new_findings_30d": 1,
                "pulse_intelligence_assistant_context_ai_calls_30d": 2,
            },
            {
                "install_id": "mixed-posture",
                "received_at": "2026-07-19 09:00:00",
                "paid_license": "false",
                "pulse_intelligence_approved_action_decisions_30d": 1,
                "pulse_intelligence_external_agent_action_requests_30d": "3",
                "pulse_intelligence_mcp_adapter_used_30d": "true",
            },
            {
                "install_id": "mixed-posture",
                "received_at": "2026-07-17 09:00:00",
                "paid_license": None,
                "pulse_intelligence_patrol_runs_30d": -1,
                "pulse_intelligence_loop_configured": "not-a-bool",
            },
        ]

        analysis = report.analyze_pulse_intelligence_install(rows)
        expected_free_start, expected_conversion = report.pulse_intelligence_observed_conversion(rows)
        self.assertEqual(analysis.observed_free_start, expected_free_start)
        self.assertEqual(analysis.observed_free_to_paid, expected_conversion)

        for key, _, bool_fields, count_fields in report.PULSE_INTELLIGENCE_OUTCOME_COHORTS:
            with self.subTest(cohort=key):
                expected_match = any(
                    report.pulse_intelligence_row_matches_cohort(row, bool_fields, count_fields)
                    for row in rows
                )
                expected_free_signal, expected_signal_conversion = (
                    report.pulse_intelligence_signal_observed_conversion(
                        rows,
                        bool_fields,
                        count_fields,
                    )
                )
                self.assertEqual(key in analysis.cohort_keys, expected_match)
                self.assertEqual(key in analysis.free_cohort_keys, expected_free_signal)
                self.assertEqual(
                    key in analysis.free_cohort_keys and analysis.first_paid_at is not None,
                    expected_signal_conversion,
                )

        expected_groups: set[str] = set()
        for row in rows:
            expected_groups.update(report.pulse_intelligence_row_signal_groups(row))
        report.pulse_intelligence_derive_signal_groups(expected_groups)
        self.assertEqual(analysis.signal_groups, expected_groups)

        for key, _, required_groups in report.PULSE_INTELLIGENCE_OPERATIONS_FUNNEL_STAGES:
            with self.subTest(stage=key):
                expected_free_signal, expected_signal_conversion = (
                    report.pulse_intelligence_stage_signal_observed_conversion(
                        rows,
                        required_groups,
                    )
                )
                observed_free_signal = all(
                    group in analysis.free_signal_groups for group in required_groups
                )
                self.assertEqual(observed_free_signal, expected_free_signal)
                self.assertEqual(
                    observed_free_signal and analysis.first_paid_at is not None,
                    expected_signal_conversion,
                )

    def test_pulse_intelligence_production_scale_performance_guard(self) -> None:
        now = datetime(2026, 7, 23, 12, tzinfo=timezone.utc)
        rows: list[dict[str, object]] = []
        latest_by_install: dict[str, dict[str, object]] = {}
        # Match the high-cardinality shape of the production 14-day window,
        # where many installs contribute a small number of heartbeat rows.
        install_count = 12_000
        rows_per_install = 10
        for install_index in range(install_count):
            install_id = f"install-{install_index:04d}"
            for row_index in range(rows_per_install):
                row: dict[str, object] = {
                    "install_id": install_id,
                    "received_at": (
                        now - timedelta(minutes=rows_per_install - row_index)
                    ).strftime("%Y-%m-%d %H:%M:%S"),
                    "paid_license": row_index >= rows_per_install // 2,
                    "pulse_intelligence_loop_configured": 1,
                    "pulse_intelligence_patrol_new_findings_30d": row_index % 3 == 0,
                    "pulse_intelligence_assistant_context_ai_calls_30d": row_index % 5,
                    "pulse_intelligence_external_agent_context_requests_30d": row_index % 7,
                    "pulse_intelligence_approved_action_decisions_30d": row_index % 11 == 0,
                    "pulse_intelligence_approved_action_attempts_30d": row_index % 13 == 0,
                    "pulse_intelligence_approved_action_successes_30d": row_index % 17 == 0,
                }
                rows.append(row)
                latest_by_install[install_id] = row

        original_parse_received_at = report.parse_received_at
        started = time.perf_counter()
        with mock.patch.object(
            report,
            "parse_received_at",
            wraps=original_parse_received_at,
        ) as parse_received_at:
            analyses = report.analyze_pulse_intelligence_rows(rows)
            report.summarize_pulse_intelligence_outcome_cohorts(
                rows,
                latest_by_install,
                now=now,
                analysis_by_install=analyses,
            )
            report.summarize_pulse_intelligence_operations_funnel(
                rows,
                latest_by_install,
                now=now,
                analysis_by_install=analyses,
            )
        elapsed = time.perf_counter() - started

        self.assertEqual(len(analyses), install_count)
        self.assertEqual(parse_received_at.call_count, len(rows))
        self.assertLess(
            elapsed,
            8.0,
            f"120,000-row high-cardinality Pulse Intelligence aggregation took {elapsed:.3f}s",
        )

    def test_is_mock_fleet_row_matches_scaled_fixture_signature(self) -> None:
        self.assertTrue(report.is_mock_fleet_row({"kubernetes_pods": 120, "vmware_hosts": 7}))
        self.assertTrue(report.is_mock_fleet_row({"kubernetes_pods": 600, "vmware_hosts": 35}))
        self.assertFalse(report.is_mock_fleet_row({"kubernetes_pods": 120, "vmware_hosts": 4}))
        self.assertFalse(report.is_mock_fleet_row({"kubernetes_pods": 119, "vmware_hosts": 7}))
        self.assertFalse(report.is_mock_fleet_row({"kubernetes_pods": 0, "vmware_hosts": 0}))
        self.assertFalse(report.is_mock_fleet_row({}))

    def test_summarize_rows_excludes_mock_fleet_rows_by_default(self) -> None:
        now = datetime.now(timezone.utc).replace(microsecond=0)
        real_row = {
            "install_id": "install-real",
            "version": "v6.1.0-rc.2",
            "platform": "binary",
            "received_at": (now - timedelta(hours=2)).strftime("%Y-%m-%d %H:%M:%S"),
            "event": "heartbeat",
            "kubernetes_pods": 37,
            "vmware_hosts": 2,
        }
        mock_row = {
            "install_id": "install-mock",
            "version": "v6.1.0-rc.2",
            "platform": "docker",
            "received_at": (now - timedelta(hours=3)).strftime("%Y-%m-%d %H:%M:%S"),
            "event": "heartbeat",
            "kubernetes_pods": 240,
            "vmware_hosts": 14,
            "pve_nodes": 10,
        }
        db_stats = {"latest_ping": real_row["received_at"], "total_rows": 2, "total_distinct_installs": 2}

        summary = report.summarize_rows(db_stats, [real_row, mock_row], published_versions=set())
        self.assertEqual(summary["active_latest"]["active_24h"], 1)
        self.assertEqual(
            summary["mock_fleet_exclusions"],
            {"enabled": True, "rows": 1, "installs": 1},
        )
        text = report.format_text(summary, "rcourtman/Pulse", 7)
        self.assertIn("mock fixture fleet excluded from window: 1 row(s) across 1 install(s)", text)

        included = report.summarize_rows(
            db_stats, [real_row, mock_row], published_versions=set(), include_mock_fleet=True
        )
        self.assertEqual(included["active_latest"]["active_24h"], 2)
        self.assertEqual(
            included["mock_fleet_exclusions"],
            {"enabled": False, "rows": 0, "installs": 0},
        )

    def test_summarize_user_base_signals_uses_latest_active_install_rows(self) -> None:
        now = datetime(2026, 7, 23, 12, tzinfo=timezone.utc)
        summary = report.summarize_user_base_signals(
            {
                "active-v2": {
                    "received_at": "2026-07-23 10:00:00",
                    "schema_version": 2,
                    "deployment_method": "docker_compose",
                    "known_install_age_bucket": "1_7d",
                    "activation_stage": "outcome_observed",
                    "time_to_first_monitored_resource_bucket": "under_15m",
                    "estate_size_bucket": "11_50",
                    "auth_configured": 1,
                    "monitoring_active": 1,
                    "outcome_observed_30d": 1,
                    "configured_connections": 3,
                    "alerts_fired_30d": 4,
                    "notification_deliveries_7d": 2,
                },
                "active-legacy": {
                    "received_at": "2026-07-22 10:00:00",
                },
                "stale": {
                    "received_at": "2026-07-01 10:00:00",
                    "schema_version": 2,
                    "configured_connections": 99,
                },
            },
            now=now,
        )

        self.assertEqual(summary["active_installs"], 2)
        self.assertEqual(
            summary["schema_versions"],
            [{"version": "2", "installs": 1}, {"version": "legacy", "installs": 1}],
        )
        deployment = next(
            item for item in summary["category_signals"] if item["field"] == "deployment_method"
        )
        self.assertEqual(
            deployment["label"],
            "Deployment method signal (best effort; upgraded installs often report container_other or binary_other)",
        )
        self.assertEqual(
            deployment["buckets"],
            [{"bucket": "docker_compose", "installs": 1}, {"bucket": "legacy_unknown", "installs": 1}],
        )
        known_age = next(
            item
            for item in summary["category_signals"]
            if item["field"] == "known_install_age_bucket"
        )
        self.assertEqual(
            known_age["label"],
            "Time since first schema-v2 lifecycle observation",
        )
        configured = next(
            item for item in summary["count_signals"] if item["field"] == "configured_connections"
        )
        self.assertEqual(configured, {
            "field": "configured_connections",
            "label": "Configured connections",
            "installs": 1,
            "total": 3,
        })

    def test_summarize_user_base_signals_keeps_notification_failure_semantics_separate(self) -> None:
        now = datetime(2026, 7, 23, 12, tzinfo=timezone.utc)
        summary = report.summarize_user_base_signals(
            {
                "legacy-attempt-failures": {
                    "received_at": "2026-07-23 10:00:00",
                    "schema_version": 2,
                    "notification_attempts_7d": 5,
                    "notification_deliveries_7d": 1,
                    "notification_failures_7d": 4,
                },
                "terminal-failures": {
                    "received_at": "2026-07-23 11:00:00",
                    "schema_version": 3,
                    "notification_attempts_7d": 4,
                    "notification_deliveries_7d": 2,
                    "notification_failures_7d": 1,
                },
            },
            now=now,
        )

        signals = {item["field"]: item for item in summary["count_signals"]}
        self.assertEqual(
            signals["notification_attempt_failures_7d_schema_v2"],
            {
                "field": "notification_attempt_failures_7d_schema_v2",
                "label": "Notification failed attempts (7d, legacy schema v2)",
                "installs": 1,
                "total": 4,
            },
        )
        self.assertEqual(
            signals["notification_terminal_failures_7d_schema_v3"],
            {
                "field": "notification_terminal_failures_7d_schema_v3",
                "label": "Notification terminal failures (7d, schema v3+)",
                "installs": 1,
                "total": 1,
            },
        )

    def test_format_text_includes_user_base_privacy_bounded_signals(self) -> None:
        rendered = report.format_text(
            {
                "db_stats": {},
                "latest_install_windows": {
                    label: {
                        "active_installs": 0,
                        "published_versions": [],
                        "non_release_versions": [],
                        "platforms": [],
                        "adoption_counts": [],
                        "feature_enabled_installs": [],
                    }
                    for label, _ in report.DEFAULT_LATEST_INSTALL_WINDOWS
                },
                "user_base_signals_7d": {
                    "active_installs": 2,
                    "schema_versions": [{"version": "2", "installs": 2}],
                    "category_signals": [{
                        "field": "activation_stage",
                        "label": "Highest observed activation stage",
                        "buckets": [{"bucket": "monitoring", "installs": 2}],
                    }],
                    "boolean_signals": [{
                        "field": "monitoring_active",
                        "label": "Monitoring currently active",
                        "installs": 2,
                    }],
                    "count_signals": [{
                        "field": "alerts_resolved_30d",
                        "label": "Alerts resolved (30d)",
                        "installs": 1,
                        "total": 4,
                    }],
                },
            },
            "rcourtman/Pulse",
            7,
        )
        self.assertIn("User-base lifecycle and outcomes (7d):", rendered)
        self.assertIn("Highest observed activation stage: monitoring 2", rendered)
        self.assertIn("Alerts resolved (30d): 4 across 1 installs", rendered)
        self.assertIn(
            "for upgraded installs it is only a lower bound, not original installation age",
            rendered,
        )
        self.assertIn(
            "upgraded installs often fall back to container_other or binary_other",
            rendered,
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
            "pulse_intelligence_value_loop_7d": {
                "active_installs": 157,
                "paid_installs": 42,
                "free_installs": 115,
                "boolean_signals": [
                    {
                        "field": "pulse_intelligence_loop_configured",
                        "label": "Loop configured",
                        "installs": 31,
                        "paid_installs": 18,
                        "free_installs": 13,
                    },
                    {
                        "field": "pulse_intelligence_governed_action_active_30d",
                        "label": "Governed action active 30d",
                        "installs": 6,
                        "paid_installs": 5,
                        "free_installs": 1,
                    },
                ],
                "count_signals": [
                    {
                        "field": "pulse_intelligence_assistant_ai_calls_30d",
                        "label": "Assistant AI calls 30d",
                        "installs": 21,
                        "paid_installs": 12,
                        "free_installs": 9,
                        "total": 88,
                        "paid_total": 61,
                        "free_total": 27,
                    },
                    {
                        "field": "pulse_intelligence_action_plans_30d",
                        "label": "Action plans 30d",
                        "installs": 4,
                        "paid_installs": 4,
                        "free_installs": 0,
                        "total": 9,
                        "paid_total": 9,
                        "free_total": 0,
                    },
                ],
            },
            "pulse_intelligence_outcome_cohorts": {
                "retention_window": "7d",
                "cohorts": [
                    {
                        "key": "loop_configured",
                        "label": "Loop configured",
                        "installs": 51,
                        "retained_7d": 31,
                        "paid_latest": 24,
                        "free_latest": 27,
                        "observed_free_starts": 19,
                        "observed_free_to_paid": 6,
                        "observed_signal_free_starts": 12,
                        "observed_signal_free_to_paid": 5,
                    },
                    {
                        "key": "assistant_activity",
                        "label": "Assistant activity",
                        "installs": 21,
                        "retained_7d": 18,
                        "paid_latest": 12,
                        "free_latest": 9,
                        "observed_free_starts": 8,
                        "observed_free_to_paid": 3,
                        "observed_signal_free_starts": 6,
                        "observed_signal_free_to_paid": 3,
                    },
                ],
            },
            "pulse_intelligence_operations_loop_funnel": {
                "retention_window": "7d",
                "stages": [
                    {
                        "key": "patrol_activity",
                        "label": "Patrol detection/investigation",
                        "required_signal_groups": ["patrol"],
                        "installs": 17,
                        "retained_7d": 14,
                        "paid_latest": 11,
                        "free_latest": 6,
                        "observed_free_starts": 7,
                        "observed_free_to_paid": 2,
                        "observed_signal_free_starts": 5,
                        "observed_signal_free_to_paid": 2,
                    },
                    {
                        "key": "complete_operations_loop",
                        "label": "Complete operations loop",
                        "required_signal_groups": ["patrol_issue", "collaboration", "governed_decision"],
                        "installs": 9,
                        "retained_7d": 8,
                        "paid_latest": 7,
                        "free_latest": 2,
                        "observed_free_starts": 4,
                        "observed_free_to_paid": 2,
                        "observed_signal_free_starts": 3,
                        "observed_signal_free_to_paid": 1,
                    },
                ],
            },
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
        self.assertIn("Target release latest-state signal coverage (7d, 6.0.0-rc.6):", rendered)
        self.assertIn("latest rolling totals show signal availability, not activity caused by this release", rendered)
        self.assertIn("  - PVE nodes: 55 installs, total 131", rendered)
        self.assertIn("  - AI enabled: 19 installs", rendered)
        self.assertIn("  - Agent hosts, Patrol enabled", rendered)
        self.assertIn("Pulse Intelligence value loop (7d):", rendered)
        self.assertIn("- paid posture: paid 42, free/community 115", rendered)
        self.assertIn("  - Loop configured: 31 installs (paid 18, free/community 13)", rendered)
        self.assertIn("  - Action plans 30d: 4 installs, total 9 (paid 4 / 9; free/community 0 / 0)", rendered)
        self.assertIn("Pulse Intelligence activation and retention:", rendered)
        self.assertIn("- source window: last 7 day(s)", rendered)
        self.assertIn("- retention definition: latest ping within 7d", rendered)
        self.assertIn(
            "  - Loop configured: 51 installs, retained 7d 31 (60.8%), latest paid 24, latest free/community 27",
            rendered,
        )
        self.assertIn(
            "observed free/community starts 19, free-to-paid 6 (31.6%), signal while free/community 12, signal-to-paid 5 (41.7%)",
            rendered,
        )
        self.assertIn(
            "  - Assistant activity: 21 installs, retained 7d 18 (85.7%), latest paid 12, latest free/community 9",
            rendered,
        )
        self.assertIn(
            "observed free/community starts 8, free-to-paid 3 (37.5%), signal while free/community 6, signal-to-paid 3 (50.0%)",
            rendered,
        )
        self.assertIn("Pulse Intelligence operations loop funnel:", rendered)
        self.assertIn(
            "  - Patrol detection/investigation: 17 installs, retained 7d 14 (82.4%), latest paid 11, latest free/community 6",
            rendered,
        )
        self.assertIn(
            "  - Complete operations loop: 9 installs, retained 7d 8 (88.9%), latest paid 7, latest free/community 2",
            rendered,
        )
        self.assertIn(
            "observed free/community starts 4, free-to-paid 2 (50.0%), signal while free/community 3, signal-to-paid 1 (33.3%)",
            rendered,
        )

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

    def test_privacy_docs_disclose_derived_pulse_intelligence_reports(self) -> None:
        repo_root = Path(__file__).resolve().parents[2]
        privacy_docs = (
            repo_root / "docs" / "PRIVACY.md",
            repo_root / "frontend-modern" / "public" / "docs" / "PRIVACY.md",
        )

        for path in privacy_docs:
            with self.subTest(path=path.relative_to(repo_root)):
                content = path.read_text(encoding="utf-8")
                self.assertIn("aggregate Pulse Intelligence adoption reports", content)
                self.assertIn("Assistant, direct external-agent, or MCP collaboration", content)
                self.assertIn("Pulse Intelligence Assistant operations loop 30d", content)
                self.assertIn("Pulse Intelligence external agent operations loop 30d", content)
                self.assertIn("Pulse Intelligence Patrol control completed operations loop 30d", content)
                self.assertIn("Pulse Intelligence MCP operations loop starter requests 30d", content)
                self.assertIn("Pulse Intelligence Assistant context AI calls 30d", content)
                self.assertIn("approved action success", content)
                self.assertIn("rejected action decisions", content)
                self.assertIn("completed Patrol control work", content)
                self.assertIn("observed free-to-paid movement", content)
                self.assertIn(
                    "Pulse Intelligence agent/MCP route in the current 30-day telemetry window",
                    content,
                )
                self.assertIn("Pulse Intelligence MCP adapter used 30d", content)
                self.assertIn("Compatibility mirror of the Patrol control completed field", content)
                self.assertNotIn("Pro activation completed-loop proof", content)
                self.assertIn("route parameters, resource IDs", content)
                self.assertIn("Those reports do not add prompts, findings", content)
                self.assertIn("account links, or exact commercial tiers", content)
                self.assertIn(
                    "not precise evidence of the original installation path",
                    content,
                )
                self.assertIn(
                    "time since Pulse first created the schema-v2 lifecycle record",
                    content,
                )


if __name__ == "__main__":
    unittest.main()
