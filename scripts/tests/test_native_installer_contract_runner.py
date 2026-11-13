#!/usr/bin/env python3
"""Execution controls for the native installer runner, including real Go exits."""
import contextlib
import importlib.util
import io
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import threading
import time
from types import SimpleNamespace
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("native_runner", ROOT / "scripts/installtests/run_unix_contracts.py")
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class InventoryControls(unittest.TestCase):
    def test_process_deadline_preserves_go_alarm_and_rejects_unbounded_input(self):
        for duration, seconds in (("10m", 600), ("100ms", .1), ("1m2.5s", 62.5), ("1h", 3600)):
            self.assertEqual(runner.process_deadline(duration), seconds + runner.PROCESS_EXIT_GRACE_SECONDS)
        for duration in ("0", "0s", "-1s", "inf", "nan", "1s trailing", "1", "1m-2s"):
            with self.subTest(duration=duration), patch.object(runner.subprocess, "Popen") as popen:
                with self.assertRaises(ValueError):
                    runner.run_check("binary", "TestOne", cwd="cwd", timeout=duration)
                popen.assert_not_called()

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
        def selected(binary, names, **kwargs):
            for name in names:
                check(binary, name, **kwargs)
        with patch.object(runner.subprocess, "run", side_effect=command), patch.object(runner, "run_checks", side_effect=selected), contextlib.redirect_stdout(io.StringIO()):
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

    def test_all_finishes_preflight_before_admitting_remaining_checks(self):
        with patch.object(runner.subprocess, "run", return_value=SimpleNamespace(stdout=self.inventory)), \
                patch.object(runner, "run_checks") as checks, contextlib.redirect_stdout(io.StringIO()):
            runner.main("all")
        self.assertEqual(checks.call_count, 2)
        self.assertEqual(checks.call_args_list[0].args[1], list(runner.PREFLIGHT))
        self.assertEqual(checks.call_args_list[1].args[1],
                         [name for name in self.names if name not in runner.PREFLIGHT])


