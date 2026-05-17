#!/usr/bin/env python3
"""Guard public AI docs against retired commercial AI shorthand."""

from __future__ import annotations

import unittest

from repo_file_io import read_repo_text


class AIRuntimeDocsPolicyTest(unittest.TestCase):
    def test_public_ai_overview_uses_productized_context_language(self) -> None:
        content = read_repo_text("docs/AI.md")

        self.assertIn(
            "Uses prior alert context, Patrol run history, and resource timelines",
            content,
        )
        self.assertIn("Investigation Context", content)
        self.assertIn("the configured LLM owns diagnosis", content)
        self.assertIn(
            "Pulse supplies context, tools, safety gates, approval state, and audit trails",
            content,
        )
        self.assertIn("Pulse does not convert them into Pulse-authored findings", content)
        self.assertNotIn("successful remediations (incident memory)", content)
        self.assertNotIn("**Incident memory**", content)
        self.assertNotIn("**Incident Memory**", content)
        self.assertNotIn("incident memory, investigation orchestration", content)
        self.assertNotIn("learns what's normal", content)
        self.assertNotIn("multi-layered intelligence platform", content)
        self.assertNotIn("capacity predictions", content)
        self.assertNotIn("Deterministic Signal Detection", content)
        self.assertNotIn("active_alert", content)
        self.assertNotIn("auto-recovery", content)
        self.assertNotRegex(content, r"(?i)understands resources before you ask")


if __name__ == "__main__":
    unittest.main()
