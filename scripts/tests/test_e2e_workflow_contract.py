#!/usr/bin/env python3
"""Guard the stable/probation Core E2E verdict boundary."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github" / "workflows" / "test-e2e.yml"


class E2EWorkflowContractTest(unittest.TestCase):
    def test_selected_probation_is_required_on_prs_and_bounded(self) -> None:
        workflow = WORKFLOW.read_text()
        probation_start = workflow.index(
            "- name: Run probation-tier E2E suite\n"
        )
        report_start = workflow.index(
            "- name: Report probation-tier outcome", probation_start
        )
        probation_step = workflow[probation_start:report_start]

        self.assertIn(
            "continue-on-error: ${{ github.event_name != 'pull_request' }}",
            probation_step,
        )
        self.assertIn("needs.scope.outputs.run_probation == 'true'", probation_step)
        self.assertIn("steps.stable.conclusion != 'skipped'", probation_step)
        self.assertIn("!cancelled()", probation_step)
        self.assertIn("timeout-minutes: 12", probation_step)
        self.assertIn("--max-failures=5", probation_step)
        self.assertIn("--global-timeout=600000", probation_step)

        verdict_start = workflow.index("- name: Check E2E results")
        verdict = workflow[verdict_start:]
        self.assertNotIn("needs.probation", verdict)
        self.assertNotIn("steps.probation", verdict)


if __name__ == "__main__":
    unittest.main()
