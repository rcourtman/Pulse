#!/usr/bin/env python3
"""Exercise inventory admission, workflow phases and real compiled Go exits."""

import contextlib
import importlib.util
import io
from pathlib import Path
import re
import subprocess
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location(
    "runner", ROOT / "scripts/installtests/run_unix_contracts.py")
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class NativeInstallerRunnerTest(unittest.TestCase):
    names = ["TestInstaller", "TestRootInstallResetSafety", "Example", "FuzzInput",
             "TestFuture", "TestRootInstallResetFuture", *runner.PREFLIGHT, *runner.FIXTURES]
    inventory = "\n".join(names + ["BenchmarkIgnored", "ok\tpackage"])

    def test_every_discovered_check_runs_once_and_reset_stays_last(self):
        early, remaining = runner.partition(self.inventory)
        self.assertEqual(early, list(runner.PREFLIGHT))
        self.assertEqual(remaining, ["TestInstaller", "Example", "FuzzInput", "TestFuture",
                                     *runner.FIXTURES, "TestRootInstallResetSafety",
                                     "TestRootInstallResetFuture"])
        self.assertEqual(sorted(early + remaining), sorted(self.names))
        self.assertEqual(len(early + remaining), len(set(early + remaining)))

    def test_compiled_order_is_preserved_within_each_inventory(self):
        for names in (self.names, list(reversed(self.names)), self.names + ["TestZ", "TestΣ"]):
            with self.subTest(names=names):
                early, remaining = runner.partition("\n".join(names))
                self.assertEqual(early, [name for name in names if name in runner.PREFLIGHT])
                self.assertEqual(remaining,
                                 [name for name in names if name not in runner.PREFLIGHT
                                  and not name.startswith("TestRootInstallReset")]
                                 + [name for name in names if name.startswith("TestRootInstallReset")])

    def test_growing_inventory_never_shares_a_check_deadline(self):
        for count in (196, 198, 500, 1100):
            names = [f"TestFixture{i:04d}" for i in range(count)] + self.names
            with self.subTest(count=count):
                _, checks = self.run_main(inventory="\n".join(names))
                self.assertEqual(len(checks), len(names))
                self.assertEqual(sorted(name for _, name, _ in checks), sorted(names))

    def test_missing_inventory_or_mandatory_matrix_stops(self):
        incomplete = ("", "ok\tpackage\n", "TestInstaller", "TestRootInstallResetSafety",
                      "TestInstaller\nTestRootInstallResetSafety",
                      "\n".join([*runner.PREFLIGHT, *runner.FIXTURES, "TestRootInstallResetSafety"]),
                      *(self.inventory.replace(name, "") for name in runner.PREFLIGHT + runner.FIXTURES),
                      "\n".join(name for name in self.names if not name.startswith("TestRootInstallReset")))
        for inventory in incomplete:
            for phase in ("all", "preflight", "remaining"):
                with self.subTest(inventory=inventory, phase=phase):
                    with mock.patch.object(runner.subprocess, "run", return_value=
                                           subprocess.CompletedProcess([], 0, stdout=inventory)) as run, \
                            mock.patch.object(runner, "run_check") as check:
                        with self.assertRaises(ValueError):
                            runner.main(phase)
                        self.assertEqual(run.call_count, 2)  # compile and discover, never execute
                        check.assert_not_called()

    def test_repeated_or_invalid_identity_stops(self):
        for name in ("TestInstaller", "TestX|.*", "Test(name)", "TestX --skip TestY"):
            with self.subTest(name=name), self.assertRaises(ValueError):
                runner.partition(self.inventory + "\n" + name)

    def run_main(self, fault=None, phase="all", inventory=None):
        commands, checks = [], []

        def run(command, **kwargs):
            commands.append((command, kwargs))
            if fault == len(commands):
                raise subprocess.CalledProcessError(23, command)
            return subprocess.CompletedProcess(command, 0, stdout=self.inventory if inventory is None else inventory)

        def check(binary, name, **kwargs):
            checks.append((binary, name, kwargs))
            if fault == len(commands) + len(checks):
                raise subprocess.CalledProcessError(23, [str(binary), name])

        with mock.patch.object(runner.subprocess, "run", side_effect=run), \
                mock.patch.object(runner, "run_check", side_effect=check), \
                contextlib.redirect_stdout(io.StringIO()):
            if fault:
                with self.assertRaises(subprocess.CalledProcessError) as raised:
                    runner.main(phase)
                self.assertEqual(raised.exception.returncode, 23)
            else:
                runner.main(phase)
        if commands:
            # TemporaryDirectory owns even unsuccessful compilation/discovery cleanup.
            self.assertFalse(Path(commands[0][0][4]).parent.exists())
        return commands, checks

    def test_compile_once_and_discover_from_the_same_binary(self):
        commands, checks = self.run_main()
        self.assertEqual(len(commands), 2)
        compile_command, compile_options = commands[0]
        binary = compile_command[4]
        self.assertEqual(compile_command, ["go", "test", "-c", "-o", binary, runner.PACKAGE])
        self.assertEqual(compile_options, {"cwd": ROOT, "check": True})
        self.assertEqual(commands[1], ([binary, "-test.list=."], {
            "cwd": ROOT / "scripts/installtests", "check": True, "capture_output": True, "text": True}))
        self.assertEqual(len(checks), len(self.names))
        for actual_binary, _, options in checks:
            self.assertEqual(str(actual_binary), binary)
            self.assertEqual(options, {"cwd": ROOT / "scripts/installtests"})

    def test_each_phased_failure_stops_without_retry(self):
        early, remaining = runner.partition(self.inventory)
        for phase, names in (("preflight", early), ("remaining", remaining), ("all", early + remaining)):
            for fault in range(1, len(names) + 3):
                with self.subTest(phase=phase, fault=fault):
                    commands, checks = self.run_main(fault, phase)
                    self.assertEqual(len(commands) + len(checks), fault)
                    self.assertEqual([name for _, name, _ in checks], names[:max(0, fault - 2)])

    def test_workflow_phases_execute_the_same_inventory_once(self):
        _, early = self.run_main(phase="preflight")
        _, remaining = self.run_main(phase="remaining")
        _, all_checks = self.run_main()
        selected = lambda checks: [name for _, name, _ in checks]
        self.assertEqual(selected(early + remaining), selected(all_checks))
        self.assertEqual(selected(early), list(runner.PREFLIGHT))

    def test_actual_commands_preserve_deadlines_and_exact_root_selection(self):
        patterns = []
        for name in self.names + ["TestΣ"]:
            process = mock.MagicMock()
            process.stdout = ["--- PASS: " + name + " (0.00s)\n"]
            process.wait.return_value = 0
            with mock.patch.object(runner.subprocess, "Popen") as popen, \
                    contextlib.redirect_stdout(io.StringIO()):
                popen.return_value.__enter__.return_value = process
                runner.run_check(Path("/fixture/installtests.test"), name, cwd=ROOT)
            command = popen.call_args.args[0]
            self.assertEqual(command, ["/fixture/installtests.test", "-test.count=1", "-test.timeout=10m",
                                       "-test.paniconexit0", "-test.v", "-test.run=^" + name + "$"])
            self.assertEqual(popen.call_args.kwargs, {"cwd": ROOT, "stdout": subprocess.PIPE,
                                                     "stderr": subprocess.STDOUT, "text": True})
            process.wait.assert_called_once_with()
            patterns.append(re.compile(command[-1].split("=", 1)[1]))
        for name in self.names + ["TestΣ"]:
            self.assertEqual(sum(bool(pattern.fullmatch(name)) for pattern in patterns), 1)
        for name in ("TestInstaller/child", "TestInstallerExtra", "TestInjected", "BenchmarkIgnored"):
            self.assertFalse(any(pattern.fullmatch(name) for pattern in patterns))

    def test_foreign_or_subtest_completion_cannot_credit_the_selected_root(self):
        for line in ("--- PASS: TestInstaller/child (0.00s)\n", "--- PASS: TestInstallerExtra (0.00s)\n",
                     "--- SKIP: TestOther (0.00s)\n", "PASS\n", ""):
            process = mock.MagicMock()
            process.stdout, process.wait.return_value = [line], 0
            with mock.patch.object(runner.subprocess, "Popen") as popen, \
                    contextlib.redirect_stdout(io.StringIO()), self.assertRaises(RuntimeError):
                popen.return_value.__enter__.return_value = process
                runner.run_check(Path("/fixture/installtests.test"), "TestInstaller", cwd=ROOT)

    def test_unknown_phase_cannot_start_go_or_hide_checks(self):
        with mock.patch.object(runner.subprocess, "run") as run:
            with self.assertRaises(ValueError):
                runner.main("skip")
            run.assert_not_called()
        result = subprocess.run(["python3", str(ROOT / "scripts/installtests/run_unix_contracts.py"),
                                 "--phase", "skip"], capture_output=True, text=True)
        self.assertEqual(result.returncode, 2)
        self.assertIn("invalid choice", result.stderr)


