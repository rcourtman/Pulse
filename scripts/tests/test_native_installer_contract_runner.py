#!/usr/bin/env python3
"""Exercise the actual inventory partition and fail-closed Go invocations."""

import importlib.util
from pathlib import Path
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

    def test_every_discovered_check_runs_once_in_one_group(self):
        first, second, reset = runner.partition(self.inventory)
        self.assertEqual(first, ["Example", "TestFuture"])
        self.assertEqual(second, ["FuzzInput", "TestInstaller"])
        self.assertEqual(reset, ["TestRootInstallResetFuture", "TestRootInstallResetSafety"])
        selected = first + second + reset
        self.assertEqual(len(selected), 6)
        self.assertEqual(len(set(selected)), 6)
        self.assertEqual(runner.GROUPS, ("general-0", "general-1", "reset"))

    def test_groups_are_deterministic_for_reordered_inventory(self):
        self.assertEqual(
            runner.partition(self.inventory),
            runner.partition("\n".join(reversed(self.inventory.splitlines()))),
        )

    def test_missing_inventory_is_not_a_passing_empty_run(self):
        for inventory in ("", "ok\tpackage\n", "TestInstaller\n", "TestRootInstallResetSafety\n", "TestInstaller\nTestRootInstallResetSafety\n"):
            with self.subTest(inventory=inventory), self.assertRaises(ValueError):
                runner.partition(inventory)

    def test_repeated_or_invalid_identity_stops(self):
        for name in ("TestInstaller", "TestX|.*", "Test(name)", "TestX --skip TestY"):
            with self.subTest(name=name), self.assertRaises(ValueError):
                runner.partition(self.inventory + name + "\n")

    def test_unicode_go_identifiers_remain_included(self):
        first, second, _ = runner.partition(self.inventory + "TestΣ\n")
        self.assertIn("TestΣ", first + second)

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
        groups = runner.partition(self.inventory)
        self.assertEqual(len(calls), 4)
        for (command, kwargs), names in zip(calls[1:], groups):
            self.assertEqual(command, ["go", "test", "-count=1", "-timeout", "10m", "-run", "^(" + "|".join(names) + ")$", runner.PACKAGE])
            self.assertTrue(kwargs["check"])
            self.assertEqual(kwargs["cwd"], ROOT)

    def test_inventory_failure_does_not_run_any_group(self):
        self.assertEqual(len(self.run_main(1)), 1)

    def test_first_failure_cannot_be_hidden_by_second_group(self):
        self.assertEqual(len(self.run_main(2)), 2)

    def test_second_group_failure_remains_terminal(self):
        self.assertEqual(len(self.run_main(3)), 3)

    def test_reset_group_failure_remains_terminal(self):
        self.assertEqual(len(self.run_main(4)), 4)


if __name__ == "__main__":
    unittest.main()
