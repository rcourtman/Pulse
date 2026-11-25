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
def server(payload, status=200, *, incomplete_body=False):
    calls = []

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            calls.append({"path": self.path, "method": self.command, "headers": dict(self.headers)})
            body = payload if isinstance(payload, bytes) else json.dumps(payload).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body) + (10 if incomplete_body else 0)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_args):
            pass

    httpd = HTTPServer(("127.0.0.1", 0), Handler)
    # Poll only the fixture server more often so teardown need not wait 500 ms.
    # This does not shorten any copied request, reader or cleanup deadline.
    thread = threading.Thread(target=httpd.serve_forever,
                              kwargs={"poll_interval": 0.05}, daemon=True)
    thread.start()
    try:
        yield httpd.server_address[1], calls
    finally:
        httpd.shutdown()
        httpd.server_close()
        thread.join(timeout=5)


class SchedulerPollingDocsTest(unittest.TestCase):
    def exercise(self, payload=None, status=200, *, header="X-API-Token", missing_header=False,
                 refused_connection=False, incomplete_body=False):
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
            with server(payload, status, incomplete_body=incomplete_body) as (port, calls):
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
                # A plausible body cannot hide a failed read or leak its error text.
                payload = dict(snapshot(), error="private fixture error")
                result, calls, _, files = self.exercise(payload, status)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(len(calls), 1)
                self.assertEqual(result.stdout, "")
                expected = ("Check access to this Pulse instance." if status in (401, 403)
                            else "No poll result.")
                self.assertEqual(result.stderr,
                                 f"Scheduler snapshot unavailable (HTTP {status}). {expected}\n")
                self.assertEqual(len(files), 1)
                self.assertEqual(json.loads(files[0][0]), payload)
                self.assertEqual(files[0][1:], (0o600, 0o700))

    def test_transport_failure_is_not_an_empty_queue(self):
        result, calls, _, _ = self.exercise(refused_connection=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls, [])
        self.assertEqual(result.stdout, "")
        self.assertEqual(result.stderr, "Scheduler snapshot unavailable (curl exit 7). No poll result.\n")

    def test_short_transfer_with_http_200_is_not_a_saved_snapshot(self):
        result, calls, _, files = self.exercise(incomplete_body=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(calls), 1)
        self.assertEqual(result.stdout, "")
        self.assertEqual(result.stderr, "Scheduler snapshot unavailable (curl exit 18). No poll result.\n")
        self.assertEqual(json.loads(files[0][0]), snapshot())
        self.assertEqual(files[0][1:], (0o600, 0o700))

    def test_missing_header_file_stops_without_an_anonymous_request(self):
        result, calls, _, _ = self.exercise(missing_header=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls, [])
        self.assertNotIn("Saved scheduler snapshot:", result.stdout)

    def test_invalid_json_and_unexpected_shapes_are_unavailable(self):
        for payload in (b"<html>login</html>", b"", b"{}\n" + json.dumps(snapshot()).encode(),
                        {}, {"enabled": True},
                        dict(snapshot(), enabled="true"), dict(snapshot(), instances=None)):
            with self.subTest(payload=payload):
                result, _, _, _ = self.exercise(payload)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")
                self.assertEqual(result.stderr,
                                 "Scheduler snapshot unavailable. The response was not a scheduler snapshot.\n")

    def test_fixed_timing_empty_queue_is_preserved_not_reported_as_healthy(self):
        payload = dict(snapshot(False), instances=[], queue={"depth": 0}, deadLetter={"count": 0})
        result, _, _, files = self.exercise(payload)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(json.loads(files[0][0])["enabled"])
        self.assertNotIn("healthy", result.stdout.lower())

    def test_local_queries_reuse_one_snapshot_and_preserve_failing_instances(self):
        queries = [block for block in blocks() if "jq " in block and ENDPOINT not in block]
        self.assertEqual(len(queries), 5)
        body = snapshot()
        body["instances"][1]["pollStatus"]["lastError"] = {
            "at": "2026-10-03T11:59:30Z", "category": "transient", "message": "private fixture error",
        }
        body["instances"].extend([
            {"key": "pmg::partial", "connection": "https://private-fixture.invalid",
             "pollStatus": {"lastSuccess": "2026-10-03T11:59:58Z", "consecutiveFailures": 0,
                            "lastError": {"at": "2026-10-03T11:59:58Z", "category": "partial",
                                          "message": "private fixture error"}},
             "breaker": {"state": "closed"}, "deadLetter": {"present": False}},
            {"key": "pbs::no-result-yet", "pollStatus": {"consecutiveFailures": 0},
             "breaker": {"state": "closed"}, "deadLetter": {"present": False}},
        ])
        with tempfile.TemporaryDirectory(prefix="saved scheduler ") as temporary:
            path = Path(temporary) / "health.json"
            path.write_text(json.dumps(body))
            results = []
            for query in queries:
                self.assertNotIn("curl", query)
                code = query.replace("/absolute/path/to/health.json", str(path))
                result = subprocess.run(["bash", "-eu", "-c", code], capture_output=True,
                                        text=True, timeout=5)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertNotIn("private fixture error", result.stdout)
                self.assertNotIn("private-fixture.invalid", result.stdout)
                decoder = json.JSONDecoder()
                text = result.stdout.strip()
                rows = []
                while text:
                    row, end = decoder.raw_decode(text)
                    rows.append(row)
                    text = text[end:].strip()
                results.append(rows)
            self.assertEqual(results[0], [{"updatedAt": snapshot()["updatedAt"], "enabled": True,
                                          "queueDepth": 3, "deadLetterCount": 1, "instanceCount": 5}])
            self.assertEqual(results[1], [
                {"key": "pve::retrying", "lastSuccess": "2026-10-03T11:59:00Z", "failures": 2,
                 "lastErrorAt": "2026-10-03T11:59:30Z", "lastErrorCategory": "transient"},
                {"key": "pve::failed", "lastSuccess": "2026-10-03T11:55:00Z", "failures": 5,
                 "lastErrorAt": None, "lastErrorCategory": None},
                {"key": "pmg::partial", "lastSuccess": "2026-10-03T11:59:58Z", "failures": 0,
                 "lastErrorAt": "2026-10-03T11:59:58Z", "lastErrorCategory": "partial"},
            ])
            self.assertEqual(results[2], [{"key": "pve::failed", "reason": "permanent_failure"}])
            self.assertEqual(results[3], [{"key": "pve::retrying", "state": "half_open"},
                                          {"key": "pve::failed", "state": "open"}])

    def test_fixed_timing_comparison_distinguishes_progress_failure_and_no_result(self):
        first = "2026-10-03T12:00:00Z"
        second = "2026-10-03T12:03:00Z"
        query = next(block for block in blocks() if "polls: [" in block)

        def payload(observed_at, healthy, failing, partial):
            def row(key, status):
                return {"key": key, "connection": "https://private-fixture.invalid", "pollStatus": status}

            def error(at, category):
                return {"at": at, "category": category, "message": "private fixture error"}

            return dict(snapshot(False), updatedAt=observed_at, instances=[
                row("pve::progress", {"lastSuccess": healthy, "consecutiveFailures": 0}),
                row("pbs::failing", {"lastSuccess": first, "lastError": error(failing, "transient"),
                                     "consecutiveFailures": 2}),
                row("pve::partial", {"lastSuccess": partial, "lastError": error(partial, "partial"),
                                     "consecutiveFailures": 0}),
                row("pbs::unchanged", {"lastSuccess": first, "consecutiveFailures": 0}),
                row("pbs::unseen", {"consecutiveFailures": 0}),
            ])

        with tempfile.TemporaryDirectory(prefix="scheduler comparison ") as temporary:
            path = Path(temporary) / "health.json"
            results = []
            for body in (payload(first, first, first, first), payload(second, second, second, second)):
                path.write_text(json.dumps(body))
                result = subprocess.run(
                    ["bash", "-eu", "-c", query.replace("/absolute/path/to/health.json", str(path))],
                    capture_output=True, text=True, timeout=5,
                )
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertNotIn("private-fixture.invalid", result.stdout)
                self.assertNotIn("private fixture error", result.stdout)
                value = json.loads(result.stdout)
                self.assertFalse(value["enabled"])
                results.append((value["updatedAt"], {
                    item["key"]: (item["lastSuccess"], item["lastErrorAt"], item["lastErrorCategory"],
                                  item["consecutiveFailures"])
                    for item in value["polls"]
                }))
            for snapshot_at, rows in results:
                self.assertEqual(rows, {
                    "pve::progress": (snapshot_at, None, None, 0),
                    "pbs::failing": (first, snapshot_at, "transient", 2),
                    "pve::partial": (snapshot_at, snapshot_at, "partial", 0),
                    "pbs::unchanged": (first, None, None, 0),
                    "pbs::unseen": (None, None, None, 0),
                })
            self.assertEqual([snapshot_at for snapshot_at, _ in results], [first, second])

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
