#!/usr/bin/env python3
"""Keep the existing contribution entrypoint safe before a public report.

These are guidance checks, not diagnostic-runtime or incident-recovery proof.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]


def report_guidance() -> str:
    text = (ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8")
    return text.split("## How To Make An Issue Useful\n", 1)[1].split("\n---\n", 1)[0]


class ContributionReportSafetyTest(unittest.TestCase):
    def test_collection_is_optional_and_distinct_from_downloading_a_result(self):
        text = " ".join(report_guidance().split())
        for statement in (
            "Diagnostics are optional, not a condition of reporting",
            "download buttons reuse that result without running checks again",
            "Run Diagnostics** can make live API and guest-agent requests",
            "Do not run it during a backup, freeze/thaw or an unresponsive-host incident",
            "Keep the original evidence instead",
            "not prove that normal monitoring has recovered or a guest has thawed",
            "docs/TROUBLESHOOTING.md#collect-diagnostics-safely",
        ):
            self.assertIn(statement, text)

    def test_review_is_required_before_public_attachment_even_for_sanitized_exports(self):
        text = " ".join(report_guidance().split())
        for statement in (
            "local file, not an upload", "GitHub (review first)",
            "Export for GitHub (sanitized)** in older versions",
            "review files and screenshots locally before posting",
            "a sanitized export is not a guarantee", "free-text errors",
            "credentials", "session cookies", "secret URLs", "private host",
            "network or personal details", "echoed in errors",
            "Keep **Full (private)** exports private", "configuration", ".env",
            "private keys", "Copy as cURL", "full network exports",
            "Never put credentials in a command line, URL or thread",
        ):
            self.assertIn(statement, text)

    def test_performance_evidence_does_not_require_risky_or_private_collection(self):
        text = " ".join(report_guidance().split())
        for statement in (
            "existing readings or safe passive observations", "Where known",
            "Pulse process", "container or the whole host", "units",
            "measurement window", "uptime", "Unavailable readings are valid evidence",
            "Do not restart, create load or change polling or retention just to measure",
            "do not attach raw profiles, heap dumps, databases or full process command lines",
        ):
            self.assertIn(statement, text)

    def test_shipped_contribution_guide_matches_the_repository_entrypoint(self):
        self.assertEqual(
            (ROOT / "CONTRIBUTING.md").read_bytes(),
            (ROOT / "frontend-modern/public/docs/CONTRIBUTING.md").read_bytes(),
        )


if __name__ == "__main__":
    unittest.main()
