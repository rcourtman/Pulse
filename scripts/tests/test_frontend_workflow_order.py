#!/usr/bin/env python3
"""An unshippable frontend must fail before the expensive full unit suite."""

from pathlib import Path
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"


class FrontendWorkflowOrderTest(unittest.TestCase):
    def setUp(self):
        self.job = yaml.safe_load(WORKFLOW.read_text())["jobs"]["frontend"]
        self.steps = self.job["steps"]

    def step(self, name):
        matches = [step for step in self.steps if step.get("name") == name]
        self.assertEqual(len(matches), 1, f"expected one {name} step")
        return matches[0]

    def test_type_build_and_size_admit_the_unit_suite_in_order(self):
        names = [step.get("name") for step in self.steps]
        ordered = [
            "Install frontend dependencies",
            "Type-check frontend",
            "Build frontend bundle (with embed copy)",
            "Check frontend bundle size budget",
            "Frontend unit tests",
            "Require frontend dependency audit",
        ]
        for name in ordered:
            self.step(name)
        positions = [names.index(name) for name in ordered]
        self.assertEqual(positions, sorted(positions))

    def test_admission_keeps_real_embed_build_and_unchanged_commands(self):
        commands = {
            "Type-check frontend": "npm run type-check",
            "Build frontend bundle (with embed copy)": "make frontend",
            "Check frontend bundle size budget": "npm run check:bundlesize",
            "Frontend unit tests": "npm run test",
        }
        for name, command in commands.items():
            with self.subTest(step=name):
                step = self.step(name)
                self.assertEqual(step["run"], command)
                # GitHub's default success() condition must stop here on a
                # failed prerequisite; neither warnings nor conditional skips
                # may turn a broken build into a successful Frontend check.
                self.assertNotIn("if", step)
                self.assertNotIn("continue-on-error", step)
                if name != "Build frontend bundle (with embed copy)":
                    self.assertEqual(step["working-directory"], "frontend-modern")
        self.assertNotIn("continue-on-error", self.job)

    def test_audit_still_reports_failure_after_an_earlier_failed_gate(self):
        audit = self.step("Audit complete frontend dependency graph")
        required = self.step("Require frontend dependency audit")
        self.assertEqual(audit["id"], "audit-complete")
        self.assertIs(audit["continue-on-error"], True)
        self.assertEqual(required["if"], "${{ !cancelled() }}")
        self.assertEqual(required["env"]["COMPLETE_AUDIT_RESULT"],
                         "${{ steps.audit-complete.outcome }}")
        self.assertNotIn("continue-on-error", required)
        self.assertIn('if [ "${COMPLETE_AUDIT_RESULT}" != success ]; then', required["run"])
        self.assertIn("exit 1", required["run"])


if __name__ == "__main__":
    unittest.main()
