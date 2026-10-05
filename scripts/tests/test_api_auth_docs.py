#!/usr/bin/env python3
"""Exercise the API guide's credential-file commands with synthetic secrets.

The HTTP tests need guest-local loopback; run via pulse-worker-source-proof.
No real Pulse instance, credentials or external network is used.
"""

from __future__ import annotations

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
DOC = ROOT / "docs/API.md"
TEST_TOKEN = "synthetic-doc-test-token"
PRIVATE_RESPONSE = json.dumps({
    "fixture": True, "token": TEST_TOKEN, "resource": "private-fixture-resource",
}).encode() + b"\n"


def auth_section() -> str:
    return DOC.read_text(encoding="utf-8").split("## 🔐 Authentication\n", 1)[1].split("\n## ", 1)[0]


def commands() -> tuple[str, str]:
    blocks = re.findall(r"```bash\n(.*?)```", auth_section(), re.DOTALL)
    preparation = next(block for block in blocks if 'vi "$auth_file"' in block)
    return preparation, recipe("/api/state/summary")


def request_helper() -> str:
    matches = [block for block in re.findall(r"```bash\n(.*?)```", auth_section(), re.DOTALL)
               if block.startswith("pulse_api() (")]
    if len(matches) != 1:
        raise AssertionError("expected one private-response helper")
    return matches[0]


def recipe(path: str) -> str:
    matches = [block for block in re.findall(r"```bash\n(.*?)```", DOC.read_text(), re.DOTALL)
               if block.startswith("pulse_api ") and path in block]
    if len(matches) != 1:
        raise AssertionError(f"expected one executable recipe for {path}")
    # A copied call is meaningful only after defining the actual guide helper
    # in that shell, not a second, hand-written curl implementation.
    return request_helper() + "\n" + matches[0]


# Independently expected HTTP operations: extracting the shell from the guide
# ensures the fixture tests what readers copy, not a second hand-written client.
FLEET_ACTION_REQUESTS = (
    ("GET", "/api/connections", None),
    ("GET", "/api/agent/resource-capabilities/vm%3A42", None),
    ("POST", "/api/actions/plan", {
        "requestId": "manual-recovery-123", "resourceId": "vm:42",
        "capabilityName": "restart", "params": {"mode": "graceful"},
        "reason": "Recover after confirmed outage",
    }),
    ("POST", "/api/actions/act_.../decision", {
        "outcome": "approved", "reason": "Inside maintenance window", "planHash": "sha256:...",
    }),
    ("POST", "/api/actions/act_.../execute", {
        "reason": "Execute approved recovery", "planHash": "sha256:...",
    }),
    ("GET", "/api/audit/actions?resourceId=vm%3A42&limit=10", None),
    ("GET", "/api/audit/actions/act_.../events", None),
)


@contextmanager
def recording_server(status: int = 200, partial: bool = False, response: bytes = b'{"fixture":true}\n'):
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def record_request(self):
            body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
            requests.append((self.path, dict(self.headers), self.command, body))
            self.send_response(status)
            if status == 302:
                self.send_header("Location", "/api/should-not-receive-credentials")
            if partial:
                self.send_header("Content-Length", str(len(response) + 100))
            self.end_headers()
            self.wfile.write(response)

        do_GET = record_request
        do_POST = record_request
        do_PUT = record_request
        do_PATCH = record_request
        do_DELETE = record_request

        def log_message(self, *_args):
            pass

    server = HTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield server.server_address[1], requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


def private_recording_server(status: int = 200, partial: bool = False):
    return recording_server(status, partial, response=PRIVATE_RESPONSE)


