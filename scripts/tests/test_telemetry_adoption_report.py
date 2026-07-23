#!/usr/bin/env python3
"""Unit tests for telemetry_adoption_report."""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from pathlib import Path
import gzip
import json
import subprocess
import sys
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
        stdout = "\n".join([json.dumps({"db_stats": db_stats}), *(json.dumps(row) for row in rows), ""])
        completed = subprocess.CompletedProcess(
            args=[],
            returncode=0,
            stdout=gzip.compress(stdout.encode("utf-8")),
            stderr=b"",
        )
        with mock.patch.object(report.subprocess, "run", return_value=completed) as run_mock:
            result = report.fetch_rows_remote("pulse-license", "/opt/licenses.sqlite", 30)
        self.assertEqual(result, {"db_stats": db_stats, "rows": rows})
        remote_script = run_mock.call_args.kwargs["input"].decode("utf-8")
        self.assertNotIn("fetchall", remote_script)
        self.assertIn("received_at >= datetime('now', ?)", remote_script)
        compile(remote_script, "<telemetry-remote-fetch>", "exec")

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
            deployment["buckets"],
            [{"bucket": "docker_compose", "installs": 1}, {"bucket": "legacy_unknown", "installs": 1}],
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
        self.assertIn("older upgraded installs are therefore lower bounds", rendered)

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
        self.assertIn("Target release signal coverage (7d, 6.0.0-rc.6):", rendered)
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


if __name__ == "__main__":
    unittest.main()
