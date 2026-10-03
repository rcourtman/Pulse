#!/usr/bin/env python3
"""Run the copied authentication recipes against synthetic local HTTP fixtures.

Use pulse-worker-source-proof: no real credentials, IdP, proxy or AI write.
Handler controls are separate; these tests prove the commands readers copy.
"""
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import threading
import unittest

from test_api_auth_docs import ROOT, TEST_TOKEN, exercise_curl, recording_server


DOCUMENTS = {name: ROOT / f"docs/{name}.md" for name in ("AI_AUTONOMY", "PROXY_AUTH")}
EXPECTED_AI = (
    ("GET", "/api/ai/patrol/autonomy", None),
    ("PUT", "/api/ai/patrol/autonomy", {
        "autonomy_level": "approval", "investigation_budget": 15, "investigation_timeout_sec": 600,
    }),
    ("PUT", "/api/settings/ai/update", {"control_level": "controlled"}),
)


def blocks(name):
    return re.findall(r"```bash\n(.*?)```", DOCUMENTS[name].read_text(), re.DOTALL)


@contextmanager
def proxy_server(status):
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.command, self.path, dict(self.headers)))
            self.send_response(status)
            self.send_header("Set-Cookie", "synthetic-session-must-not-be-printed")
            self.send_header("Location", "/unexpected-redirect-target")
            self.end_headers()
            self.wfile.write(b"synthetic-private-settings-must-not-be-printed")

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