def exercise_curl(case: unittest.TestCase, home: Path, header_text: str, port: int, request: str,
                  *, private_response: bool = False):
    """Execute the exact copied recipe; capture argv and hostile curl defaults."""
    header = home / ".config/pulse/api-header"
    header.parent.mkdir(parents=True, exist_ok=True)
    header.write_text(header_text + "\n")
    header.chmod(0o644 if private_response else 0o600)
    # Other guides import this fixture for credential transport alone. Keep
    # that contract separate from API.md's response-file/helper contract.
    if private_response:
        retained = header.parent / "api-response.retained"
        if not retained.exists():
            retained.write_bytes(b"previous private response\n")
            retained.chmod(0o600)
        previous = {file: file.read_bytes() for file in header.parent.glob("api-response.*")}
    real_curl = shutil.which("curl")
    case.assertIsNotNone(real_curl, "curl is required to exercise the documented command")
    tools = home / "tools"
    tools.mkdir(exist_ok=True)
    recorder = tools / "curl"
    recorder.write_text(
        "#!/usr/bin/env python3\nimport json, os, sys\n"
        "from pathlib import Path\n"
        "with Path(os.environ['ARGV_RECEIPT']).open('a') as f: f.write(json.dumps(sys.argv[1:]) + '\\n')\n"
        "os.execv(os.environ['REAL_CURL'], [os.environ['REAL_CURL'], *sys.argv[1:]])\n"
    )
    recorder.chmod(0o700)
    receipt = home / "argv.json"
    receipt.write_text("")
    trace = home / "curl-trace.txt"
    # An existing curl configuration must not turn safe argv into a trace
    # containing the header file's credential, or inject another header.
    (home / ".curlrc").write_text(
        f'header = "X-Curlrc-Injected: yes"\nverbose\ninsecure\nlocation\ntrace-ascii = "{trace}"\n'
    )
    env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}",
               CURL_HOME=str(home), XDG_CONFIG_HOME=str(home / ".config"), TMPDIR=str(home),
               REAL_CURL=real_curl, ARGV_RECEIPT=str(receipt))
    for key in list(env):
        if key.lower().endswith("_proxy"):
            del env[key]
    request = request.replace("http://127.0.0.1:7655", f"http://127.0.0.1:{port}")
    result = subprocess.run(["bash", "-eu", "-c", request], env=env, capture_output=True, timeout=10)
    calls = [json.loads(line) for line in receipt.read_text().splitlines()]
    case.assertEqual(len(calls), 1, "a copied step must invoke curl once, even on failure")
    argv = calls[0]
    case.assertEqual(argv[0], "--disable", "curl defaults must be disabled by the first option")
    case.assertNotIn(TEST_TOKEN, " ".join(argv))
    case.assertIn("@" + str(header), argv)
    for marker in (TEST_TOKEN, "private-fixture-resource"):
        case.assertNotIn(marker.encode(), result.stdout + result.stderr, "private response leaked")
    case.assertFalse(trace.exists(), "local curl configuration must not create a credential trace")
    if private_response:
        for option, value in (("--connect-timeout", "5"), ("--max-time", "20")):
            case.assertIn(option, argv)
            case.assertEqual(argv[argv.index(option) + 1], value)
        responses = set(header.parent.glob("api-response.*")) - previous.keys()
        case.assertEqual(len(responses), 1, "each request must retain a new response file")
        response = responses.pop()
        case.assertIn(str(response).encode(), result.stdout)
        case.assertEqual(response.read_bytes(), PRIVATE_RESPONSE)
        for path, mode in ((header.parent, 0o700), (header, 0o600), (response, 0o600)):
            case.assertEqual(stat.S_IMODE(path.stat().st_mode), mode)
        for path, content in previous.items():
            case.assertEqual(path.read_bytes(), content, "an earlier response was overwritten")
    return result


