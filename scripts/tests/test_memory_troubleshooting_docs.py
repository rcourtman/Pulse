#!/usr/bin/env python3
"""Exercise the copied resident-memory recipe, not a Pulse memory repair.

The fixture adapter runs real awk over synthetic status files. A separate
check reads only a disposable child process's live Linux counters; it does not
inspect a host service, database, heap or container daemon.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/TROUBLESHOOTING.md"
COUNTERS = "VmRSS: 8192 kB\nRssAnon: 6144 kB\nRssFile: 1536 kB\nRssShmem: 512 kB\nVmSwap: 1024 kB\n"
IDENTITY = "4321 Tue Oct 6 00:00:00 2026"


def section() -> str:
    return DOC.read_text(encoding="utf-8").split(
        "#### Memory use keeps growing\n", 1
    )[1].split("\n#### ", 1)[0]


def recipe() -> str:
    blocks = re.findall(r"```bash\n(.*?)```", section(), re.DOTALL)
    if len(blocks) != 1:
        raise AssertionError("expected one bounded memory recipe")
    return blocks[0]


READER = '''#!/usr/bin/env python3
import json, os, pathlib, subprocess, sys, time
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
root = pathlib.Path(os.environ['RECIPE_FIXTURE'])
log = root / 'calls.jsonl'
previous = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
count = sum(call['name'] == name for call in previous)
with log.open('a') as stream:
    stream.write(json.dumps({'name': name, 'args': args}) + '\\n')
if name == os.environ.get('HANG_TOOL'):
    time.sleep(30)
if name == 'date':
    assert args == ['-u', '+%Y-%m-%dT%H:%M:%SZ']
    print('2026-10-06T00:00:00Z')
elif name == 'systemctl':
    assert args == ['show', 'pulse', '--property=MainPID', '--value']
    print(os.environ.get(f'PID_{count}', '4321'))
    sys.exit(int(os.environ.get(f'SERVICE_EXIT_{count}', '0')))
elif name == 'ps':
    assert args == ['-p', '4321', '-o', 'pid=,lstart=']
    assert os.environ['TZ'] == 'UTC'
    print(os.environ.get(f'IDENTITY_{count}', '4321 Tue Oct 6 00:00:00 2026'))
    sys.exit(int(os.environ.get(f'PROCESS_EXIT_{count}', '0')))
elif name == 'awk':
    assert args[-1] == '/proc/4321/status'
    target = root / ('missing' if os.environ.get('STATUS_MISSING') else 'status')
    sys.exit(subprocess.run([os.environ['REAL_AWK'], *args[:-1], str(target)]).returncode)
else:
    raise AssertionError('unexpected command')
'''


def run_owned_recipe(copied: str, env: dict[str, str]) -> subprocess.CompletedProcess:
    """Keep the existing ten-second assertion; clean up our shell on every exit."""
    with subprocess.Popen(["bash", "-c", copied], env=env, text=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                          start_new_session=True) as process:
        try:
            stdout, stderr = process.communicate(timeout=10)
        except subprocess.TimeoutExpired:
            # Never target a service or another test's group. TimeoutExpired
            # remains an error, not a passing or empty sample.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.communicate(timeout=5)
            raise
        return subprocess.CompletedProcess(process.args, process.returncode, stdout, stderr)


class MemoryTroubleshootingDocsTest(unittest.TestCase):
    def exercise(self, *, status: str = COUNTERS, copied: str | None = None,
                 **settings: str):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            for name in ("date", "systemctl", "ps", "awk"):
                command = directory / name
                command.write_text(READER, encoding="utf-8")
                command.chmod(0o700)
            (directory / "status").write_text(status, encoding="utf-8")
            env = dict(os.environ, PATH=f"{directory}:{os.environ['PATH']}",
                       RECIPE_FIXTURE=str(directory), REAL_AWK=shutil.which("awk"), **settings)
            result = run_owned_recipe(copied if copied is not None else recipe(), env)
            log = directory / "calls.jsonl"
            calls = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
            return result, calls

    def assert_no_memory_claim(self, result):
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("Pulse process (PID and UTC start)", result.stdout)
        self.assertNotRegex(result.stdout, r"VmRSS:|RssAnon:|RssFile:|RssShmem:|VmSwap:")

    def test_shipped_guide_is_the_exercised_source_and_shell_is_valid(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes())
        result = subprocess.run(["bash", "-n"], input=recipe(), text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_only_five_counters_and_stable_process_identity_leave_the_reader(self):
        result, calls = self.exercise(status="Name: private-workload\nUid: 1001 1001\n"
                                      + COUNTERS + "VmSize: 1234567 kB\nThreads: 19\n")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("2026-10-06T00:00:00Z", result.stdout)
        self.assertIn(IDENTITY, result.stdout)
        self.assertIn(COUNTERS, result.stdout)
        for private in ("private-workload", "Uid:", "VmSize:", "Threads:"):
            self.assertNotIn(private, result.stdout)
        self.assertEqual([call["name"] for call in calls],
                         ["date", "systemctl", "ps", "awk", "systemctl", "ps"])

    def test_missing_or_invalid_pid_stops_before_a_process_read(self):
        for pid in ("", "0", "not-a-pid", "4321 extra"):
            with self.subTest(pid=pid):
                result, calls = self.exercise(PID_0=pid)
                self.assert_no_memory_claim(result)
                self.assertIn("sample unavailable", result.stderr)
                self.assertEqual([call["name"] for call in calls], ["date", "systemctl"])

    def test_service_and_process_failures_remain_failures(self):
        for key, names in (
            ("SERVICE_EXIT_0", ["date", "systemctl"]),
            ("PROCESS_EXIT_0", ["date", "systemctl", "ps"]),
            ("SERVICE_EXIT_1", ["date", "systemctl", "ps", "awk", "systemctl"]),
            ("PROCESS_EXIT_1", ["date", "systemctl", "ps", "awk", "systemctl", "ps"]),
        ):
            with self.subTest(key=key):
                result, calls = self.exercise(**{key: "3"})
                self.assert_no_memory_claim(result)
                self.assertEqual(result.returncode, 3)
                self.assertEqual([call["name"] for call in calls], names)

    def test_empty_initial_identity_is_not_a_complete_sample(self):
        result, calls = self.exercise(IDENTITY_0="")
        self.assert_no_memory_claim(result)
        self.assertIn("identity unavailable", result.stderr)
        self.assertNotIn("awk", [call["name"] for call in calls])

    def test_missing_unreadable_or_incomplete_status_does_not_print_partial_counters(self):
        for status, settings in ((COUNTERS, {"STATUS_MISSING": "1"}),
                                 ("", {}), (COUNTERS.replace("RssAnon: 6144 kB\n", ""), {})):
            with self.subTest(status=status, settings=settings):
                result, calls = self.exercise(status=status, **settings)
                self.assert_no_memory_claim(result)
                self.assertEqual([call["name"] for call in calls],
                                 ["date", "systemctl", "ps", "awk"])

    def test_malformed_units_numbers_duplicates_and_extra_fields_fail_closed(self):
        for value in ("-1 kB", "unknown kB", "1 MB", "1", "1 kB extra"):
            with self.subTest(value=value):
                result, _ = self.exercise(status=COUNTERS.replace("8192 kB", value))
                self.assert_no_memory_claim(result)
        result, _ = self.exercise(status=COUNTERS + "VmRSS: 8192 kB\n")
        self.assert_no_memory_claim(result)

    def test_service_restart_or_pid_reuse_discards_the_captured_counters(self):
        for settings in ({"PID_1": "9999"}, {"PID_1": "0"},
                         {"IDENTITY_1": "4321 Tue Oct 6 00:00:01 2026"},
                         {"IDENTITY_1": ""}):
            with self.subTest(settings=settings):
                result, _ = self.exercise(**settings)
                self.assert_no_memory_claim(result)
                self.assertIn("discard this sample", result.stderr)

    def test_measured_zero_is_retained_without_interpreting_missing_as_zero(self):
        result, _ = self.exercise(status=re.sub(r"\d+ kB", "0 kB", COUNTERS))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.count("0 kB"), 5)

    def test_hung_service_reader_is_unavailable_before_the_test_deadline(self):
        started = time.monotonic()
        result, calls = self.exercise(HANG_TOOL="systemctl")
        self.assertLess(time.monotonic() - started, 10)
        self.assertEqual(result.returncode, 124, result.stderr)
        self.assert_no_memory_claim(result)
        self.assertIn("sample unavailable", result.stderr)
        self.assertEqual([call["name"] for call in calls], ["date", "systemctl"])

    def test_hung_counter_reader_never_publishes_identity_or_partial_memory(self):
        started = time.monotonic()
        result, calls = self.exercise(HANG_TOOL="awk")
        self.assertLess(time.monotonic() - started, 10)
        self.assertEqual(result.returncode, 124, result.stderr)
        self.assert_no_memory_claim(result)
        self.assertIn("sample unavailable", result.stderr)
        self.assertEqual([call["name"] for call in calls], ["date", "systemctl", "ps", "awk"])

    def test_missing_deadline_utility_is_not_a_sample_or_an_unbounded_fallback(self):
        copied = recipe().replace("timeout --kill-after=1s 8s bash",
                                  "pulse-missing-memory-deadline --kill-after=1s 8s bash")
        self.assertNotEqual(copied, recipe())
        result, calls = self.exercise(copied=copied)
        self.assertEqual(result.returncode, 127, result.stderr)
        self.assert_no_memory_claim(result)
        self.assertIn("sample unavailable", result.stderr)
        self.assertEqual(calls, [])

    @unittest.skipUnless(sys.platform.startswith("linux"), "Linux /proc recipe")
    def test_real_linux_status_of_an_owned_disposable_process(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            child = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(30)"])
            try:
                command = directory / "systemctl"
                command.write_text("#!/bin/sh\nprintf '%s\\n' \"$RECIPE_CHILD_PID\"\n", encoding="utf-8")
                command.chmod(0o700)
                env = dict(os.environ, PATH=f"{directory}:{os.environ['PATH']}",
                           RECIPE_CHILD_PID=str(child.pid))
                result = run_owned_recipe(recipe(), env)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("Pulse process (PID and UTC start)", result.stdout)
                for field in ("VmRSS", "RssAnon", "RssFile", "RssShmem", "VmSwap"):
                    self.assertRegex(result.stdout, rf"(?m)^{field}:\s+[0-9]+ kB$")
                self.assertNotIn("Name:", result.stdout)
                self.assertNotIn("Uid:", result.stdout)
            finally:
                child.terminate()
                child.wait(timeout=5)

    def test_guidance_distinguishes_scope_units_and_unsafe_shortcuts(self):
        guide = " ".join(section().split())
        for phrase in ("container, not just Pulse", "own cache accounting", "Go heap size",
                       "inside the Pulse container", "in both `systemctl` calls",
                       "1,024 bytes", "approximate and not an atomic snapshot",
                       "not specifically the Go heap", "different run",
                       "do not measure Pulse RSS", "do not assume its PID 1 is Pulse",
                       "requires GNU `timeout`", "initial UTC timestamp alone",
                       "do not remove the deadline", "does not stop or restart Pulse",
                       "Do not restart Pulse, drop caches, force garbage collection",
                       "Do not post a heap dump", "existing screenshots remain useful"):
            self.assertIn(phrase, guide)
        self.assertNotRegex(recipe(),
                            r"(?<!-)\b(?:sudo|restart|kill|rm|truncate|sqlite3|curl|printenv|cat)\b"
                            r"|--follow|--token|cmdline|environ|smaps")


if __name__ == "__main__":
    unittest.main()
