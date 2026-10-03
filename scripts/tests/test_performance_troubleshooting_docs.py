#!/usr/bin/env python3
"""Exercise the guide's bounded I/O recipes with synthetic OS/container readers.

The real awk filters run against fixtures. No Pulse process, database, Docker
daemon or host service is inspected. These checks establish recipe behaviour,
not native write rates, CPU relief or an operational repair.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/TROUBLESHOOTING.md"


def section() -> str:
    return DOC.read_text(encoding="utf-8").split(
        "#### Excessive CPU, writes or database growth\n", 1
    )[1].split("\n### Notifications\n", 1)[0]


def recipes() -> dict[str, str]:
    blocks = re.findall(r"```bash\n(.*?)```", section(), re.DOTALL)
    if len(blocks) != 2:
        raise AssertionError("expected two bounded deployment recipes")
    return {"docker" if "# Docker:" in block else "systemd": block for block in blocks}


# All external commands record exact arguments. sudo is a test-only adapter:
# it replaces only the documented /proc/PID/io input and executes real awk.
READER = '''#!/usr/bin/env python3
import json, os, pathlib, subprocess, sys
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
root = pathlib.Path(os.environ['RECIPE_FIXTURE'])
log = root / 'calls.jsonl'
previous = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
count = sum(call['name'] == name for call in previous)
with log.open('a') as stream:
    stream.write(json.dumps({'name': name, 'args': args}) + '\\n')
if name == 'date':
    print('2026-10-03T00:00:00Z' if count == 0 else '2026-10-03T00:01:00Z')
elif name == 'sleep':
    assert args == ['60']
elif name == 'systemctl':
    print(os.environ.get('FIXTURE_PID', '4321'))
    sys.exit(int(os.environ.get('SYSTEMCTL_EXIT', '0')))
elif name == 'ps':
    assert os.environ['TZ'] == 'UTC'
    print('4321 Sat Oct 3 00:00:00 2026')
    sys.exit(int(os.environ.get('PS_EXIT', '0')))
elif name == 'sudo':
    assert args[0] == 'awk'
    assert args[-1] == '/proc/4321/io'
    target = root / ('missing' if os.environ.get('IO_MISSING') else f'io-{count}')
    result = subprocess.run([os.environ['REAL_AWK'], *args[1:-1], str(target)])
    sys.exit(result.returncode)
elif name == 'docker':
    if args[0] == 'inspect':
        print('Started=2026-10-02T20:00:00Z')
    elif args[0] == 'stats':
        text = os.environ.get('DOCKER_STATS', 'CPU=12.50% Memory=100MiB / 1GiB BlockIO=1MB / 12MB')
        if text:
            print(text)
    else:
        raise AssertionError('unsupported docker command')
    setting = 'DOCKER_INSPECT_EXIT' if args[0] == 'inspect' else 'DOCKER_STATS_EXIT'
    sys.exit(int(os.environ.get(setting, '0')))
else:
    raise AssertionError('unexpected command')
'''


class PerformanceTroubleshootingDocsTest(unittest.TestCase):
    def exercise(self, deployment: str, *, counters: tuple[str, str] | None = None,
                 **settings: str):
        import shutil

        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            for name in ("date", "sleep", "systemctl", "ps", "sudo", "docker"):
                command = directory / name
                command.write_text(READER, encoding="utf-8")
                command.chmod(0o700)
            counters = counters or (
                "rchar: 987654\nwchar: 999999\nwrite_bytes: 1000\ncancelled_write_bytes: 20\n",
                "rchar: 999999\nwchar: 999999\nwrite_bytes: 13000\ncancelled_write_bytes: 30\n",
            )
            for index, text in enumerate(counters):
                (directory / f"io-{index}").write_text(text, encoding="utf-8")
            env = dict(os.environ, PATH=f"{directory}:{os.environ['PATH']}",
                       RECIPE_FIXTURE=str(directory), REAL_AWK=shutil.which("awk"), **settings)
            result = subprocess.run(["bash", "-c", recipes()[deployment]], env=env,
                                    text=True, capture_output=True, timeout=10)
            calls = [json.loads(line) for line in (directory / "calls.jsonl").read_text().splitlines()]
            return result, calls

    def test_shipped_document_matches_the_exercised_source(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes())

    def test_systemd_samples_only_write_counters_and_process_identity(self):
        result, calls = self.exercise("systemd")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([call["name"] for call in calls],
                         ["date", "systemctl", "ps", "sudo", "sleep",
                          "date", "systemctl", "ps", "sudo"])
        for call in calls:
            if call["name"] == "systemctl":
                self.assertEqual(call["args"], ["show", "pulse", "--property=MainPID", "--value"])
            elif call["name"] == "ps":
                self.assertEqual(call["args"], ["-p", "4321", "-o", "pid=,lstart="])
        self.assertIn("write_bytes: 1000", result.stdout)
        self.assertIn("write_bytes: 13000", result.stdout)
        self.assertIn("cancelled_write_bytes: 20", result.stdout)
        self.assertNotIn("rchar", result.stdout)
        self.assertNotIn("wchar", result.stdout)

    def test_unavailable_pid_never_reads_a_process_or_sleeps(self):
        for pid in ("", "0", "not-a-pid", "4321 extra"):
            with self.subTest(pid=pid):
                result, calls = self.exercise("systemd", FIXTURE_PID=pid)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("sample unavailable", result.stderr)
                self.assertEqual([c["name"] for c in calls], ["date", "systemctl"])

    def test_service_and_process_read_failures_stop_the_collection(self):
        for settings, expected in (({"SYSTEMCTL_EXIT": "3"}, ["date", "systemctl"]),
                                   ({"PS_EXIT": "4"}, ["date", "systemctl", "ps"])):
            with self.subTest(settings=settings):
                result, calls = self.exercise("systemd", **settings)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual([c["name"] for c in calls], expected)

    def test_missing_process_io_is_failure_not_zero_writes(self):
        result, calls = self.exercise("systemd", IO_MISSING="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("write_bytes:", result.stdout)
        self.assertNotIn("sleep", [c["name"] for c in calls])

    def test_incomplete_process_counters_do_not_become_a_success(self):
        for text in ("", "rchar: 999\n", "write_bytes: 1000\n"):
            with self.subTest(text=text):
                result, calls = self.exercise("systemd", counters=(text, text))
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("sleep", [c["name"] for c in calls])

    def test_decreasing_counters_are_retained_not_turned_into_a_rate(self):
        result, _ = self.exercise("systemd", counters=(
            "write_bytes: 13000\ncancelled_write_bytes: 30\n",
            "write_bytes: 1000\ncancelled_write_bytes: 20\n",
        ))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("write_bytes: 13000", result.stdout)
        self.assertIn("write_bytes: 1000", result.stdout)
        self.assertNotRegex(result.stdout, r"GB/day|bytes/s|bytes: -[0-9]+")

    def test_docker_reads_only_start_time_and_selected_statistics_twice(self):
        result, calls = self.exercise("docker")
        self.assertEqual(result.returncode, 0, result.stderr)
        expected = [
            ["inspect", "--format", "Started={{.State.StartedAt}}", "pulse"],
            ["stats", "--no-stream", "--format",
             "CPU={{.CPUPerc}} Memory={{.MemUsage}} BlockIO={{.BlockIO}}", "pulse"],
        ]
        self.assertEqual([c["args"] for c in calls if c["name"] == "docker"], expected * 2)
        self.assertEqual([c["name"] for c in calls],
                         ["date", "docker", "docker", "sleep", "date", "docker", "docker"])
        self.assertEqual(result.stdout.count("BlockIO=1MB / 12MB"), 2)

    def test_empty_docker_statistics_are_unavailable_not_zero(self):
        result, calls = self.exercise("docker", DOCKER_STATS="")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("no zero inferred", result.stderr)
        self.assertNotIn("sleep", [c["name"] for c in calls])

    def test_docker_reader_failure_stops_before_another_sample(self):
        for settings, expected in (
            ({"DOCKER_INSPECT_EXIT": "2"}, ["date", "docker"]),
            ({"DOCKER_STATS_EXIT": "3"}, ["date", "docker", "docker"]),
        ):
            with self.subTest(settings=settings):
                result, calls = self.exercise("docker", **settings)
                self.assertEqual(result.returncode, int(next(iter(settings.values()))))
                self.assertEqual([c["name"] for c in calls], expected)

    def test_guidance_keeps_scope_units_baselines_and_data_safety(self):
        guide = " ".join(section().split())
        for phrase in ("same PID and process start time", "actual elapsed seconds",
                       "not a measured day's total", "not bytes per second",
                       "not a recent sampling window", "different scopes",
                       "inside the Pulse container", "does not make that data a clean",
                       "Do not enable Debug", "remove database indexes", "tmpfs",
                       "Do not post databases", "manually redacted error"):
            self.assertIn(phrase, guide)
        for recipe in recipes().values():
            self.assertNotRegex(recipe, r"\b(?:restart|kill|rm|truncate|sqlite3|curl|printenv)\b|--follow|--token")


if __name__ == "__main__":
    unittest.main()
