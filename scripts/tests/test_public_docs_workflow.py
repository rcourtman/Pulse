#!/usr/bin/env python3
"""Docs-only CI must exercise safety guidance, not just its Markdown shape.

Read the canonical block-YAML surface (also checked by workflow trust), then
execute its actual smoke command with disposable test adapters. No provider,
host service, backup or guest-agent operation is used.
"""

from __future__ import annotations

import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/public-docs.yml"
SAFETY_STEP = "Exercise backup safety and diagnostic recipes"
TESTS = (
    "test_pve_backup_troubleshooting_docs.py",
    "test_vm_disk_diagnostics.py",
    "test_memory_troubleshooting_docs.py",
    "test_performance_troubleshooting_docs.py",
    "test_troubleshooting_logs.py",
)


def step_body(source: str, name: str) -> str:
    marker = f"      - name: {name}\n"
    if source.count(marker) != 1:
        raise AssertionError(f"expected exactly one {name} step")
    return source.split(marker, 1)[1].split("\n      - name:", 1)[0]


def safety_command(source: str) -> str:
    body = step_body(source, SAFETY_STEP)
    marker = "        run: |\n"
    if body.count(marker) != 1:
        raise AssertionError("expected one literal safety command")
    return textwrap.dedent(body.split(marker, 1)[1]).strip()


class PublicDocsWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.source = WORKFLOW.read_text(encoding="utf-8")

    def test_both_events_cover_docs_checks_runner_and_helper(self):
        triggers = self.source.split("\non:\n", 1)[1].split("\npermissions:", 1)[0]
        required = {
            "*.md", "docs/**", "frontend-modern/public/docs/**",
            "scripts/check_public_docs.py", "scripts/check_docs_mirror.py",
            "scripts/readme_entrypoints_test.py", "scripts/tests/run.sh",
            "scripts/tests/test_public_docs_workflow.py", "scripts/test-vm-disk.sh",
            ".github/workflows/public-docs.yml",
            *(f"scripts/tests/{name}" for name in TESTS),
        }
        for event in ("pull_request", "push"):
            with self.subTest(event=event):
                block = triggers.split(f"  {event}:\n", 1)[1]
                block = re.split(r"\n  [a-z_]+:\n", block, maxsplit=1)[0]
                paths = re.findall(r'^      - "([^"]+)"$', block, re.MULTILINE)
                self.assertTrue(required.issubset(paths), required.difference(paths))
                self.assertEqual(len(paths), len(set(paths)))
        self.assertIn("  push:\n    branches:\n      - main\n", triggers)

    def test_exact_existing_suites_run_without_dependency_setup(self):
        command = safety_command(self.source).replace("\\\n", " ")
        self.assertEqual(shlex.split(command), ["scripts/tests/run.sh", *TESTS])
        for name in TESTS:
            self.assertTrue((ROOT / "scripts/tests" / name).is_file())
        self.assertNotIn("actions/setup-", self.source)
        self.assertNotRegex(self.source, r"\b(?:npm|pip|go) (?:ci|install|build|test)\b")

    def test_existing_document_admissions_are_preserved(self):
        for name, command in (
            ("Validate public documentation", "python3 scripts/check_public_docs.py"),
            ("Check getting-started safety and current plans", "python3 scripts/readme_entrypoints_test.py"),
            ("Check shipped docs mirror sync", "python3 scripts/check_docs_mirror.py"),
            ("Check documentation safety workflow contract", "python3 scripts/tests/test_public_docs_workflow.py"),
        ):
            with self.subTest(step=name):
                self.assertIn(f"        run: {command}\n", step_body(self.source, name))

    def test_safety_checks_cannot_be_conditionally_skipped_or_softened(self):
        self.assertNotRegex(self.source, r"(?m)^\s*(?:if|continue-on-error):")
        self.assertIn("permissions:\n  contents: read\n", self.source)
        self.assertIn("          persist-credentials: false\n", self.source)
        self.assertIn("    timeout-minutes: 10\n", self.source)

    def run_adapters(self, failing: str = ""):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            tests = root / "scripts/tests"
            tests.mkdir(parents=True)
            shutil.copyfile(ROOT / "scripts/tests/run.sh", tests / "run.sh")
            (tests / "run.sh").chmod(0o700)
            for name in TESTS:
                (tests / name).write_text(
                    "import os\nfrom pathlib import Path\n"
                    f"with Path(os.environ['CALLS']).open('a') as log: log.write({name!r} + '\\n')\n"
                    f"raise SystemExit({1 if name == failing else 0})\n",
                    encoding="utf-8",
                )
            calls = root / "calls"
            result = subprocess.run(
                ["bash", "-e", "-o", "pipefail", "-c", safety_command(self.source)],
                cwd=root, env=dict(os.environ, CALLS=str(calls)),
                text=True, capture_output=True, timeout=15,
            )
            return result, calls.read_text().splitlines()

    def test_actual_workflow_command_runs_every_suite(self):
        result, calls = self.run_adapters()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(calls, list(TESTS))
        self.assertIn("Summary: 5/5 passed", result.stdout)

    def test_each_failed_suite_is_a_failed_workflow_command(self):
        for failing in TESTS:
            with self.subTest(failing=failing):
                result, calls = self.run_adapters(failing)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(calls, list(TESTS))
                self.assertIn("Summary: 4/5 passed", result.stdout)
                self.assertIn("Failures: 1", result.stdout)


if __name__ == "__main__":
    unittest.main()