class SchedulerControls(unittest.TestCase):
    def test_free_slot_refills_while_first_is_blocked_and_output_stays_ordered(self):
        barrier = threading.Barrier(2)
        tail_finished = threading.Event()
        lock = threading.Lock()
        active, peak, finished = set(), [0], set()
        names = ["TestFirst", "TestSecond", "TestThird", "TestFourth", "TestLast"]
        output = io.StringIO()
        def check(binary, name, *, cwd, output):
            self.assertEqual(binary, "exact-binary")
            self.assertEqual(cwd, "exact-cwd")
            with lock:
                active.add(name)
                peak[0] = max(peak[0], len(active))
                if name not in names[:2]:
                    self.assertIn("TestFirst", active)
                    self.assertIn(names[names.index(name) - 1], finished)
            if name in names[:2]:
                barrier.wait(timeout=5)
            if name == "TestFirst":
                self.assertTrue(tail_finished.wait(timeout=5), "free slot was not refilled")
            output.write(f"complete {name}\n")
            with lock:
                active.remove(name)
                finished.add(name)
            if name == "TestLast":
                tail_finished.set()
        with patch.object(runner, "run_check", side_effect=check), contextlib.redirect_stdout(output):
            runner.run_checks("exact-binary", names, cwd="exact-cwd")
        self.assertEqual(peak[0], 2)
        self.assertEqual(finished, set(names))
        self.assertEqual([line for line in output.getvalue().splitlines() if line.startswith("complete")],
                         [f"complete {name}" for name in names])

    def test_observed_failure_drains_sibling_and_starts_no_later_check(self):
        # Fail either position, including a fast second result while the first
        # is still active. Both outputs survive before the failure is raised.
        for failed in ("TestFirst", "TestSecond"):
            with self.subTest(failed=failed):
                barrier = threading.Barrier(2)
                failure_observed = threading.Event()
                names, finished = [], set()
                output, errors = io.StringIO(), io.StringIO()
                actual_wait = runner.wait
                def wait(*args, **kwargs):
                    result = actual_wait(*args, **kwargs)
                    if any(future.exception() for future in result[0]):
                        failure_observed.set()
                    return result
                def check(binary, name, *, cwd, output):
                    names.append(name)
                    barrier.wait(timeout=5)
                    if name != failed:
                        self.assertTrue(failure_observed.wait(timeout=5))
                    output.write(f"observed {name}\n")
                    finished.add(name)
                    if name == failed:
                        raise subprocess.CalledProcessError(23, [binary, name])
                with patch.object(runner, "run_check", side_effect=check), patch.object(runner, "wait", side_effect=wait), \
                        contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors), \
                        self.assertRaises(subprocess.CalledProcessError) as raised:
                    runner.run_checks("exact-binary", ["TestFirst", "TestSecond", "TestNever"], cwd="exact-cwd")
                self.assertEqual(raised.exception.returncode, 23)
                self.assertEqual(sorted(names), ["TestFirst", "TestSecond"])
                self.assertEqual(finished, set(names))
                self.assertIn("observed TestFirst\nobserved TestSecond\n", output.getvalue())
                self.assertIn("check failed: " + failed, errors.getvalue())
                self.assertNotIn("TestNever", output.getvalue() + errors.getvalue())

    def test_refilled_slot_failure_is_observed_before_earlier_output_can_finish(self):
        first_started, failure_observed = threading.Event(), threading.Event()
        started, finished = [], []
        output, errors = io.StringIO(), io.StringIO()
        actual_wait = runner.wait
        def wait(*args, **kwargs):
            result = actual_wait(*args, **kwargs)
            if any(future.exception() for future in result[0]):
                failure_observed.set()
            return result
        def check(binary, name, *, cwd, output):
            started.append(name)
            if name == "TestFirst":
                first_started.set()
                self.assertTrue(failure_observed.wait(timeout=5), "later failure was hidden by output order")
            else:
                self.assertTrue(first_started.wait(timeout=5))
            output.write(f"complete {name}\n")
            finished.append(name)
            if name == "TestRefilledFailure":
                raise subprocess.CalledProcessError(31, [binary, name])
        with patch.object(runner, "run_check", side_effect=check), patch.object(runner, "wait", side_effect=wait), \
                contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors), \
                self.assertRaises(subprocess.CalledProcessError) as raised:
            runner.run_checks("binary", ["TestFirst", "TestSecond", "TestRefilledFailure", "TestNever"], cwd="cwd")
        self.assertEqual(raised.exception.returncode, 31)
        self.assertEqual(set(started), {"TestFirst", "TestSecond", "TestRefilledFailure"})
        self.assertEqual(set(finished), set(started))
        self.assertEqual([line for line in output.getvalue().splitlines() if line.startswith("complete")],
                         ["complete TestFirst", "complete TestSecond", "complete TestRefilledFailure"])
        self.assertIn("check failed: TestRefilledFailure", errors.getvalue())
        self.assertNotIn("TestNever", output.getvalue() + errors.getvalue())

    def test_both_failures_and_complete_output_are_retained(self):
        output, errors = io.StringIO(), io.StringIO()
        def check(binary, name, *, cwd, output):
            output.write(name + "\n" + "fixture bytes Σ" * 10000 + "\n")
            if name == "TestFirst":
                raise RuntimeError("missing completion")
            raise subprocess.CalledProcessError(29, [binary, name])
        with patch.object(runner, "run_check", side_effect=check), \
                contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors), \
                self.assertRaisesRegex(RuntimeError, "missing completion"):
            runner.run_checks("binary", ["TestFirst", "TestSecond", "TestNever"], cwd="cwd")
        self.assertEqual(output.getvalue().count("fixture bytes Σ"), 20000)
        self.assertIn("check failed: TestFirst: missing completion", errors.getvalue())
        self.assertIn("check failed: TestSecond:", errors.getvalue())
        self.assertIn("29", errors.getvalue())

    def test_single_preflight_and_empty_selection_start_no_extra_check(self):
        for names in ([], list(runner.PREFLIGHT)):
            with self.subTest(names=names), patch.object(runner, "run_check") as check, \
                    contextlib.redirect_stdout(io.StringIO()):
                runner.run_checks("binary", names, cwd="cwd")
                self.assertEqual(check.call_count, len(names))
                if names:
                    self.assertEqual(check.call_args.args, ("binary", names[0]))


class CompiledGoControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.directory = tempfile.TemporaryDirectory(prefix="native-runner-control-")
        cls.cwd = Path(cls.directory.name)
        cls.binary = cls.cwd / "controls.test"
        (cls.cwd / "go.mod").write_text("module native.runner.controls\n\ngo 1.24\n")
        (cls.cwd / "controls_test.go").write_text('''package controls
import ("fmt"; "os"; "os/exec"; "strconv"; "testing"; "time")
func TestPass(t *testing.T) { t.Run("child", func(t *testing.T) {}) }
func TestPassExtra(t *testing.T) { t.Fatal("selector must stay anchored") }
func TestSkip(t *testing.T) { t.Skip("explicit platform skip") }
func TestFail(t *testing.T) { t.Fatal("real failure") }
func TestExitZero(t *testing.T) { os.Exit(0) }
func TestTimeout(t *testing.T) { time.Sleep(time.Hour) }
func outputDescendant(t *testing.T) {
    child := exec.Command(os.Args[0], "-test.run=^TestOutputDescendant$", "-test.timeout=30s")
    child.Env = append(os.Environ(), "NATIVE_RUNNER_OUTPUT_CHILD=1")
    child.Stdout, child.Stderr = os.Stdout, os.Stderr
    if err := child.Start(); err != nil { t.Fatal(err) }
    if err := os.WriteFile("child-pid", []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil { t.Fatal(err) }
    deadline := time.Now().Add(5 * time.Second)
    for {
        if _, err := os.Stat("child-ready"); err == nil { return }
        if time.Now().After(deadline) { t.Fatal("output child did not start") }
        time.Sleep(time.Millisecond)
    }
}
func TestOutputDescendant(t *testing.T) {
    if os.Getenv("NATIVE_RUNNER_OUTPUT_CHILD") != "1" { t.Skip("child fixture only") }
    fmt.Println("owned child output Σ")
    if err := os.WriteFile("child-ready", []byte("ready"), 0600); err != nil { t.Fatal(err) }
    time.Sleep(time.Hour)
}
func TestFailedWithOutputDescendant(t *testing.T) { outputDescendant(t); t.Fatal("original parent failure") }
func TestPassedWithOutputDescendant(t *testing.T) { outputDescendant(t) }
func TestTimeoutWithOutputDescendant(t *testing.T) { outputDescendant(t); time.Sleep(time.Hour) }
func TestDrain(t *testing.T) {
    deadline := time.Now().Add(5 * time.Second)
    for {
        if _, err := os.Stat("release-drain"); err == nil { return }
        if time.Now().After(deadline) { t.Fatal("sibling was not drained after observed failure") }
        time.Sleep(time.Millisecond)
    }
}
func TestConcurrentA(t *testing.T) { concurrent(t, "A", "B") }
func TestConcurrentB(t *testing.T) { concurrent(t, "B", "A") }
func concurrent(t *testing.T, own, peer string) {
    if err := os.WriteFile(own, []byte("ready"), 0600); err != nil { t.Fatal(err) }
    deadline := time.Now().Add(5 * time.Second)
    for {
        if _, err := os.Stat(peer); err == nil { return }
        if time.Now().After(deadline) { t.Fatal("sibling did not start concurrently") }
        time.Sleep(time.Millisecond)
    }
}
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
        self.assertTrue(popen.call_args.kwargs["start_new_session"])
        self.assertNotEqual(popen.call_args.kwargs["stdout"], subprocess.PIPE)

    @unittest.skipUnless(os.name == "posix", "native Unix process-group control")
    def test_inherited_output_cannot_hide_failed_passed_or_timed_out_parent(self):
        for name in ("TestFailedWithOutputDescendant", "TestPassedWithOutputDescendant",
                     "TestTimeoutWithOutputDescendant"):
            with self.subTest(name=name), tempfile.TemporaryDirectory(dir=self.cwd) as directory:
                output, errors, done = io.StringIO(), [], threading.Event()
                def observe():
                    try:
                        runner.run_check(self.binary, name, cwd=directory, timeout="1s", output=output)
                    except Exception as error:
                        errors.append(error)
                    finally:
                        done.set()
                thread = threading.Thread(target=observe)
                thread.start()
                # A held descriptor is not a still-running parent. The child
                # has a 30s safety alarm; this observes completion before that
                # independent alarm, rather than waiting for it to grant EOF.
                observed = done.wait(timeout=5)
                child = Path(directory) / "child-pid"
                try:
                    self.assertTrue(observed, "parent observation still depends on child pipe EOF")
                    self.assertTrue(child.exists())
                    pid = int(child.read_text())
                    # A zombie is stopped but not yet reaped by the VM's init;
                    # do not claim that signalling a group proves full cleanup.
                    deadline = time.monotonic() + 2
                    while time.monotonic() < deadline:
                        try:
                            os.kill(pid, 0)
                        except ProcessLookupError:
                            break
                        proc = Path(f"/proc/{pid}/stat")
                        if proc.exists() and proc.read_text().rsplit(") ", 1)[1].startswith("Z "):
                            break
                        time.sleep(.01)
                    else:
                        self.fail("owned output child is still running")
                    self.assertEqual(len(errors), 1)
                    text = output.getvalue()
                    self.assertIn("owned child output Σ", text)
                    if name == "TestPassedWithOutputDescendant":
                        self.assertIsInstance(errors[0], RuntimeError)
                        self.assertIn("left an owned process group", str(errors[0]))
                        self.assertIn("--- PASS: " + name, text)
                    else:
                        self.assertIsInstance(errors[0], subprocess.CalledProcessError)
                        self.assertEqual(errors[0].returncode, 2)
                        self.assertIn("original parent failure" if name.startswith("TestFailed")
                                      else "test timed out after 1s", text)
                finally:
                    # Even an adverse run of the old implementation must not
                    # leave this synthetic child behind or wait its full alarm.
                    if child.exists():
                        try:
                            os.kill(int(child.read_text()), signal.SIGKILL)
                        except ProcessLookupError:
                            pass
                    thread.join(timeout=5)
                    self.assertFalse(thread.is_alive())

    @unittest.skipUnless(os.name == "posix", "native Unix process-group control")
    def test_external_deadline_retains_output_when_the_program_ignores_go_flags(self):
        with tempfile.TemporaryDirectory(dir=self.cwd) as directory:
            fixture = Path(directory) / "stalled"
            fixture.write_text(f"#!{sys.executable}\nimport time\n"
                               "print('before independent deadline', flush=True)\n"
                               "time.sleep(30)\n")
            fixture.chmod(0o700)
            output = io.StringIO()
            processes = []
            actual_popen = subprocess.Popen
            def start(*args, **kwargs):
                process = actual_popen(*args, **kwargs)
                processes.append(process)
                return process
            with patch.object(runner, "PROCESS_EXIT_GRACE_SECONDS", .2), \
                    patch.object(runner.subprocess, "Popen", side_effect=start), \
                    self.assertRaises(subprocess.TimeoutExpired):
                runner.run_check(fixture, "TestStalled", cwd=directory, timeout="100ms", output=output)
            self.assertEqual(output.getvalue(), "before independent deadline\n")
            # The exact child was reaped, not merely signalled or hidden by a
            # manufactured test completion marker.
            self.assertEqual(len(processes), 1)
            self.assertEqual(processes[0].returncode, -signal.SIGKILL)

    def test_failure_exit_zero_and_timeout_cannot_pass(self):
        for name in ("TestFail", "TestExitZero", "TestTimeout"):
            with self.subTest(name=name), contextlib.redirect_stdout(io.StringIO()), self.assertRaises(subprocess.CalledProcessError):
                runner.run_check(self.binary, name, cwd=self.cwd, timeout="100ms")

    def test_no_matching_test_cannot_pass(self):
        with contextlib.redirect_stdout(io.StringIO()), self.assertRaises(RuntimeError):
            runner.run_check(self.binary, "TestMissing", cwd=self.cwd)

    def test_actual_go_processes_overlap_and_keep_separate_verdicts(self):
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            runner.run_checks(self.binary, ["TestConcurrentA", "TestConcurrentB", "TestPass"], cwd=self.cwd)
        text = output.getvalue()
        for name in ("TestConcurrentA", "TestConcurrentB", "TestPass"):
            self.assertEqual(text.count("--- PASS: " + name + " ("), 1)
        self.assertLess(text.index("--- PASS: TestConcurrentA"), text.index("--- PASS: TestConcurrentB"))
        self.assertLess(text.index("--- PASS: TestConcurrentB"), text.index("=== RUN   TestPass"))

    def test_actual_failure_does_not_erase_sibling_or_run_a_later_check(self):
        for failed in ("TestFail", "TestExitZero", "TestTimeout", "TestMissing"):
            with self.subTest(failed=failed), tempfile.TemporaryDirectory(dir=self.cwd) as directory:
                output, errors = io.StringIO(), io.StringIO()
                actual = runner.run_check
                actual_wait = runner.wait
                def wait(*args, **kwargs):
                    result = actual_wait(*args, **kwargs)
                    if any(future.exception() for future in result[0]):
                        (Path(directory) / "release-drain").write_text("observed failure")
                    return result
                def bounded(binary, name, **kwargs):
                    return actual(binary, name, timeout="5s" if name == "TestDrain" else "100ms", **kwargs)
                with patch.object(runner, "run_check", side_effect=bounded), patch.object(runner, "wait", side_effect=wait), \
                        contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors), \
                        self.assertRaises((subprocess.CalledProcessError, RuntimeError)):
                    runner.run_checks(self.binary, [failed, "TestDrain", "TestPassExtra"], cwd=directory)
                self.assertIn("--- PASS: TestDrain (", output.getvalue())
                self.assertNotIn("=== RUN   TestPassExtra", output.getvalue())
                self.assertIn("check failed: " + failed, errors.getvalue())


if __name__ == "__main__":
    unittest.main(verbosity=2)
