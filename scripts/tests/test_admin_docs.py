#!/usr/bin/env python3
"""Execute the administration guides with synthetic credentials and loopback.

Run with pulse-worker-source-proof; these fixtures do not perform real role,
tenant or audit operations. The Go documentation tests check the actual handlers.
"""

from __future__ import annotations

import json
from pathlib import Path
import re
import tempfile
import unittest

from test_api_auth_docs import (ROOT, TEST_TOKEN, exercise_curl, recording_server,
                                private_recording_server, request_helper)


DOCS = ROOT / "docs"
GUIDES = ("RBAC", "AUDIT_LOGGING", "MULTI_TENANT")
EXPECTED = {
    "RBAC": (
        ("POST", "/api/admin/roles", {
            "id": "alert-manager", "name": "Alert Manager",
            "description": "Can view and manage alerts",
            "permissions": [{"action": "read", "resource": "alerts"},
                            {"action": "write", "resource": "alerts"},
                            {"action": "read", "resource": "nodes"}],
        }),
        ("GET", "/api/admin/roles", None),
        ("PUT", "/api/admin/roles/alert-manager", {
            "name": "Alert Manager", "description": "Updated description",
            "permissions": [{"action": "read", "resource": "alerts"},
                            {"action": "write", "resource": "alerts"},
                            {"action": "read", "resource": "nodes"},
                            {"action": "read", "resource": "ai"}],
        }),
        ("DELETE", "/api/admin/roles/alert-manager", None),
        ("GET", "/api/admin/users", None),
        ("PUT", "/api/admin/users/jane/roles", {"roleIds": ["alert-manager", "viewer"]}),
        ("PUT", "/api/admin/users/jane/roles", {"roleIds": []}),
        ("DELETE", "/api/admin/users/jane", None),
    ),
    "AUDIT_LOGGING": (
        ("GET", "/api/audit?limit=50", None),
        ("GET", "/api/audit?limit=50&event=login&startTime=2026-01-01T00:00:00Z&endTime=2026-01-31T23:59:59Z&success=false", None),
        ("GET", "/api/audit/summary", None),
        ("GET", "/api/audit/export?event=login&startTime=2026-01-01T00:00:00Z&endTime=2026-01-31T23:59:59Z", None),
        ("GET", "/api/audit/6b3c9c3c-9a2f-4b3c-9a3b-3d0e8c5c5d45/verify", None),
    ),
    "MULTI_TENANT": (
        ("GET", "/api/orgs/production-datacenter/members", None),
        ("GET", "/api/orgs/production-datacenter/shares/incoming", None),
    ),
}


def recipes(name: str) -> list[str]:
    requests = []
    for block in re.findall(r"```bash\n(.*?)```", (DOCS / f"{name}.md").read_text(), re.DOTALL):
        # The audit read examples share a fence but are independent operations.
        for step in re.split(r"\n(?=# )", block):
            if "curl " in step or "pulse_api " in step:
                requests.append(step)
    return requests


def executable_recipe(name: str, step: str) -> str:
    # Use the actual shared helper, not a test-only safe client. Legacy curl
    # snippets remain executable so the parent control can expose their leak.
    if name == "AUDIT_LOGGING" and "pulse_api " in step:
        return request_helper() + "\n" + step
    return step