# Adapted from Richard's 79a35fbe2e real-execution controls. Compilation is
# deliberately real: mocks cannot prove Go's timeout, skip or os.Exit handling.
class CompiledGoControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.directory = tempfile.TemporaryDirectory(prefix="native-runner-control-")
        cls.cwd = Path(cls.directory.name)
        cls.binary = cls.cwd / "controls.test"
        (cls.cwd / "go.mod").write_text("module native.runner.controls\n\ngo 1.24\n")
        (cls.cwd / "controls_test.go").write_text('''package controls
import ("fmt"; "os"; "testing"; "time")
func TestPass(t *testing.T) {}
func TestSkip(t *testing.T) { t.Skip("explicit platform skip") }
func TestFail(t *testing.T) { t.Fatal("real failure") }
func TestExitZero(t *testing.T) { os.Exit(0) }
func TestTimeout(t *testing.T) { time.Sleep(time.Hour) }
func FuzzSeeds(f *testing.F) { f.Add("seed"); f.Fuzz(func(t *testing.T, s string) {}) }
func Example_output() { fmt.Println("installer"); /* Output: installer */ }
''')
        subprocess.run(["go", "test", "-c", "-o", str(cls.binary), "."], cwd=cls.cwd, check=True, timeout=90)

    @classmethod
    def tearDownClass(cls):
        cls.directory.cleanup()

    def test_real_test_skip_example_and_fuzz_seed_complete(self):
        for name in ("TestPass", "TestSkip", "Example_output", "FuzzSeeds"):
            with self.subTest(name=name), contextlib.redirect_stdout(io.StringIO()):
                runner.run_check(self.binary, name, cwd=self.cwd)

    def test_failure_exit_zero_and_timeout_cannot_pass(self):
        for name in ("TestFail", "TestExitZero", "TestTimeout"):
            with self.subTest(name=name), contextlib.redirect_stdout(io.StringIO()), \
                    self.assertRaises(subprocess.CalledProcessError):
                runner.run_check(self.binary, name, cwd=self.cwd, timeout="100ms")

    def test_no_matching_test_cannot_pass(self):
        with contextlib.redirect_stdout(io.StringIO()), self.assertRaises(RuntimeError):
            runner.run_check(self.binary, "TestMissing", cwd=self.cwd)


if __name__ == "__main__":
    unittest.main(verbosity=2)
