#!/usr/bin/env python3
"""Exercise copied incident-log recipes with synthetic service log readers.

This checks shell behaviour, not an installed Pulse service or Docker daemon.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import re
import select
import shutil
import signal
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import Mock, call, patch


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/TROUBLESHOOTING.md"
REQUEST_ID = "trace.123:abc"
JSON_LOG = json.dumps({
    "level": "warn", "request_id": REQUEST_ID, "status": 503,
    "message": "Request failed",
}, separators=(",", ":")) + "\n"
CONSOLE_LOG = f"WRN Request failed request_id={REQUEST_ID} status=503\n"
DECOY_LOG = "WRN Request failed request_id=traceX123:abc status=503\n"


def section(heading: str = "### Correlate Logs with Requests") -> str:
    return DOC.read_text(encoding="utf-8").split(
        heading + "\n", 1
    )[1].split("\n### ", 1)[0]


def recipes(heading: str = "### Correlate Logs with Requests") -> dict[str, str]:
    blocks = re.findall(r"```bash\n(.*?)```", section(heading), re.DOTALL)
    return {"docker" if "# Docker" in block else "journalctl": block
            for block in blocks}


READER = f'''#!{sys.executable}
import json, os, pathlib, signal, sys, time
pathlib.Path(os.environ['READER_ARGV']).write_text(json.dumps(sys.argv[1:]))
pathlib.Path(os.environ['READER_PID']).write_text(json.dumps({{
    'pid': os.getpid(),
    'start': pathlib.Path('/proc/self/stat').read_text().split(')', 1)[1].split()[19],
}}))
sys.stdout.write(pathlib.Path(os.environ['LOG_STDOUT_FILE']).read_text())
sys.stderr.write(pathlib.Path(os.environ['LOG_STDERR_FILE']).read_text())
sys.stdout.flush()
sys.stderr.flush()
if os.environ.get('READER_HANG'):
    if os.environ['READER_HANG'] in ('ignore-term', 'ignore-term-closed-pipes'):
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
    if os.environ['READER_HANG'] == 'ignore-term-closed-pipes':
        os.close(1)
        os.close(2)
    time.sleep(30)
sys.exit(int(os.environ.get('READER_EXIT', '0')))
'''


def reader_state(pid: int):
    """Read identity/state only for the PID recorded by our synthetic reader."""
    try:
        fields = Path(f"/proc/{pid}/stat").read_text().rsplit(")", 1)[1].split()
    except FileNotFoundError:
        return None
    return {"state": fields[0], "start": fields[19]}


def observe_reader(identity):
    """Keep the 500 ms exit bound, then separately prove owned emergency cleanup.

    A pidfd exit event includes zombies and cannot refer to a recycled PID.
    Check the recorded start time before waiting or signalling that handle.
    """
    report = {"pid": identity["pid"], "state": None, "survived": False,
              "identity": "gone", "cleanup": "not-needed"}
    try:
        pid_fd = os.pidfd_open(identity["pid"])
    except ProcessLookupError:
        return report
    try:
        state = reader_state(identity["pid"])
        if state is None:
            return report
        report["state"] = state["state"]
        if state["start"] != identity["start"]:
            report["identity"] = "changed"
            return report
        report["identity"] = "matched"
        poller = select.poll()
        poller.register(pid_fd, select.POLLIN)
        if poller.poll(500):
            return report
        state = reader_state(identity["pid"])
        report["state"] = state["state"] if state else None
        # Recheck after the last observation, without extending its deadline.
        # The old sample/sleep loop could flag an exit during its final sleep.
        if (state is None or state["start"] != identity["start"]
                or poller.poll(0)):
            return report
        report["survived"] = True
        try:
            signal.pidfd_send_signal(pid_fd, signal.SIGKILL)
        except ProcessLookupError:
            pass
        # Cleanup is not extra time to pass: a survivor still fails the test.
        report["cleanup"] = "terminated" if poller.poll(500) else "unobserved"
        return report
    finally:
        os.close(pid_fd)


def exercise_log_recipe(reader: str, copied: str, *, stdout: str = "", stderr: str = "",
                        exit_code: int = 0, hang: str = "", missing_timeout: bool = False,
                        missing_tool: str = "", unavailable_temp: bool = False):
    """Own and clean up only this synthetic shell's process group on failure."""
    with tempfile.TemporaryDirectory() as temporary:
        directory = Path(temporary)
        command = directory / reader
        command.write_text(READER, encoding="utf-8")
        command.chmod(0o700)
        argv = directory / "argv.json"
        pid_file = directory / "reader.pid"
        stdout_file, stderr_file = directory / "stdout.txt", directory / "stderr.txt"
        # Files keep the large-record controls out of exec's argument/environment
        # size limit. These are synthetic bytes, never a real journal or token.
        stdout_file.write_text(stdout, encoding="utf-8")
        stderr_file.write_text(stderr, encoding="utf-8")
        log_directory = directory / "log-temporary"
        log_directory.mkdir(mode=0o700)
        cleanup_file = directory / "cleanup.jsonl"
        real_rm = shutil.which("rm")
        (directory / "rm").write_text(f'''#!{sys.executable}
import json, os, pathlib, stat, sys
path = pathlib.Path(sys.argv[2])
assert sys.argv[1] == '--' and len(sys.argv) == 3
assert path.parent == pathlib.Path(os.environ['LOG_TEMPORARY'])
assert path.name.startswith('pulse-log.')
info = path.stat()
with pathlib.Path(os.environ['LOG_CLEANUP']).open('a') as receipt:
    receipt.write(json.dumps({{'mode': stat.S_IMODE(info.st_mode), 'bytes': info.st_size}}) + '\\n')
os.execv({real_rm!r}, [{real_rm!r}] + sys.argv[1:])
''', encoding="utf-8")
        (directory / "rm").chmod(0o700)
        if missing_tool:
            for tool in ("bash", "timeout", "head", "mktemp", "wc", "cat", "grep"):
                if tool != missing_tool:
                    (directory / tool).symlink_to(shutil.which(tool))
            if missing_tool == "rm":
                (directory / "rm").unlink()
        path = (str(directory) if missing_timeout or missing_tool
                else f"{directory}:{os.environ['PATH']}")
        env = dict(os.environ, PATH=path, READER_ARGV=str(argv), READER_PID=str(pid_file),
                   LOG_STDOUT_FILE=str(stdout_file), LOG_STDERR_FILE=str(stderr_file),
                   READER_EXIT=str(exit_code), READER_HANG=hang,
                   LOG_TEMPORARY=str(log_directory), LOG_CLEANUP=str(cleanup_file),
                   TMPDIR=str(log_directory / "absent" if unavailable_temp else log_directory))
        with subprocess.Popen([shutil.which("bash"), "-c", copied], env=env,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                              text=True, start_new_session=True) as process:
            try:
                output, errors = process.communicate(timeout=10)
            except subprocess.TimeoutExpired:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                process.communicate(timeout=5)
                raise
        result = subprocess.CompletedProcess(process.args, process.returncode, output, errors)
        result.log_cleanup = ([json.loads(line) for line in cleanup_file.read_text().splitlines()]
                              if cleanup_file.exists() else [])
        # The reader is a child of timeout/the collection shell, not ours to
        # waitpid. Enforce survival in the helper, not just selected callers.
        # Do not print log contents when reporting a failed collection.
        result.reader_running = False
        if pid_file.exists():
            result.reader_observation = observe_reader(json.loads(pid_file.read_text()))
            result.reader_running = result.reader_observation["survived"]
            if result.reader_running:
                raise AssertionError(
                    "synthetic reader survived the copied collection shell; "
                    f"shell_exit={result.returncode}; "
                    f"reader={json.dumps(result.reader_observation, sort_keys=True)}"
                )
        if list(log_directory.iterdir()):
            raise AssertionError("copied recipe left a private log file behind; "
                                 f"shell_exit={result.returncode}")
        return result, json.loads(argv.read_text()) if argv.exists() else None