class AuthenticationDiagnosticDocsTest(unittest.TestCase):
    def test_shell_recipes_never_contain_credentials_or_trace_options(self):
        for name in DOCUMENTS:
            with self.subTest(guide=name):
                shell = "\n".join(blocks(name))
                self.assertNotRegex(shell, r"(?:admin:admin|\s-u\s|--user\b|X-Proxy-Secret:|\b\w*TOKEN=|--cookie\b)")
                self.assertNotRegex(shell, r"(?:--insecure|--verbose|--trace\S*|--location|\s-k\b)")
                self.assertIn("curl --disable", shell)
                self.assertIn("--header \"@", shell)

    def test_docs_explain_permission_transport_and_end_to_end_limits(self):
        ai = DOCUMENTS["AI_AUTONOMY"].read_text()
        for text in ("settings:write", "even for GET", "changes Patrol mode", "after an uncertain write",
                     "HTTPS", "not use a write as an", "private header file"):
            self.assertIn(text, ai)
        proxy = DOCUMENTS["PROXY_AUTH"].read_text()
        for text in ("not expose the", "not a successful denial test", "not** your proxy's header",
                     "Do not add an API token", "old behaviour", "always supplying a non-empty role"):
            self.assertIn(text, proxy)
        self.assertNotIn("that must also be refused", proxy.split("## Pulse 5.x", 1)[1])

    def test_shipped_mirrors_match(self):
        for name, doc in DOCUMENTS.items():
            self.assertEqual(doc.read_bytes(), (ROOT / f"frontend-modern/public/docs/{name}.md").read_bytes())

    def test_ai_recipes_send_only_the_intended_method_path_and_body(self):
        recipes = blocks("AI_AUTONOMY")
        self.assertEqual(len(recipes), len(EXPECTED_AI))
        with recording_server() as (port, requests):
            for key, value in (("X-API-Token", TEST_TOKEN), ("Authorization", "Bearer " + TEST_TOKEN)):
                for recipe, (method, path, body) in zip(recipes, EXPECTED_AI):
                    with self.subTest(header=key, method=method, path=path), tempfile.TemporaryDirectory() as temporary:
                        before = len(requests)
                        result = exercise_curl(self, Path(temporary), f"{key}: {value}", port, recipe)
                        self.assertEqual(result.returncode, 0, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1)
                        sent_path, headers, sent_method, sent_body = requests[-1]
                        self.assertEqual((sent_method, sent_path), (method, path))
                        self.assertEqual(headers[key], value)
                        self.assertNotIn("X-Curlrc-Injected", headers)
                        self.assertEqual(json.loads(sent_body) if body is not None else sent_body,
                                         body if body is not None else b"")

    def test_ai_recipes_report_401_and_403_without_submitting_a_later_step(self):
        for status in (401, 403):
            with recording_server(status) as (port, requests):
                for recipe in blocks("AI_AUTONOMY"):
                    with self.subTest(status=status, recipe=recipe), tempfile.TemporaryDirectory() as temporary:
                        before = len(requests)
                        result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port, recipe)
                        self.assertEqual(result.returncode, 22)
                        self.assertEqual(len(requests), before + 1)

    def exercise_proxy(self, home, port, role, *, editor_exit=0):
        recipes = blocks("PROXY_AUTH")
        self.assertEqual(len(recipes), 1)
        recipe = recipes[0].replace("http://127.0.0.1:7655", f"http://127.0.0.1:{port}")
        headers = home / "synthetic-proxy-headers"
        headers.write_text(f"X-Proxy-Secret: {TEST_TOKEN}\nX-Authentik-Username: testuser\n" + role)
        tools = home / "tools"
        tools.mkdir()
        editor = tools / "vi"
        editor.write_text(
            "#!/usr/bin/env python3\nimport json, os, stat, sys\nfrom pathlib import Path\n"
            "p=Path(sys.argv[1]); d={'file':str(p), 'file_mode':stat.S_IMODE(p.stat().st_mode), "
            "'directory_mode':stat.S_IMODE(p.parent.stat().st_mode)}\n"
            "Path(os.environ['EDITOR_RECEIPT']).write_text(json.dumps(d))\n"
            "p.write_bytes(Path(os.environ['FIXTURE_HEADERS']).read_bytes())\n"
            "sys.exit(int(os.environ['EDITOR_EXIT']))\n"
        )
        editor.chmod(0o700)
        recorder = tools / "curl"
        recorder.write_text(
            "#!/usr/bin/env python3\nimport json, os, sys\nfrom pathlib import Path\n"
            "Path(os.environ['ARGV_RECEIPT']).write_text(json.dumps(sys.argv[1:]))\n"
            "os.execv(os.environ['REAL_CURL'], [os.environ['REAL_CURL'], *sys.argv[1:]])\n"
        )
        recorder.chmod(0o700)
        real_curl = shutil.which("curl")
        self.assertIsNotNone(real_curl)
        trace = home / "curl-trace"
        (home / ".curlrc").write_text(f'header = "X-Curlrc-Injected: yes"\ntrace-ascii = "{trace}"\n')
        env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}", CURL_HOME=str(home),
                   XDG_CONFIG_HOME=str(home / ".config"), TMPDIR=str(home),
                   FIXTURE_HEADERS=str(headers), EDITOR_RECEIPT=str(home / "editor.json"),
                   ARGV_RECEIPT=str(home / "argv.json"), REAL_CURL=real_curl, EDITOR_EXIT=str(editor_exit))
        for key in list(env):
            if key.lower().endswith("_proxy"):
                del env[key]
        result = subprocess.run(["bash", "-c", recipe], env=env, capture_output=True, timeout=10)
        edited = json.loads((home / "editor.json").read_text())
        self.assertEqual((edited["directory_mode"], edited["file_mode"]), (0o700, 0o600))
        self.assertFalse(Path(edited["file"]).parent.exists(), "temporary credential directory must be cleaned")
        self.assertFalse(trace.exists())
        if not editor_exit:
            argv = json.loads((home / "argv.json").read_text())
            self.assertEqual(argv[0], "--disable")
            self.assertIn("@" + edited["file"], argv)
            self.assertNotIn(TEST_TOKEN, " ".join(argv))
        else:
            self.assertFalse((home / "argv.json").exists())
        self.assertNotIn(TEST_TOKEN.encode(), result.stdout + result.stderr)
        return result

    def test_proxy_probe_preserves_roles_reports_status_only_and_never_follows_redirects(self):
        for status in (200, 401, 403, 302):
            with proxy_server(status) as (port, requests):
                # curl's colon-only form removes a header. A semicolon sends
                # an explicitly empty header; cover both as well as omission.
                for role, expected_role in (
                    ("X-Proxy-Roles: none\n", "none"),
                    ("", None),
                    ("X-Proxy-Roles:\n", None),
                    ("X-Proxy-Roles;\n", ""),
                ):
                    with self.subTest(status=status, role=role), tempfile.TemporaryDirectory() as temporary:
                        before = len(requests)
                        result = self.exercise_proxy(Path(temporary), port, role)
                        self.assertEqual(result.returncode, 0, result.stderr.decode())
                        self.assertEqual(result.stdout, f"HTTP {status}\n".encode())
                        self.assertEqual(result.stderr, b"")
                        self.assertEqual(len(requests), before + 1)
                        method, path, headers = requests[-1]
                        self.assertEqual((method, path), ("GET", "/api/system/settings"))
                        self.assertEqual(headers["X-Proxy-Secret"], TEST_TOKEN)
                        self.assertEqual(headers["X-Authentik-Username"], "testuser")
                        self.assertEqual(headers.get("X-Proxy-Roles"), expected_role)
                        for unexpected in ("X-API-Token", "Authorization", "Cookie", "X-Curlrc-Injected"):
                            self.assertNotIn(unexpected, headers)

    def test_proxy_editor_failure_cleans_up_and_sends_no_request(self):
        with proxy_server(200) as (port, requests), tempfile.TemporaryDirectory() as temporary:
            result = self.exercise_proxy(Path(temporary), port, "", editor_exit=73)
            self.assertEqual(result.returncode, 73)
            self.assertEqual(requests, [])


if __name__ == "__main__":
    unittest.main()
