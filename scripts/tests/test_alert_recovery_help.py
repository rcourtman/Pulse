"""Keep the passive usage-alert help reachable and its safety distinctions intact.

These are documentation checks, not reproduction of a reporter's incident.
The companion browser recipe consumes the real Docs page and shipped Markdown.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/TROUBLESHOOTING.md"
HEADING = "A usage alert stays open after usage drops"
FRAGMENT = "a-usage-alert-stays-open-after-usage-drops"


def recovery_help():
    text = DOC.read_text(encoding="utf-8")
    return text.split("#### " + HEADING + "\n", 1)[1].split("\n#### ", 1)[0]


class AlertRecoveryHelpTest(unittest.TestCase):
    def test_symptom_index_reaches_the_shipped_help(self):
        text = DOC.read_text(encoding="utf-8")
        index = text.split("## ⚡ Quick Fixes", 1)[0]
        self.assertIn("TROUBLESHOOTING.md#" + FRAGMENT, index)
        self.assertEqual(text.count("#### " + HEADING + "\n"), 1)
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes())

    def test_help_separates_identity_freshness_and_hysteresis(self):
        text = " ".join(recovery_help().split())
        for distinction in (
            "same connection, resource and metric", "filesystem or mountpoint",
            "`/data` does not establish recovery for `/`", "recorded breach value",
            "value and its timestamp", "a single lower sample",
            "not measured recovery", "90% trigger and 85% clear",
            "88% can leave an alert open", "85% or lower is eligible",
            "example values", "averaging window and recovery delay",
            "beyond the configured recovery delay and ordinary evaluations",
        ):
            self.assertIn(distinction, text)

    def test_incident_history_acknowledgement_and_delivery_are_not_recovery(self):
        text = " ".join(recovery_help().split())
        for distinction in (
            "Alerts → Overview", "Alerts → History", "recorded events",
            "not necessarily an active alert", "Show acknowledged",
            "does not repair or clear the condition", "If the incident cleared",
            "absence of a message does not prove the condition is still active",
        ):
            self.assertIn(distinction, text)

    def test_guidance_does_not_transfer_an_unsafe_reproduction_to_the_reporter(self):
        text = " ".join(recovery_help().split())
        for boundary in (
            "do not force a guest-agent probe or repeat a backup",
            "include only details not already supplied", "unresolved symptom",
            "Unknown settings or times can remain unknown",
            "do not post full configuration or diagnostics",
            "Do not acknowledge, disable monitoring, alter thresholds, restart Pulse or delete history",
        ):
            self.assertIn(boundary, text)
        self.assertNotRegex(text, r"```|\bcurl\b|--token|[?&]token=")

    def test_explanatory_links_keep_their_existing_targets(self):
        for target in re.findall(r"\]\(([^)]+)\)", recovery_help()):
            path, _, fragment = target.partition("#")
            destination = ROOT / "docs" / path if path else DOC
            self.assertTrue(destination.is_file(), target)
            # These headings have plain ASCII names and no duplicate slugs.
            headings = re.findall(r"^#{1,6} (.+)$", destination.read_text(), re.MULTILINE)
            slugs = {re.sub(r"[^\w\- ]", "", h.lower()).replace(" ", "-") for h in headings}
            self.assertIn(fragment, slugs, target)


if __name__ == "__main__":
    unittest.main()