class SyntheticReaderObservationTest(unittest.TestCase):
    def test_exit_at_final_observation_is_not_reported_as_survival(self):
        poller = Mock()
        poller.poll.side_effect = [[], [(41, select.POLLIN)]]
        with patch.object(os, "pidfd_open", return_value=41), \
                patch.object(os, "close"), \
                patch(__name__ + ".reader_state", return_value={"state": "S", "start": "7"}), \
                patch.object(select, "poll", return_value=poller), \
                patch.object(signal, "pidfd_send_signal") as stop:
            report = observe_reader({"pid": 123, "start": "7"})
        self.assertFalse(report["survived"])
        self.assertEqual(poller.poll.call_args_list, [call(500), call(0)])
        stop.assert_not_called()

    def test_recycled_pid_is_neither_waited_on_nor_signalled(self):
        with patch.object(os, "pidfd_open", return_value=41), \
                patch.object(os, "close"), \
                patch(__name__ + ".reader_state", return_value={"state": "S", "start": "8"}), \
                patch.object(select, "poll") as poll, \
                patch.object(signal, "pidfd_send_signal") as stop:
            report = observe_reader({"pid": 123, "start": "7"})
        self.assertEqual(report["identity"], "changed")
        poll.assert_not_called()
        stop.assert_not_called()

    def test_unobserved_cleanup_is_not_reported_as_termination(self):
        poller = Mock()
        poller.poll.return_value = []
        with patch.object(os, "pidfd_open", return_value=41), \
                patch.object(os, "close"), \
                patch(__name__ + ".reader_state", return_value={"state": "S", "start": "7"}), \
                patch.object(select, "poll", return_value=poller), \
                patch.object(signal, "pidfd_send_signal") as stop:
            report = observe_reader({"pid": 123, "start": "7"})
        self.assertTrue(report["survived"])
        self.assertEqual(report["cleanup"], "unobserved")
        self.assertEqual(poller.poll.call_args_list, [call(500), call(0), call(500)])
        stop.assert_called_once_with(41, signal.SIGKILL)

    def test_leaked_fixture_fails_with_shell_state_and_observed_cleanup(self):
        # Deliberately unsafe *synthetic control*, never a published recipe.
        # Let the shell finish while only its recorded fake reader stays alive.
        copied = '''
docker logs pulse </dev/null >/dev/null 2>&1 &
for attempt in {1..100}; do
  [ ! -s "$READER_PID" ] || exit 0
  sleep 0.01
done
exit 3
'''
        with self.assertRaisesRegex(
                AssertionError, r'shell_exit=0; reader=.*"cleanup": "terminated".*"state": "[RS]"'):
            exercise_log_recipe("docker", copied, hang="ignore-term-closed-pipes")


