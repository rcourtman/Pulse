#!/usr/bin/env python3
"""Keep External watchdog Help aligned with AlertDeadManDestinationSection.tsx.

Static source/document checks only: no watchdog URL, network request, or live
alert operation is performed.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/AVAILABILITY.md"
MIRROR = ROOT / "frontend-modern/public/docs/AVAILABILITY.md"
COMPONENT = ROOT / "frontend-modern/src/features/alerts/AlertDeadManDestinationSection.tsx"


def plain(text: str) -> str:
    return " ".join(text.replace("**", "").replace("`", "").split())


class WatchdogHelpDocsTest(unittest.TestCase):
    def setUp(self) -> None:
        self.doc = DOC.read_text(encoding="utf-8")
        self.guide = plain(self.doc)
        self.component = COMPONENT.read_text(encoding="utf-8")

    def test_shipped_guide_matches(self) -> None:
        self.assertEqual(DOC.read_bytes(), MIRROR.read_bytes())

    def test_watchdog_status_badges_match_runtime_presentation(self) -> None:
        for label in (
            "Heartbeat healthy",
            "No recent heartbeat",
            "Status read overdue",
            "Status unavailable",
            "Delivery failing",
            "Monitoring stalled",
            "Starting",
            "Not configured",
            "Configuration invalid",
            "Configuration unavailable",
        ):
            with self.subTest(label=label):
                self.assertIn(f"'{label}'", self.component)
                self.assertIn(label, self.guide)

    def test_watchdog_polling_and_interruption_details_match_component(self) -> None:
        self.assertIn("DEAD_MAN_STATUS_POLL_MS = 30_000", self.component)
        self.assertIn("DEAD_MAN_STATUS_READ_OVERDUE_MS = 2 * DEAD_MAN_STATUS_POLL_MS", self.component)
        self.assertIn("recommendedGraceSeconds", self.component)
        self.assertIn("Last interruption", self.component)
        self.assertIn("clean restart", self.component)
        self.assertIn("unexpected stop", self.component)

        for phrase in (
            "Alerts → Notifications → External watchdog",
            "3-minute grace period",
            "A live status read does not mean a heartbeat was sent",
            "two 30-second poll intervals (60 seconds)",
            "Use Refresh to retry",
            "last known snapshot",
            "Last interruption",
            "clean restart",
            "unexpected stop",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, self.guide)


if __name__ == "__main__":
    unittest.main()
