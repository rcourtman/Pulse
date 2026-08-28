#!/usr/bin/env python3
"""Guard the stable/probation Core E2E verdict boundary."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github" / "workflows" / "test-e2e.yml"


class E2EWorkflowContractTest(unittest.TestCase):
    def test_probation_failures_are_bounded_and_non_gating(self) -> None:
        workflow = WORKFLOW.read_text()
        probation_start = workflow.index(
            "- name: Run probation-tier E2E suite (non-gating)"
        )
        report_start = workflow.index(
            "- name: Report probation-tier outcome", probation_start
        )
        probation_step = workflow[probation_start:report_start]

        self.assertIn("continue-on-error: true", probation_step)
        self.assertIn("timeout-minutes: 12", probation_step)
        self.assertIn("--max-failures=5", probation_step)
        self.assertIn("--global-timeout=600000", probation_step)

        verdict_start = workflow.index("- name: Check E2E results")
        verdict = workflow[verdict_start:]
        self.assertNotIn("needs.probation", verdict)
        self.assertNotIn("steps.probation", verdict)


if __name__ == "__main__":
    unittest.main()
