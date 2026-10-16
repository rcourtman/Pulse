#!/usr/bin/env python3
"""Execute the copied ZFS reader using synthetic, unprivileged zpool processes.

No pool, host, service, device or credential is accessed. Kernel-blocked I/O
and installed ZFS recovery are explicitly outside this recipe proof.
"""

from pathlib import Path
import json
import re
import subprocess
import sys
import unittest

from test_troubleshooting_logs import exercise_log_recipe


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/ZFS_MONITORING.md"
LIST_ARGS = ["list", "-o", "name,size,alloc,free,health", "tank"]
STATUS_ARGS = ["status", "tank"]
LABELS = "Pool capacity and health:\n\nPool device status:\n"
PRIVATE = "synthetic-private-device-and-serial"


def section():
    return DOC.read_text(encoding="utf-8").split(
        "## Read native pool status safely\n", 1
    )[1].split("\n## ", 1)[0]


def recipe():
    blocks = re.findall(r"```bash\n(.*?)```", section(), re.DOTALL)
    if len(blocks) != 1:
        raise AssertionError("expected one combined ZFS status reader")
    return blocks[0]


ZPOOL_READER = f'''#!{sys.executable}
import json, os, pathlib, signal, sys, time
argv_file = pathlib.Path(os.environ['READER_ARGV'])
calls = json.loads(argv_file.read_text()) if argv_file.exists() else []
calls.append(sys.argv[1:])
argv_file.write_text(json.dumps(calls))
pathlib.Path(os.environ['READER_PID']).write_text(json.dumps({{
    'pid': os.getpid(),
    'start': pathlib.Path('/proc/self/stat').read_text().split(')', 1)[1].split()[19],
}}))
settings = json.loads(pathlib.Path(os.environ['LOG_STDOUT_FILE']).read_text())[sys.argv[1]]
sys.stdout.write(settings.get('stdout', ''))
sys.stderr.write(settings.get('stderr', ''))
sys.stdout.flush()
sys.stderr.flush()
if settings.get('hang'):
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    if settings['hang'] == 'closed-pipes':
        os.close(1)
        os.close(2)
    time.sleep(30)
sys.exit(settings.get('exit', 0))
'''


