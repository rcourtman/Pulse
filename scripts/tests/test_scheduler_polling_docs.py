#!/usr/bin/env python3
"""Exercise copied scheduler diagnostics against secret-free local HTTP fixtures.

Run the HTTP tests through pulse-worker-source-proof for guest-local loopback.
The documentation, not a second hand-written client, supplies every command.
"""

from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/api/SCHEDULER_HEALTH.md"
ROLLOUT = ROOT / "docs/operations/ADAPTIVE_POLLING_ROLLOUT.md"
ENDPOINT = "/api/monitoring/scheduler/health"
TOKEN = "synthetic-scheduler-doc-token"


def blocks():
    return re.findall(r"```bash\n(.*?)```", DOC.read_text(), re.DOTALL)


def request():
    # This also selects the old first query for a parent-source differential.
    return next(block for block in blocks() if ENDPOINT in block)


def snapshot(enabled=True):
    return {
        "updatedAt": "2026-10-03T12:00:00Z", "enabled": enabled,
        "queue": {"depth": 3, "dueWithinSeconds": 1, "perType": {"pve": 3}},
        "deadLetter": {"count": 1, "tasks": []}, "breakers": [], "staleness": [],
        "instances": [
            {"key": "pve::healthy", "pollStatus": {"lastSuccess": "2026-10-03T11:59:58Z", "consecutiveFailures": 0},
             "breaker": {"state": "closed"}, "deadLetter": {"present": False}},
            {"key": "pve::retrying", "pollStatus": {"lastSuccess": "2026-10-03T11:59:00Z", "consecutiveFailures": 2},
             "breaker": {"state": "half_open"}, "deadLetter": {"present": False}},
            {"key": "pve::failed", "pollStatus": {"lastSuccess": "2026-10-03T11:55:00Z", "consecutiveFailures": 5},
             "breaker": {"state": "open"}, "deadLetter": {"present": True, "reason": "permanent_failure"}},
        ],
    }


@contextmanager
def server(payload, status=200):
    calls = []

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            calls.append({"path": self.path, "method": self.command, "headers": dict(self.headers)})
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(payload if isinstance(payload, bytes) else json.dumps(payload).encode())

        def log_message(self, *_args):
            pass

    httpd = HTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=httpd.serve_forever, daemon=True)
    thread.start()
    try:
        yield httpd.server_address[1], calls
    finally:
        httpd.shutdown()
        httpd.server_close()
        thread.join(timeout=5)