class APIAuthDocsTest(unittest.TestCase):
    def test_credentials_are_not_documented_as_command_arguments(self):
        section = auth_section()
        bash = "\n".join(re.findall(r"```bash\n(.*?)```", section, re.DOTALL))
        self.assertNotRegex(bash, r"(?:X-API-Token:|Authorization:|Bearer\s)")
        self.assertIn('--header "@$auth_file"', request_helper())
        self.assertIn("HTTPS", section)
        self.assertIn("does not verify authentication", section)
        self.assertIn("monitoring:read", section)
        self.assertIn("Do not paste tokens", section)

    def test_all_shell_recipes_keep_credentials_out_of_arguments_and_defaults(self):
        bash = "\n".join(re.findall(r"```bash\n(.*?)```", DOC.read_text(), re.DOTALL))
        self.assertNotRegex(bash, r"(?:\b\w*TOKEN=|X-API-Token:|Authorization:|Bearer\s)")
        self.assertNotRegex(bash, r"(?:--insecure|--verbose|--trace\S*|--location|\s-k\b)")
        blocks = re.findall(r"```bash\n(.*?)```", DOC.read_text(), re.DOTALL)
        requests = [block for block in blocks if block.startswith("pulse_api ")]
        self.assertEqual(len(requests), 8)
        self.assertEqual([block for block in blocks if "curl " in block], [request_helper()],
                         "an example must not bypass the private-response helper")
        self.assertIn("curl --disable --fail-with-body ", request_helper())
        for request in requests:
            if request.startswith("pulse_api POST "):
                self.assertIn("--data-binary @-", request_helper())
                self.assertIn("<<'JSON'", request)

    def test_every_documented_shell_recipe_parses(self):
        for block in re.findall(r"```bash\n(.*?)```", DOC.read_text(), re.DOTALL):
            with self.subTest(block=block):
                subprocess.run(["bash", "-n", "-c", block], check=True, capture_output=True)

    def test_action_guidance_preserves_scope_and_review_boundaries(self):
        guide = DOC.read_text()
        for scope in ("monitoring:read", "settings:read", "actions:plan", "actions:approve",
                      "actions:execute", "audit:read"):
            self.assertIn(scope, guide)
        for boundary in ("steps separately", "bound to an authorised user", "licensed audit logging",
                         "separation of duties", "step-up", "losing the connection",
                         "terminal result", "planHash", "URL-encode"):
            self.assertIn(boundary, guide)

    def test_shipped_copy_is_identical(self):
        self.assertEqual(DOC.read_bytes(), (ROOT / "frontend-modern/public/docs/API.md").read_bytes())

    def test_preparation_protects_new_and_existing_header_files(self):
        preparation, _ = commands()
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            tools = home / "tools"
            tools.mkdir()
            # The editor must be interactive in the real recipe; this fixture
            # verifies its filename without putting a synthetic token in argv.
            editor = tools / "vi"
            editor.write_text('#!/bin/sh\n[ "$#" = 1 ] && [ "$1" = "$HOME/.config/pulse/api-header" ]\n')
            editor.chmod(0o700)
            env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}")
            header = home / ".config/pulse/api-header"
            for existing in (False, True):
                if existing:
                    header.write_text(f"X-API-Token: {TEST_TOKEN}\n")
                    header.chmod(0o644)
                subprocess.run(["bash", "-eu", "-c", preparation], env=env, check=True)
                self.assertEqual(stat.S_IMODE(header.stat().st_mode), 0o600)
                self.assertEqual(stat.S_IMODE(header.parent.stat().st_mode), 0o700)
                self.assertEqual(header.read_text(), f"X-API-Token: {TEST_TOKEN}\n" if existing else "")

    def test_preparation_rejects_symlinked_paths_without_editing_or_chmodding_targets(self):
        preparation, _request = commands()
        for component in (".config", ".config/pulse", ".config/pulse/api-header"):
            with self.subTest(component=component), tempfile.TemporaryDirectory() as temporary:
                home = Path(temporary) / "home"
                home.mkdir()
                target = Path(temporary) / "target"
                if component.endswith("api-header"):
                    target.write_text("preserve target\n")
                    target.chmod(0o644)
                else:
                    target.mkdir()
                    target.chmod(0o755)
                link = home / component
                link.parent.mkdir(parents=True, exist_ok=True)
                link.symlink_to(target)
                tools = home / "tools"
                tools.mkdir()
                editor = tools / "vi"
                editor.write_text('#!/bin/sh\ntouch "$HOME/editor-called"\n')
                editor.chmod(0o700)
                before = stat.S_IMODE(target.stat().st_mode)
                env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}")
                result = subprocess.run(["bash", "-c", preparation], env=env, capture_output=True, timeout=10)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse((home / "editor-called").exists())
                self.assertEqual(stat.S_IMODE(target.stat().st_mode), before)
                if target.is_file():
                    self.assertEqual(target.read_text(), "preserve target\n")
                else:
                    self.assertEqual(list(target.iterdir()), [])

    def test_preparation_failure_stops_before_editor_in_an_ordinary_shell(self):
        preparation, _request = commands()
        for failing_command in ("mkdir", "touch", "chmod"):
            with self.subTest(command=failing_command), tempfile.TemporaryDirectory() as temporary:
                home = Path(temporary)
                tools = home / "tools"
                tools.mkdir()
                failure = tools / failing_command
                failure.write_text("#!/bin/sh\nexit 73\n")
                failure.chmod(0o700)
                editor = tools / "vi"
                editor.write_text('#!/bin/sh\n: > "$HOME/editor-called"\n')
                editor.chmod(0o700)
                env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}")
                result = subprocess.run(["bash", "-c", preparation], env=env, capture_output=True, timeout=10)
                self.assertEqual(result.returncode, 73)
                self.assertFalse((home / "editor-called").exists())

    def run_documented_request(self, home: Path, header_text: str, port: int, request=None):
        return exercise_curl(self, home, header_text, port, request if request is not None else commands()[1],
                             private_response=True)

    def test_header_file_sends_each_supported_header_without_exposing_argv(self):
        with tempfile.TemporaryDirectory() as temporary, private_recording_server() as (port, requests):
            for key, value in (("X-API-Token", TEST_TOKEN), ("Authorization", "Bearer " + TEST_TOKEN)):
                with self.subTest(header=key):
                    result = self.run_documented_request(Path(temporary), f"{key}: {value}", port)
                    self.assertEqual(result.returncode, 0, result.stderr.decode())
                    self.assertEqual(requests[-1][0], "/api/state/summary")
                    self.assertEqual(requests[-1][1][key], value)
                    self.assertNotIn("X-Curlrc-Injected", requests[-1][1])

    def test_fleet_action_recipes_send_expected_methods_paths_and_json(self):
        with tempfile.TemporaryDirectory() as temporary, private_recording_server() as (port, requests):
            for key, value in (("X-API-Token", TEST_TOKEN), ("Authorization", "Bearer " + TEST_TOKEN)):
                for method, path, body in FLEET_ACTION_REQUESTS:
                    with self.subTest(header=key, method=method, path=path):
                        before = len(requests)
                        result = self.run_documented_request(Path(temporary), f"{key}: {value}", port,
                                                             recipe(path))
                        self.assertEqual(result.returncode, 0, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1, "each step sends only its own request")
                        sent_path, headers, sent_method, sent_body = requests[-1]
                        self.assertEqual((sent_method, sent_path), (method, path))
                        self.assertEqual(headers[key], value)
                        other_key = "Authorization" if key == "X-API-Token" else "X-API-Token"
                        self.assertNotIn(other_key, headers)
                        self.assertNotIn("X-Curlrc-Injected", headers)
                        if body is None:
                            self.assertEqual(sent_body, b"")
                        else:
                            self.assertEqual(headers["Content-Type"], "application/json")
                            self.assertEqual(json.loads(sent_body), body)

    def test_each_fleet_action_recipe_surfaces_auth_errors_without_following_steps(self):
        for status in (401, 403):
            with tempfile.TemporaryDirectory() as temporary, private_recording_server(status) as (port, requests):
                for _method, path, _body in FLEET_ACTION_REQUESTS:
                    with self.subTest(status=status, path=path):
                        before = len(requests)
                        result = self.run_documented_request(Path(temporary), "X-API-Token: " + TEST_TOKEN,
                                                             port, recipe(path))
                        self.assertEqual(result.returncode, 22, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1)

    def test_http_auth_errors_fail_instead_of_looking_successful(self):
        for status in (401, 403):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as temporary:
                with private_recording_server(status) as (port, requests):
                    result = self.run_documented_request(Path(temporary), "X-API-Token: " + TEST_TOKEN, port)
                    self.assertEqual(result.returncode, 22, result.stderr.decode())
                    self.assertEqual(len(requests), 1)

    def test_redirect_and_partial_post_fail_privately_without_retry(self):
        for status, partial, exitcode in ((302, False, 1), (200, True, 18)):
            with self.subTest(status=status, partial=partial), tempfile.TemporaryDirectory() as temporary:
                with private_recording_server(status, partial) as (port, requests):
                    result = self.run_documented_request(Path(temporary), "X-API-Token: " + TEST_TOKEN,
                                                         port, recipe("/api/actions/act_.../execute"))
                    self.assertEqual(result.returncode, exitcode, result.stderr.decode())
                    self.assertEqual(len(requests), 1, "uncertain POST must not be repeated or redirected")

    def test_private_response_assertion_rejects_console_output_regression(self):
        request = commands()[1]
        self.assertEqual(request.count('--output "$result_file" '), 1)
        unsafe = request.replace('--output "$result_file" ', "")
        with tempfile.TemporaryDirectory() as temporary, private_recording_server() as (port, requests):
            with self.assertRaisesRegex(AssertionError, "private response leaked"):
                self.run_documented_request(Path(temporary), "X-API-Token: " + TEST_TOKEN, port, unsafe)
            self.assertEqual(len(requests), 1)


if __name__ == "__main__":
    unittest.main()
