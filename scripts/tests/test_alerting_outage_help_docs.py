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
        work_progress = (ROOT / "internal/monitoring/poll_work_progress.go").read_text(encoding="utf-8")
        http_metrics = (ROOT / "internal/api/http_metrics.go").read_text(encoding="utf-8")
        routes = (ROOT / "internal/api/router_routes_registration.go").read_text(encoding="utf-8")

        for metric, source in (
            ("pulse_monitor_poll_total", monitoring_metrics),
            ("pulse_monitor_poll_last_success_timestamp", monitoring_metrics),
            ("pulse_scheduler_breaker_retry_seconds", monitoring_metrics),
            ("pulse_scheduler_queued_poll_overdue_seconds", queue_metrics),
            ("pulse_scheduler_claimed_poll_overdue_seconds", work_progress),
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


if __name__ == "__main__":
    unittest.main()
