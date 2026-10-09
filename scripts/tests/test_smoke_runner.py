#!/usr/bin/env python3
"""Exercise the real smoke runner's discovery admission and failure collection."""

from __future__ import annotations

import os
from pathlib import Path
import shlex
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
RUNNER = ROOT / "scripts/tests/run.sh"


class SmokeRunnerTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="pulse-smoke-runner-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.tests = self.root / "scripts/tests"
        self.tests.mkdir(parents=True)
        self.runner = self.tests / "run.sh"
        shutil.copyfile(RUNNER, self.runner)
        self.runner.chmod(0o700)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.calls = self.root / "calls"
        self.environment = {
            **os.environ,
            "PATH": f"{self.bin}{os.pathsep}{os.environ['PATH']}",
            "SMOKE_FIXTURE_CALLS": str(self.calls),
        }

    def python_fixture(self, name, status=0):
        path = self.tests / name
        path.write_text(
            "import os\nfrom pathlib import Path\n"
            f"with Path(os.environ['SMOKE_FIXTURE_CALLS']).open('a') as log: log.write({name!r} + '\\n')\n"
            f"raise SystemExit({status})\n",
            encoding="utf-8",
        )
        return path

    def shell_fixture(self, name, status=0):
        path = self.tests / name
        path.write_text(
            f"#!/bin/bash\nprintf '%s\\n' {shlex.quote(name)} >> \"$SMOKE_FIXTURE_CALLS\"\nexit {status}\n",
            encoding="utf-8",
        )
        path.chmod(0o700)
        return path

    def producer(self, name, status, paths=()):
        path = self.bin / name
        lines = ["#!/bin/bash"]
        # A failed sort consumes stdin like the real sorter, then can emit a
        # valid prefix. Neither a find nor sort prefix is a complete inventory.
        if name == "sort":
            lines.append("cat >/dev/null")
        lines.extend(f"printf '%s\\n' {shlex.quote(str(item))}" for item in paths)
        lines.append(f"exit {status}")
        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
        path.chmod(0o700)

    def run_runner(self, *arguments):
        return subprocess.run(
            ["bash", str(self.runner), *map(str, arguments)],
            cwd=self.root,
            env=self.environment,
            capture_output=True,
            text=True,
            timeout=15,
        )

    def observed_calls(self):
        return self.calls.read_text(encoding="utf-8").splitlines() if self.calls.exists() else []

    def assert_discovery_rejected(self, result):
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Failed to discover the complete smoke test inventory", result.stderr)
        self.assertNotIn("Summary:", result.stdout)
        self.assertEqual(self.observed_calls(), [])

    def test_failed_find_cannot_admit_partial_passing_inventory(self):
        passing = self.python_fixture("test_first.py")
        self.python_fixture("test_second.py", 7)
        self.producer("find", 23, [passing])
        self.assert_discovery_rejected(self.run_runner())

    def test_failed_sort_cannot_admit_partial_passing_inventory(self):
        passing = self.python_fixture("test_first.py")
        self.python_fixture("test_second.py", 7)
        self.producer("sort", 24, [passing])
        self.assert_discovery_rejected(self.run_runner())

    def test_empty_failed_find_is_discovery_failure(self):
        self.python_fixture("test_first.py")
        self.producer("find", 23)
        self.assert_discovery_rejected(self.run_runner())

    def test_empty_failed_sort_is_discovery_failure(self):
        self.python_fixture("test_first.py")
        self.producer("sort", 24)
        self.assert_discovery_rejected(self.run_runner())

    def test_successful_empty_discovery_is_not_one_empty_test_or_a_pass(self):
        result = self.run_runner()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("No tests found under", result.stderr)
        self.assertNotIn("Summary:", result.stdout)
        self.assertEqual(self.observed_calls(), [])

    def test_full_inventory_preserves_sorted_python_and_shell_checks(self):
        self.python_fixture("test_z.py")
        self.python_fixture("test_a.py")
        self.shell_fixture("test-b.sh")
        self.python_fixture("not_a_test.py", 7)
        nested = self.tests / "nested"
        nested.mkdir()
        (nested / "test_ignored.py").write_text("raise SystemExit(7)\n", encoding="utf-8")
        result = self.run_runner()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.observed_calls(), ["test-b.sh", "test_a.py", "test_z.py"])
        self.assertIn("Summary: 3/3 passed", result.stdout)

    def test_every_failure_is_collected_and_remains_gating(self):
        self.shell_fixture("test-a.sh", 3)
        self.python_fixture("test_a.py", 7)
        self.python_fixture("test_z.py")
        result = self.run_runner()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.observed_calls(), ["test-a.sh", "test_a.py", "test_z.py"])
        self.assertIn("Summary: 1/3 passed", result.stdout)
        self.assertIn("Failures: 2", result.stdout)

    def test_explicit_subset_keeps_order_and_does_not_discover(self):
        self.python_fixture("test_a.py")
        self.shell_fixture("test-b.sh")
        self.python_fixture("test_unselected.py", 7)
        self.producer("find", 23)
        self.producer("sort", 24)
        result = self.run_runner("test_a.py", "test-b.sh")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.observed_calls(), ["test_a.py", "test-b.sh"])
        self.assertIn("Summary: 2/2 passed", result.stdout)

    def test_unknown_explicit_test_stops_before_any_execution(self):
        self.python_fixture("test_first.py")
        result = self.run_runner("test_first.py", "test_missing.py")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Unknown test: test_missing.py", result.stderr)
        self.assertEqual(self.observed_calls(), [])

    def test_failed_explicit_test_still_runs_remaining_selected_tests(self):
        failing = self.python_fixture("test_first.py", 7)
        passing = self.shell_fixture("test-second.sh")
        result = self.run_runner(failing, passing)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.observed_calls(), ["test_first.py", "test-second.sh"])
        self.assertIn("Summary: 1/2 passed", result.stdout)


if __name__ == "__main__":
    unittest.main()
