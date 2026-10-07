#!/usr/bin/env python3
"""Execute both copied performance recipes with synthetic readers.

Real awk validates fixtures and real GNU timeout enforces the whole-group deadline. Only
this test's shell group can be killed by the test cleanup; no Pulse service,
database, Docker socket, workload or destination is used.
"""
from __future__ import annotations

import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/TROUBLESHOOTING.md"
CONTAINER_ID = "a" * 64
DOCKER_IDENTITY = f"{CONTAINER_ID} 2026-10-02T20:00:00Z true 0"
PROCESS_IDENTITY = "4321 Sat Oct 3 00:00:00 2026"
DEFAULT_IO = ("rchar: 987654\nwchar: 999999\nwrite_bytes: 1000\ncancelled_write_bytes: 20\n",
              "rchar: 999999\nwchar: 999999\nwrite_bytes: 13000\ncancelled_write_bytes: 30\n")


def section() -> str:
    return DOC.read_text(encoding="utf-8").split(
        "#### Excessive CPU, writes or database growth\n", 1
    )[1].split("\n### Notifications\n", 1)[0]


def recipes() -> dict[str, str]:
    blocks = re.findall(r"```bash\n(.*?)```", section(), re.DOTALL)
    if len(blocks) != 2:
        raise AssertionError("expected two deployment recipes")
    return {"docker" if "# Docker:" in block else "systemd": block for block in blocks}


READER = '''#!/usr/bin/env python3
import json, os, pathlib, signal, subprocess, sys, time
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
root = pathlib.Path(os.environ['RECIPE_FIXTURE'])
log = root / 'calls.jsonl'
previous = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
count = sum(call['name'] == name for call in previous)
if name == 'docker':
    count = sum(call['name'] == name and call['args'][0] == args[0] for call in previous)
with log.open('a') as stream:
    stream.write(json.dumps({'name': name, 'args': args}) + '\\n')
if os.environ.get('HANG_TOOL') == name:
    if os.environ.get('IGNORE_TERM'):
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
    (root / 'hung-reader.json').write_text(json.dumps({'pid': os.getpid(), 'group': os.getpgrp()}))
    time.sleep(100)
if name == 'date':
    assert args == ['-u', '+%Y-%m-%dT%H:%M:%SZ']
    print('2026-10-03T00:00:00Z' if count == 0 else '2026-10-03T00:01:00Z')
elif name == 'sleep':
    assert args == ['60']
elif name == 'systemctl':
    assert args == ['show', 'pulse', '--property=MainPID', '--value']
    print(os.environ.get(f'PID_{count}', os.environ.get('FIXTURE_PID', '4321')))
    sys.exit(int(os.environ.get('SYSTEMCTL_EXIT', '0')))
elif name == 'ps':
    assert args == ['-p', '4321', '-o', 'pid=,lstart=']
    assert os.environ['TZ'] == 'UTC'
    print(os.environ.get(f'PROCESS_{count}', '4321 Sat Oct 3 00:00:00 2026'))
    sys.exit(int(os.environ.get('PS_EXIT', '0')))
elif name == 'awk':
    if args[-1] == '/proc/4321/io':
        index = sum(call['name'] == name and call['args'][-1] == args[-1] for call in previous)
        fixture = root / ('missing' if os.environ.get('IO_MISSING') else f'io-{index}')
    else:
        assert args[-1] == '/proc/4321/stat'
        index = sum(call['name'] == name and call['args'][-1] == args[-1] for call in previous)
        # comm may contain spaces and right parentheses. Starttime is field 22.
        ticks = os.environ.get(f'TICKS_{index}', '10000')
        fixture = root / f'stat-{index}'
        fixture.write_text('4321 (private ) worker) S ' + ' '.join(['0'] * 18 + [ticks]) + '\\n')
    if os.environ.get('AWK_EXIT'):
        print('private permission error', file=sys.stderr)
        sys.exit(int(os.environ['AWK_EXIT']))
    sys.exit(subprocess.run([os.environ['REAL_AWK'], *args[:-1], str(fixture)]).returncode)
elif name == 'sudo':
    assert args[0] in ('awk', '-n')
    offset = 2 if args[0] == '-n' else 1
    assert args[offset-1] == 'awk' and args[-1] == '/proc/4321/io'
    target = root / ('missing' if os.environ.get('IO_MISSING') else f'io-{count}')
    if os.environ.get('SUDO_EXIT'):
        print('private permission error', file=sys.stderr)
        sys.exit(int(os.environ['SUDO_EXIT']))
    sys.exit(subprocess.run([os.environ['REAL_AWK'], *args[offset:-1], str(target)]).returncode)
elif name == 'docker':
    if args[0] == 'inspect':
        identity = os.environ.get(f'INSPECT_{count}', os.environ['DOCKER_IDENTITY'])
        if args[1:] == ['--format', 'Started={{.State.StartedAt}}', 'pulse']:
            print('Started=' + (identity.split()[1] if identity else ''))
        else:
            print(identity)
    elif args[0] == 'stats':
        text = os.environ.get('DOCKER_STATS', 'CPU=12.50% Memory=100MiB / 1GiB BlockIO=1MB / 12MB')
        if text:
            print(text)
    else:
        raise AssertionError('unsupported docker command')
    setting = 'DOCKER_INSPECT_EXIT' if args[0] == 'inspect' else 'DOCKER_STATS_EXIT'
    if os.environ.get(setting):
        print('private daemon error', file=sys.stderr)
    sys.exit(int(os.environ.get(setting, '0')))
else:
    raise AssertionError('unexpected command')
'''


