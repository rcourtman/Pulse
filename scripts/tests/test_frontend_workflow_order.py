#!/usr/bin/env python3
"""Full frontend proof stays required across isolated file-level unit shards."""

import os
from pathlib import Path
import subprocess
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"


class FrontendWorkflowOrderTest(unittest.TestCase):
    def setUp(self):
        self.workflow = yaml.safe_load(WORKFLOW.read_text())
        self.jobs = self.workflow["jobs"]
        self.job = self.jobs["frontend-build"]
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
            "Bind the validated frontend embed",
            "Transfer the validated frontend embed",
            "Admit frontend unit shards",
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

    def test_unit_admission_is_after_bundle_transfer_but_before_the_audit_verdict(self):
        admission = self.step("Admit frontend unit shards")
        self.assertEqual(admission["id"], "unit-admission")
        self.assertEqual(admission["env"], {
            "EMBED_ARTIFACT_ID": "${{ steps.embed-upload.outputs.artifact-id }}",
        })
        self.assertIn('echo \'admitted=true\' >> "$GITHUB_OUTPUT"', admission["run"])
        self.assertEqual(self.job["outputs"], {
            "unit_admitted": "${{ steps.unit-admission.outputs.admitted }}",
            "embed_sha256": "${{ steps.embed-packet.outputs.sha256 }}",
            "embed_artifact_id": "${{ steps.embed-upload.outputs.artifact-id }}",
        })
        self.assertEqual(self.job["needs"], "changes")
        self.assertEqual(self.job["if"], "needs.changes.outputs.code == 'true'")
        self.assertEqual(self.job["timeout-minutes"], 30)
        # GitHub's default success() is essential: a failed/hidden build or transfer
        # phase must never produce the output admitting expensive unit shards.
        for step in self.steps:
            if step["name"].startswith("Retain bounded "):
                self.assertEqual(step["if"], "always()")
            elif step["name"] != "Require frontend dependency audit":
                self.assertNotIn("if", step)
            if step["name"] != "Audit complete frontend dependency graph":
                self.assertNotIn("continue-on-error", step)
        self.assertEqual(self.steps[-1]["name"], "Require frontend dependency audit")

    def test_four_isolated_shards_use_complete_vitest_inventory(self):
        unit = self.jobs["frontend-unit"]
        self.assertEqual(unit["needs"], ["changes", "frontend-build"])
        self.assertEqual(unit["if"], "${{ !cancelled() && needs.changes.outputs.code == 'true' && needs.frontend-build.outputs.unit_admitted == 'true' }}")
        self.assertEqual(unit["strategy"], {
            "fail-fast": False, "matrix": {"shard": [1, 2, 3, 4]},
        })
        self.assertEqual(unit["runs-on"], "ubuntu-24.04")
        self.assertEqual(unit["timeout-minutes"], 30)
        self.assertNotIn("continue-on-error", unit)
        self.assertNotIn("env", unit)
        steps = unit["steps"]
        self.assertEqual([step["name"] for step in steps], [
            "Checkout repository", "Set up Node.js", "Install frontend dependencies", "Frontend unit tests",
        ])
        for step in steps:
            self.assertNotIn("if", step)
            self.assertNotIn("continue-on-error", step)
            if step["name"] != "Frontend unit tests":
                self.assertEqual(step, self.step(step["name"]))
        self.assertEqual(steps[-1], {
            "name": "Frontend unit tests", "working-directory": "frontend-modern",
            "run": "npm run test -- --shard=${{ matrix.shard }}/4",
        })
        self.assertEqual(self.workflow["permissions"], {"contents": "read"})

    def test_runtime_and_units_are_independent_required_siblings(self):
        runtime = self.jobs["frontend-checks"]
        unit = self.jobs["frontend-unit"]
        self.assertEqual(runtime["needs"], ["changes", "frontend-build"])
        self.assertEqual(runtime["if"], unit["if"])
        self.assertEqual(runtime["runs-on"], "ubuntu-24.04")
        self.assertEqual(runtime["timeout-minutes"], 30)
        self.assertNotIn("continue-on-error", runtime)
        names = [step["name"] for step in runtime["steps"]]
        ordered = ["Check validated frontend embed identity",
                   "Download this run's validated frontend embed",
                   "Verify and restore the validated frontend embed",
                   "Set up Go for real-embed verification", "Verify backend with real frontend embed",
                   "Check current-source unattended recovery on native systemd",
                   "Check ordinary notification policy and restart"]
        self.assertEqual([names.index(name) for name in ordered], sorted(names.index(name) for name in ordered))
        for step in runtime["steps"]:
            self.assertNotIn("continue-on-error", step)
            if step["name"].startswith("Retain bounded "):
                self.assertEqual(step["if"], "always()")
            else:
                self.assertNotIn("if", step)

    def verdict(self, **values):
        step = self.jobs["frontend"]["steps"][0]
        env = dict(os.environ, CHANGE_RESULT="success", CODE_CHANGED="true",
                   BUILD_RESULT="success", CHECKS_RESULT="success", UNIT_RESULT="success")
        env.update(values)
        return subprocess.run(["bash", "-e", "-u", "-o", "pipefail", "-c", step["run"]],
                              env=env, text=True, capture_output=True, timeout=5)

    def test_required_name_depends_on_classification_checks_and_every_shard(self):
        verdict = self.jobs["frontend"]
        self.assertEqual(verdict["name"], "Frontend")
        self.assertEqual(sum(job.get("name") == "Frontend" for job in self.jobs.values()), 1)
        self.assertEqual(verdict["needs"], ["changes", "frontend-build", "frontend-checks", "frontend-unit"])
        self.assertEqual(verdict["if"], "always()")
        self.assertNotIn("continue-on-error", verdict)
        self.assertEqual(len(verdict["steps"]), 1)
        step = verdict["steps"][0]
        self.assertNotIn("if", step)
        self.assertNotIn("continue-on-error", step)
        self.assertEqual(step["env"], {
            "CHANGE_RESULT": "${{ needs.changes.result }}",
            "CODE_CHANGED": "${{ needs.changes.outputs.code }}",
            "BUILD_RESULT": "${{ needs.frontend-build.result }}",
            "CHECKS_RESULT": "${{ needs.frontend-checks.result }}",
            "UNIT_RESULT": "${{ needs.frontend-unit.result }}",
        })
        result = self.verdict()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("every unit shard passed", result.stdout)
        for name in ("CHANGE_RESULT", "BUILD_RESULT", "CHECKS_RESULT", "UNIT_RESULT"):
            for state in ("failure", "cancelled", "skipped", "", "unknown", "pending", "SUCCESS"):
                with self.subTest(name=name, state=state):
                    result = self.verdict(**{name: state})
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn("::error::Frontend proof", result.stdout)
        for state in ("false", "", "unknown", "TRUE"):
            with self.subTest(classification=state):
                self.assertNotEqual(self.verdict(CODE_CHANGED=state).returncode, 0)

    def test_only_consistent_inapplicable_proof_can_skip_the_unit_inventory(self):
        values = dict(CODE_CHANGED="false", BUILD_RESULT="skipped", CHECKS_RESULT="skipped", UNIT_RESULT="skipped")
        result = self.verdict(**values)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("no frontend tests ran", result.stdout)
        for name in ("CHANGE_RESULT", "BUILD_RESULT", "CHECKS_RESULT", "UNIT_RESULT", "CODE_CHANGED"):
            states = ("failure", "cancelled", "", "unknown")
            if name in ("BUILD_RESULT", "CHECKS_RESULT", "UNIT_RESULT"):
                states += ("success",)
            for state in states:
                with self.subTest(name=name, state=state):
                    self.assertNotEqual(self.verdict(**{**values, name: state}).returncode, 0)

    def test_complete_audit_failure_still_fails_after_build_admits_units(self):
        required = self.step("Require frontend dependency audit")
        for state in ("success", "failure", "cancelled", "skipped", "", "unknown"):
            with self.subTest(state=state):
                result = subprocess.run(["bash", "-e", "-u", "-o", "pipefail", "-c", required["run"]],
                                        env=dict(os.environ, COMPLETE_AUDIT_RESULT=state),
                                        text=True, capture_output=True, timeout=5)
                self.assertEqual(result.returncode == 0, state == "success")


if __name__ == "__main__":
    unittest.main()
