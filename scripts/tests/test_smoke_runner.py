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
        self.selector = self.root / "scripts/select-smoke-test-shard.py"
        shutil.copyfile(ROOT / "scripts/select-smoke-test-shard.py", self.selector)
        self.weights = self.root / ".github/scripts/smoke-test-seconds.txt"
        self.weights.parent.mkdir(parents=True)
        self.weights.write_text("DEFAULT_WEIGHT 1\n", encoding="utf-8")
        self.doc_weights = self.weights.with_name("docs-test-seconds.txt")
        self.doc_weights.write_text("DEFAULT_WEIGHT 1\n", encoding="utf-8")
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

    def test_failure_summary_keeps_observed_exits_in_execution_order(self):
        self.python_fixture("test_python.py", 7)
        self.shell_fixture("test-shell.sh", 3)
        self.python_fixture("test_later.py")
        result = self.run_runner("test_python.py", "test-shell.sh", "test_later.py")
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertEqual(self.observed_calls(), ["test_python.py", "test-shell.sh", "test_later.py"])
        self.assertTrue(result.stdout.endswith(
            "Summary: 1/3 passed\nFailures: 2\nFailed suites:\n"
            "  scripts/tests/test_python.py (exit 7)\n"
            "  scripts/tests/test-shell.sh (exit 3)\n"), result.stdout)

    def test_failure_summary_reports_nonexecutable_suite_and_keeps_later_tests(self):
        blocked = self.shell_fixture("test-blocked.sh")
        blocked.chmod(0o600)
        self.python_fixture("test_later.py")
        result = self.run_runner(blocked, "test_later.py")
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertEqual(self.observed_calls(), ["test_later.py"])
        self.assertTrue(result.stdout.endswith(
            "Summary: 1/2 passed\nFailures: 1\nFailed suites:\n"
            "  scripts/tests/test-blocked.sh (exit 126)\n"), result.stdout)

    def test_failure_summary_reports_exit_137_without_diagnosing_a_signal(self):
        self.shell_fixture("test-exit.sh", 137)
        result = self.run_runner("test-exit.sh")
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertEqual(self.observed_calls(), ["test-exit.sh"])
        self.assertTrue(result.stdout.endswith(
            "Summary: 0/1 passed\nFailures: 1\nFailed suites:\n"
            "  scripts/tests/test-exit.sh (exit 137)\n"), result.stdout)
        self.assertNotIn("signal", result.stdout.lower())
        self.assertNotIn("oom", result.stdout.lower())

    def test_failure_summary_preserves_literal_paths_with_spaces_and_percent(self):
        name = "test percent%s.py"
        self.python_fixture(name, 9)
        result = self.run_runner(name)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertEqual(self.observed_calls(), [name])
        self.assertTrue(result.stdout.endswith(
            "Summary: 0/1 passed\nFailures: 1\nFailed suites:\n"
            f"  scripts/tests/{name} (exit 9)\n"), result.stdout)

    def test_failure_summary_names_only_failures_in_both_sharded_modes(self):
        names = [f"test_{index}_docs.py" for index in range(8)]
        statuses = {name: index + 1 if index < 4 else 0 for index, name in enumerate(names)}
        for name, status in statuses.items():
            self.python_fixture(name, status)
        for mode, count in (("--smoke-shard", 2), ("--docs-shard", 4)):
            for shard in range(count):
                with self.subTest(mode=mode, shard=shard):
                    before = len(self.observed_calls())
                    result = self.run_runner(mode, shard)
                    calls = self.observed_calls()[before:]
                    self.assertEqual(calls, names[shard::count])
                    self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                    failures = [name for name in calls if statuses[name]]
                    expected = "Failed suites:\n" + "".join(
                        f"  scripts/tests/{name} (exit {statuses[name]})\n" for name in failures)
                    self.assertTrue(result.stdout.endswith(expected), result.stdout)

    def test_success_does_not_emit_a_failure_list_and_keeps_child_output(self):
        fixture = self.python_fixture("test_success.py")
        fixture.write_text("import sys\nprint('child stdout')\n"
                           "print('child stderr', file=sys.stderr)\n" + fixture.read_text(), encoding="utf-8")
        result = self.run_runner(fixture)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.observed_calls(), ["test_success.py"])
        self.assertEqual(result.stdout.count("child stdout"), 1)
        self.assertEqual(result.stderr, "child stderr\n")
        self.assertTrue(result.stdout.endswith("Summary: 1/1 passed\n"), result.stdout)
        self.assertNotIn("Failed suites:", result.stdout)
        self.assertNotIn("FAIL", result.stdout)

    def test_docs_shards_cover_current_and_future_suites_once_without_unrelated_tests(self):
        names = ["test_admin_docs.py", "test_api_auth_docs.py", "test_docs_reader.py",
                 "test_future_docs.py", "test_recovery_docs.py", "test_security_docs.py",
                 "test_troubleshooting_logs.py", "test_vm_disk_diagnostics.py"]
        for name in names:
            self.python_fixture(name)
        names.append("test-retired-trial-acquisition-docs.sh")
        self.shell_fixture(names[-1])
        self.python_fixture("test_backend.py", 7)
        self.shell_fixture("test-agent-runtime.sh", 7)
        nested = self.tests / "nested"
        nested.mkdir()
        (nested / "test_nested_docs.py").write_text("raise SystemExit(7)\n")
        seen = []
        for shard in range(4):
            before = len(self.observed_calls())
            result = self.run_runner("--docs-shard", shard)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            calls = self.observed_calls()[before:]
            self.assertEqual(calls, sorted(names)[shard::4])
            seen.extend(calls)
            self.assertIn(f"Documentation shard {shard}/4:", result.stdout)
        self.assertEqual(sorted(seen), sorted(names))
        self.assertEqual(len(seen), len(set(seen)))

    def test_every_doc_failure_remains_gating_and_collects_the_remaining_shard(self):
        names = [f"test_{index}_docs.py" for index in range(8)]
        for index, name in enumerate(names):
            self.python_fixture(name, 7 if index < 4 else 0)
        for shard in range(4):
            before = len(self.observed_calls())
            result = self.run_runner("--docs-shard", shard)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(self.observed_calls()[before:], names[shard::4])
            self.assertIn("Summary: 1/2 passed", result.stdout)
            self.assertIn("Failures: 1", result.stdout)

    def test_docs_discovery_rejects_failed_find_and_sort_before_execution(self):
        passing = self.python_fixture("test_first_docs.py")
        self.python_fixture("test_second_docs.py", 7)
        for name in ("find", "sort"):
            with self.subTest(producer=name):
                self.producer(name, 23, [passing])
                self.assert_discovery_rejected(self.run_runner("--docs-shard", 0))
                (self.bin / name).unlink()

    def test_invalid_doc_shard_arguments_stop_before_discovery_or_execution(self):
        self.python_fixture("test_first_docs.py")
        self.producer("find", 23)
        for args in ((), ("-1",), ("4",), ("00",), ("one",), ("0", "test_first_docs.py")):
            with self.subTest(args=args):
                result = self.run_runner("--docs-shard", *args)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Usage:", result.stderr)
                self.assertNotIn("Failed to discover", result.stderr)
                self.assertEqual(self.observed_calls(), [])

    def test_empty_doc_inventory_or_shard_is_not_a_pass(self):
        self.python_fixture("test_unrelated.py")
        empty = self.run_runner("--docs-shard", 0)
        self.assertNotEqual(empty.returncode, 0)
        self.assertIn("No tests found", empty.stderr)
        self.python_fixture("test_first_docs.py")
        empty = self.run_runner("--docs-shard", 3)
        self.assertNotEqual(empty.returncode, 0)
        self.assertIn("No tests found", empty.stderr)
        self.assertEqual(self.observed_calls(), [])

    def test_smoke_shards_cover_all_shell_python_and_future_suites_once(self):
        names = ["test_a.py", "test_backend.py", "test_future.py", "test_recovery_docs.py",
                 "test_security.py", "test_z.py"]
        for name in names:
            self.python_fixture(name)
        names.extend(["test-a.sh", "test-z.sh"])
        for name in names[-2:]:
            self.shell_fixture(name)
        self.python_fixture("not_a_test.py", 7)
        nested = self.tests / "nested"
        nested.mkdir()
        (nested / "test_nested.py").write_text("raise SystemExit(7)\n")
        seen = []
        for shard in range(2):
            before = len(self.observed_calls())
            result = self.run_runner("--smoke-shard", shard)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            calls = self.observed_calls()[before:]
            self.assertEqual(calls, sorted(names)[shard::2])
            seen.extend(calls)
            self.assertIn(f"Smoke shard {shard}/2: 4 of 8 suites", result.stdout)
        self.assertEqual(sorted(seen), sorted(names))
        self.assertEqual(len(seen), len(set(seen)))

    def test_smoke_shards_keep_failures_gating_and_continue_selected_suites(self):
        names = [f"test_{index}.py" for index in range(6)]
        for index, name in enumerate(names):
            self.python_fixture(name, 7 if index < 2 else 0)
        for shard in range(2):
            before = len(self.observed_calls())
            result = self.run_runner("--smoke-shard", shard)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(self.observed_calls()[before:], names[shard::2])
            self.assertIn("Summary: 2/3 passed", result.stdout)
            self.assertIn("Failures: 1", result.stdout)

    def test_smoke_discovery_rejects_failed_find_and_sort_before_execution(self):
        passing = self.python_fixture("test_first.py")
        self.python_fixture("test_second.py", 7)
        for name in ("find", "sort"):
            with self.subTest(producer=name):
                self.producer(name, 23, [passing])
                self.assert_discovery_rejected(self.run_runner("--smoke-shard", 0))
                (self.bin / name).unlink()

    def test_invalid_smoke_shard_stops_before_discovery_or_execution(self):
        self.python_fixture("test_first.py")
        self.producer("find", 23)
        for args in ((), ("-1",), ("2",), ("00",), ("one",), ("0", "test_first.py")):
            with self.subTest(args=args):
                result = self.run_runner("--smoke-shard", *args)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Usage:", result.stderr)
                self.assertNotIn("Failed to discover", result.stderr)
                self.assertEqual(self.observed_calls(), [])

    def test_empty_smoke_inventory_or_shard_is_not_a_pass(self):
        empty = self.run_runner("--smoke-shard", 0)
        self.assertNotEqual(empty.returncode, 0)
        self.assertIn("No tests found", empty.stderr)
        self.python_fixture("test_first.py")
        empty = self.run_runner("--smoke-shard", 1)
        self.assertNotEqual(empty.returncode, 0)
        self.assertIn("No tests found", empty.stderr)
        self.assertEqual(self.observed_calls(), [])


    def test_measured_weights_separate_slow_suites_without_changing_commands(self):
        names = [f"test_{index}.py" for index in range(8)]
        seconds = {name: 100 if index % 2 else 1 for index, name in enumerate(names)}
        for name in names:
            self.python_fixture(name)
        self.weights.write_text("DEFAULT_WEIGHT 1\n" + "".join(
            f"{name} {value}\n" for name, value in seconds.items()))
        seen = []
        for shard in range(2):
            before = len(seen)
            result = self.run_runner("--smoke-shard", shard)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            calls = self.observed_calls()[before:]
            seen.extend(calls)
            self.assertEqual(calls, sorted(calls))
            self.assertEqual(sum(seconds[name] for name in calls), 202)
        self.assertEqual(sorted(seen), names)
        self.assertEqual(len(seen), len(set(seen)))

    def test_unknown_renamed_and_stale_hints_never_change_discovered_coverage(self):
        names = ["test_a.py", "test_new.py", "test_renamed.py", "test-z.sh"]
        for name in names[:-1]:
            self.python_fixture(name)
        self.shell_fixture(names[-1])
        self.weights.write_text("DEFAULT_WEIGHT 2.5\ntest_a.py 20\ntest_removed.py 9999\n")
        for shard in range(2):
            result = self.run_runner("--smoke-shard", shard)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        calls = self.observed_calls()
        self.assertEqual(sorted(calls), sorted(names))
        self.assertEqual(len(calls), len(set(calls)))

    def test_weighted_shards_still_collect_every_failure_and_later_suite(self):
        names = [f"test_{index}.py" for index in range(8)]
        for index, name in enumerate(names):
            self.python_fixture(name, 7 if index in (1, 3) else 0)
        self.weights.write_text("DEFAULT_WEIGHT 1\n" + "".join(
            f"{name} {100 if index % 2 else 1}\n" for index, name in enumerate(names)))
        for shard in range(2):
            result = self.run_runner("--smoke-shard", shard)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("Summary: 3/4 passed", result.stdout)
            self.assertIn("Failures: 1", result.stdout)
        self.assertEqual(sorted(self.observed_calls()), names)

    def test_failed_selector_cannot_admit_a_partial_passing_shard(self):
        passing = self.python_fixture("test_a.py")
        self.python_fixture("test_b.py", 7)
        self.selector.write_text(f"print({str(passing)!r})\nraise SystemExit(23)\n")
        result = self.run_runner("--smoke-shard", 0)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Failed to partition the complete smoke test inventory", result.stderr)
        self.assertNotIn("Summary:", result.stdout)
        self.assertEqual(self.observed_calls(), [])

    def test_missing_or_invalid_weights_reject_before_any_execution(self):
        self.python_fixture("test_first.py")
        cases = [None, "", "test_first.py 1\n", "DEFAULT_WEIGHT 1\nDEFAULT_WEIGHT 2\n",
                 "DEFAULT_WEIGHT 1\ntest_first.py 1\ntest_first.py 2\n",
                 "DEFAULT_WEIGHT 1\n../test_first.py 1\n", "DEFAULT_WEIGHT 1\ninvalid row extra\n"]
        cases.extend(f"DEFAULT_WEIGHT {value}\n" for value in ("nan", "Infinity", "0", "-1", "bad"))
        cases.extend(f"DEFAULT_WEIGHT 1\ntest_first.py {value}\n"
                     for value in ("nan", "Infinity", "0", "-1", "bad"))
        for contents in cases:
            with self.subTest(contents=contents):
                if contents is None:
                    self.weights.unlink()
                else:
                    self.weights.write_text(contents)
                result = self.run_runner("--smoke-shard", 0)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Failed to partition the complete smoke test inventory", result.stderr)
                self.assertNotIn("Summary:", result.stdout)
                self.assertEqual(self.observed_calls(), [])

    def test_unsharded_and_explicit_modes_need_no_selector_or_weights(self):
        self.python_fixture("test_first_docs.py")
        self.weights.unlink()
        self.doc_weights.unlink()
        self.selector.unlink()
        for arguments in ((), ("test_first_docs.py",)):
            with self.subTest(arguments=arguments):
                result = self.run_runner(*arguments)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn("Summary: 1/1 passed", result.stdout)

    def test_each_sharded_mode_only_consumes_its_own_hints(self):
        self.python_fixture("test_first_docs.py")
        self.weights.write_text("invalid smoke hints\n")
        docs = self.run_runner("--docs-shard", 0)
        self.assertEqual(docs.returncode, 0, docs.stdout + docs.stderr)
        self.weights.write_text("DEFAULT_WEIGHT 1\n")
        self.doc_weights.write_text("invalid docs hints\n")
        smoke = self.run_runner("--smoke-shard", 0)
        self.assertEqual(smoke.returncode, 0, smoke.stdout + smoke.stderr)

    def test_doc_weights_separate_long_suites_without_changing_commands(self):
        names = [f"test_{index:02d}_docs.py" for index in range(16)]
        seconds = {name: 100 if index % 4 == 1 else 1 for index, name in enumerate(names)}
        for index, name in enumerate(names):
            # Each shard must retain an adverse verdict and all later suites.
            self.python_fixture(name, 7 if index % 4 == 1 else 0)
        self.doc_weights.write_text("DEFAULT_WEIGHT 1\n" + "".join(
            f"{name} {value}\n" for name, value in seconds.items()))
        seen = []
        for shard in range(4):
            before = len(self.observed_calls())
            result = self.run_runner("--docs-shard", shard)
            calls = self.observed_calls()[before:]
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(calls, sorted(calls))
            self.assertEqual(sum(seconds[name] for name in calls), 103)
            self.assertIn("Summary: 3/4 passed", result.stdout)
            self.assertIn("Failures: 1", result.stdout)
            seen.extend(calls)
        self.assertEqual(sorted(seen), names)
        self.assertEqual(len(seen), len(set(seen)))

    def test_doc_hints_cannot_add_removed_or_omit_new_and_legacy_suites(self):
        names = ["test_a_docs.py", "test_new_docs.py", "test_renamed_docs.py",
                 "test_vm_disk_diagnostics.py", "test_troubleshooting_logs.py"]
        for name in names:
            self.python_fixture(name)
        names.append("test-retired-trial-acquisition-docs.sh")
        self.shell_fixture(names[-1])
        self.doc_weights.write_text("DEFAULT_WEIGHT 2.5\ntest_a_docs.py 20\ntest_removed_docs.py 9999\n")
        for shard in range(4):
            result = self.run_runner("--docs-shard", shard)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        calls = self.observed_calls()
        self.assertEqual(sorted(calls), sorted(names))
        self.assertEqual(len(calls), len(set(calls)))

    def test_failed_doc_selector_cannot_admit_a_partial_passing_shard(self):
        passing = self.python_fixture("test_a_docs.py")
        self.python_fixture("test_b_docs.py", 7)
        self.selector.write_text(f"print({str(passing)!r})\nraise SystemExit(23)\n")
        result = self.run_runner("--docs-shard", 0)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Failed to partition the complete documentation test inventory", result.stderr)
        self.assertNotIn("Summary:", result.stdout)
        self.assertEqual(self.observed_calls(), [])

    def test_missing_or_invalid_doc_hints_reject_before_any_execution(self):
        self.python_fixture("test_first_docs.py")
        cases = [None, "", "test_first_docs.py 1\n", "DEFAULT_WEIGHT 1\nDEFAULT_WEIGHT 2\n",
                 "DEFAULT_WEIGHT 1\ntest_first_docs.py 1\ntest_first_docs.py 2\n",
                 "DEFAULT_WEIGHT 1\n../test_first_docs.py 1\n", "DEFAULT_WEIGHT 1\ninvalid row extra\n"]
        cases.extend(f"DEFAULT_WEIGHT {value}\n" for value in ("nan", "Infinity", "0", "-1", "bad"))
        cases.extend(f"DEFAULT_WEIGHT 1\ntest_first_docs.py {value}\n"
                     for value in ("nan", "Infinity", "0", "-1", "bad"))
        for contents in cases:
            with self.subTest(contents=contents):
                if contents is None:
                    self.doc_weights.unlink()
                else:
                    self.doc_weights.write_text(contents)
                result = self.run_runner("--docs-shard", 0)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Failed to partition the complete documentation test inventory", result.stderr)
                self.assertNotIn("Summary:", result.stdout)
                self.assertEqual(self.observed_calls(), [])

    def test_checked_in_hints_partition_complete_real_inventory_once(self):
        tests = sorted(str(path) for path in (ROOT / "scripts/tests").iterdir()
                       if path.is_file() and (path.match("test-*.sh") or path.match("test_*.py")))
        seen = []
        for shard in range(2):
            result = subprocess.run(
                ["python3", str(ROOT / "scripts/select-smoke-test-shard.py"),
                 str(ROOT / ".github/scripts/smoke-test-seconds.txt"), "2", str(shard)],
                input="\n".join(tests) + "\n", capture_output=True, text=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            selected = result.stdout.splitlines()
            self.assertTrue(selected)
            self.assertEqual(selected, sorted(selected))
            seen.extend(selected)
        self.assertEqual(sorted(seen), tests)
        self.assertEqual(len(seen), len(set(seen)))

    def test_selector_rejects_invalid_or_duplicate_inventory_and_shard_numbers(self):
        for count, index, inventory in [(2, 0, "test_a.py\ntest_a.py\n"),
                                        (2, 0, "not_a_test.py\n"),
                                        (2, 0, "test_a.py\n\ntest_b.py\n"),
                                        (0, 0, "test_a.py\n"),
                                        (2, 2, "test_a.py\n"),
                                        (2, -1, "test_a.py\n")]:
            with self.subTest(count=count, index=index, inventory=inventory):
                result = subprocess.run(
                    ["python3", str(self.selector), str(self.weights), str(count), str(index)],
                    input=inventory, capture_output=True, text=True, timeout=15)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")


if __name__ == "__main__":
    unittest.main()
