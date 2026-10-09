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
        general, reset = runner.partition(self.inventory)
        self.assertEqual(general, ["TestInstaller", "Example", "FuzzInput", "TestFuture"])
        self.assertEqual(reset, ["TestRootInstallResetSafety", "TestRootInstallResetFuture"])
        self.assertEqual(len(set(general + reset)), 6)

    def test_missing_inventory_is_not_a_passing_empty_run(self):
        for inventory in ("", "ok\tpackage\n", "TestInstaller\n", "TestRootInstallResetSafety\n"):
            with self.subTest(inventory=inventory), self.assertRaises(ValueError):
                runner.partition(inventory)

    def test_repeated_or_invalid_identity_stops(self):
        for name in ("TestInstaller", "TestX|.*", "Test(name)", "TestX --skip TestY"):
            with self.subTest(name=name), self.assertRaises(ValueError):
                runner.partition(self.inventory + name + "\n")

    def test_unicode_go_identifiers_remain_included(self):
        general, _ = runner.partition(self.inventory + "TestΣ\n")
        self.assertIn("TestΣ", general)

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
        general, reset = runner.partition(self.inventory)
        for (command, kwargs), names in zip(calls[1:], (general, reset)):
            self.assertEqual(command, ["go", "test", "-count=1", "-timeout", "10m", "-run", "^(" + "|".join(names) + ")$", runner.PACKAGE])
            self.assertTrue(kwargs["check"])
            self.assertEqual(kwargs["cwd"], ROOT)
        self.assertEqual(len(calls), 3)

    def test_inventory_failure_does_not_run_any_group(self):
        self.assertEqual(len(self.run_main(1)), 1)

    def test_first_failure_cannot_be_hidden_by_second_group(self):
        self.assertEqual(len(self.run_main(2)), 2)

    def test_second_group_failure_remains_terminal(self):
        self.assertEqual(len(self.run_main(3)), 3)


if __name__ == "__main__":
    unittest.main()