class TroubleshootingLogRecipesTest(unittest.TestCase):
    def exercise(self, reader: str, *, stdout: str = "", stderr: str = "",
                 exit_code: int = 0, recipe: str | None = None, **settings):
        copied = (recipe if recipe is not None else recipes()[reader]).replace(
            "REQUEST_ID='abc123'", f"REQUEST_ID='{REQUEST_ID}'"
        )
        result, argv = exercise_log_recipe(reader, copied, stdout=stdout, stderr=stderr,
                                          exit_code=exit_code, **settings)
        expected = (["-u", "pulse", "--since", "15 minutes ago", "--lines", "1000", "--no-pager"]
                    if reader == "journalctl" else ["logs", "--since", "15m", "--tail", "1000", "pulse"])
        self.assertEqual(argv, None if settings.get("missing_timeout") else expected)
        return result

    def test_shipped_document_is_the_same_guide(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes())
        self.assertEqual(set(recipes()), {"journalctl", "docker"})

    def test_literal_id_matches_json_and_console_without_regex_false_matches(self):
        for reader in recipes():
            for log in (JSON_LOG, CONSOLE_LOG):
                with self.subTest(reader=reader, format=log):
                    result = self.exercise(reader, stdout=DECOY_LOG + log)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(result.stdout, log)
                    self.assertEqual(result.stderr, "")

    def test_logs_written_to_stderr_are_searchable_for_both_readers(self):
        for reader in recipes():
            with self.subTest(reader=reader):
                result = self.exercise(reader, stdout=DECOY_LOG, stderr=JSON_LOG)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout, JSON_LOG)

    def test_no_match_is_nonzero_and_does_not_print_unrelated_logs(self):
        for reader in recipes():
            with self.subTest(reader=reader):
                result = self.exercise(reader, stdout=DECOY_LOG)
                self.assertEqual(result.returncode, 1)
                self.assertEqual(result.stdout, "")
                self.assertEqual(result.stderr, "")

    def test_log_reader_failure_remains_failure_even_with_a_matching_partial_log(self):
        error = "synthetic log-reader access failure\n"
        for reader in recipes():
            with self.subTest(reader=reader):
                result = self.exercise(reader, stdout=JSON_LOG, stderr=error, exit_code=2)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stdout, "")
                self.assertNotIn(error, result.stderr)
                self.assertIn("Log read unavailable (exit 2)", result.stderr)

    def test_hung_readers_withhold_partial_matches_and_stop_by_deadline(self):
        for reader in recipes():
            with self.subTest(reader=reader):
                result = self.exercise(reader, stdout=JSON_LOG, hang="term")
                self.assertEqual(result.returncode, 137, result.stderr)
                self.assertEqual(result.stdout, "")
                self.assertIn("Log read unavailable", result.stderr)

    def test_missing_timeout_stops_before_either_reader(self):
        for reader in recipes():
            with self.subTest(reader=reader):
                result = self.exercise(reader, missing_timeout=True)
                self.assertEqual(result.returncode, 1)
                self.assertEqual(result.stdout, "")
                self.assertIn("no unbounded fallback", result.stderr)

    def test_console_only_selector_is_a_negative_control_for_json(self):
        copied = recipes()["journalctl"].replace(
            'grep -aF -- "$REQUEST_ID"', 'grep -aF -- "request_id=$REQUEST_ID"'
        )
        result = self.exercise("journalctl", stdout=JSON_LOG, recipe=copied)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, "")

    def test_guidance_keeps_browser_credentials_and_missing_logs_in_context(self):
        guide = " ".join(section().split())
        for boundary in ("Copy only the `X-Request-ID`", "Copy as cURL",
                         "WebSocket upgrades", "not sanitised", "HTTP 5xx",
                         "HTTP 4xx", "not successful requests", "debug level",
                         "does not prove the request succeeded",
                         "rather than repeating the failed action",
                         "do not enable debug logging or retry a state-changing request"):
            self.assertIn(boundary, guide)
        self.assertNotRegex("\n".join(recipes().values()),
                            r"curl\b|--follow\b|\s-f\b|--token\b|--api-token\b")


