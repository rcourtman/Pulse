#!/usr/bin/env python3
"""Keep failed browser setup out of the E2E test budget, without stale apt success."""

import os
from pathlib import Path
import re
import socket
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/test-e2e.yml"
POLICY = ROOT / "scripts/playwright-apt.conf"
INSTALL_POLICY = (
    "sudo install -m 0644 scripts/playwright-apt.conf "
    "/etc/apt/apt.conf.d/99pulse-playwright-acquire"
)


def job_block(workflow, job):
    return re.split(r"(?m)^  \S", workflow.split(f"\n  {job}:\n", 1)[1], maxsplit=1)[0]


def step_blocks(job):
    return re.split(r"(?m)^      - name: ", job)[1:]


class PlaywrightAcquisitionTest(unittest.TestCase):
    def assert_workflow_policy(self, workflow):
        for job in ("offline-org-provisioning", "e2e", "agent-registration"):
            with self.subTest(job=job):
                block = job_block(workflow, job)
                steps = step_blocks(block)
                policies = [i for i, step in enumerate(steps) if INSTALL_POLICY in step]
                installers = [
                    (i, step) for i, step in enumerate(steps)
                    if "npx playwright install --with-deps " in step
                ]
                self.assertEqual(len(policies), 1)
                self.assertEqual(len(installers), 1)
                i, install = installers[0]
                self.assertLess(policies[0], i)
                for step in (steps[policies[0]], install):
                    self.assertNotIn("continue-on-error", step)
                    self.assertNotIn("if:", step)
                    self.assertNotIn("||", step)
                # WebKit's complete OS graph was still downloading when the
                # former ten-minute cap killed a job before any test ran.
                # Only that larger setup gets more aggregate acquisition time.
                budget = 30 if job == "e2e" else 10
                self.assertIn(f"timeout-minutes: {budget}\n", install)
                self.assertIn("working-directory: tests/integration", install)
                browsers = "chromium webkit" if job == "e2e" else "chromium"
                self.assertIn(f"npx playwright install --with-deps {browsers}\n", install)
                # Browser setup must finish successfully before real journeys.
                self.assertLess(i, next(
                    n for n, step in enumerate(steps)
                    if step.startswith("Run stable-tier E2E suite")
                    or step.startswith("Prove authenticated default")
                    or step.startswith("Run agent registration lifecycle")
                ))
                if job == "e2e":
                    self.assertRegex(block, r"(?m)^    timeout-minutes: 60$")
                    self.assertNotRegex(install, r"(?m)^        timeout-minutes: 10$")

    def test_all_browser_jobs_bound_setup_without_weakening_gates(self):
        self.assert_workflow_policy(WORKFLOW.read_text())

    def test_policy_and_tests_trigger_both_pr_and_main_validation(self):
        workflow = WORKFLOW.read_text()
        for event in ("pull_request", "push"):
            block = re.split(r"(?m)^  \S", workflow.split(f"  {event}:\n", 1)[1], maxsplit=1)[0]
            self.assertIn("'scripts/playwright-apt.conf'", block)
            self.assertIn("'scripts/tests/test_playwright_acquisition.py'", block)
        selection = job_block(workflow, "tier-selection")
        self.assertLess(
            selection.index("python3 scripts/tests/test_playwright_acquisition.py"),
            selection.index("npm ci"),
        )

    def test_unbounded_and_ignored_setup_mutants_are_rejected(self):
        workflow = WORKFLOW.read_text()
        # Each mutation targets the fault, not an unrelated YAML assertion.
        for original, replacement in (
            ("        timeout-minutes: 10\n", ""),
            ("        timeout-minutes: 30\n", "        timeout-minutes: 10\n"),
            ("    timeout-minutes: 60\n", "    timeout-minutes: 90\n"),
            (INSTALL_POLICY, "echo no acquisition policy"),
            ("run: npx playwright install --with-deps chromium\n",
             "run: npx playwright install --with-deps chromium || true\n"),
        ):
            with self.subTest(mutation=replacement):
                # subTest failures cannot be caught as an ordinary assertion,
                # so collect the mutant's real unittest result independently.
                class Mutant(PlaywrightAcquisitionTest):
                    def runTest(inner):
                        inner.assert_workflow_policy(workflow.replace(original, replacement))

                result = unittest.TestResult()
                Mutant().run(result)
                self.assertFalse(result.wasSuccessful())
                self.assertTrue(result.failures)
                self.assertFalse(result.errors)

    def test_real_apt_parser_loads_only_the_four_expected_controls(self):
        with tempfile.TemporaryDirectory() as temp:
            config = Path(temp) / "apt.conf"
            config.write_text(
                'Dir::Etc::parts "-";\nDir::Etc::main "-";\n' + POLICY.read_text()
            )
            parsed = subprocess.run(
                ["apt-config", "dump"], env={**os.environ, "APT_CONFIG": str(config)},
                capture_output=True, text=True, timeout=10, check=True,
            ).stdout
        expected = {
            "Acquire::http::Timeout": "30",
            "Acquire::https::Timeout": "30",
            "Acquire::Retries": "2",
            "APT::Update::Error-Mode": "any",
        }
        entries = dict(re.findall(r'^(\S+) "([^"\n]*)";', POLICY.read_text(), re.M))
        self.assertEqual(entries, expected)
        for key, value in expected.items():
            self.assertIn(f'{key} "{value}";', parsed)
        self.assertNotIn("AllowUnauthenticated", parsed)
        self.assertNotIn("Verify-Peer", parsed)

    def test_incomplete_index_is_a_real_apt_failure_not_a_warning_pass(self):
        # An unlistening reserved loopback port gives a definite transport
        # failure. No external request, privileged apt operation, host list,
        # post-update hook or authentication bypass is involved.
        with socket.socket() as sock, tempfile.TemporaryDirectory() as temp:
            sock.bind(("127.0.0.1", 0))
            root = Path(temp)
            (root / "lists/partial").mkdir(parents=True)
            (root / "cache/archives/partial").mkdir(parents=True)
            sources = root / "sources.list"
            sources.write_text(f"deb http://127.0.0.1:{sock.getsockname()[1]} stable main\n")
            config = root / "apt.conf"
            prefix = (
                'Dir::Etc::parts "-";\nDir::Etc::main "-";\n'
                f'Dir::Etc::sourcelist "{sources}";\nDir::Etc::sourceparts "-";\n'
                f'Dir::State::lists "{root / "lists"}";\n'
                f'Dir::Cache "{root / "cache"}";\n'
                'Dir::State::status "/dev/null";\nDebug::NoLocking "true";\n'
            )
            for strict, expected_exit in ((True, 100), (False, 0)):
                policy = POLICY.read_text()
                if not strict:
                    policy = policy.replace('APT::Update::Error-Mode "any";', "")
                config.write_text(prefix + policy)
                result = subprocess.run(
                    ["apt-get", "update"],
                    env={**os.environ, "APT_CONFIG": str(config), "LC_ALL": "C"},
                    capture_output=True, text=True, timeout=20,
                )
                with self.subTest(strict=strict):
                    self.assertEqual(result.returncode, expected_exit, result.stderr)
                    self.assertIn("Failed to fetch http://127.0.0.1:", result.stderr)
                    self.assertIn("Connection refused", result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
