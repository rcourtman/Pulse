#!/usr/bin/env python3
"""Keep passive version help tied to the real current/report/target displays."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "### Find the running server and agent versions\n"
ANCHOR = "#find-the-running-server-and-agent-versions"


class RunningVersionHelpDocsTest(unittest.TestCase):
    def setUp(self):
        self.source = (ROOT / "docs/TROUBLESHOOTING.md").read_text()
        self.guide = " ".join(
            self.source.split(HEADING, 1)[1].split("\n### ", 1)[0].split()
        )

    def test_workflow_entry_and_getting_help_reach_the_same_guidance(self):
        self.assertIn(ANCHOR, self.source.split("## ⚡ Quick Fixes", 1)[0])
        self.assertIn(ANCHOR, self.source.split("## 🆘 Getting Help", 1)[1])
        self.assertEqual(self.source.count(HEADING), 1)

    def test_server_instruction_names_the_running_field_not_the_update(self):
        copy = (ROOT / "frontend-modern/src/utils/updatesPresentation.ts").read_text()
        panel = (ROOT / "frontend-modern/src/components/Settings/UpdatesSettingsPanel.tsx").read_text()
        self.assertIn("currentVersionLabel: 'Server version'", copy)
        self.assertIn("{props.versionInfo()?.version || 'Loading...'}", panel)
        self.assertIn("**Server version**, not the available update", self.guide)
        self.assertIn("Keep any beta/RC suffix or development build identity", self.guide)
        self.assertIn("A newer server does not establish", self.guide)

    def test_agent_instruction_distinguishes_last_report_from_target(self):
        model = (ROOT / "frontend-modern/src/components/Settings/connectionsTableModel.ts").read_text()
        workspace = (ROOT / "frontend-modern/src/components/Settings/InfrastructureWorkspace.tsx").read_text()
        self.assertIn("connection.agentVersion?.trim()", model)
        self.assertIn("connection.expectedAgentVersion?.trim()", model)
        self.assertIn("detail: `${currentVersion} -> ${expectedVersion}`", model)
        self.assertIn("badgeLabel: 'Version target'", model)
        self.assertIn("Pulse Agent version", workspace)
        self.assertIn("infoCard('Last seen'", workspace)
        for fact in (
            "reported version on the left", "target on the right",
            "**Version target** alone is not a reported agent version",
            "last report does not prove what is running now",
            "agent as **none**, not the server's version",
            "UNIFIED_AGENT.md#auto-update",
        ):
            with self.subTest(fact=fact):
                self.assertIn(fact, self.guide)

    def test_image_helper_and_recovery_are_not_treated_as_running_source(self):
        for fact in (
            "`latest` is mutable", "not proof that a running container was replaced",
            "do not substitute a GitHub release tag", "`helm-chart-…`",
            "not a Pulse server archive", "**version at the failure**",
            "**intended version**", "**version observed after recovery**",
            "Today's display does not reconstruct the failing run",
            "verified download signature does not prove completion",
        ):
            with self.subTest(fact=fact):
                self.assertIn(fact, self.guide)

    def test_unknown_is_valid_without_mutation_or_private_exports(self):
        self.assertNotIn("```", self.guide)
        for boundary in (
            "write **unknown**", "do not widen access, reinstall or re-enrol",
            "rerun an update or enable Debug", "Do not share full container inspections",
            "resolved Compose configuration", "service environments or command lines",
            "locally reviewed, redacted excerpt", "already supplied does not need",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, self.guide)

    def test_shipped_help_is_byte_identical(self):
        self.assertEqual(
            (ROOT / "docs/TROUBLESHOOTING.md").read_bytes(),
            (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes(),
        )


if __name__ == "__main__":
    unittest.main()
