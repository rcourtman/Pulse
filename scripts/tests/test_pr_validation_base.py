#!/usr/bin/env python3
"""Exercise exact checked-out PR ranges with real Git histories and workflow shell."""

import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[2]
HELPER_PATH = Path("scripts/release_control/pr_validation_base.py")
WORKFLOW = ROOT / ".github/workflows/canonical-governance.yml"


class PRValidationBaseTest(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory(prefix="pulse-pr-validation-base-")
        self.addCleanup(directory.cleanup)
        self.repo = Path(directory.name)
        self.environment = {
            **{key: value for key, value in os.environ.items() if not key.startswith("GIT_")},
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_AUTHOR_NAME": "Test Maintainer",
            "GIT_AUTHOR_EMAIL": "test@example.invalid",
            "GIT_COMMITTER_NAME": "Test Maintainer",
            "GIT_COMMITTER_EMAIL": "test@example.invalid",
        }
        self.git("init", "-q")
        self.tree = self.git("mktree", input="")
        self.root = self.commit("root")
        self.event_base = self.commit("target when event was recorded", self.root)
        self.target = self.commit("already landed target change", self.event_base)
        self.source = self.commit("PR source change", self.event_base)
        self.merge = self.commit("provider tested merge", self.target, self.source)
        self.git("update-ref", "HEAD", self.merge)
        helper = self.repo / HELPER_PATH
        helper.parent.mkdir(parents=True)
        shutil.copyfile(ROOT / HELPER_PATH, helper)

    def git(self, *arguments, input=None):
        return subprocess.run(
            ["git", *arguments], cwd=self.repo, env=self.environment, input=input,
            check=True, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        ).stdout.strip()

    def commit(self, message, *parents):
        arguments = ["commit-tree", self.tree, "-m", message]
        for parent in parents:
            arguments.extend(["-p", parent])
        return self.git(*arguments)

    def invoke(self, *, checkout=None, source=None, event_base=None):
        return subprocess.run(
            [sys.executable, str(self.repo / HELPER_PATH),
             "--commit=" + (self.merge if checkout is None else checkout),
             "--source-head=" + (self.source if source is None else source),
             "--event-base=" + (self.event_base if event_base is None else event_base)],
            cwd=self.repo, env=self.environment, text=True,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10,
        )

    def assert_refused(self, result):
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(result.stdout, "")
        self.assertIn("cannot verify PR validation base", result.stderr)

    def test_stale_event_base_selects_the_actual_tested_target(self):
        for event_base in (self.event_base, self.target):
            with self.subTest(event_base=event_base):
                result = self.invoke(event_base=event_base)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.strip(), self.target)
                changed = self.git("rev-list", "--reverse", "--no-merges",
                                   result.stdout.strip() + ".." + self.merge).splitlines()
                self.assertEqual(changed, [self.source])
                self.assertNotIn(self.target, changed)

    def test_wrong_checkout_source_and_unrelated_base_cannot_select_a_range(self):
        unrelated = self.commit("unrelated root")
        for arguments in (
            {"checkout": self.source}, {"source": self.root},
            {"event_base": unrelated}, {"event_base": ""},
            {"event_base": "--help"}, {"checkout": "abc123"},
            {"source": "0" * 40},
        ):
            with self.subTest(arguments=arguments):
                self.assert_refused(self.invoke(**arguments))

    def test_only_the_provider_two_parent_shape_is_accepted(self):
        octopus = self.commit("three parents", self.target, self.source,
                              self.commit("other root"))
        for checkout in (self.source, octopus):
            with self.subTest(checkout=checkout):
                self.git("update-ref", "HEAD", checkout)
                self.assert_refused(self.invoke(checkout=checkout))

    def workflow_step(self):
        text = WORKFLOW.read_text()
        start = text.index("      - name: Determine governance diff range\n")
        return text[start:text.index("\n      - name:", start + 1)]

    def run_workflow_step(self, event, *, source=None):
        step = self.workflow_step()
        command = textwrap.dedent(step.split("        run: |\n", 1)[1])
        output = self.repo / "workflow-output"
        output.unlink(missing_ok=True)
        result = subprocess.run(
            ["bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", command],
            cwd=self.repo, text=True, timeout=10,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            env={
                **self.environment,
                "PULSE_GOVERNANCE_EVENT": event,
                "PULSE_GOVERNANCE_CHECKOUT": self.merge,
                "PULSE_GOVERNANCE_PR_HEAD": self.source if source is None else source,
                "PULSE_GOVERNANCE_PR_BASE": self.event_base,
                "PULSE_GOVERNANCE_BEFORE": self.event_base,
                "GITHUB_OUTPUT": str(output),
            },
        )
        return result, output.read_text() if output.exists() else ""

    def test_actual_workflow_uses_verified_range_and_stops_on_mismatched_source(self):
        step = self.workflow_step()
        for variable, expression in (
            ("EVENT", "github.event_name"),
            ("CHECKOUT", "github.sha"),
            ("PR_HEAD", "github.event.pull_request.head.sha"),
            ("PR_BASE", "github.event.pull_request.base.sha"),
            ("BEFORE", "github.event.before"),
        ):
            self.assertIn("PULSE_GOVERNANCE_" + variable + ": ${{ " + expression + " }}", step)
        result, output = self.run_workflow_step("pull_request")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(output, f"range={self.target}...{self.merge}\n")
        result, output = self.run_workflow_step("pull_request", source=self.root)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(output, "")

    def test_push_and_dispatch_keep_their_existing_ranges(self):
        for event, expected in (
            ("push", f"{self.event_base}...{self.merge}"),
            ("workflow_dispatch", "HEAD^...HEAD"),
        ):
            with self.subTest(event=event):
                result, output = self.run_workflow_step(event)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(output, f"range={expected}\n")


if __name__ == "__main__":
    unittest.main()
