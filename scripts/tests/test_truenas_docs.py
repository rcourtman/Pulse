#!/usr/bin/env python3
"""Exercise TrueNAS connection recipes without putting credentials in argv.

All credentials are synthetic. HTTP checks require the offline source-proof VM.
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
    blocks = re.findall(r"```bash\n(.*?)```", DOC.read_text(), re.DOTALL)
    preparation = next(block for block in blocks if "umask 077" in block)
    requests = [block for block in blocks if block.startswith("curl ")]
    return preparation, requests


@contextmanager
def server(status=200):
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            body = self.rfile.read(int(self.headers["Content-Length"]))
            requests.append((self.path, dict(self.headers), json.loads(body)))
            self.send_response(status)
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
        self.assertIn("journalctl -u pulse -n 100 --no-pager", bash)
        self.assertIn("docker logs --tail 100 pulse 2>&1", bash)
        self.assertIn("does not establish a live reading", text)
        self.assertIn("Do not upload a full browser network capture", text)

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
                        self.assertIn(b'{"fixture":true}', result.stdout)


if __name__ == "__main__":
    unittest.main()