class GeneralLogEvidenceTest(unittest.TestCase):
    """The general help entry must reach the same bounded readers, without an ID."""

    def test_getting_help_reaches_bounded_readers_without_a_second_attempt(self):
        help_text = " ".join(section("## 🆘 Getting Help").split())
        self.assertIn("[bounded Pulse log readers](#inspect-notification-logs)", help_text)
        self.assertNotRegex(help_text, r"journalctl\b|docker logs\b")
        for boundary in ("Pulse server's deployment, not the monitored target",
                         "record limit alone does not bound a hung reader",
                         "do not use an unbounded substitute",
                         "No request ID is required",
                         "do not repeat the failed action"):
            self.assertIn(boundary, help_text)
        self.assertIn("also apply to other server errors",
                      " ".join(section("### Inspect Notification Logs").split()))

    def exercise(self, reader: str, **settings):
        copied = recipes("### Inspect Notification Logs")[reader]
        self.assertNotIn("REQUEST_ID", copied)
        result, argv = exercise_log_recipe(reader, copied, **settings)
        expected = (["-u", "pulse", "--since", "15 minutes ago", "--lines", "200", "--no-pager"]
                    if reader == "journalctl" else ["logs", "--since", "15m", "--tail", "200", "pulse"])
        self.assertEqual(argv, None if settings.get("missing_timeout") else expected)
        return result

    def test_general_readers_keep_complete_stdout_and_stderr_without_an_id(self):
        for reader in recipes("### Inspect Notification Logs"):
            with self.subTest(reader=reader):
                result = self.exercise(reader, stdout="server error\n", stderr="detail\n")
                self.assertEqual(result.returncode, 0, result.stderr)
                # The two streams can flush in either order; require both
                # records exactly once, not cross-stream chronology.
                self.assertCountEqual(result.stdout.splitlines(), ["server error", "detail"])
                self.assertEqual(result.stderr, "")

    def test_general_readers_withhold_partial_errors_on_failed_reads(self):
        for reader in recipes("### Inspect Notification Logs"):
            with self.subTest(reader=reader):
                result = self.exercise(reader, stdout="partial error\n",
                                       stderr="private access error\n", exit_code=2)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stdout, "")
                self.assertIn("Log read unavailable (exit 2)", result.stderr)
                self.assertNotIn("private access error", result.stderr)

    def test_general_readers_stop_and_withhold_partial_errors_at_real_deadline(self):
        for reader in recipes("### Inspect Notification Logs"):
            with self.subTest(reader=reader):
                result = self.exercise(reader, stdout="partial error\n", hang="term")
                self.assertEqual(result.returncode, 137, result.stderr)
                self.assertEqual(result.stdout, "")
                self.assertIn("Log read unavailable (exit 137)", result.stderr)

    def test_general_readers_do_not_fall_back_when_timeout_is_missing(self):
        for reader in recipes("### Inspect Notification Logs"):
            with self.subTest(reader=reader):
                result = self.exercise(reader, missing_timeout=True)
                self.assertEqual(result.returncode, 1)
                self.assertEqual(result.stdout, "")
                self.assertIn("no unbounded fallback", result.stderr)


