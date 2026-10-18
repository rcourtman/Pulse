#!/usr/bin/env python3
"""Execution controls for the native installer runner, including real Go exits."""
import contextlib
import importlib.util
import io
from pathlib import Path
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("native_runner", ROOT / "scripts/installtests/run_unix_contracts.py")
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class InventoryControls(unittest.TestCase):
    def setUp(self):
        self.names = [f"TestFixture{i:03d}" for i in range(500)]
        self.names += ["ExampleInstaller", "FuzzInstaller", "TestFutureFixture",
                       "TestRootInstallResetSafety", "TestΣ", *runner.PREFLIGHT]
        self.inventory = "\n".join(self.names + ["BenchmarkIgnored", "ok package 0.1s"])

    def test_growing_inventory_is_exhaustive_disjoint_and_ordered(self):
        early, remaining = runner.partition(self.inventory)
        self.assertEqual(early, list(runner.PREFLIGHT))
        self.assertEqual(remaining, [name for name in self.names if name not in runner.PREFLIGHT])
        self.assertEqual(sorted(early + remaining), sorted(self.names))
        for invalid in ["", "TestOnly", "TestOne\nTestTwo", self.inventory + "\nTestFixture000",
                        self.inventory + "\nTestUnsafe|TestInjected", self.inventory + "\nFuzzBad.*",
                        self.inventory + "\nTest(name)", self.inventory + "\nTestX --skip TestY",
                        self.inventory.replace("TestRootInstallResetSafety\n", ""),
                        "\n".join(self.names[:-1])]:
            with self.subTest(invalid=invalid[-60:]), self.assertRaises(ValueError):
                runner.partition(invalid)

    def test_discovery_order_survives_reversed_and_odd_inventories(self):
        for names in (list(reversed(self.names)), self.names + ["TestFutureOdd"]):
            with self.subTest(last=names[-1]):
                early, remaining = runner.partition("\n".join(names))
                self.assertEqual(early, [name for name in names if name in runner.PREFLIGHT])
                self.assertEqual(remaining, [name for name in names if name not in runner.PREFLIGHT])

    def execute(self, phase, failure_at=None):
        calls, checks = [], []
        def command(argv, **kwargs):
            calls.append(argv)
            self.assertTrue(kwargs["check"])
            self.assertEqual(kwargs["cwd"], runner.ROOT if len(calls) == 1 else runner.ROOT / "scripts/installtests")
            if len(calls) == failure_at:
                raise subprocess.CalledProcessError(23, argv)
            return SimpleNamespace(stdout=self.inventory)
        def check(binary, name, **kwargs):
            checks.append(name)
            self.assertEqual(kwargs["cwd"], runner.ROOT / "scripts/installtests")
            if len(checks) + 2 == failure_at:
                raise subprocess.CalledProcessError(23, [str(binary), name])
        with patch.object(runner.subprocess, "run", side_effect=command), patch.object(runner, "run_check", side_effect=check), contextlib.redirect_stdout(io.StringIO()):
            if failure_at:
                with self.assertRaises(subprocess.CalledProcessError):
                    runner.main(phase)
            else:
                runner.main(phase)
        if failure_at:
            self.assertEqual(len(calls) + len(checks), failure_at)
        self.assertEqual(calls[0][:4], ["go", "test", "-c", "-o"])
        if len(calls) > 1:
            self.assertEqual(Path(calls[0][4]).name, "installtests.test")
            self.assertEqual(calls[1], [calls[0][4], "-test.list=."])
            self.assertEqual(calls[0][-1], runner.PACKAGE)
        return checks

    def test_workflow_phases_execute_every_check_once_and_propagate_failure(self):
        all_checks = self.execute("all")
        self.assertEqual(self.execute("preflight") + self.execute("remaining"), all_checks)
        self.assertEqual(len(all_checks), len(set(all_checks)))
        self.assertEqual(set(all_checks), set(self.names))
        # Exercise every possible failure position, including the final reset,
        # without replaying 500 mocked checks for each injected failure.
        self.names = ["TestOne", "ExampleInstaller", "FuzzInstaller", "TestRootInstallResetSafety", *runner.PREFLIGHT]
        self.inventory = "\n".join(self.names)
        for phase in ("all", "preflight", "remaining"):
            expected = self.execute(phase)
            for failure in range(1, len(expected) + 3):
                with self.subTest(phase=phase, failure=failure):
                    attempted = self.execute(phase, failure_at=failure)
                    self.assertEqual(attempted, expected[:max(0, failure - 2)])

    def test_unknown_phase_starts_no_go_process(self):
        with patch.object(runner.subprocess, "run") as run:
            with self.assertRaises(ValueError):
                runner.main("skip")
            run.assert_not_called()
        result = subprocess.run(["python3", "-I", "-S", str(ROOT / "scripts/installtests/run_unix_contracts.py"),
                                 "--phase", "skip"], capture_output=True, text=True)
        self.assertEqual(result.returncode, 2)
        self.assertIn("invalid choice", result.stderr)


class CompiledGoControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.directory = tempfile.TemporaryDirectory(prefix="native-runner-control-")
        cls.cwd = Path(cls.directory.name)
        cls.binary = cls.cwd / "controls.test"
        (cls.cwd / "go.mod").write_text("module native.runner.controls\n\ngo 1.24\n")
        (cls.cwd / "controls_test.go").write_text('''package controls
import ("fmt"; "os"; "testing"; "time")
func TestPass(t *testing.T) { t.Run("child", func(t *testing.T) {}) }
func TestPassExtra(t *testing.T) { t.Fatal("selector must stay anchored") }
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

    def test_actual_command_retains_deadline_count_and_exact_selection(self):
        with patch.object(runner.subprocess, "Popen", wraps=subprocess.Popen) as popen, contextlib.redirect_stdout(io.StringIO()):
            runner.run_check(self.binary, "TestPass", cwd=self.cwd)
        self.assertEqual(popen.call_args.args[0], [str(self.binary), "-test.count=1", "-test.timeout=10m",
                                                  "-test.paniconexit0", "-test.v", "-test.run=^TestPass$"])
        self.assertEqual(popen.call_args.kwargs["cwd"], self.cwd)

    def test_failure_exit_zero_and_timeout_cannot_pass(self):
        for name in ("TestFail", "TestExitZero", "TestTimeout"):
            with self.subTest(name=name), contextlib.redirect_stdout(io.StringIO()), self.assertRaises(subprocess.CalledProcessError):
                runner.run_check(self.binary, name, cwd=self.cwd, timeout="100ms")

    def test_no_matching_test_cannot_pass(self):
        with contextlib.redirect_stdout(io.StringIO()), self.assertRaises(RuntimeError):
            runner.run_check(self.binary, "TestMissing", cwd=self.cwd)


if __name__ == "__main__":
    unittest.main(verbosity=2)