class AdminDocsTest(unittest.TestCase):
    def test_shell_recipes_never_expose_credentials_or_override_tls(self):
        for name in GUIDES:
            with self.subTest(guide=name):
                steps = recipes(name)
                self.assertEqual(len(steps), len(EXPECTED[name]))
                for step in steps:
                    self.assertNotRegex(step, r"(?:\b\w*TOKEN=|Authorization:|X-API-Token:|Bearer\s|--cookie\b)")
                    self.assertNotRegex(step, r"(?:--insecure|--verbose|--trace\S*|--location|\s-k\b)")
                    if name == "AUDIT_LOGGING":
                        self.assertIn("pulse_api GET ", step)
                        self.assertNotIn("curl ", step)
                    else:
                        self.assertIn('curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header"', step)
                    if re.search(r"--request (POST|PUT)", step):
                        self.assertIn("--data-binary @-", step)
                        self.assertIn("<<'JSON'", step)

    def test_guides_explain_authority_and_sensitive_output(self):
        for name in GUIDES:
            guide = (DOCS / f"{name}.md").read_text()
            with self.subTest(guide=name):
                self.assertIn("API.md#-authentication", guide)
                self.assertIn("HTTPS", guide)
                self.assertIn("certificate verification", guide)
        rbac = (DOCS / "RBAC.md").read_text()
        for boundary in ("full-access (`*`)", "bound to an authorised administrator", "cannot be modified or deleted", "self-escalation", "complete list", "URL-encode"):
            self.assertIn(boundary, rbac)
        audit = (DOCS / "AUDIT_LOGGING.md").read_text()
        for boundary in ("audit:read", "audit_logging", "private directory", "redacted excerpt", "does not reuse filters",
                         "in the same Bash session", "not an installed Pulse command", "not list pagination",
                         "ignores malformed time filters", "partial file", "does not follow redirects or retry"):
            self.assertIn(boundary, audit)
        org = (DOCS / "MULTI_TENANT.md").read_text()
        for boundary in ("session-based user authentication", "403 session_required", "CSRF", "settings:read", "organization binding", "pending invitation", "accept the share", "accessRole"):
            self.assertIn(boundary, org)
        self.assertNotRegex(org, r"\| `PATCH` \|")

    def test_shipped_copies_are_identical(self):
        for name in (*GUIDES, "API"):
            with self.subTest(guide=name):
                self.assertEqual((DOCS / f"{name}.md").read_bytes(),
                                 (ROOT / "frontend-modern/public/docs" / f"{name}.md").read_bytes())

    def test_all_recipes_send_exact_methods_paths_and_bodies_with_each_header(self):
        with recording_server() as (port, requests):
            for name, operations in EXPECTED.items():
                steps = recipes(name)
                self.assertEqual(len(steps), len(operations))
                for key, value in (("X-API-Token", TEST_TOKEN), ("Authorization", "Bearer " + TEST_TOKEN)):
                    for step, (method, path, body) in zip(steps, operations):
                        with self.subTest(guide=name, method=method, path=path, header=key), tempfile.TemporaryDirectory(prefix="admin docs ") as temporary:
                            before = len(requests)
                            result = exercise_curl(self, Path(temporary), f"{key}: {value}", port, executable_recipe(name, step))
                            self.assertEqual(result.returncode, 0, result.stderr.decode())
                            self.assertEqual(len(requests), before + 1, "a copied step sends exactly one request")
                            sent_path, headers, sent_method, sent_body = requests[-1]
                            self.assertEqual((sent_method, sent_path), (method, path))
                            self.assertEqual(headers[key], value)
                            self.assertNotIn("X-Curlrc-Injected", headers)
                            self.assertNotIn("Authorization" if key == "X-API-Token" else "X-API-Token", headers)
                            if body is None:
                                self.assertEqual(sent_body, b"")
                            else:
                                self.assertEqual(headers["Content-Type"], "application/json")
                                self.assertEqual(json.loads(sent_body), body)

    def test_every_recipe_surfaces_auth_and_license_errors_without_next_steps(self):
        for status in (401, 402, 403):
            with recording_server(status) as (port, requests):
                for name in GUIDES:
                    for step in recipes(name):
                        with self.subTest(status=status, guide=name), tempfile.TemporaryDirectory() as temporary:
                            before = len(requests)
                            result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port, executable_recipe(name, step))
                            self.assertEqual(result.returncode, 22, result.stderr.decode())
                            self.assertEqual(len(requests), before + 1)
                            self.assertNotIn(b"Saved private export", result.stdout)
                            # curl preserves an error body, in stdout or the requested file.
                            exports = list(Path(temporary).glob("*/audit-export.json"))
                            if name == "AUDIT_LOGGING":
                                responses = list((Path(temporary) / ".config/pulse").glob("api-response.*"))
                                self.assertEqual(len(responses), 1)
                                output = responses[0].read_bytes()
                                self.assertNotIn(b'{"fixture":true}', result.stdout)
                            else:
                                output = exports[0].read_bytes() if exports else result.stdout
                            self.assertIn(b'{"fixture":true}', output)

    def test_audit_reads_keep_private_success_bodies_in_new_owner_only_files(self):
        self.assertEqual(len(recipes("AUDIT_LOGGING")), len(EXPECTED["AUDIT_LOGGING"]))
        with private_recording_server() as (port, requests), tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            for header in (f"X-API-Token: {TEST_TOKEN}", f"Authorization: Bearer {TEST_TOKEN}"):
                for step, (method, path, _) in zip(recipes("AUDIT_LOGGING"), EXPECTED["AUDIT_LOGGING"]):
                    with self.subTest(path=path, header=header.split(":")[0]):
                        before = len(requests)
                        result = exercise_curl(self, home, header, port,
                                               executable_recipe("AUDIT_LOGGING", step), private_response=True)
                        self.assertEqual(result.returncode, 0, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1)
                        self.assertEqual((requests[-1][2], requests[-1][0]), (method, path))
                        self.assertIn(b"HTTP 200", result.stdout)
            # One already-retained response plus ten distinct new responses.
            self.assertEqual(len(list((home / ".config/pulse").glob("api-response.*"))), 11)

    def test_audit_reads_keep_private_http_error_bodies_and_fail(self):
        self.assertEqual(len(recipes("AUDIT_LOGGING")), len(EXPECTED["AUDIT_LOGGING"]))
        for status in (400, 401, 402, 403, 500, 503):
            with private_recording_server(status) as (port, requests):
                for step in recipes("AUDIT_LOGGING"):
                    with self.subTest(status=status, step=step), tempfile.TemporaryDirectory() as temporary:
                        before = len(requests)
                        result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port,
                                               executable_recipe("AUDIT_LOGGING", step), private_response=True)
                        self.assertEqual(result.returncode, 22, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1)
                        self.assertIn(f"HTTP {status}".encode(), result.stdout)

    def test_audit_reads_refuse_redirect_success_and_incomplete_transfers(self):
        self.assertEqual(len(recipes("AUDIT_LOGGING")), len(EXPECTED["AUDIT_LOGGING"]))
        for status, partial, expected_exit in ((302, False, 1), (200, True, 18)):
            with private_recording_server(status, partial) as (port, requests):
                for step in recipes("AUDIT_LOGGING"):
                    with self.subTest(status=status, partial=partial, step=step), tempfile.TemporaryDirectory() as temporary:
                        before = len(requests)
                        result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port,
                                               executable_recipe("AUDIT_LOGGING", step), private_response=True)
                        self.assertEqual(result.returncode, expected_exit, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1, "must not follow redirects or retry partial reads")


if __name__ == "__main__":
    unittest.main()