class ZFSStatusDocsTest(unittest.TestCase):
    def exercise(self, *, list_result=None, status_result=None, **settings):
        return exercise_log_recipe(
            "zpool", recipe(), reader_source=ZPOOL_READER,
            stdout=json.dumps({"list": list_result or {}, "status": status_result or {}}),
            **settings,
        )

    def assert_cleanup(self, result):
        self.assertFalse(result.reader_running, "synthetic reader survived collection")
        self.assertEqual(len(result.log_cleanup), 1)
        self.assertEqual(result.log_cleanup[0]["mode"], 0o600)
        self.assertLessEqual(result.log_cleanup[0]["bytes"], 262145)

    def test_mirror_and_existing_entry_point_reach_one_combined_reader(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/ZFS_MONITORING.md").read_bytes())
        intro = DOC.read_text().split("## Read native pool status safely", 1)[0]
        self.assertIn("[bounded native status reader](#read-native-pool-status-safely)", intro)
        self.assertNotIn("```bash", intro)

    def test_copied_shell_syntax_and_read_only_native_arguments(self):
        copied = recipe()
        subprocess.run(["bash", "-n", "-c", copied], check=True, capture_output=True)
        result, calls = self.exercise(list_result={"stdout": "tank 10T 2T 8T ONLINE\n"},
                                      status_result={"stdout": "state: ONLINE\n"})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls, [LIST_ARGS, STATUS_ARGS])
        self.assertIn("tank 10T 2T 8T ONLINE", result.stdout)
        self.assertIn("state: ONLINE", result.stdout)
        self.assertEqual(result.stderr, "")
        self.assertIn("timeout --signal=KILL 8s bash -o pipefail", copied)
        self.assertNotRegex(copied, r"\bsudo\b|\bcurl\b|\bsystemctl\b|\bpveum\b|"
                                   r"zpool\s+(?:scrub|clear|export|import|replace|destroy)")
        self.assert_cleanup(result)

    def test_success_preserves_both_streams_without_claiming_redaction(self):
        result, _ = self.exercise(list_result={"stdout": "capacity\n", "stderr": "list note\n"},
                                  status_result={"stdout": "health\n", "stderr": PRIVATE + "\n"})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertCountEqual(result.stdout.splitlines(), ["Pool capacity and health:",
            "capacity", "list note", "", "Pool device status:", "health", PRIVATE])
        self.assertEqual(result.stderr, "")
        self.assert_cleanup(result)

    def test_first_failure_stops_before_second_command_and_withholds_private_output(self):
        result, calls = self.exercise(list_result={"stdout": "partial\n", "stderr": PRIVATE, "exit": 3})
        self.assertEqual(result.returncode, 3)
        self.assertEqual(calls, [LIST_ARGS])
        self.assertEqual(result.stdout, "")
        self.assertIn("Pool status unavailable (exit 3)", result.stderr)
        self.assertNotIn(PRIVATE, result.stderr)
        self.assert_cleanup(result)

    def test_second_failure_withholds_the_successful_first_result_too(self):
        result, calls = self.exercise(list_result={"stdout": PRIVATE},
                                      status_result={"stdout": "partial", "stderr": PRIVATE, "exit": 4})
        self.assertEqual(result.returncode, 4)
        self.assertEqual(calls, [LIST_ARGS, STATUS_ARGS])
        self.assertEqual(result.stdout, "")
        self.assertIn("Pool status unavailable (exit 4)", result.stderr)
        self.assertNotIn(PRIVATE, result.stderr)
        self.assert_cleanup(result)

    def test_complete_result_at_byte_limit_is_not_truncated(self):
        body = "x" * (262144 - len(LABELS))
        result, calls = self.exercise(list_result={"stdout": body})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls, [LIST_ARGS, STATUS_ARGS])
        self.assertEqual(len(result.stdout.encode()), 262144)
        self.assertEqual(result.stdout.count("x"), len(body))
        self.assert_cleanup(result)

    def test_oversized_results_never_escape_the_private_capture(self):
        for stage in ("list", "status"):
            with self.subTest(stage=stage):
                settings = {stage + "_result": {"stdout": PRIVATE + "x" * 300000}}
                result, _ = self.exercise(**settings)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")
                self.assertIn("Pool status unavailable", result.stderr)
                self.assertNotIn(PRIVATE, result.stderr)
                self.assert_cleanup(result)

    def test_hung_commands_ignore_term_but_the_group_deadline_still_ends_them(self):
        for stage, hang in (("list", "open-pipes"), ("status", "closed-pipes")):
            with self.subTest(stage=stage):
                result, calls = self.exercise(**{stage + "_result": {"stdout": PRIVATE, "hang": hang}})
                self.assertEqual(result.returncode, 137, result.stderr)
                self.assertEqual(calls, [LIST_ARGS] if stage == "list" else [LIST_ARGS, STATUS_ARGS])
                self.assertEqual(result.stdout, "")
                self.assertIn("Pool status unavailable (exit 137)", result.stderr)
                self.assertNotIn(PRIVATE, result.stderr)
                self.assert_cleanup(result)

    def test_every_missing_utility_stops_before_either_zpool_command(self):
        for tool in ("timeout", "bash", "zpool", "head", "mktemp", "wc", "rm", "cat"):
            with self.subTest(tool=tool):
                result, calls = self.exercise(missing_tool=tool)
                self.assertEqual(result.returncode, 1, result.stderr)
                self.assertIsNone(calls)
                self.assertEqual(result.stdout, "")
                self.assertIn("no unbounded fallback", result.stderr)
                self.assertEqual(result.log_cleanup, [])

    def test_unavailable_private_temp_stops_before_reading_the_pool(self):
        result, calls = self.exercise(unavailable_temp=True)
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertIsNone(calls)
        self.assertEqual(result.stdout, "")
        self.assertIn("private temporary file could not be created", result.stderr)
        self.assertEqual(result.log_cleanup, [])

    def test_guide_keeps_native_deadline_privacy_and_recovery_limits_explicit(self):
        text = " ".join(section().split())
        for boundary in ("on the affected Proxmox host", "not inside the Pulse server container",
                         "already authorised", "do not add privileges", "read-only command can still wait",
                         "backup freeze/thaw", "not a new status command",
                         "deadline covers both commands and the byte limiter",
                         "kernel-blocked I/O operation may not exit promptly", "do not start another reader",
                         "not an empty pool or proof of health", "not an atomic snapshot",
                         "Pulse API collection, backup integrity or guest thaw",
                         "not sanitised", "device paths and serial numbers", "not the full result",
                         "does not erase terminal recordings"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)


if __name__ == "__main__":
    unittest.main()
