#!/usr/bin/env python3
"""Exercise copied log-correlation recipes with synthetic service log readers.

This checks shell behaviour, not an installed Pulse service or Docker daemon.
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
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/TROUBLESHOOTING.md"
REQUEST_ID = "trace.123:abc"
JSON_LOG = json.dumps({
    "level": "warn", "request_id": REQUEST_ID, "status": 503,
    "message": "Request failed",
}, separators=(",", ":")) + "\n"
CONSOLE_LOG = f"WRN Request failed request_id={REQUEST_ID} status=503\n"
DECOY_LOG = "WRN Request failed request_id=traceX123:abc status=503\n"


def section() -> str:
    return DOC.read_text(encoding="utf-8").split(
        "### Correlate Logs with Requests\n", 1
    )[1].split("\n### ", 1)[0]


def recipes() -> dict[str, str]:
    blocks = re.findall(r"```bash\n(.*?)```", section(), re.DOTALL)
    return {"docker" if "# Docker" in block else "journalctl": block
            for block in blocks}


READER = f'''#!{sys.executable}
import json, os, pathlib, signal, sys, time
pathlib.Path(os.environ['READER_ARGV']).write_text(json.dumps(sys.argv[1:]))
sys.stdout.write(os.environ.get('LOG_STDOUT', ''))
sys.stderr.write(os.environ.get('LOG_STDERR', ''))
sys.stdout.flush()
sys.stderr.flush()
if os.environ.get('READER_HANG'):
    if os.environ['READER_HANG'] == 'ignore-term':
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
    time.sleep(30)
sys.exit(int(os.environ.get('READER_EXIT', '0')))
'''


def exercise_log_recipe(reader: str, copied: str, *, stdout: str = "", stderr: str = "",
                        exit_code: int = 0, hang: str = "", missing_timeout: bool = False):
    """Own and clean up only this synthetic shell's process group on failure."""
    with tempfile.TemporaryDirectory() as temporary:
        directory = Path(temporary)
        command = directory / reader
        command.write_text(READER, encoding="utf-8")
        command.chmod(0o700)
        argv = directory / "argv.json"
        path = str(directory) if missing_timeout else f"{directory}:{os.environ['PATH']}"
        env = dict(os.environ, PATH=path, READER_ARGV=str(argv), LOG_STDOUT=stdout,
                   LOG_STDERR=stderr, READER_EXIT=str(exit_code), READER_HANG=hang)
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
        return result, json.loads(argv.read_text()) if argv.exists() else None


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
                self.assertEqual(result.returncode, 124, result.stderr)
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
            'grep -F -- "$REQUEST_ID"', 'grep -F -- "request_id=$REQUEST_ID"'
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


if __name__ == "__main__":
    unittest.main()
