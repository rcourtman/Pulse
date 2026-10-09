#!/usr/bin/env python3
"""Exercise the actual inventory partition and fail-closed Go invocations."""

import importlib.util
from pathlib import Path
import re
import subprocess
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location(
    "runner", ROOT / "scripts/installtests/run_unix_contracts.py")
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class NativeInstallerRunnerTest(unittest.TestCase):
    inventory = "TestInstaller\nTestRootInstallResetSafety\nExample\nFuzzInput\nTestFuture\nTestRootInstallResetFuture\nok\tpackage\n"
    expected_groups = (
        ["Example", "TestFuture"],
        ["FuzzInput", "TestInstaller"],
        ["TestRootInstallResetFuture", "TestRootInstallResetSafety"],
    )

    def test_every_discovered_check_runs_once_in_one_group(self):
        groups = runner.partition(self.inventory)
        self.assertEqual(runner.GROUPS, ("general-0", "general-1", "reset"))
        self.assertEqual(groups, self.expected_groups)
        selected = [name for group in groups for name in group]
        self.assertEqual(len(selected), 6)
        self.assertEqual(len(set(selected)), 6)

    def test_partition_order_and_odd_inventory_are_exhaustive(self):
        reversed_inventory = "\n".join(reversed(self.inventory.splitlines()))
        self.assertEqual(runner.partition(reversed_inventory), self.expected_groups)
        self.assertEqual(runner.partition(self.inventory + "TestZ\n"), (
            ["Example", "TestFuture", "TestZ"],
            self.expected_groups[1],
            self.expected_groups[2],
        ))

    def test_missing_inventory_is_not_a_passing_empty_run(self):
        for inventory in ("", "ok\tpackage\n", "TestInstaller\n", "TestRootInstallResetSafety\n",
                          "TestInstaller\nTestRootInstallResetSafety\n"):
            with self.subTest(inventory=inventory), self.assertRaises(ValueError):
                runner.partition(inventory)

    def test_repeated_or_invalid_identity_stops(self):
        for name in ("TestInstaller", "TestX|.*", "Test(name)", "TestX --skip TestY"):
            with self.subTest(name=name), self.assertRaises(ValueError):
                runner.partition(self.inventory + name + "\n")

    def test_unicode_go_identifiers_remain_included(self):
        groups = runner.partition(self.inventory + "TestΣ\n")
        self.assertIn("TestΣ", [name for group in groups[:2] for name in group])

    def run_main(self, fault=None):
        calls = []

        def run(command, **kwargs):
            calls.append((command, kwargs))
            if fault == len(calls):
                raise subprocess.CalledProcessError(23, command)
            return subprocess.CompletedProcess(command, 0, stdout=self.inventory)

        with mock.patch.object(runner.subprocess, "run", side_effect=run), mock.patch("builtins.print"):
            if fault:
                with self.assertRaises(subprocess.CalledProcessError) as raised:
                    runner.main()
                self.assertEqual(raised.exception.returncode, 23)
            else:
                runner.main()
        return calls

    def test_actual_commands_preserve_deadlines_count_and_complete_selection(self):
        calls = self.run_main()
        self.assertEqual(calls[0][0], ["go", "test", "-list", ".", runner.PACKAGE])
        self.assertTrue(calls[0][1]["capture_output"])
        self.assertTrue(calls[0][1]["text"])
        # Check the count before zip: a missing final reset invocation must not
        # silently truncate the comparison and become a passing control.
        self.assertEqual(len(calls), 4)
        for (command, kwargs), names in zip(calls[1:], self.expected_groups):
            self.assertEqual(command, ["go", "test", "-count=1", "-timeout", "10m", "-run", "^(" + "|".join(names) + ")$", runner.PACKAGE])
            self.assertTrue(kwargs["check"])
            self.assertEqual(kwargs["cwd"], ROOT)

    def test_command_selectors_do_not_broaden_the_inventory(self):
        calls = self.run_main()
        self.assertEqual(len(calls), 4)
        patterns = [re.compile(command[6]) for command, _ in calls[1:]]
        for name in (name for group in self.expected_groups for name in group):
            with self.subTest(name=name):
                self.assertEqual(sum(bool(pattern.fullmatch(name)) for pattern in patterns), 1)
        for name in ("TestInstaller/child", "TestInstallerExtra", "TestInjected", "BenchmarkIgnored"):
            with self.subTest(name=name):
                self.assertFalse(any(pattern.fullmatch(name) for pattern in patterns))

    def test_inventory_failure_does_not_run_any_group(self):
        self.assertEqual(len(self.run_main(1)), 1)

    def test_first_general_failure_cannot_be_hidden_by_later_groups(self):
        self.assertEqual(len(self.run_main(2)), 2)

    def test_second_general_failure_remains_terminal(self):
        self.assertEqual(len(self.run_main(3)), 3)

    def test_reset_failure_remains_terminal(self):
        self.assertEqual(len(self.run_main(4)), 4)


if __name__ == "__main__":
    unittest.main()
