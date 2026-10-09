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
import time
import unittest

from test_api_auth_docs import (ROOT, TEST_TOKEN, exercise_curl,
                                private_recording_server, request_helper)


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


def executable_ai_recipe(recipe):
    # Exercise the shared guide helper, not a second implementation. Keep old
    # direct-curl recipes runnable for the exact-parent regression control.
    if "pulse_api " in recipe:
        return request_helper() + "\n" + recipe
    return recipe


def markdown_section(source, heading):
    # Emoji presentation selectors do not change the heading's meaning. Keep
    # exact heading identity and uniqueness, rather than matching the whole doc
    # when the intended safety section is missing or duplicated.
    sections = re.findall(r"^## ([^\n]+)\n(.*?)(?=^## |\Z)", source, re.MULTILINE | re.DOTALL)
    matching = [body for title, body in sections
                if title.replace("\ufe0f", "") == heading.replace("\ufe0f", "")]
    if len(matching) != 1:
        raise AssertionError(f"Expected exactly one {heading} section, found {len(matching)}")
    return " ".join(matching[0].split())


@contextmanager
def proxy_server(status):
    requests = []
    release_stall = threading.Event()

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.command, self.path, dict(self.headers)))
            if status == "stall":
                release_stall.wait(timeout=20)
                return
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
        release_stall.set()
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
                if name == "AI_AUTONOMY":
                    self.assertEqual(len(blocks(name)), len(EXPECTED_AI))
                    self.assertTrue(all(recipe.startswith("pulse_api ") for recipe in blocks(name)))
                    self.assertNotIn("curl ", shell)
                else:
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

    def test_ai_guidance_keeps_response_privacy_and_uncertain_writes_explicit(self):
        guide = " ".join(DOCUMENTS["AI_AUTONOMY"].read_text().split())
        for boundary in ("in the same Bash session", "not an installed Pulse command",
                         "new owner-only file", "preserving earlier responses",
                         "five-second connection", "twenty-second whole-request",
                         "does not follow redirects or retry", "not a script to run",
                         "in the helper", "redacted error", "partial response",
                         "HTTP success is not proof", "after a change was applied",
                         "If that read is unavailable, stop", "Supporting PUT in the helper grants no additional access",
                         "do not repeat the write blindly"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, guide)

    def test_ai_recipes_send_only_the_intended_method_path_and_body(self):
        recipes = blocks("AI_AUTONOMY")
        self.assertEqual(len(recipes), len(EXPECTED_AI))
        with private_recording_server() as (port, requests):
            for key, value in (("X-API-Token", TEST_TOKEN), ("Authorization", "Bearer " + TEST_TOKEN)):
                for recipe, (method, path, body) in zip(recipes, EXPECTED_AI):
                    with self.subTest(header=key, method=method, path=path), tempfile.TemporaryDirectory() as temporary:
                        before = len(requests)
                        result = exercise_curl(self, Path(temporary), f"{key}: {value}", port,
                                               executable_ai_recipe(recipe), private_response=True)
                        self.assertEqual(result.returncode, 0, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1)
                        sent_path, headers, sent_method, sent_body = requests[-1]
                        self.assertEqual((sent_method, sent_path), (method, path))
                        self.assertEqual(headers[key], value)
                        self.assertNotIn("X-Curlrc-Injected", headers)
                        self.assertEqual(json.loads(sent_body) if body is not None else sent_body,
                                         body if body is not None else b"")

    def test_ai_recipes_keep_private_http_errors_without_submitting_a_later_step(self):
        for status in (400, 401, 402, 403, 500, 503):
            with private_recording_server(status) as (port, requests):
                for recipe in blocks("AI_AUTONOMY"):
                    with self.subTest(status=status, recipe=recipe), tempfile.TemporaryDirectory() as temporary:
                        before = len(requests)
                        result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port,
                                               executable_ai_recipe(recipe), private_response=True)
                        self.assertEqual(result.returncode, 22)
                        self.assertEqual(len(requests), before + 1)
                        self.assertIn(f"HTTP {status}".encode(), result.stdout)

    def test_ai_recipes_keep_private_success_bodies_and_preserve_previous_responses(self):
        with private_recording_server() as (port, requests), tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            for header in (f"X-API-Token: {TEST_TOKEN}", f"Authorization: Bearer {TEST_TOKEN}"):
                for recipe, (method, path, body) in zip(blocks("AI_AUTONOMY"), EXPECTED_AI):
                    with self.subTest(method=method, header=header.split(":")[0]):
                        before = len(requests)
                        result = exercise_curl(self, home, header, port,
                                               executable_ai_recipe(recipe), private_response=True)
                        self.assertEqual(result.returncode, 0, result.stderr.decode())
                        self.assertIn(b"HTTP 200", result.stdout)
                        self.assertEqual(len(requests), before + 1)
                        self.assertEqual((requests[-1][2], requests[-1][0]), (method, path))
                        self.assertEqual(json.loads(requests[-1][3]) if body is not None else requests[-1][3],
                                         body if body is not None else b"")
            # One pre-existing file and one new private response per request.
            self.assertEqual(len(list((home / ".config/pulse").glob("api-response.*"))), 7)

    def test_ai_recipes_do_not_follow_redirects_or_repeat_incomplete_writes(self):
        for status, partial, expected_exit in ((302, False, 1), (200, True, 18)):
            with private_recording_server(status, partial) as (port, requests):
                for recipe, (method, path, body) in zip(blocks("AI_AUTONOMY"), EXPECTED_AI):
                    with self.subTest(method=method, status=status, partial=partial), tempfile.TemporaryDirectory() as temporary:
                        before = len(requests)
                        result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port,
                                               executable_ai_recipe(recipe), private_response=True)
                        self.assertEqual(result.returncode, expected_exit, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1, "must not redirect or repeat a submitted write")
                        self.assertEqual((requests[-1][2], requests[-1][0]), (method, path))
                        self.assertEqual(json.loads(requests[-1][3]) if body is not None else requests[-1][3],
                                         body if body is not None else b"")

    def exercise_proxy(self, home, port, role, *, editor_exit=0, curl_exit=0):
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
            "if os.environ['CURL_EXIT'] != '0':\n"
            "    print('000', end=''); sys.exit(int(os.environ['CURL_EXIT']))\n"
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
                   ARGV_RECEIPT=str(home / "argv.json"), REAL_CURL=real_curl, EDITOR_EXIT=str(editor_exit), CURL_EXIT=str(curl_exit))
        for key in list(env):
            if key.lower().endswith("_proxy"):
                del env[key]
        result = subprocess.run(["bash", "-c", recipe], env=env, capture_output=True, timeout=15)
        edited = json.loads((home / "editor.json").read_text())
        self.assertEqual((edited["directory_mode"], edited["file_mode"]), (0o700, 0o600))
        self.assertFalse(Path(edited["file"]).parent.exists(), "temporary credential directory must be cleaned")
        self.assertFalse(trace.exists())
        if not editor_exit:
            argv = json.loads((home / "argv.json").read_text())
            self.assertEqual(argv[0], "--disable")
            self.assertIn("@" + edited["file"], argv)
            self.assertNotIn(TEST_TOKEN, " ".join(argv))
            for option, value in (("--connect-timeout", "5"), ("--max-time", "10")):
                self.assertIn(option, argv)
                self.assertEqual(argv[argv.index(option) + 1], value)
            self.assertNotIn("--retry", argv)
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

    def test_proxy_setup_cannot_omit_role_choice_and_authenticated_boundary(self):
        source = DOCUMENTS["PROXY_AUTH"].read_text()
        proxy = " ".join(source.split())
        quick = markdown_section(source, "🚀 Quick Start")
        for phrase in ("every proxy-authenticated user an administrator",
                       "PROXY_AUTH_ROLE_HEADER=X-Authentik-Groups",
                       "PROXY_AUTH_ADMIN_ROLE=<exact-idp-admin-group>",
                       "actual group separator", "Do not remove role gating",
                       "successful IdP authentication", "authenticator is unavailable",
                       "headers-only middleware does not authenticate",
                       "authenticated non-admin and a signed-out session",
                       "existing administrator recovery path"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, proxy if phrase == "actual group separator" else quick)
        boundary = markdown_section(source, "⚠ Header Trust Boundary")
        for phrase in ("replace", "never append", "first", "username and configured role header",
                       "must also come from the successful authenticator", "not be reachable"):
            self.assertIn(phrase, boundary)

    def test_proxy_boundary_heading_preserves_identity_not_emoji_presentation(self):
        source = DOCUMENTS["PROXY_AUTH"].read_text()
        expected = markdown_section(source, "⚠ Header Trust Boundary")
        self.assertIn("never append", expected)
        self.assertEqual(markdown_section(source.replace("## ⚠ Header", "## ⚠️ Header"),
                                          "⚠ Header Trust Boundary"), expected)
        for changed in (source.replace("## ⚠ Header Trust Boundary", "## ⚠ Different Boundary"),
                        source + "\n## ⚠️ Header Trust Boundary\nDuplicate section.\n"):
            with self.subTest(changed=changed[-50:]), self.assertRaisesRegex(AssertionError, "exactly one"):
                markdown_section(changed, "⚠ Header Trust Boundary")

    def test_provider_mappings_are_not_headers_only_deployment_recipes(self):
        proxy = " ".join(DOCUMENTS["PROXY_AUTH"].read_text().split())
        examples = proxy.split("## 📦 Examples", 1)[1].split("## 🔧 Troubleshooting", 1)[0]
        self.assertIn("not complete provider deployment recipes", examples)
        self.assertNotRegex(examples, r"```(?:yaml|nginx)")
        for phrase in ("does not configure Authentik", "static username or admin group",
                       "auth_request_set", "failed or unavailable subrequest",
                       "tunnel is a transport, not an Access policy",
                       "does not validate a Cloudflare Access JWT",
                       "replace those headers", "all-users-admin"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, examples)

    def test_proxy_transport_failure_has_no_role_result_or_retry(self):
        # Deterministic adverse transport results exercise the copied shell's
        # exit handling, not a second hand-written request implementation.
        for curl_exit in (7, 28, 35, 60):
            with self.subTest(curl_exit=curl_exit), tempfile.TemporaryDirectory() as temporary:
                result = self.exercise_proxy(Path(temporary), 1, "", curl_exit=curl_exit)
                self.assertEqual(result.returncode, curl_exit)
                self.assertEqual(result.stdout, b"")
                self.assertIn(f"curl exit {curl_exit}".encode(), result.stderr)
                self.assertIn(b"no role result. Stop here.", result.stderr)

    def test_proxy_stalled_real_transport_stops_and_cleans_up(self):
        with proxy_server("stall") as (port, requests), tempfile.TemporaryDirectory() as temporary:
            started = time.monotonic()
            result = self.exercise_proxy(Path(temporary), port, "X-Proxy-Roles: none\n")
            elapsed = time.monotonic() - started
            self.assertEqual(result.returncode, 28, result.stderr.decode())
            self.assertEqual(result.stdout, b"")
            self.assertIn(b"no role result. Stop here.", result.stderr)
            self.assertEqual(len(requests), 1)
            self.assertGreaterEqual(elapsed, 9)
            self.assertLess(elapsed, 15)

    def test_proxy_editor_failure_cleans_up_and_sends_no_request(self):
        with proxy_server(200) as (port, requests), tempfile.TemporaryDirectory() as temporary:
            result = self.exercise_proxy(Path(temporary), port, "", editor_exit=73)
            self.assertEqual(result.returncode, 73)
            self.assertEqual(requests, [])


if __name__ == "__main__":
    unittest.main()
