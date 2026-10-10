#!/usr/bin/env python3
"""Exercise the existing node API guide with synthetic private credentials.

All HTTP is guest-local loopback, not a platform or production probe.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile
import unittest

import test_api_auth_docs as auth


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/API.md"
PLATFORM_SECRET = "synthetic-platform-token-secret"


def section() -> str:
    return DOC.read_text().split("## 🖥️ Nodes & Config\n", 1)[1].split("\n## ", 1)[0]


def preparation() -> str:
    blocks = re.findall(r"```bash\n(.*?)```", section(), re.DOTALL)
    return next(block for block in blocks if 'vi "$node_request_file"' in block)


def sample() -> dict:
    add_node = section().split("### Add Node\n", 1)[1].split("\n### ", 1)[0]
    blocks = re.findall(r"```json\n(.*?)```", add_node, re.DOTALL)
    if len(blocks) != 1:
        raise AssertionError("expected one node configuration specimen")
    return json.loads(blocks[0])


def request() -> str:
    blocks = re.findall(r"```bash\n(.*?)```", section(), re.DOTALL)
    return next(block for block in blocks if block.startswith("pulse_api POST "))


class NodeAPIDocsTest(unittest.TestCase):
    def test_specimen_uses_scoped_token_and_explicit_tls(self):
        payload = sample()
        self.assertEqual(payload["type"], "pve")
        self.assertTrue(payload["host"].startswith("https://"))
        self.assertNotIn("user", payload)
        self.assertNotIn("password", payload)
        self.assertNotIn("root@pam", json.dumps(payload))
        self.assertEqual(payload["tokenName"], "pulse-monitor@pve!pulse-readonly")
        self.assertEqual(payload["tokenValue"], "<platform-token-secret>")
        self.assertIs(payload["verifySSL"], True)
        schema = (ROOT / "internal/api/configapi/config_handlers.go").read_text()
        schema = schema.split("type NodeConfigRequest struct {", 1)[1].split("\n}", 1)[0]
        fields = set(re.findall(r'json:"([^",]+)', schema))
        self.assertLessEqual(set(payload), fields, "the copied fields must be accepted by this request type")

    def test_scopes_live_tests_and_connection_identity_are_explicit(self):
        text = section()
        routes = ("", "/test-connection", "/test-config", "/{id}", "/{id}/test", "/{id}/refresh-cluster")
        for route in routes:
            method = "PUT" if route == "/{id}" else "POST"
            self.assertIn(f"`{method} /api/config/nodes{route}` (admin, `settings:write`)", text)
        self.assertIn("`GET /api/config/nodes` (admin, `settings:read`)", text)
        self.assertIn("`DELETE /api/config/nodes/{id}` (admin, `settings:write`)", text)
        for boundary in ("not individual VMs or agents", "returned\nconnection `id`",
                         "no save", "live platform requests", "not a passive read",
                         "freeze/thaw", "not all\ncollection permissions",
                         "does not\nselect authorised nodes"):
            self.assertIn(boundary, text)
        self.assertNotIn("Validation Only", text)

    def test_credentials_trust_and_uncertain_changes_are_separate(self):
        text = section()
        for boundary in ("Pulse API token", "Proxmox token", "different credentials",
                         "does **not** enable certificate verification",
                         "independent trusted channel", "not established trust",
                         "Do not clear an existing pin", "can\nalso try to create a token and ACLs",
                         "without retries", "lost response", "re-read the\nsaved inventory",
                         "masked credentials", "not the platform workloads or the upstream token",
                         "Collection and related alert coverage can stop",
                         "reports and shared repositories"):
            self.assertIn(boundary, text)
        self.assertNotRegex("\n".join(re.findall(r"```bash\n(.*?)```", text, re.DOTALL)),
                            r"root@pam|tokenValue|password|--insecure|--location|--retry")

    def test_copied_preparation_is_private_unique_and_stops_on_failure(self):
        with tempfile.TemporaryDirectory() as tmp:
            home = Path(tmp)
            tools = home / "tools"
            tools.mkdir()
            calls = home / "editor-calls.jsonl"
            editor = tools / "vi"
            editor.write_text(
                "#!/usr/bin/env python3\nimport json, os, sys\nfrom pathlib import Path\n"
                "with Path(os.environ['EDITOR_CALLS']).open('a') as f: f.write(json.dumps(sys.argv[1:]) + '\\n')\n"
                "Path(sys.argv[1]).write_text('{\"fixture\": true}')\n"
            )
            editor.chmod(0o700)
            env = dict(os.environ, HOME=str(home), TMPDIR=str(home),
                       PATH=f"{tools}:{os.environ['PATH']}", EDITOR_CALLS=str(calls))
            created = []
            for _ in range(2):
                result = subprocess.run(["bash", "-eu", "-c", preparation()], env=env,
                                        capture_output=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout, b"", "preparation must not print credentials")
                argv = json.loads(calls.read_text().splitlines()[-1])
                self.assertEqual(len(argv), 1)
                path = Path(argv[0])
                self.assertEqual(path.name, "node.json")
                self.assertEqual(path.read_text(), '{"fixture": true}')
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
                self.assertEqual(stat.S_IMODE(path.parent.stat().st_mode), 0o700)
                created.append(path)
            self.assertNotEqual(*created, "a new preparation must not overwrite the previous request")
            before = calls.read_bytes()
            failed = subprocess.run(["bash", "-eu", "-c", preparation()],
                                    env=dict(env, TMPDIR=str(home / "missing")),
                                    capture_output=True, timeout=10)
            self.assertNotEqual(failed.returncode, 0)
            self.assertEqual(calls.read_bytes(), before, "failed preparation must not open an editor")
            self.assertEqual(created[0].read_text(), '{"fixture": true}')

    def test_copied_file_dispatch_is_private_bounded_and_never_retried(self):
        payload = sample()
        payload["tokenValue"] = PLATFORM_SECRET
        # Independent operation expectation: the fixture must not accept a
        # different route, method, credential or body just because it was copied.
        self.assertEqual(request().strip(), 'pulse_api POST /api/config/nodes < "$node_request_file"')
        for status, partial in ((201, False), (401, False), (403, False),
                                (302, False), (500, False), (200, True)):
            with self.subTest(status=status, partial=partial), tempfile.TemporaryDirectory() as tmp:
                home = Path(tmp)
                body = home / "node.json"
                body.write_text(json.dumps(payload))
                body.chmod(0o600)
                script = f'node_request_file={json.dumps(str(body))}\n' + auth.request_helper() + "\n" + request()
                with auth.private_recording_server(status, partial) as (port, calls):
                    result = auth.exercise_curl(self, home, "X-API-Token: " + auth.TEST_TOKEN,
                                                port, script, private_response=True)
                self.assertEqual(len(calls), 1)
                path, headers, method, data = calls[0]
                self.assertEqual((path, method), ("/api/config/nodes", "POST"))
                self.assertEqual(headers["X-API-Token"], auth.TEST_TOKEN)
                self.assertEqual(headers["Content-Type"], "application/json")
                self.assertEqual(json.loads(data), payload)
                self.assertEqual(body.read_text(), json.dumps(payload))
                argv = (home / "argv.json").read_text()
                self.assertNotIn(PLATFORM_SECRET, argv)
                self.assertNotIn(PLATFORM_SECRET.encode(), result.stdout + result.stderr)
                self.assertEqual(result.returncode == 0, status == 201 and not partial)

    def test_missing_request_file_sends_nothing(self):
        with tempfile.TemporaryDirectory() as tmp:
            home = Path(tmp)
            with auth.recording_server() as (port, calls):
                script = f'node_request_file={json.dumps(str(home / "absent.json"))}\n'
                script += auth.request_helper().replace("http://127.0.0.1:7655", f"http://127.0.0.1:{port}")
                script += "\n" + request()
                result = subprocess.run(["bash", "-eu", "-c", script], capture_output=True, timeout=10)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(calls, [], "missing private input must not fall back to another request body")

    def test_all_documented_node_shell_parses(self):
        for block in re.findall(r"```bash\n(.*?)```", section(), re.DOTALL):
            subprocess.run(["bash", "-n", "-c", block], check=True, capture_output=True)

    def test_private_response_helper_is_preserved(self):
        helper = auth.request_helper()
        self.assertIn('curl --disable --fail-with-body --silent --show-error', helper)
        self.assertIn('--header "@$auth_file"', helper)
        self.assertIn('--data-binary @-', helper)
        self.assertIn('--output "$result_file"', helper)
        self.assertNotIn('--retry', helper)


if __name__ == "__main__":
    unittest.main()
