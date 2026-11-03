#!/usr/bin/env python3
"""Exercise changed-area admission with shallow, exact-tree Git fixtures."""

import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"
ALL_CHECKS = dict.fromkeys(("code", "frontend_deps", "go_compile", "triage_helper"), "true")
NO_CHECKS = dict.fromkeys(ALL_CHECKS, "false")


class ComparisonCheckoutTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.seed = self.root / "seed"
        self.seed.mkdir()
        self.git(self.seed, "init", "-q", "-b", "main")
        self.git(self.seed, "config", "user.email", "fixture@example.invalid")
        self.git(self.seed, "config", "user.name", "CI fixture")
        for name in ("ci_browser_proof_only.py", "ci_triage_only.py", "ci_completion_only.py",
                     "go-test-compile-required.sh"):
            dest = self.seed / "scripts" / name
            dest.parent.mkdir(exist_ok=True)
            shutil.copyfile(ROOT / "scripts" / name, dest)
        self.write("docs/readme.md", "initial\n")
        self.base = self.commit()
        # A depth-two checkout cannot cover this range. Unrelated refs must not
        # be imported to obtain the two trees needed by the actual classifier.
        self.git(self.seed, "branch", "unrelated", self.base)
        self.git(self.seed, "tag", "unrelated-tag", self.base)
        for number in range(8):
            self.write("docs/readme.md", f"history {number}\n")
            self.commit()
        workflow = yaml.safe_load(WORKFLOW.read_text())
        self.job = workflow["jobs"]["changes"]
        self.prepare = next(step for step in self.job["steps"]
                            if step.get("name") == "Fetch exact comparison base")
        self.classifier = next(step["run"] for step in self.job["steps"]
                               if step.get("id") == "filter")

    def git(self, cwd, *args):
        result = subprocess.run(["git", *args], cwd=cwd, text=True,
                                capture_output=True, timeout=15)
        self.assertEqual(result.returncode, 0, f"git {args}: {result.stdout}{result.stderr}")
        return result.stdout.strip()

    def write(self, name, content):
        path = self.seed / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)

    def commit(self):
        self.git(self.seed, "add", ".")
        self.git(self.seed, "commit", "-qm", "fixture")
        return self.git(self.seed, "rev-parse", "HEAD")

    def checkout(self):
        self.repo = self.root / "checkout"
        self.git(self.root, "clone", "-q", "--depth=1", "--single-branch", "--no-tags",
                 self.seed.as_uri(), str(self.repo))
        self.head = self.git(self.repo, "rev-parse", "HEAD")
        self.assertEqual(self.git(self.repo, "rev-list", "--count", "HEAD"), "1")
        absent = subprocess.run(["git", "cat-file", "-e", self.base], cwd=self.repo,
                                capture_output=True, timeout=5)
        self.assertNotEqual(absent.returncode, 0)

    def run_prepare(self, base, extra_env=None):
        result = subprocess.run(["bash", "-c", self.prepare["run"]], cwd=self.repo,
                                env={**os.environ, "COMPARISON_BASE": base, **(extra_env or {})},
                                capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.git(self.repo, "rev-parse", "HEAD"), self.head)
        self.assertEqual(self.git(self.repo, "status", "--porcelain"), "")
        return result

    def classify(self, base, event="push"):
        script = self.classifier
        for name, value in {"github.event_name": event,
                            "github.event.pull_request.base.sha": base,
                            "github.event.before": base, "github.sha": self.head}.items():
            script = script.replace("${{ " + name + " }}", value)
        self.assertNotIn("${{", script)
        output = self.root / "outputs"
        output.write_text("")
        result = subprocess.run(["bash", "-c", script], cwd=self.repo,
                                env={**os.environ, "GITHUB_OUTPUT": str(output),
                                     "GOVERNANCE_TARGET_BRANCH": "main"},
                                capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return dict(line.split("=", 1) for line in output.read_text().splitlines())

    def test_only_admission_checkout_changes_not_history_secret_scan(self):
        checkout = next(step for step in self.job["steps"] if "uses" in step)
        self.assertEqual(checkout["with"]["fetch-depth"], 1)
        self.assertIs(checkout["with"]["persist-credentials"], False)
        self.assertEqual(self.job["timeout-minutes"], 5)
        self.assertEqual(self.prepare["env"]["COMPARISON_BASE"],
                         "${{ github.event_name == 'pull_request' && github.event.pull_request.base.sha || github.event.before }}")
        steps = self.job["steps"]
        self.assertLess(steps.index(self.prepare),
                        next(index for index, step in enumerate(steps) if step.get("id") == "filter"))
        secret = yaml.safe_load(WORKFLOW.read_text())["jobs"]["secret-scan"]
        checkout = next(step for step in secret["steps"] if "uses" in step)
        self.assertEqual(checkout["with"]["fetch-depth"], 0)
        self.assertNotIn("if", secret)

    def test_old_base_fetches_only_exact_tree_and_preserves_runtime_selection(self):
        self.write("internal/fixture.go", "package fixture\n")
        self.write("frontend-modern/package-lock.json", "{}\n")
        self.commit()
        self.checkout()
        self.run_prepare(self.base)
        self.assertEqual(self.git(self.repo, "rev-list", "--count", "--all"), "1")
        self.assertEqual(self.git(self.repo, "tag", "--list"), "")
        self.assertNotIn("unrelated", self.git(self.repo, "branch", "-a"))
        self.assertEqual(self.git(self.repo, "rev-list", "--count", self.base), "1")
        self.assertEqual(self.classify(self.base),
                         {**ALL_CHECKS, "triage_helper": "false"})
        self.assertEqual(self.classify(self.base, "pull_request"),
                         {**ALL_CHECKS, "triage_helper": "false"})

    def test_docs_and_receipt_only_ranges_keep_the_existing_decisions(self):
        self.checkout()
        self.run_prepare(self.base)
        self.assertEqual(self.classify(self.base), NO_CHECKS)
        # Compare two exact trees without requiring their intervening ancestry.
        receipt_base = self.head
        (self.repo / "frontend-modern/browser-verification").mkdir(parents=True)
        (self.repo / "frontend-modern/browser-verification/fixture.json").write_text("{}\n")
        self.git(self.repo, "-c", "user.name=CI fixture", "-c",
                 "user.email=fixture@example.invalid", "add", ".")
        self.git(self.repo, "-c", "user.name=CI fixture", "-c",
                 "user.email=fixture@example.invalid", "commit", "-qm", "receipt")
        self.head = self.git(self.repo, "rev-parse", "HEAD")
        self.assertEqual(self.classify(receipt_base), NO_CHECKS)

    def test_absent_or_invalid_base_cannot_become_a_fetch_option_or_skip_tests(self):
        self.checkout()
        for base in ("", "0" * 40, "--upload-pack=unexpected", self.base + "\n"):
            with self.subTest(base=base):
                result = self.run_prepare(base)
                self.assertIn("No comparison base", result.stdout)
        for base in ("", "0" * 40):
            self.assertEqual(self.classify(base), ALL_CHECKS)

    def test_failed_fetch_retains_full_checks_without_widening_refs(self):
        self.checkout()
        self.git(self.repo, "remote", "set-url", "origin", str(self.root / "absent"))
        result = self.run_prepare(self.base)
        self.assertIn("::warning::Comparison base fetch failed", result.stdout)
        self.assertEqual(self.classify(self.base), ALL_CHECKS)

    def test_bounded_fetch_timeout_retains_full_checks(self):
        self.checkout()
        commands = self.root / "commands"
        commands.mkdir()
        timeout = commands / "timeout"
        # Check production's exact deadline argv, then shorten only this inert
        # timeout fixture. Actual Git and both workflow scalars remain in use.
        timeout.write_text('#!/bin/bash\nset -eu\n'
                           'test "$1" = --kill-after=5s\ntest "$2" = 60s\nshift 2\n'
                           f'exec {shutil.which("timeout")} --kill-after=0.1s 0.2s "$@"\n')
        timeout.chmod(0o700)
        git = commands / "git"
        git.write_text('#!/bin/bash\n'
                       'if [ "${3:-}" = fetch ]; then sleep 5; exit 0; fi\n'
                       f'exec {shutil.which("git")} "$@"\n')
        git.chmod(0o700)
        result = self.run_prepare(self.base, {"PATH": str(commands) + ":" + os.environ["PATH"]})
        self.assertIn("::warning::Comparison base fetch failed", result.stdout)
        self.assertEqual(self.classify(self.base), ALL_CHECKS)


if __name__ == "__main__":
    unittest.main()
