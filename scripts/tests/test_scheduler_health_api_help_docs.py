#!/usr/bin/env python3
"""Verify shipped Scheduler Health API Help matches live runtime structs and routes.

Binds the in-app API.md Scheduler Health section and docs/api/SCHEDULER_HEALTH.md
to the live Go response structs, route registration, and shipped Help headings.
"""

from __future__ import annotations

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
API_DOC = ROOT / "docs/API.md"
SHIPPED_API_DOC = ROOT / "frontend-modern/public/docs/API.md"
SCHEDULER_DOC = ROOT / "docs/api/SCHEDULER_HEALTH.md"


def scheduler_section() -> str:
    text = API_DOC.read_text(encoding="utf-8")
    marker = "### Scheduler Health\n"
    if marker not in text:
        raise AssertionError("missing Scheduler Health section in docs/API.md")
    after = text.split(marker, 1)[1]
    return after.split("\n### ", 1)[0]


def heading_slug(title: str) -> str:
    slug = title.strip().lower()
    slug = re.sub(r"[^\w\s-]", "", slug)
    return re.sub(r"\s+", "-", slug).strip("-")


class SchedulerHealthAPIHelpDocsTest(unittest.TestCase):
    def test_shipped_api_copy_matches_source(self) -> None:
        self.assertEqual(API_DOC.read_bytes(), SHIPPED_API_DOC.read_bytes())

    def test_route_methods_and_auth_match_server_registration(self) -> None:
        section = " ".join(scheduler_section().split())
        routes = (ROOT / "internal/api/router_routes_monitoring.go").read_text(
            encoding="utf-8"
        )
        router = (ROOT / "internal/api/router.go").read_text(encoding="utf-8")

        self.assertIn(
            'r.mux.HandleFunc("/api/monitoring/scheduler/health", RequireAuth(r.config, r.handleSchedulerHealth))',
            routes,
        )
        self.assertIn(
            "req.Method != http.MethodGet && req.Method != http.MethodHead",
            router.split("func (r *Router) handleSchedulerHealth", 1)[1][:400],
        )
        self.assertIn("`GET /api/monitoring/scheduler/health` (also accepts `HEAD`)", section)
        self.assertIn("No specific scope is enforced.", section)

    def test_documented_fields_match_go_response_structs(self) -> None:
        section = " ".join(scheduler_section().split())
        monitor = (ROOT / "internal/monitoring/monitor.go").read_text(encoding="utf-8")
        task_queue = (ROOT / "internal/monitoring/task_queue.go").read_text(
            encoding="utf-8"
        )

        for field, source in (
            ("updatedAt", monitor),
            ("enabled", monitor),
            ("queue", monitor),
            ("depth", task_queue),
            ("dueWithinSeconds", task_queue),
            ("perType", task_queue),
            ("oldestOverdueSeconds", task_queue),
            ("deadLetter", monitor),
            ("count", monitor),
            ("tasks", monitor),
            ("breakers", monitor),
            ("instances", monitor),
            ("key", monitor),
            ("pollStatus", monitor),
            ("lastSuccess", monitor),
            ("lastError", monitor),
            ("at", monitor),
            ("category", monitor),
            ("message", monitor),
            ("consecutiveFailures", monitor),
            ("breaker", monitor),
            ("state", monitor),
            ("retryAt", monitor),
            ("present", monitor),
            ("reason", monitor),
            ("nextRetry", monitor),
        ):
            with self.subTest(field=field):
                self.assertIn(field, section)
                self.assertRegex(source, rf'json:"{re.escape(field)}(?:,omitempty)?"')

    def test_runtime_boundaries_and_retirement_notes_present(self) -> None:
        section = " ".join(scheduler_section().split())
        sched_doc = " ".join(SCHEDULER_DOC.read_text(encoding="utf-8").split())
        availability_poller = (
            ROOT / "internal/monitoring/availability_poller.go"
        ).read_text(encoding="utf-8")

        self.assertIn("RefreshAvailabilityTargets", availability_poller)

        for boundary in (
            "`dueWithinSeconds` is a **task count**",
            "not a duration",
            "excludes future retries, claimed work already held by a worker and dead-letter entries",
            "Deleting a saved availability check removes its queued poll",
            "catalogue read failure leaves queued work unchanged",
            "`category: \"partial\"` and `consecutiveFailures: 0`",
            "Having no error rows does not prove polling ran.",
            "Passing `retryAt` does not prove a poll ran",
            "`nextRetry` does not schedule a poll.",
            "This endpoint covers scheduled polling only.",
            "that host or Docker agent reports arrived",
            "that alert rules ran.",
            "never in a command line or URL",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, section)

        self.assertIn(
            "Deleting a saved availability check retires its queued poll",
            sched_doc,
        )
        self.assertIn(
            "failed catalogue read leaves existing scheduled work unchanged",
            sched_doc,
        )

    def test_in_app_links_target_real_shipped_docs_and_headings(self) -> None:
        section = scheduler_section()
        links = re.findall(r"\[[^\]]+\]\(([^)]+)\)", section)
        self.assertGreaterEqual(len(links), 4)
        for target in links:
            self.assertFalse(target.startswith("http"), target)
            if target.startswith("#"):
                doc_path = SHIPPED_API_DOC
                fragment = target[1:]
            else:
                file_part, _, fragment = target.partition("#")
                doc_path = ROOT / "frontend-modern/public/docs" / file_part
            self.assertTrue(doc_path.is_file(), f"missing shipped doc for link {target}")
            if fragment and not fragment.startswith("-"):
                headings = re.findall(
                    r"^#{1,6}\s+(.+)$", doc_path.read_text(encoding="utf-8"), re.M
                )
                slugs = {heading_slug(h) for h in headings}
                self.assertIn(fragment, slugs, f"missing heading #{fragment} in {doc_path.name}")


if __name__ == "__main__":
    unittest.main()
