#!/usr/bin/env python3
"""Verify the shipped Alerting outage-detection Help matches live runtime signals.

Binds in-app ALERTING.md to the actual Prometheus metrics, agent report routes,
History source labels, and shipped Help headings without touching a live server.
"""

from __future__ import annotations

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/ALERTING.md"
SHIPPED = ROOT / "frontend-modern/public/docs/ALERTING.md"


def outage_section() -> str:
    text = DOC.read_text(encoding="utf-8")
    marker = "## Detect an outage of Pulse itself\n"
    if marker not in text:
        raise AssertionError("missing outage-detection section in docs/ALERTING.md")
    return text.split(marker, 1)[1]


def heading_slug(title: str) -> str:
    slug = title.strip().lower()
    slug = re.sub(r"[^\w\s-]", "", slug)
    return re.sub(r"\s+", "-", slug).strip("-")


class AlertingOutageHelpDocsTest(unittest.TestCase):
    def test_shipped_copy_matches_source(self):
        self.assertEqual(DOC.read_bytes(), SHIPPED.read_bytes())

    def test_distinguishes_device_history_machines_and_docker_reports(self):
        section = " ".join(outage_section().split())
        history_model = (
            ROOT / "frontend-modern/src/components/shared/historyChartModel.ts"
        ).read_text(encoding="utf-8")
        machine_model = (
            ROOT / "frontend-modern/src/features/standalone/agentMachineTableModel.ts"
        ).read_text(encoding="utf-8")

        for label in ("Latest from device", "Latest recorded"):
            self.assertIn(label, history_model)
            self.assertIn(f"**{label}**", section)
        self.assertIn("label: 'Last seen'", machine_model)
        self.assertIn("**Last seen**", section)
        self.assertIn("a new chart point alone does not prove Pulse polled again", section)
        self.assertIn("Check **Docker** hosts separately", section)
        self.assertIn("machine reports keep arriving from the same agent", section)

    def test_prometheus_scheduler_and_agent_signals_match_runtime_source(self):
        section = " ".join(outage_section().split())
        monitoring_metrics = (ROOT / "internal/monitoring/metrics.go").read_text(encoding="utf-8")
        queue_metrics = (ROOT / "internal/monitoring/poll_queue_metrics.go").read_text(encoding="utf-8")
        dead_letter_metrics = (ROOT / "internal/monitoring/poll_dead_letter_metrics.go").read_text(encoding="utf-8")
        work_progress = (ROOT / "internal/monitoring/poll_work_progress.go").read_text(encoding="utf-8")
        http_metrics = (ROOT / "internal/api/http_metrics.go").read_text(encoding="utf-8")
        routes = (ROOT / "internal/api/router_routes_registration.go").read_text(encoding="utf-8")

        for metric, source in (
            ("pulse_monitor_poll_total", monitoring_metrics),
            ("pulse_monitor_poll_last_success_timestamp", monitoring_metrics),
            ("pulse_scheduler_breaker_retry_seconds", monitoring_metrics),
            ("pulse_scheduler_queued_poll_overdue_seconds", queue_metrics),
            ("pulse_scheduler_claimed_poll_overdue_seconds", work_progress),
            ("pulse_scheduler_dead_letter_depth", dead_letter_metrics),
            ("pulse_http_requests_total", http_metrics),
        ):
            with self.subTest(metric=metric):
                self.assertIn(metric, section)
                needle = (
                    metric.removeprefix("pulse_monitor_")
                    if metric.startswith("pulse_monitor_")
                    else metric.removeprefix("pulse_http_")
                    if metric.startswith("pulse_http_")
                    else metric
                )
                self.assertIn(f'"{needle}"', source)

        for stage in ("waiting_for_slot", "running"):
            self.assertIn(f'"{stage}"', work_progress)
            self.assertIn(f'`stage="{stage}"`', section)

        for route in (
            "/api/agents/agent/report",
            "/api/agents/host/report",
            "/api/agents/docker/report",
        ):
            self.assertIn(f'"{route}"', routes)
            self.assertIn(f"`{route}`", section)

        for boundary in (
            "an absent series on an older build is not zero",
            "not that a poll ran",
            "Zero does not prove polling recovered",
            "Do not add the routes together",
            "without restarting Pulse or sending test traffic",
            "Neither alert above checks whether Pulse evaluated its alert rules",
        ):
            self.assertIn(boundary, section)

    def test_in_app_links_target_real_shipped_docs_and_headings(self):
        section = outage_section()
        links = re.findall(r"\[[^\]]+\]\(([^)]+)\)", section)
        self.assertGreaterEqual(len(links), 5)
        for target in links:
            self.assertFalse(target.startswith("http"), target)
            if target.startswith("#"):
                doc_path = SHIPPED
                fragment = target[1:]
            else:
                file_part, _, fragment = target.partition("#")
                doc_path = ROOT / "frontend-modern/public/docs" / file_part
            self.assertTrue(doc_path.is_file(), f"missing shipped doc for link {target}")
            if fragment:
                headings = re.findall(r"^#{1,6}\s+(.+)$", doc_path.read_text(encoding="utf-8"), re.M)
                slugs = {heading_slug(h) for h in headings}
                self.assertIn(fragment, slugs, f"missing anchor #{fragment} in {doc_path.name}")

    def test_recovery_severity_continuity_help_matches_dispatch_receipts(self):
        text = DOC.read_text(encoding="utf-8")
        section = " ".join(
            text.split("## Confirm recovery separately\n", 1)[1]
            .split("\n## ", 1)[0]
            .split()
        )
        dispatch = (
            ROOT / "internal/notifications/notification_dispatch.go"
        ).read_text(encoding="utf-8")
        self.assertIn("filterResolvedJobsByDeliveryReceipt(", dispatch)
        self.assertIn("notifiedLevel", dispatch)
        self.assertIn("cloned.Level = notifiedLevel", dispatch)
        for phrase in (
            "keeps the severity level from the firing notification that destination actually received",
            "stepped down from critical to warning before clearing",
            "match on the alert `ID` rather than relying on severity alone",
        ):
            self.assertIn(phrase, section)

    def test_active_alert_reevaluation_help_matches_runtime_contract(self):
        alerting_section = " ".join(outage_section().split())
        api_doc = (ROOT / "docs/API.md").read_text(encoding="utf-8")
        shipped_api_doc = (ROOT / "frontend-modern/public/docs/API.md").read_text(encoding="utf-8")
        self.assertEqual(api_doc, shipped_api_doc)

        marker = "### Active alerts\n"
        self.assertIn(marker, api_doc)
        active_alerts_section = " ".join(
            api_doc.split(marker, 1)[1].split("\n---", 1)[0].split()
        )

        alerts_handler = (ROOT / "internal/api/alerting/alerts.go").read_text(encoding="utf-8")
        alert_model = (ROOT / "internal/alerts/model.go").read_text(encoding="utf-8")
        models_go = (ROOT / "internal/models/models.go").read_text(encoding="utf-8")
        persistence = (ROOT / "internal/alerts/active_persistence.go").read_text(encoding="utf-8")

        self.assertIn('case path == "active" && r.Method == http.MethodGet:', alerts_handler)
        self.assertIn("config.ScopeMonitoringRead", alerts_handler)
        self.assertIn("h.GetActiveAlerts(w, r)", alerts_handler)
        self.assertIn("clone.MetricStatus = nil", persistence)

        for field, source in (
            ("id", alert_model),
            ("startTime", alert_model),
            ("lastSeen", alert_model),
            ("metricStatus", alert_model),
            ("phase", models_go),
            ("value", models_go),
            ("observedAt", models_go),
            ("trigger", models_go),
            ("recovery", models_go),
            ("lastBreachAt", models_go),
        ):
            with self.subTest(field=field):
                self.assertRegex(
                    source, rf'json:"{re.escape(field)}(?:,(?:omitempty|omitzero))?"'
                )
                self.assertIn(field, active_alerts_section)

        for phase in ("breaching", "latched", "recovering"):
            with self.subTest(phase=phase):
                self.assertIn(f'"{phase}"', models_go)
                self.assertIn(f"`{phase}`", active_alerts_section)
                self.assertIn(f"`{phase}`", alerting_section)

        for phrase in (
            "GET /api/alerts/active",
            "monitoring:read",
            "both `id` and `startTime`",
            "metricStatus.observedAt",
            "metricStatus.lastBreachAt",
            "omitted from durable checkpoints",
            "do not lower thresholds or force a test alert",
        ):
            with self.subTest(alerting_phrase=phrase):
                self.assertIn(phrase, alerting_section)

        for phrase in (
            "GET /api/alerts/active",
            "monitoring:read",
            "stripped from durable alert snapshots",
            "never in a command line or URL",
            "ALERTING.md#check-whether-an-open-threshold-alert-was-reevaluated",
        ):
            with self.subTest(api_phrase=phrase):
                self.assertIn(phrase, active_alerts_section)


if __name__ == "__main__":
    unittest.main()
