#!/usr/bin/env python3
"""Keep operator safety guidance distinct from model and executor assurances.

These are documentation regressions, not runtime action qualification.
"""
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]


class AISafetyHelpTest(unittest.TestCase):
    def setUp(self):
        self.guide = (ROOT / "docs/AI_AUTONOMY.md").read_text()
        self.plain = " ".join(self.guide.split())

    def test_intro_describes_planning_not_chat_execution(self):
        intro = self.guide.split("## Patrol Modes", 1)[0]
        self.assertIn("plan infrastructure actions", intro)
        self.assertNotIn("can execute commands during a chat session", intro)
        self.assertIn("[what the guardrails establish](#safety-guardrails)", intro)

    def test_pattern_filter_is_not_a_guarantee_of_safety(self):
        self.assertNotIn("Commands that could cause data loss", self.guide)
        self.assertIn("not a proof that every other operation is safe", self.plain)
        self.assertIn("Typed lifecycle capabilities", self.plain)
        self.assertIn("intentionally stop or restart a workload", self.plain)
        self.assertIn("does not turn Assistant chat into an execution path", self.plain)

    def test_verification_is_separate_and_source_qualified(self):
        self.assertNotIn("enforced by the FSM", self.guide)
        self.assertNotIn("cannot return to idle", self.guide)
        for boundary in ("Execution and verification are separate results",
                         "Confirmed by independent observer", "Confirmed by executing agent",
                         "Outcome not verified", "Outcome inconclusive", "Outcome contradicted",
                         "postcondition at its observation time", "not current health",
                         "Do not repeat an uncertain operation"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, self.plain)
        presentation = (ROOT / "frontend-modern/src/features/actions/actionPresentation.ts").read_text()
        for label in ("Confirmed by independent observer", "Confirmed by executing agent",
                      "Outcome not verified", "Outcome inconclusive", "Outcome contradicted"):
            self.assertIn(label, presentation)

    def test_operator_checks_do_not_require_another_action_or_probe(self):
        for boundary in ("Review the existing action in **Actions**",
                         "not a guarantee of a correct diagnosis",
                         "does not reverse an action already dispatched",
                         "Do not run a command, change mode or provoke a failure",
                         "[Assistant safety architecture](ASSISTANT_SAFETY.md)"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, self.plain)
        self.assertIn("### Keep a Resource Off-Limits", self.guide)
        self.assertIn("Never auto-remediate", self.guide)

    def test_shipped_guides_and_index_preserve_the_route(self):
        for name in ("AI_AUTONOMY", "AI"):
            self.assertEqual((ROOT / f"docs/{name}.md").read_bytes(),
                             (ROOT / f"frontend-modern/public/docs/{name}.md").read_bytes())
        self.assertIn("[Patrol modes and safety](AI_AUTONOMY.md)",
                      (ROOT / "docs/README.md").read_text())
        self.assertNotIn("Detailed FSM states",
                         (ROOT / "docs/AI.md").read_text())


if __name__ == "__main__":
    unittest.main()