class SeverityLogEvidenceTest(unittest.TestCase):
    """Priority filtering must not bypass the incident reader's safety bounds."""

    heading = "#### Filter Pulse's systemd journal by severity"

    def copied(self):
        # Stop at the next H4; the generic section helper stops at H3.
        text = section(self.heading).split("\n#### ", 1)[0]
        blocks = re.findall(r"```bash\n(.*?)```", text, re.DOTALL)
        self.assertEqual(len(blocks), 1)
        return blocks[0]

    def exercise(self, **settings):
        result, argv = exercise_log_recipe("journalctl", self.copied(), **settings)
        expected = ["-u", "pulse", "-p", "warning", "--since", "15 minutes ago",
                    "--lines", "200", "--no-pager"]
        self.assertEqual(argv, None if settings.get("missing_timeout") else expected)
        return result

    def test_priority_reader_keeps_incident_bounds_without_privilege_or_mutation(self):
        copied = self.copied()
        for bound in ("--signal=KILL 8s", "--since '15 minutes ago'",
                      "--lines 200", "--no-pager"):
            self.assertIn(bound, copied)
        self.assertNotRegex(copied, r"\b(?:sudo|systemctl|curl|grep)\b|--follow")
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes())

    def test_success_keeps_both_complete_streams(self):
        result = self.exercise(stdout=JSON_LOG, stderr=CONSOLE_LOG)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertCountEqual(result.stdout.splitlines(),
                              (JSON_LOG + CONSOLE_LOG).splitlines())
        self.assertEqual(result.stderr, "")

    def test_failed_reader_withholds_partial_warning_and_private_error(self):
        result = self.exercise(stdout=JSON_LOG, stderr="synthetic private error\n", exit_code=2)
        self.assertEqual(result.returncode, 2)
        self.assertEqual(result.stdout, "")
        self.assertIn("Log read unavailable (exit 2)", result.stderr)
        self.assertNotIn("synthetic private error", result.stderr)

    def test_real_deadline_withholds_partial_warning(self):
        result = self.exercise(stdout=JSON_LOG, hang="term")
        self.assertEqual(result.returncode, 137, result.stderr)
        self.assertEqual(result.stdout, "")
        self.assertIn("Log read unavailable (exit 137)", result.stderr)

    def test_missing_timeout_stops_before_reading(self):
        result = self.exercise(missing_timeout=True)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, "")
        self.assertIn("no unbounded fallback", result.stderr)

    def test_empty_success_does_not_invent_a_warning_or_reader_error(self):
        result = self.exercise()
        self.assertEqual((result.returncode, result.stdout, result.stderr), (0, "", ""))

    def test_old_unit_and_sharing_guidance_does_not_require_incident_reinstallation(self):
        guide = " ".join(section(self.heading).split("\n#### ", 1)[0].split())
        for boundary in ("inside the Pulse container, not on the Proxmox host",
                         "not sanitised", "#inspect-notification-logs",
                         "An empty filtered read does not prove", "info priority",
                         "bounded unfiltered systemd reader", "check locally",
                         "do not post the full unit or environment",
                         "Do not reinstall or restart Pulse just to obtain logs",
                         "planned maintenance window", "settings and data"):
            self.assertIn(boundary, guide)
        self.assertNotIn("systemctl cat", guide)


