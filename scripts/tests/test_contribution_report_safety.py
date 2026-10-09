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
    def contribution_surfaces(self):
        return (
            " ".join((ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8").split()),
            " ".join((ROOT / ".github/PULL_REQUEST_TEMPLATE.md").read_text(encoding="utf-8").split()),
        )

    def test_no_new_external_pr_submission_instructions(self):
        guide, template = self.contribution_surfaces()
        for text in (guide, template):
            with self.subTest(surface=text[:40]):
                self.assertIn("not accepting unsolicited external pull requests", text)
                self.assertNotRegex(text, r"(?i)(?:^|[.!?]\s+|\d+\.\s+)Open a PR\b")
        self.assertNotIn("Submitting Requested Changes", guide)
        self.assertNotIn("Every requested PR", guide)

    def test_patch_is_optional_evidence_in_existing_issue(self):
        guide, template = self.contribution_surfaces()
        self.assertIn("## Sharing a tested patch", guide)
        self.assertIn("**existing issue**", guide)
        self.assertIn("do not open a new pull request", guide)
        self.assertIn("not a condition of reporting", guide)
        self.assertIn("not a promise that the change will be used", guide)
        self.assertIn("https://github.com/rcourtman/Pulse/blob/main/CONTRIBUTING.md#sharing-a-tested-patch", template)
        self.assertIn("Do not duplicate an existing report", template)

    def test_old_requested_contributions_keep_their_disposition(self):
        guide, template = self.contribution_surfaces()
        self.assertIn("one requested in an earlier conversation", guide)
        self.assertIn("reply and a disposition in its own thread", guide)
        self.assertIn("equivalent maintainer fix with credit and a commit link", guide)
        self.assertIn("already requested in an earlier conversation", template)
        self.assertIn("reply and disposition in this thread", template)
        for text in (guide, template):
            with self.subTest(surface=text[:40]):
                self.assertNotIn("please close it and open an issue instead", text)
                self.assertIn("refile", text)
        self.assertIn("Do not recreate it or refile evidence", guide)
        self.assertIn("do not recreate the PR or refile its evidence", template)

    def test_code_evidence_preserves_safety_and_publication_limits(self):
        guide, template = self.contribution_surfaces()
        for statement in (
            "source version", "tests actually run and known limits",
            "do not repeat an unsafe failure", "production installation",
            "public branch also exposes its commit history",
            "[SECURITY.md](SECURITY.md)", "fix reply credits its author",
            "release availability and reporter confirmation remain separate facts",
        ):
            self.assertIn(statement, guide)
        self.assertIn("test does not establish release availability", template)

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
