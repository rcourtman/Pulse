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
        self.assertNotIn("successful remediations (incident memory)", content)
        self.assertNotIn("**Incident memory**", content)
        self.assertNotIn("**Incident Memory**", content)
        self.assertNotIn("incident memory, investigation orchestration", content)


if __name__ == "__main__":
    unittest.main()