class LogByteBoundsTest(unittest.TestCase):
    """Read the actual server and agent entry points, not a duplicate helper."""

    def copied_recipes(self):
        copied = []
        for heading in ("### Inspect Notification Logs", "### Correlate Logs with Requests"):
            copied.extend((heading, reader, recipe)
                          for reader, recipe in recipes(heading).items())
        copied.append(("severity", "journalctl", SeverityLogEvidenceTest().copied()))
        agent_doc = (ROOT / "docs/UNIFIED_AGENT.md").read_text()
        agent_section = agent_doc.split("### Collect agent logs safely\n", 1)[1].split("\n### ", 1)[0]
        agent_blocks = re.findall(r"```bash\n(.*?)```", agent_section, re.DOTALL)
        self.assertEqual(len(agent_blocks), 1)
        copied.append(("agent", "journalctl", agent_blocks[0]))
        self.assertEqual(len(copied), 6)
        return copied

    def exercise(self, reader, recipe, **settings):
        result, argv = exercise_log_recipe(reader, recipe, **settings)
        self.assertFalse(result.reader_running, "reader survived the copied collection shell")
        for cleanup in result.log_cleanup:
            self.assertEqual(cleanup["mode"], 0o600)
            self.assertLessEqual(cleanup["bytes"], 262145)
        return result, argv

    def test_exact_byte_limit_preserves_a_complete_record_and_private_file_cleanup(self):
        log = "abc123 " + "x" * (262144 - 8) + "\n"
        self.assertEqual(len(log.encode()), 262144)
        for heading, reader, recipe in self.copied_recipes():
            with self.subTest(section=heading, reader=reader):
                result, _ = self.exercise(reader, recipe, stdout=log)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout, log)
                self.assertEqual(result.stderr, "")
                self.assertEqual(result.log_cleanup, [{"mode": 0o600, "bytes": 262144}])

    def test_single_large_record_and_large_stderr_are_unavailable_not_a_partial_match(self):
        for heading, reader, recipe in self.copied_recipes():
            for stream in ("stdout", "stderr"):
                with self.subTest(section=heading, reader=reader, stream=stream):
                    result, _ = self.exercise(reader, recipe, **{stream: "abc123 " + "x" * 1048576})
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(result.stdout, "")
                    self.assertIn("unavailable", result.stderr)
                    self.assertNotIn("abc123", result.stderr)
                    self.assertEqual(result.log_cleanup, [{"mode": 0o600, "bytes": 262145}])

    def test_limit_counts_actual_bytes_including_unicode_nuls_and_trailing_newlines(self):
        for heading, reader, recipe in self.copied_recipes():
            for log in ("abc123" + "\n" * 262139, "é" * 131073, "\0" * 262145):
                with self.subTest(section=heading, reader=reader, first_byte=repr(log[0])):
                    result, _ = self.exercise(reader, recipe, stdout=log)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(result.stdout, "")
                    self.assertIn("unavailable", result.stderr)
                    self.assertEqual(result.log_cleanup, [{"mode": 0o600, "bytes": 262145}])

    def test_failed_reader_still_has_no_excerpt_and_removes_its_private_buffer(self):
        for heading, reader, recipe in self.copied_recipes():
            with self.subTest(section=heading, reader=reader):
                result, _ = self.exercise(reader, recipe, stdout="abc123 partial\n", exit_code=2)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stdout, "")
                self.assertEqual(len(result.log_cleanup), 1)

    def test_missing_limiter_or_private_temp_path_stops_before_the_reader(self):
        for heading, reader, recipe in self.copied_recipes():
            for settings in ({"missing_tool": "head"}, {"unavailable_temp": True}):
                with self.subTest(section=heading, reader=reader, settings=settings):
                    result, argv = self.exercise(reader, recipe, **settings)
                    self.assertIsNone(argv)
                    self.assertEqual(result.returncode, 1)
                    self.assertEqual(result.stdout, "")
                    self.assertIn("unavailable", result.stderr)
                    self.assertEqual(result.log_cleanup, [])

    def test_hard_deadline_terminates_closed_pipe_term_resistant_readers_and_cleans_buffer(self):
        selected = [("docker", recipes("### Inspect Notification Logs")["docker"]),
                    ("journalctl", self.copied_recipes()[-1][2])]
        for reader, recipe in selected:
            with self.subTest(reader=reader):
                result, _ = self.exercise(reader, recipe, stdout="partial private log\n",
                                          hang="ignore-term-closed-pipes")
                self.assertEqual(result.returncode, 137, result.stderr)
                self.assertEqual(result.stdout, "")
                self.assertNotIn("partial private log", result.stderr)
                self.assertEqual(len(result.log_cleanup), 1)

    def test_each_entry_keeps_collection_deadline_pipe_failures_and_byte_limiter_together(self):
        for heading, reader, recipe in self.copied_recipes():
            with self.subTest(section=heading, reader=reader):
                subprocess.run(["bash", "-n", "-c", recipe], check=True, capture_output=True)
                for bound in ("umask 077", "timeout --signal=KILL 8s bash -o pipefail",
                              "2>&1 | head -c 262145", '" -gt 262144 ]', "trap 'rm --"):
                    self.assertIn(bound, recipe)
                self.assertNotIn("--foreground", recipe)


if __name__ == "__main__":
    unittest.main()
