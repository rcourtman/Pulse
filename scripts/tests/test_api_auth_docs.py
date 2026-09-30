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


def auth_section() -> str:
    return DOC.read_text(encoding="utf-8").split("## 🔐 Authentication\n", 1)[1].split("\n## ", 1)[0]


def commands() -> tuple[str, str]:
    blocks = re.findall(r"```bash\n(.*?)```", auth_section(), re.DOTALL)
    preparation = next(block for block in blocks if "umask 077" in block)
    request = next(block for block in blocks if block.startswith("curl "))
    return preparation, request


@contextmanager
def recording_server(status: int = 200):
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.path, dict(self.headers)))
            self.send_response(status)
            self.end_headers()
            self.wfile.write(b'{"fixture":true}\n')

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


class APIAuthDocsTest(unittest.TestCase):
    def test_credentials_are_not_documented_as_command_arguments(self):
        section = auth_section()
        bash = "\n".join(re.findall(r"```bash\n(.*?)```", section, re.DOTALL))
        self.assertNotRegex(bash, r"(?:X-API-Token:|Authorization:|Bearer\s)")
        self.assertIn('--header "@$HOME/.config/pulse/api-header"', bash)
        self.assertIn("HTTPS", section)
        self.assertIn("does not verify authentication", section)
        self.assertIn("monitoring:read", section)
        self.assertIn("Do not paste tokens", section)

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
                self.assertEqual(header.read_text(), f"X-API-Token: {TEST_TOKEN}\n" if existing else "")

    def run_documented_request(self, home: Path, header_text: str, port: int):
        _, request = commands()
        header = home / ".config/pulse/api-header"
        header.parent.mkdir(parents=True, exist_ok=True)
        header.write_text(header_text + "\n")
        header.chmod(0o600)
        real_curl = shutil.which("curl")
        self.assertIsNotNone(real_curl, "curl is required to exercise the documented command")
        tools = home / "tools"
        tools.mkdir(exist_ok=True)
        recorder = tools / "curl"
        recorder.write_text(
            "#!/usr/bin/env python3\nimport json, os, sys\n"
            "from pathlib import Path\n"
            "Path(os.environ['ARGV_RECEIPT']).write_text(json.dumps(sys.argv[1:]))\n"
            "os.execv(os.environ['REAL_CURL'], [os.environ['REAL_CURL'], *sys.argv[1:]])\n"
        )
        recorder.chmod(0o700)
        receipt = home / "argv.json"
        env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}",
                   REAL_CURL=real_curl, ARGV_RECEIPT=str(receipt))
        request = request.replace("http://127.0.0.1:7655", f"http://127.0.0.1:{port}")
        result = subprocess.run(["bash", "-eu", "-c", request], env=env, capture_output=True, timeout=10)
        argv = json.loads(receipt.read_text())
        self.assertNotIn(TEST_TOKEN, " ".join(argv))
        self.assertIn("@" + str(header), argv)
        self.assertNotIn(b"synthetic-doc-test-token", result.stdout + result.stderr)
        return result

    def test_header_file_sends_each_supported_header_without_exposing_argv(self):
        with tempfile.TemporaryDirectory() as temporary, recording_server() as (port, requests):
            for key, value in (("X-API-Token", TEST_TOKEN), ("Authorization", "Bearer " + TEST_TOKEN)):
                with self.subTest(header=key):
                    result = self.run_documented_request(Path(temporary), f"{key}: {value}", port)
                    self.assertEqual(result.returncode, 0, result.stderr.decode())
                    self.assertEqual(requests[-1][0], "/api/state/summary")
                    self.assertEqual(requests[-1][1][key], value)

    def test_http_auth_errors_fail_instead_of_looking_successful(self):
        for status in (401, 403):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as temporary:
                with recording_server(status) as (port, _):
                    result = self.run_documented_request(Path(temporary), "X-API-Token: " + TEST_TOKEN, port)
                    self.assertEqual(result.returncode, 22, result.stderr.decode())
                    self.assertIn(b'{"fixture":true}', result.stdout)


if __name__ == "__main__":
    unittest.main()