def run_owned(copied: str, env: dict[str, str]):
    with subprocess.Popen(["bash", "-c", copied], env=env, text=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                          start_new_session=True) as process:
        try:
            stdout, stderr = process.communicate(timeout=83)
        except subprocess.TimeoutExpired:
            # This is an adverse test error, never an empty/passing sample.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.communicate(timeout=5)
            raise
        return subprocess.CompletedProcess(process.args, process.returncode, stdout, stderr)


class PerformanceTroubleshootingDocsTest(unittest.TestCase):
    def exercise(self, deployment: str, *, counters: tuple[str, str] = DEFAULT_IO,
                 copied: str | None = None, **settings: str):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            for name in ("date", "sleep", "systemctl", "ps", "awk", "sudo", "docker"):
                command = directory / name
                command.write_text(READER, encoding="utf-8")
                command.chmod(0o700)
            for index, text in enumerate(counters):
                (directory / f"io-{index}").write_text(text, encoding="utf-8")
            env = dict(os.environ, PATH=f"{directory}:{os.environ['PATH']}",
                       RECIPE_FIXTURE=str(directory), REAL_AWK=shutil.which("awk"),
                       DOCKER_IDENTITY=DOCKER_IDENTITY, **settings)
            result = run_owned(copied if copied is not None else recipes()[deployment], env)
            hung_reader = directory / "hung-reader.json"
            if hung_reader.exists():
                pid = json.loads(hung_reader.read_text())["pid"]
                state = Path(f"/proc/{pid}/stat")
                try:
                    # A reaped process or a zombie cannot keep running a reader.
                    remaining = state.read_text().rsplit(") ", 1)[1].split()[0]
                except FileNotFoundError:
                    remaining = None
                self.assertIn(remaining, (None, "Z"), "owned reader survived the collection deadline")
            log = directory / "calls.jsonl"
            calls = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
            return result, calls

    def assert_unavailable(self, result):
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, "", "an incomplete pair must not be published")
        self.assertIn("unavailable", result.stderr)
        self.assertNotIn("private", result.stderr)

    def test_shipped_source_and_shell_syntax(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes())
        for copied in recipes().values():
            result = subprocess.run(["bash", "-n"], input=copied, text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("timeout --signal=KILL 80s bash", copied)

    def test_systemd_samples_only_valid_write_counters_and_stable_identity(self):
        result, calls = self.exercise("systemd")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([c["name"] for c in calls],
                         ["systemctl", "ps", "awk", "date", "awk", "systemctl", "ps", "awk",
                          "sleep", "systemctl", "ps", "awk", "date", "awk", "systemctl", "ps", "awk"])
        self.assertFalse(any(c["name"] == "sudo" for c in calls))
        self.assertEqual([c["args"][-1] for c in calls if c["name"] == "awk"],
                         ["/proc/4321/stat", "/proc/4321/io", "/proc/4321/stat"] * 2)
        for value in (PROCESS_IDENTITY, "start ticks=10000", "write_bytes: 1000",
                      "write_bytes: 13000", "cancelled_write_bytes: 20", "cancelled_write_bytes: 30"):
            self.assertIn(value, result.stdout)
        for forbidden in ("rchar", "wchar", "private ) worker"):
            self.assertNotIn(forbidden, result.stdout)

    def test_invalid_pid_stops_before_process_reads(self):
        for pid in ("", "0", "not-a-pid", "4321 extra"):
            with self.subTest(pid=pid):
                result, calls = self.exercise("systemd", FIXTURE_PID=pid)
                self.assert_unavailable(result)
                self.assertEqual([c["name"] for c in calls], ["systemctl"])

    def test_failed_or_empty_identity_does_not_publish_counters(self):
        for settings in ({"SYSTEMCTL_EXIT": "3"}, {"PS_EXIT": "4"}, {"PROCESS_0": ""},
                         {"TICKS_0": ""}, {"TICKS_0": "bad"}, {"IO_MISSING": "1"}, {"AWK_EXIT": "5"}):
            with self.subTest(settings=settings):
                result, calls = self.exercise("systemd", **settings)
                self.assert_unavailable(result)
                self.assertNotIn("sleep", [c["name"] for c in calls])

    def test_partial_malformed_or_duplicate_counters_are_withheld(self):
        for text in ("", "rchar: 999\n", "write_bytes: 1000\n", "write_bytes: nope\ncancelled_write_bytes: 2\n",
                     "write_bytes: 1000 bytes\ncancelled_write_bytes: 2\n",
                     "write_bytes: 1000\nwrite_bytes: 1001\n",
                     "write_bytes: 1000\ncancelled_write_bytes: 2\nwrite_bytes: 1001\n"):
            with self.subTest(text=text):
                result, _ = self.exercise("systemd", counters=(text, text))
                self.assert_unavailable(result)

    def test_pid_start_or_same_second_reuse_during_or_between_reads_is_withheld(self):
        for settings in ({"PID_1": "5432"}, {"PID_2": "5432"},
                         {"PROCESS_1": "4321 Sat Oct 3 00:00:01 2026"},
                         {"PROCESS_2": "4321 Sat Oct 3 00:00:01 2026"},
                         {"TICKS_1": "10001"}, {"TICKS_2": "10001", "TICKS_3": "10001"}):
            with self.subTest(settings=settings):
                result, _ = self.exercise("systemd", **settings)
                self.assert_unavailable(result)

    def test_second_sample_failure_withholds_the_completed_first_sample(self):
        result, _ = self.exercise("systemd", counters=(DEFAULT_IO[0], "write_bytes: 99\n"))
        self.assert_unavailable(result)

    def test_decreasing_counters_remain_raw_not_a_rate(self):
        result, _ = self.exercise("systemd", counters=(DEFAULT_IO[1], DEFAULT_IO[0]))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("write_bytes: 13000", result.stdout)
        self.assertIn("write_bytes: 1000", result.stdout)
        self.assertNotRegex(result.stdout, r"GB/day|bytes/s|bytes: -[0-9]+")

    def test_docker_targets_the_full_id_and_checks_its_running_instance(self):
        result, calls = self.exercise("docker")
        self.assertEqual(result.returncode, 0, result.stderr)
        expected = ["inspect", "--type", "container", "--format",
                    "{{.Id}} {{.State.StartedAt}} {{.State.Running}} {{.RestartCount}}", "pulse"]
        self.assertEqual([c["args"] for c in calls if c["name"] == "docker" and c["args"][0] == "inspect"],
                         [expected] * 4)
        self.assertEqual([c["args"] for c in calls if c["name"] == "docker" and c["args"][0] == "stats"],
                         [["stats", "--no-stream", "--format",
                           "CPU={{.CPUPerc}} Memory={{.MemUsage}} BlockIO={{.BlockIO}}", CONTAINER_ID]] * 2)
        self.assertEqual(result.stdout.count(DOCKER_IDENTITY), 2)
        self.assertEqual(result.stdout.count("BlockIO=1MB / 12MB"), 2)

    def test_invalid_stopped_or_unstarted_docker_identity_is_unavailable(self):
        for identity in ("", DOCKER_IDENTITY.replace(CONTAINER_ID, "short"),
                         DOCKER_IDENTITY.replace("true", "false"),
                         DOCKER_IDENTITY.replace("2026-10-02T20:00:00Z", "0001-01-01T00:00:00Z"),
                         DOCKER_IDENTITY.replace("2026-10-02T20:00:00Z", "bad"),
                         DOCKER_IDENTITY + " extra", DOCKER_IDENTITY + "\n" + DOCKER_IDENTITY):
            with self.subTest(identity=identity):
                result, calls = self.exercise("docker", INSPECT_0=identity)
                self.assert_unavailable(result)
                self.assertFalse(any(c["name"] == "docker" and c["args"][0] == "stats" for c in calls))

    def test_empty_malformed_or_multiple_docker_stats_are_unavailable(self):
        for text in ("", "--", "CPU=-- Memory=-- / -- BlockIO=-- / --", "CPU=bad% Memory=1 / 2 BlockIO=1 / 2",
                     "CPU=1% Memory=1 / 2", "CPU=1% Memory=1 / 2 BlockIO=1 / 2\nprivate extra"):
            with self.subTest(text=text):
                result, _ = self.exercise("docker", DOCKER_STATS=text)
                self.assert_unavailable(result)

    def test_docker_errors_preserve_exit_without_partial_or_raw_error_output(self):
        for setting, value in (("DOCKER_INSPECT_EXIT", "2"), ("DOCKER_STATS_EXIT", "3")):
            with self.subTest(setting=setting):
                result, calls = self.exercise("docker", **{setting: value})
                self.assert_unavailable(result)
                self.assertEqual(result.returncode, int(value))
                self.assertNotIn("sleep", [c["name"] for c in calls])

    def test_docker_replacement_restart_or_stop_during_and_between_reads_is_unavailable(self):
        for identity in (DOCKER_IDENTITY.replace(CONTAINER_ID, "b" * 64),
                         DOCKER_IDENTITY.replace("20:00:00", "20:01:00"),
                         DOCKER_IDENTITY[:-1] + "1", DOCKER_IDENTITY.replace("true", "false")):
            for index in (1, 2, 3):
                with self.subTest(identity=identity, index=index):
                    result, _ = self.exercise("docker", **{f"INSPECT_{index}": identity})
                    self.assert_unavailable(result)

    def test_missing_timeout_stops_before_any_reader(self):
        # No timeout in this private PATH. bash is invoked by absolute path.
        with tempfile.TemporaryDirectory() as temporary:
            for deployment, copied in recipes().items():
                with self.subTest(deployment=deployment):
                    result = subprocess.run([shutil.which("bash"), "-c", copied],
                                            env=dict(os.environ, PATH=temporary),
                                            text=True, capture_output=True, timeout=3)
                    self.assert_unavailable(result)
                    self.assertIn("GNU timeout is required", result.stderr)

    def test_real_deadline_stops_a_hung_reader_in_both_recipes(self):
        for deployment, name in (("systemd", "systemctl"), ("docker", "docker")):
            with self.subTest(deployment=deployment):
                started = time.monotonic()
                result, _ = self.exercise(deployment, HANG_TOOL=name)
                self.assert_unavailable(result)
                self.assertEqual(result.returncode, 137)
                self.assertLess(time.monotonic() - started, 83)

    def test_real_whole_group_deadline_stops_a_reader_ignoring_term(self):
        started = time.monotonic()
        result, _ = self.exercise("docker", HANG_TOOL="docker", IGNORE_TERM="1")
        self.assert_unavailable(result)
        self.assertEqual(result.returncode, 137)
        self.assertLess(time.monotonic() - started, 83)

    def test_guidance_preserves_scope_units_baselines_and_data_safety(self):
        guide = " ".join(section().split())
        for phrase in ("same PID and process start time", "actual elapsed seconds", "not a measured day's total",
                       "not bytes per second", "not a recent sampling window", "different scopes",
                       "inside the Pulse container", "does not make that data a clean", "Do not enable Debug",
                       "remove database indexes", "tmpfs", "Do not post databases", "manually redacted error",
                       "no partial identity", "no unbounded fallback", "full container ID", "does not elevate privileges"):
            self.assertIn(phrase, guide)
        for copied in recipes().values():
            self.assertNotRegex(copied, r"\b(?:sudo|restart|rm|truncate|sqlite3|curl|printenv)\b|--follow|--token|(?:^|[;\n])\s*kill\b")


if __name__ == "__main__":
    unittest.main()
