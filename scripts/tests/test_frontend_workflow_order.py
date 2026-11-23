#!/usr/bin/env python3
"""The complete unit suite and the real frontend build must both reach the required verdict."""

import os
from pathlib import Path
import subprocess
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"
TRIAGE_VERDICT_STEP = "Require applicable triage proof when runtime is not applicable"
HELP_VERDICT_STEP = "Require selected Help proof"
RUNTIME_VERDICT_STEP = "Require complete frontend proof"


class FrontendWorkflowOrderTest(unittest.TestCase):
    def setUp(self):
        self.workflow = yaml.safe_load(WORKFLOW.read_text())
        self.jobs = self.workflow["jobs"]
        self.job = self.jobs["frontend-build"]
        self.steps = self.job["steps"]

    def step(self, name, *, job="frontend-build"):
        matches = [step for step in self.jobs[job]["steps"] if step.get("name") == name]
        self.assertEqual(len(matches), 1, f"expected one {name} step")
        return matches[0]

    def test_preinstall_serialization_step_runs_its_registered_graph_controls(self):
        # Run the actual dependency-free pre-install recipe, not just the
        # Python workflow assertions. Its Node controls also read this workflow
        # and must follow the producer when build and runtime jobs are split.
        # Otherwise a stale job selector can pass local proof and stop CI before
        # npm installation, leaving all expensive proof inapplicable.
        check = self.step("Check Solid serialization dependency edges")
        self.assertEqual(check, {
            "name": "Check Solid serialization dependency edges",
            "run": "node --test scripts/tests/check-frontend-serialization.test.mjs\n"
                   "node scripts/check-frontend-serialization.mjs\n",
        })
        names = [step.get("name") for step in self.steps]
        self.assertLess(names.index(check["name"]), names.index("Install frontend dependencies"))
        result = subprocess.run(["bash", "-e", "-u", "-o", "pipefail", "-c", check["run"]],
                                cwd=ROOT, text=True, capture_output=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_type_build_and_size_admit_native_runtime_in_order(self):
        names = [step.get("name") for step in self.steps]
        ordered = [
            "Install frontend dependencies",
            "Type-check frontend",
            "Build frontend bundle (with embed copy)",
            "Check frontend bundle size budget",
            "Bind the validated frontend embed",
            "Transfer the validated frontend embed",
            "Admit frontend runtime",
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

    def test_runtime_admission_is_after_bundle_transfer_but_before_the_audit_verdict(self):
        admission = self.step("Admit frontend runtime")
        self.assertEqual(admission["id"], "runtime-admission")
        self.assertEqual(admission["env"], {
            "EMBED_ARTIFACT_ID": "${{ steps.embed-upload.outputs.artifact-id }}",
        })
        self.assertIn('echo \'admitted=true\' >> "$GITHUB_OUTPUT"', admission["run"])
        self.assertEqual(self.job["outputs"], {
            "runtime_admitted": "${{ steps.runtime-admission.outputs.admitted }}",
            "embed_sha256": "${{ steps.embed-packet.outputs.sha256 }}",
            "embed_artifact_id": "${{ steps.embed-upload.outputs.artifact-id }}",
        })
        self.assertEqual(self.job["needs"], "changes")
        self.assertEqual(self.job["if"], "needs.changes.outputs.code == 'true'")
        self.assertEqual(self.job["timeout-minutes"], 30)
        # GitHub's default success() is essential: a failed/hidden build or transfer
        # phase must never produce the output admitting native runtime.
        for step in self.steps:
            if step["name"].startswith("Retain bounded "):
                self.assertEqual(step["if"], "always()")
            elif step["name"] != "Require frontend dependency audit":
                self.assertNotIn("if", step)
            if step["name"] != "Audit complete frontend dependency graph":
                self.assertNotIn("continue-on-error", step)
        self.assertEqual(self.steps[-1]["name"], "Require frontend dependency audit")

    def test_one_unit_runner_starts_at_classification_and_runs_the_complete_inventory(self):
        unit = self.jobs["frontend-unit"]
        self.assertEqual(unit["needs"], "changes")
        self.assertEqual(unit["if"], "needs.changes.outputs.code == 'true'")
        self.assertNotIn("strategy", unit)
        self.assertEqual(unit["runs-on"], "ubuntu-24.04")
        self.assertEqual(unit["timeout-minutes"], 30)
        self.assertNotIn("continue-on-error", unit)
        self.assertNotIn("env", unit)
        steps = unit["steps"]
        self.assertEqual([step["name"] for step in steps], [
            "Checkout repository", "Set up Node.js", "Install frontend dependencies", "Frontend unit tests",
            "Retain frontend unit observations",
        ])
        for step in steps[:-1]:
            self.assertNotIn("if", step)
            self.assertNotIn("continue-on-error", step)
            if step["name"] != "Frontend unit tests":
                self.assertEqual(step, self.step(step["name"]))
        self.assertEqual(steps[-2], {
            "name": "Frontend unit tests", "working-directory": "frontend-modern",
            "env": {
                "PULSE_UNIT_RECEIPT": "${{ runner.temp }}/frontend-unit-${{ github.run_attempt }}.json",
                "PULSE_UNIT_SOURCE_SHA": "${{ github.sha }}",
            },
            "run": "npm run test -- "
                   "--reporter=default --reporter=./scripts/ci-unit-receipts.mjs",
        })
        self.assertEqual(steps[-1], {
            "name": "Retain frontend unit observations", "if": "always()",
            "uses": "actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9",
            "with": {
                "name": "frontend-unit-${{ github.run_attempt }}",
                "path": "${{ runner.temp }}/frontend-unit-${{ github.run_attempt }}.json",
                "if-no-files-found": "warn", "retention-days": 7,
            },
        })
        self.assertEqual(self.workflow["permissions"], {"contents": "read"})

    def test_unit_reporter_controls_are_registered_and_run_before_dependency_work(self):
        step = self.step("Validate frontend unit evidence reporter")
        self.assertEqual(step["run"], "node --test scripts/tests/frontend-unit-receipts.test.mjs")
        names = [step["name"] for step in self.steps]
        self.assertLess(names.index(step["name"]), names.index("Install frontend dependencies"))
        result = subprocess.run(["node", "--test", "scripts/tests/frontend-unit-receipts.test.mjs"],
                                cwd=ROOT, text=True, capture_output=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_runtime_and_units_are_independent_required_siblings(self):
        runtime = self.jobs["frontend-checks"]
        unit = self.jobs["frontend-unit"]
        self.assertEqual(runtime["needs"], ["changes", "frontend-build"])
        self.assertEqual(unit["needs"], "changes")
        self.assertEqual(runtime["if"], "${{ !cancelled() && needs.changes.outputs.code == 'true' && needs.frontend-build.outputs.runtime_admitted == 'true' }}")
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
        env = dict(os.environ, CHANGE_RESULT="success", CODE_CHANGED="true",
                   BUILD_RESULT="success", CHECKS_RESULT="success", UNIT_RESULT="success",
                   TRIAGE_REQUIRED="false", TRIAGE_RESULT="skipped",
                   HELP_ONLY="false", HELP_RESULT="skipped", HELP_COMPLETED="")
        env.update(values)

        def run(name):
            step = self.step(name, job="frontend")
            return subprocess.run(["bash", "-e", "-u", "-o", "pipefail", "-c", step["run"]],
                                  env=env, text=True, capture_output=True, timeout=5)

        # GitHub skips the triage step for runtime changes. The other steps
        # use default success(); a failed required proof blocks the verdict.
        if env["CODE_CHANGED"] == "false":
            triage = run(TRIAGE_VERDICT_STEP)
            if triage.returncode:
                return triage
        help_proof = run(HELP_VERDICT_STEP)
        if help_proof.returncode:
            return help_proof
        return run(RUNTIME_VERDICT_STEP)

    def test_required_name_depends_on_classification_checks_and_unit_suite(self):
        verdict = self.jobs["frontend"]
        self.assertEqual(verdict["name"], "Frontend")
        self.assertEqual(sum(job.get("name") == "Frontend" for job in self.jobs.values()), 1)
        self.assertEqual(verdict["needs"],
                         ["changes", "frontend-build", "frontend-checks", "frontend-unit",
                          "triage-helper", "help-proof"])
        self.assertEqual(verdict["if"], "always()")
        self.assertEqual(verdict["timeout-minutes"], 5)
        self.assertNotIn("continue-on-error", verdict)
        self.assertEqual([item.get("name") for item in verdict["steps"]],
                         [TRIAGE_VERDICT_STEP, HELP_VERDICT_STEP, RUNTIME_VERDICT_STEP])
        triage = self.step(TRIAGE_VERDICT_STEP, job="frontend")
        self.assertEqual(triage["if"], "always() && needs.changes.outputs.code == 'false'")
        self.assertNotIn("continue-on-error", triage)
        self.assertEqual(triage["env"], {
            "TRIAGE_REQUIRED": "${{ needs.changes.outputs.triage_helper }}",
            "TRIAGE_RESULT": "${{ needs.triage-helper.result }}",
        })
        step = self.step(RUNTIME_VERDICT_STEP, job="frontend")
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
        self.assertIn("the unit suite passed", result.stdout)
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
        for required, state in (("false", "skipped"), ("true", "success")):
            with self.subTest(required=required, triage=state):
                result = self.verdict(**values, TRIAGE_REQUIRED=required, TRIAGE_RESULT=state)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn("no frontend tests ran", result.stdout)
        for required, state in (("true", "skipped"), ("true", "failure"), ("true", "cancelled"),
                                ("true", ""), ("false", ""), ("", "skipped")):
            with self.subTest(required=required, triage=state):
                result = self.verdict(**values, TRIAGE_REQUIRED=required, TRIAGE_RESULT=state)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("::error::Issue-triage proof", result.stdout)
                self.assertNotIn("no frontend tests ran", result.stdout)
        for name in ("CHANGE_RESULT", "BUILD_RESULT", "CHECKS_RESULT", "UNIT_RESULT", "CODE_CHANGED"):
            states = ("failure", "cancelled", "", "unknown")
            if name in ("BUILD_RESULT", "CHECKS_RESULT", "UNIT_RESULT"):
                states += ("success",)
            for state in states:
                with self.subTest(name=name, state=state):
                    self.assertNotEqual(self.verdict(**{**values, name: state}).returncode, 0)

    def test_main_push_help_requires_protected_scope_and_completed_checks(self):
        changes = self.jobs["changes"]
        self.assertEqual(changes["timeout-minutes"], 5)
        scope = self.step("Identify a mirrored Help-only main push", job="changes")
        self.assertEqual(scope["id"], "help-scope")
        self.assertEqual(scope["if"],
                         "github.event_name == 'push' && github.ref == 'refs/heads/main' && "
                         "github.ref_protected && steps.filter.outputs.code == 'true' && "
                         "steps.filter.outputs.go_compile == 'false' && "
                         "steps.filter.outputs.frontend_deps == 'false' && "
                         "steps.filter.outputs.triage_helper == 'false'")
        self.assertEqual(scope["env"], {
            "BASE_SHA": "${{ github.event.before }}", "TESTED_SHA": "${{ github.sha }}",
        })
        steps = changes["steps"]
        mirror = self.step("Check shipped docs before dependency work", job="changes")
        self.assertLess(steps.index(mirror), steps.index(scope))
        self.assertEqual(changes["outputs"]["help_only"],
                         "${{ steps.help-scope.outputs.selected == 'true' && 'true' || 'false' }}")
        self.assertEqual(changes["outputs"]["code"],
                         "${{ steps.help-scope.outputs.selected == 'true' && 'false' || steps.filter.outputs.code }}")

        job = self.jobs["help-proof"]
        self.assertEqual(job["needs"], "changes")
        self.assertEqual(job["if"], "needs.changes.outputs.help_only == 'true'")
        self.assertEqual(job["runs-on"], "ubuntu-24.04")
        self.assertEqual(job["timeout-minutes"], 12)
        self.assertNotIn("continue-on-error", job)
        checkout = self.step("Checkout the selected Help source", job="help-proof")
        self.assertEqual(checkout["uses"], self.step("Checkout repository", job="frontend-unit")["uses"])
        self.assertEqual(checkout["with"], {"ref": "${{ github.sha }}", "persist-credentials": False})
        setup = self.step("Set up Node.js for main-push Help checks", job="help-proof")
        proof = self.step("Check Help test source and rendered guides", job="help-proof")
        self.assertEqual(proof["id"], "help-proof")
        unit_setup = self.step("Set up Node.js", job="frontend-unit")
        self.assertEqual(setup["uses"], unit_setup["uses"])
        self.assertEqual(setup["with"], unit_setup["with"])
        self.assertEqual(proof["timeout-minutes"], 9)
        self.assertEqual(proof["working-directory"], "frontend-modern")
        self.assertEqual(proof["env"], {"EXPECTED_SOURCE_SHA": "${{ github.sha }}"})
        self.assertEqual(job["outputs"], {"completed": "${{ steps.help-proof.outputs.completed }}"})
        self.assertEqual(job["steps"], [checkout, setup, proof])
        for step in (checkout, setup, proof):
            self.assertNotIn("if", step)
            self.assertNotIn("continue-on-error", step)

    def test_selected_help_must_complete_before_frontend_can_pass(self):
        gate = self.step(HELP_VERDICT_STEP, job="frontend")
        self.assertNotIn("if", gate)
        self.assertNotIn("continue-on-error", gate)
        self.assertEqual(gate["env"], {
            "CHANGE_RESULT": "${{ needs.changes.result }}",
            "CODE_CHANGED": "${{ needs.changes.outputs.code }}",
            "HELP_ONLY": "${{ needs.changes.outputs.help_only }}",
            "HELP_RESULT": "${{ needs.help-proof.result }}",
            "HELP_COMPLETED": "${{ needs.help-proof.outputs.completed }}",
        })
        selected = dict(CODE_CHANGED="false", BUILD_RESULT="skipped", CHECKS_RESULT="skipped",
                        UNIT_RESULT="skipped", HELP_ONLY="true", HELP_RESULT="success",
                        HELP_COMPLETED="true")
        result = self.verdict(**selected)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for key, outcomes in (
            ("HELP_RESULT", ("failure", "cancelled", "skipped", "", "unknown")),
            ("HELP_COMPLETED", ("false", "", "unknown")),
            ("HELP_ONLY", ("false", "", "unknown")),
            ("CHANGE_RESULT", ("failure", "cancelled", "skipped", "")),
            ("CODE_CHANGED", ("true", "", "unknown")),
        ):
            for outcome in outcomes:
                with self.subTest(key=key, outcome=outcome):
                    result = self.verdict(**{**selected, key: outcome})
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn("Inspect the mirrored Help selection and proof", result.stdout)
        for result, completed in (("success", "true"), ("failure", ""), ("skipped", "true")):
            with self.subTest(unselected_result=result, completed=completed):
                self.assertNotEqual(self.verdict(HELP_RESULT=result, HELP_COMPLETED=completed).returncode, 0)

    def test_complete_audit_failure_still_fails_after_build_admits_runtime(self):
        required = self.step("Require frontend dependency audit")
        for state in ("success", "failure", "cancelled", "skipped", "", "unknown"):
            with self.subTest(state=state):
                result = subprocess.run(["bash", "-e", "-u", "-o", "pipefail", "-c", required["run"]],
                                        env=dict(os.environ, COMPLETE_AUDIT_RESULT=state),
                                        text=True, capture_output=True, timeout=5)
                self.assertEqual(result.returncode == 0, state == "success")


if __name__ == "__main__":
    unittest.main()
