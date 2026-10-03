#!/usr/bin/env python3
"""Execute the administration guides with synthetic credentials and loopback.

Run with pulse-worker-source-proof; these fixtures do not perform real role,
tenant or audit operations. The Go documentation tests check the actual handlers.
"""

from __future__ import annotations

import json
from pathlib import Path
import re
import stat
import tempfile
import unittest

from test_api_auth_docs import ROOT, TEST_TOKEN, exercise_curl, recording_server


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
        ("GET", "/api/audit?event=login&startTime=2026-01-01T00:00:00Z&endTime=2026-01-31T23:59:59Z&success=false", None),
        ("GET", "/api/audit/summary", None),
        ("GET", "/api/audit/export", None),
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
            if "curl " in step:
                requests.append(step)
    return requests


class AdminDocsTest(unittest.TestCase):
    def test_shell_recipes_never_expose_credentials_or_override_tls(self):
        for name in GUIDES:
            with self.subTest(guide=name):
                steps = recipes(name)
                self.assertEqual(len(steps), len(EXPECTED[name]))
                for step in steps:
                    self.assertNotRegex(step, r"(?:\b\w*TOKEN=|Authorization:|X-API-Token:|Bearer\s|--cookie\b)")
                    self.assertNotRegex(step, r"(?:--insecure|--verbose|--trace\S*|--location|\s-k\b)")
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
        for boundary in ("audit:read", "audit_logging", "private directory", "redacted error", "does not reuse filters"):
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
                            result = exercise_curl(self, Path(temporary), f"{key}: {value}", port, step)
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
                            result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port, step)
                            self.assertEqual(result.returncode, 22, result.stderr.decode())
                            self.assertEqual(len(requests), before + 1)
                            self.assertNotIn(b"Saved private export", result.stdout)
                            # curl preserves an error body, in stdout or the requested file.
                            exports = list(Path(temporary).glob("*/audit-export.json"))
                            output = exports[0].read_bytes() if exports else result.stdout
                            self.assertIn(b'{"fixture":true}', output)

    def test_audit_export_creates_a_private_new_directory_and_file(self):
        step = next(step for step in recipes("AUDIT_LOGGING") if "/api/audit/export" in step)
        with recording_server() as (port, _), tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            for _ in range(2):
                result = exercise_curl(self, home, f"X-API-Token: {TEST_TOKEN}", port, step)
                self.assertEqual(result.returncode, 0, result.stderr.decode())
                output = Path(result.stdout.decode().strip().removeprefix("Saved private export to "))
                self.assertEqual(output.parent.parent, home)
                self.assertEqual(stat.S_IMODE(output.parent.stat().st_mode), 0o700)
                self.assertEqual(stat.S_IMODE(output.stat().st_mode), 0o600)
                self.assertEqual(json.loads(output.read_bytes()), {"fixture": True})
            self.assertEqual(len(list(home.glob("*/audit-export.json"))), 2)


if __name__ == "__main__":
    unittest.main()
