#!/usr/bin/env python3
"""Keep existing reporting guidance reachable without another questionnaire."""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
TOPICS = {
    "Versions and installation failures": "An API-only connection can have no agent.",
    "Failed or interrupted upgrades": "A verified download signature establishes artifact authenticity",
    "Platform and collection paths": "A Docker-hosted Pulse server can monitor a VM",
    "Several hosts or clusters": "Display names and VMIDs are not globally unique",
    "Missing readings and unresponsive workloads": "Responsiveness is not proof that every filesystem is writable",
    "Pulse performance and memory": "A plateau does not dismiss an increased memory requirement",
    "Guest memory disagreements": "RSS alone is not total guest usage",
    "Notification delivery and identity": "A successful Test does not establish ordinary delivery",
    "Sign-in and access reports": "Keep any working administrator session open",
}


def reporting_navigation_errors(source: str) -> list[str]:
    """Check destinations and relevant content, not headings copied into a TOC."""
    errors = []
    index = re.search(r"^## Find reporting guidance\n(.*?)(?=^## |\Z)", source, re.M | re.S)
    if not index:
        return ["missing early reporting index"]
    if index.start() > source.find("## Intake contract"):
        errors.append("reporting index follows the detailed guidance")
    links = re.findall(r"^- \[([^\]]+)\]\(#([^)]+)\)$", index[1], re.M)
    if len(links) != len(TOPICS) or {name for name, _ in links} != set(TOPICS):
        errors.append("reporting topics missing or repeated in index")
    for title, phrase in TOPICS.items():
        fragment = title.lower().replace(" ", "-")
        if (title, fragment) not in links:
            errors.append(f"missing direct link: {title}")
        sections = re.findall(r"^### " + re.escape(title) + r"\n(.*?)(?=^#{1,3} |\Z)", source, re.M | re.S)
        if len(sections) != 1 or phrase not in " ".join(sections[0].split()):
            errors.append(f"missing or misplaced topic: {title}")
    return errors


class IssueTriageDocsTest(unittest.TestCase):
    def setUp(self):
        self.source = (ROOT / "docs/ISSUE_TRIAGE.md").read_text(encoding="utf-8")

    def test_topics_have_early_direct_links_and_relevant_headings(self):
        self.assertEqual(reporting_navigation_errors(self.source), [])

    def test_shipped_copy_matches_and_existing_fragments_survive(self):
        self.assertEqual(self.source, (ROOT / "frontend-modern/public/docs/ISSUE_TRIAGE.md").read_text())
        for heading in ("## Intake contract", "### Sign-in and access reports",
                        "## Required disposition", "## Automation boundary"):
            self.assertEqual(self.source.count(heading + "\n"), 1)
        index = self.source.split("## Find reporting guidance\n", 1)[1].split("## Intake contract", 1)[0]
        for phrase in ("not a checklist of extra evidence", "unknown details are valid",
                       "does not need to be filed again"):
            self.assertIn(phrase, " ".join(index.split()))

    def test_missing_links_wrong_fragments_and_misplaced_topics_are_detected(self):
        for title in TOPICS:
            fragment = title.lower().replace(" ", "-")
            with self.subTest(topic=title):
                self.assertTrue(reporting_navigation_errors(self.source.replace(
                    f"- [{title}](#{fragment})\n", "")))
                self.assertTrue(reporting_navigation_errors(self.source.replace(
                    f"](#{fragment})", "](#missing-destination)")))
                self.assertTrue(reporting_navigation_errors(self.source.replace(
                    f"### {title}\n", f"**{title}**\n")))


if __name__ == "__main__":
    unittest.main()