class SchedulerPollingDocsTest(unittest.TestCase):
    def exercise(self, payload=None, status=200, *, header="X-API-Token", missing_header=False,
                 refused_connection=False):
        payload = snapshot() if payload is None else payload
        with tempfile.TemporaryDirectory(prefix="pulse polling ") as temporary:
            directory = Path(temporary)
            home = directory / "home"
            credential = home / ".config/pulse/api-header"
            credential.parent.mkdir(parents=True, mode=0o700)
            if not missing_header:
                credential.write_text(f"{header}: {'Bearer ' if header == 'Authorization' else ''}{TOKEN}\n")
                credential.chmod(0o600)
            # --disable must defeat unsafe user defaults, not just omit -v itself.
            (home / ".curlrc").write_text("verbose\n")
            tools = directory / "tools"
            tools.mkdir()
            curl = tools / "curl"
            curl.write_text("#!/usr/bin/env python3\nimport json, os, sys\n"
                            "from pathlib import Path\n"
                            "Path(os.environ['CURL_ARGV']).write_text(json.dumps(sys.argv[1:]))\n"
                            "os.execv(os.environ['REAL_CURL'], ['curl', *sys.argv[1:]])\n")
            curl.chmod(0o700)
            argv_file = directory / "argv.json"
            env = dict(os.environ, HOME=str(home), TMPDIR=str(directory),
                       PATH=f"{tools}:{os.environ['PATH']}", CURL_ARGV=str(argv_file),
                       REAL_CURL=shutil.which("curl"))
            with server(payload, status) as (port, calls):
                # A closed fixture port supplies an actual connection refusal.
                if refused_connection:
                    with server(payload) as (closed_port, _):
                        pass
                    port = closed_port
                code = request().replace("http://127.0.0.1:7655", f"http://127.0.0.1:{port}")
                code = code.replace("http://HOST:7655", f"http://127.0.0.1:{port}")
                result = subprocess.run(["bash", "-c", code], env=env, text=True,
                                        capture_output=True, timeout=25)
                saved = list(directory.glob("pulse-scheduler.*/health.json"))
                files = [(path.read_bytes(), stat.S_IMODE(path.stat().st_mode),
                          stat.S_IMODE(path.parent.stat().st_mode)) for path in saved]
                return result, calls, json.loads(argv_file.read_text()), files

    def test_one_authenticated_request_with_both_supported_header_files(self):
        for header in ("X-API-Token", "Authorization"):
            with self.subTest(header=header):
                result, calls, args, files = self.exercise(header=header)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(len(calls), 1)
                self.assertEqual((calls[0]["path"], calls[0]["method"]), (ENDPOINT, "GET"))
                self.assertEqual(calls[0]["headers"][header],
                                 ("Bearer " if header == "Authorization" else "") + TOKEN)
                self.assertNotIn("Authorization" if header == "X-API-Token" else "X-API-Token",
                                 calls[0]["headers"])
                self.assertEqual(args[0], "--disable")
                self.assertNotIn(TOKEN, " ".join(args) + result.stdout + result.stderr)
                self.assertIn("Saved scheduler snapshot:", result.stdout)
                self.assertEqual(json.loads(files[0][0]), snapshot())
                self.assertEqual(files[0][1:], (0o600, 0o700))

    def test_request_has_bounded_timeouts_and_never_follows_redirects(self):
        _, _, args, _ = self.exercise()
        self.assertEqual(args[args.index("--connect-timeout") + 1], "5")
        self.assertEqual(args[args.index("--max-time") + 1], "15")
        self.assertNotIn("--location", args)
        self.assertNotIn("--insecure", args)

    def test_http_errors_and_redirects_never_become_successful_snapshots(self):
        for status in (301, 401, 403, 503):
            with self.subTest(status=status):
                # Even a body shaped like health must not hide a failed request.
                result, calls, _, _ = self.exercise(snapshot(), status)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(len(calls), 1)
                self.assertNotIn("Saved scheduler snapshot:", result.stdout)

    def test_transport_failure_is_not_an_empty_queue(self):
        result, calls, _, _ = self.exercise(refused_connection=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls, [])
        self.assertNotIn("Saved scheduler snapshot:", result.stdout)

    def test_missing_header_file_stops_without_an_anonymous_request(self):
        result, calls, _, _ = self.exercise(missing_header=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls, [])
        self.assertNotIn("Saved scheduler snapshot:", result.stdout)

    def test_invalid_json_and_unexpected_shapes_are_unavailable(self):
        for payload in (b"<html>login</html>", {}, {"enabled": True},
                        dict(snapshot(), enabled="true"), dict(snapshot(), instances=None)):
            with self.subTest(payload=payload):
                result, _, _, _ = self.exercise(payload)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("Saved scheduler snapshot:", result.stdout)

    def test_disabled_empty_scheduler_is_preserved_not_reported_as_healthy(self):
        payload = dict(snapshot(False), instances=[], queue={"depth": 0}, deadLetter={"count": 0})
        result, _, _, files = self.exercise(payload)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(json.loads(files[0][0])["enabled"])
        self.assertNotIn("healthy", result.stdout.lower())

    def test_local_queries_reuse_one_snapshot_and_preserve_failing_instances(self):
        queries = [block for block in blocks() if "jq " in block and ENDPOINT not in block]
        self.assertEqual(len(queries), 4)
        with tempfile.TemporaryDirectory(prefix="saved scheduler ") as temporary:
            path = Path(temporary) / "health.json"
            path.write_text(json.dumps(snapshot()))
            results = []
            for query in queries:
                self.assertNotIn("curl", query)
                code = query.replace("/absolute/path/to/health.json", str(path))
                result = subprocess.run(["bash", "-eu", "-c", code], capture_output=True,
                                        text=True, timeout=5)
                self.assertEqual(result.returncode, 0, result.stderr)
                decoder = json.JSONDecoder()
                text = result.stdout.strip()
                rows = []
                while text:
                    row, end = decoder.raw_decode(text)
                    rows.append(row)
                    text = text[end:].strip()
                results.append(rows)
            self.assertEqual(results[0], [{"updatedAt": snapshot()["updatedAt"], "enabled": True,
                                          "queueDepth": 3, "deadLetterCount": 1, "instanceCount": 3}])
            self.assertEqual([r["key"] for r in results[1]], ["pve::retrying", "pve::failed"])
            self.assertEqual(results[2], [{"key": "pve::failed", "reason": "permanent_failure"}])
            self.assertEqual(results[3], [{"key": "pve::retrying", "state": "half_open"},
                                          {"key": "pve::failed", "state": "open"}])

    def test_response_example_is_valid_json_with_the_current_field_names(self):
        example = json.loads(re.findall(r"```json\n(.*?)```", DOC.read_text(), re.DOTALL)[0])
        self.assertEqual(set(example), set(snapshot()))
        self.assertIsInstance(example["enabled"], bool)
        self.assertIn("lastSuccess", example["instances"][0]["pollStatus"])

    def test_rollout_restores_precedence_and_does_not_rewrite_shared_temp_settings(self):
        text = ROLLOUT.read_text()
        self.assertNotIn("sudo mv", text)
        self.assertNotIn("watch -n", text)
        self.assertNotIn('X-API-Token: $TOKEN', text)
        self.assertIn("environment value overrides `system.json`", text)
        self.assertIn("ADAPTIVE_POLLING_ENABLED=false", text)
        self.assertIn("merely removing `true`", text)
        self.assertIn("interrupts monitoring and notifications", text)
        self.assertIn("not a monitoring-health verdict", (ROOT / "docs/monitoring/ADAPTIVE_POLLING.md").read_text())


if __name__ == "__main__":
    unittest.main()
