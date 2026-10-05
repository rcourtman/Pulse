#!/usr/bin/env python3
"""Exercise TrueNAS connection and diagnostic recipes safely.

All credentials are synthetic. HTTP checks require the offline source-proof VM.
Log-reader controls establish bounds and exit handling, not native NAS recovery
or automatic redaction.
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
DOC = ROOT / "docs/TRUENAS.md"
TOKEN = "synthetic-pulse-token"
NAS_KEY = "synthetic-truenas-key"
PAYLOAD = {"name": "test-nas", "host": "https://nas.example.invalid",
           "username": "key-owner", "apiKey": NAS_KEY}


def commands():
    section = DOC.read_text().split("### Testing and adding a connection (API)\n", 1)[1]
    section = section.split("\n## Troubleshooting", 1)[0]
    blocks = re.findall(r"```bash\n(.*?)```", section, re.DOTALL)
    if len(blocks) != 3:
        raise AssertionError("expected separate preparation, test and save recipes")
    return blocks[0], blocks[1:]


@contextmanager
def server(status=200):
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            body = self.rfile.read(int(self.headers["Content-Length"]))
            requests.append((self.path, dict(self.headers), json.loads(body)))
            self.send_response(status if status != 200 or self.path.endswith("/test") else 201)
            self.end_headers()
            self.wfile.write(b'{"fixture":true}\n')

        def log_message(self, *_args):
            pass

    http = HTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=http.serve_forever, daemon=True)
    thread.start()
    try:
        yield http.server_address[1], requests
    finally:
        http.shutdown()
        http.server_close()
        thread.join(timeout=5)


class TrueNASDocsTest(unittest.TestCase):
    def test_shipped_copy_and_safe_diagnostics(self):
        self.assertEqual(DOC.read_bytes(), (ROOT / "frontend-modern/public/docs/TRUENAS.md").read_bytes())
        text = DOC.read_text()
        bash = "\n".join(re.findall(r"```bash\n(.*?)```", text, re.DOTALL))
        self.assertNotRegex(bash, r"Authorization:|Bearer\s|\$TOKEN|apiKey|--insecure")
        self.assertIn("journalctl -u pulse --since '15 minutes ago' --lines 100 --no-pager", bash)
        self.assertIn("docker logs --since 15m --tail 100 pulse", bash)
        self.assertIn("does not establish a live reading", text)
        self.assertIn("Do not upload a full browser network capture", text)

    def log_section(self):
        return DOC.read_text().split("### No data appearing after adding connection\n", 1)[1].split(
            "### Inventory works but CPU, memory or History is missing", 1)[0]

    def log_recipes(self):
        blocks = re.findall(r"```bash\n(.*?)```", self.log_section(), re.DOTALL)
        self.assertEqual(len(blocks), 2, "one bounded reader per deployment")
        return {"docker" if "# Docker" in block else "journalctl": block for block in blocks}

    def exercise_log(self, reader, recipe, *, stdout="", stderr="", exitcode=0):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            command = directory / reader
            command.write_text(
                "#!/usr/bin/env python3\nimport json, os, pathlib, sys\n"
                "pathlib.Path(os.environ['LOG_ARGV']).write_text(json.dumps(sys.argv[1:]))\n"
                "sys.stdout.write(os.environ['LOG_STDOUT'])\n"
                "sys.stderr.write(os.environ['LOG_STDERR'])\n"
                "sys.exit(int(os.environ['LOG_EXIT']))\n"
            )
            command.chmod(0o700)
            argv_path = directory / "argv.json"
            env = dict(os.environ, PATH=f"{directory}:{os.environ['PATH']}",
                       LOG_ARGV=str(argv_path), LOG_STDOUT=stdout, LOG_STDERR=stderr,
                       LOG_EXIT=str(exitcode))
            result = subprocess.run(["bash", "-c", recipe], env=env, capture_output=True,
                                    text=True, timeout=5)
            self.assertTrue(argv_path.exists(), "copied recipe must invoke the log reader")
            return result, json.loads(argv_path.read_text())

    def test_log_recipes_bound_the_read_and_keep_unfiltered_output(self):
        expected = {
            "journalctl": ["-u", "pulse", "--since", "15 minutes ago", "--lines", "100", "--no-pager"],
            "docker": ["logs", "--since", "15m", "--tail", "100", "pulse"],
        }
        self.assertEqual(set(self.log_recipes()), set(expected))
        for reader, recipe in self.log_recipes().items():
            with self.subTest(reader=reader):
                result, argv = self.exercise_log(reader, recipe, stdout="startup failed\n",
                                                stderr="TrueNAS collection failed\n")
                self.assertEqual(argv, expected[reader])
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout, "startup failed\n")
                self.assertEqual(result.stderr, "TrueNAS collection failed\n")
                self.assertNotRegex(recipe, r"[|<>]|\b(?:grep|curl|inspect|restart|printenv)\b|--follow")

    def test_log_reader_failure_is_preserved_even_with_a_matching_partial_line(self):
        for reader, recipe in self.log_recipes().items():
            for output in ("", "TrueNAS polling\n"):
                with self.subTest(reader=reader, output=output):
                    result, _ = self.exercise_log(reader, recipe, stdout=output,
                                                  stderr="synthetic access failure\n", exitcode=2)
                    self.assertEqual(result.returncode, 2)
                    self.assertEqual(result.stdout, output)
                    self.assertEqual(result.stderr, "synthetic access failure\n")

    def test_legacy_pipelines_demonstrate_the_masked_reader_failure(self):
        legacy = {
            "journalctl": "journalctl -u pulse -n 100 --no-pager | grep -i truenas",
            "docker": "docker logs --tail 100 pulse 2>&1 | grep -i truenas",
        }
        for reader, recipe in legacy.items():
            with self.subTest(reader=reader):
                result, _ = self.exercise_log(reader, recipe, stdout="TrueNAS polling\n",
                                              stderr="synthetic access failure\n", exitcode=2)
                self.assertEqual(result.returncode, 0, "old recipe must expose the masked failure")
                self.assertIn("TrueNAS polling", result.stdout)
        result, _ = self.exercise_log("docker", legacy["docker"], stdout="storage init failed\n")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, "", "old filter discards related non-TrueNAS errors")

    def test_empty_or_sensitive_logs_are_not_claimed_to_be_safe_or_healthy(self):
        for reader, recipe in self.log_recipes().items():
            result, _ = self.exercise_log(reader, recipe)
            self.assertEqual(result.returncode, 0)
            self.assertEqual(result.stdout + result.stderr, "")
            result, _ = self.exercise_log(reader, recipe, stderr="synthetic-secret-echo\n")
            self.assertIn("synthetic-secret-echo", result.stderr)
        prose = " ".join(self.log_section().split())
        for boundary in ("inside the Pulse container", "actual service or container name",
                         "failed read, not", "successful empty read is inconclusive",
                         "not sanitised", "not the whole excerpt", "manually redacted",
                         "anything echoed in an error", "Do not enable Debug"):
            self.assertIn(boundary, prose)

    def test_diagnostics_export_guidance_uses_current_copy_and_preserves_live_check_boundary(self):
        text = " ".join(self.log_section().split())
        copy = (ROOT / "frontend-modern/src/utils/diagnosticsPresentation.ts").read_text()
        for field in ("exportFullLabel", "exportGithubLabel", "runActionLabel"):
            label = re.search(rf"{field}: '([^']+)'", copy).group(1)
            self.assertIn(label, text)
        for boundary in ("without running the checks again", "nothing is uploaded",
                         "review it before sharing", "keep **Full (private)** private",
                         "makes live API and guest-agent requests", "during a backup, freeze/thaw",
                         "unresponsive-host incident", "Keep the existing observations",
                         "TROUBLESHOOTING.md#collect-diagnostics-safely"):
            self.assertIn(boundary, text)
        self.assertNotIn("Export for GitHub (sanitized)", text)

    def test_core_graph_guidance_distinguishes_request_from_reply(self):
        text = DOC.read_text().split("### Inventory works but CPU, memory or History is missing", 1)[1]
        text = text.split("### Stale TrueNAS data", 1)[0]
        for required in (
            "signed-in **TrueNAS** browser session, not the Pulse page",
            "In Firefox on macOS", "**WS**", "**Response** pane",
            "Show **All** messages", "clear any message search for `reporting.get_data`",
            'outgoing `"msg":"method"`', 'incoming `"msg":"result"`',
            "**same `id` as that request**", "expand its `result` (or `error`)",
            "`cputemp` is temperature, not CPU usage",
            "response, not the\n   outgoing request's `params`",
            "one `data`\n   row", "no matching incoming reply",
            "Omit authentication messages, keys, cookies, private hostnames and addresses",
            "Do not upload a full browser network capture or paste code into the browser",
        ):
            with self.subTest(required=required):
                self.assertIn(required, text)
        self.assertNotIn("```", text)

    def test_polling_guidance_preserves_evidence_before_a_manual_test(self):
        text = DOC.read_text().split("### Stale TrueNAS data", 1)[1].split("### Disabling", 1)[0]
        # Markdown wrapping and sentence capitalization do not change the
        # safety guidance. Still require every complete warning and field.
        prose = " ".join(text.split()).casefold()
        for required in (
            "Record the existing state before testing or restarting",
            "signed-in Pulse admin browser session",
            "`/api/truenas/connections`", "read-only request",
            "`poll.intervalSeconds`", "`poll.lastAttemptAt`",
            "`poll.lastSuccessAt`", "`poll.consecutiveFailures`", "`poll.lastError`",
            "`observed.collectedAt`", "`transport.reconnects`",
            "two configured polling cycles", "completed attempt, not an in-flight request",
            "without\n   pressing **Test Connection**",
            "without refreshing `observed.collectedAt`",
            "a successful test is not evidence",
            "Session turnover alone does not establish failed authentication",
            "do not stop Apps or reproduce a hang",
            "Do not post the full connection response",
        ):
            with self.subTest(required=required):
                self.assertIn(" ".join(required.split()).casefold(), prose)
        self.assertNotIn("```", text)
        self.assertNotIn("Check TrueNAS connectivity and API key validity", text)
        # Every suggested field belongs to the existing read-only projection,
        # not a made-up diagnostic endpoint or a field in the secret payload.
        schemas = {
            "poll": (ROOT / "internal/monitoring/truenas_poller.go").read_text().split(
                "type TrueNASConnectionPollStatus struct {", 1)[1].split("\n}", 1)[0],
            "observed": (ROOT / "internal/monitoring/truenas_poller.go").read_text().split(
                "type TrueNASConnectionObservedSummary struct {", 1)[1].split("\n}", 1)[0],
            "transport": (ROOT / "internal/truenas/transport.go").read_text().split(
                "type TransportStatus struct {", 1)[1].split("\n}", 1)[0],
        }
        fields = re.findall(r"`(poll|observed|transport)\.([A-Za-z]+)`", text)
        self.assertGreater(len(fields), 8)
        for projection, field in fields:
            with self.subTest(field=f"{projection}.{field}"):
                self.assertRegex(schemas[projection], rf'json:"{field}(?:,omitempty)?"')

    def test_general_help_links_to_polling_checks_and_uses_the_actual_default(self):
        source = (ROOT / "internal/config/truenas.go").read_text()
        default = re.search(r"const defaultTrueNASPollIntervalSecs = (\d+)", source).group(1)
        general = (ROOT / "docs/TROUBLESHOOTING.md").read_text()
        self.assertEqual((ROOT / "docs/TROUBLESHOOTING.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes())
        for text in (DOC.read_text(), general):
            self.assertIn(f"{default} seconds by default", text)
        self.assertIn("TRUENAS.md#stale-truenas-data", general)
        self.assertIn("before testing or restarting", general)
        self.assertIn("a successful connection test is not proof", general)
        self.assertNotIn("(~30 seconds)", DOC.read_text())
        self.assertNotIn("cycle (30s)", general)

    def test_preparation_protects_new_and_existing_payload(self):
        preparation, _ = commands()
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            tools = home / "tools"
            tools.mkdir()
            editor = tools / "vi"
            editor.write_text('#!/bin/sh\n[ "$#" = 1 ] && [ "$1" = "$HOME/.config/pulse/truenas-connection.json" ]\n')
            editor.chmod(0o700)
            env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}")
            payload = home / ".config/pulse/truenas-connection.json"
            for existing in (False, True):
                if existing:
                    payload.write_text(json.dumps(PAYLOAD))
                    payload.chmod(0o644)
                subprocess.run(["bash", "-eu", "-c", preparation], env=env, check=True)
                self.assertEqual(stat.S_IMODE(payload.stat().st_mode), 0o600)
                self.assertEqual(payload.read_text(), json.dumps(PAYLOAD) if existing else "")

    def request(self, home, command, port):
        private = home / ".config/pulse"
        private.mkdir(parents=True, exist_ok=True)
        for name, content in (("api-header", f"X-API-Token: {TOKEN}\n"),
                              ("truenas-connection.json", json.dumps(PAYLOAD))):
            path = private / name
            path.write_text(content)
            path.chmod(0o600)
        real_curl = shutil.which("curl")
        self.assertIsNotNone(real_curl)
        tools = home / "tools"
        tools.mkdir(exist_ok=True)
        recorder = tools / "curl"
        recorder.write_text(
            "#!/usr/bin/env python3\nimport json, os, sys\nfrom pathlib import Path\n"
            "Path(os.environ['ARGV_RECEIPT']).write_text(json.dumps(sys.argv[1:]))\n"
            "os.execv(os.environ['REAL_CURL'], [os.environ['REAL_CURL'], *sys.argv[1:]])\n"
        )
        recorder.chmod(0o700)
        receipt = home / "argv.json"
        env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}",
                   REAL_CURL=real_curl, ARGV_RECEIPT=str(receipt))
        command = command.replace("http://127.0.0.1:7655", f"http://127.0.0.1:{port}")
        result = subprocess.run(["bash", "-eu", "-c", command], env=env, capture_output=True, timeout=10)
        argv = json.loads(receipt.read_text())
        self.assertIn("@" + str(private / "api-header"), argv)
        self.assertIn("@" + str(private / "truenas-connection.json"), argv)
        for secret in (TOKEN, NAS_KEY):
            self.assertNotIn(secret, " ".join(argv))
            self.assertNotIn(secret.encode(), result.stdout + result.stderr)
        output = Path(argv[argv.index("--output") + 1])
        self.assertEqual(output.parent, private)
        self.assertEqual(stat.S_IMODE(output.stat().st_mode), 0o600)
        self.assertEqual(output.read_bytes(), b'{"fixture":true}\n')
        self.assertNotIn(b'{"fixture":true}', result.stdout + result.stderr)
        return result

    def test_test_and_save_send_header_and_json_from_private_files(self):
        _, recipes = commands()
        self.assertEqual(len(recipes), 2)
        with tempfile.TemporaryDirectory() as temporary, server() as (port, requests):
            for command, endpoint in zip(recipes, ("/api/truenas/connections/test", "/api/truenas/connections")):
                result = self.request(Path(temporary), command, port)
                self.assertEqual(result.returncode, 0, result.stderr.decode())
                path, headers, payload = requests[-1]
                self.assertEqual(path, endpoint)
                self.assertEqual(headers["X-API-Token"], TOKEN)
                self.assertEqual(headers["Content-Type"], "application/json")
                self.assertEqual(payload, PAYLOAD)

    def test_http_auth_failure_is_not_reported_as_success(self):
        _, recipes = commands()
        for status in (401, 403):
            for command in recipes:
                with self.subTest(status=status, command=command), tempfile.TemporaryDirectory() as temporary:
                    with server(status) as (port, _):
                        result = self.request(Path(temporary), command, port)
                        self.assertEqual(result.returncode, 22, result.stderr.decode())
                        self.assertNotIn(b'{"fixture":true}', result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
