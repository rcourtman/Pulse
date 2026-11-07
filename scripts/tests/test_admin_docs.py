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
    if "pulse_api " in step:
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
                    self.assertIn("pulse_api ", step)
                    self.assertNotIn("curl ", step)
                    if re.search(r"(?:--request|pulse_api) (POST|PUT)", step):
                        self.assertIn("--data-binary @-", request_helper() if "pulse_api " in step else step)
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
                         "reject malformed or empty supplied", "before reading events", "not `success` or list", "partial file", "does not follow redirects or retry"):
            self.assertIn(boundary, audit)
        for boundary in ("10,000 matching events", "export_too_large", "not a partial download",
                         "truncated summary", "not a byte-size", "query-duration guarantee"):
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

    def test_api_reference_uses_its_custom_role_and_links_full_policy_guidance(self):
        reference = (DOCS / "API.md").read_text()

        def section(heading):
            return reference.split(f"### {heading}\n", 1)[1].split("\n### ", 1)[0]

        create = json.loads(re.search(r"```json\n(.*?)```", section("Create Role"), re.S)[1])
        assignment = json.loads(re.search(r"```json\n(.*?)```", section("Set User Roles"), re.S)[1])
        self.assertEqual(create["id"], EXPECTED["RBAC"][0][2]["id"])
        self.assertNotIn(create["id"], ("admin", "operator", "viewer", "auditor"))
        self.assertEqual(assignment["roleIds"], [create["id"], "viewer"])
        self.assertEqual(create["permissions"], [
            {"action": "read", "resource": "alerts"},
            {"action": "write", "resource": "alerts"},
        ])
        update = section("Update Role")
        self.assertIn("Full policy replacement.", update)
        self.assertIn("[Preserve saved fields](RBAC.md#before-using-the-api)", update)
        self.assertNotIn("name, description, or permissions", update)

    def test_rbac_setup_guidance_matches_navigation_and_failure_states(self):
        guide = (DOCS / "RBAC.md").read_text()
        readable = " ".join(guide.replace("**", "").split())
        frontend = ROOT / "frontend-modern/src"
        navigation = (frontend / "components/Settings/settingsNavCatalog.ts").read_text()
        for panel in ("roles", "users"):
            with self.subTest(panel=panel):
                entry = re.search(
                    rf"id: 'security-{panel}',(.*?)(?=\n\s+id:|\Z)",
                    navigation, re.DOTALL,
                ).group(1)
                self.assertIn("features: ['rbac']", entry)
                self.assertIn("hideWhenUnavailable: true", entry)
                self.assertIn(f"requiredCapability: '{panel}'", entry)
        for label, source in (
            ("Pro runtime missing", "utils/licensePresentation.ts"),
            ("Open Pulse Pro downloads", "utils/licensePresentation.ts"),
            ("Roles are unavailable for this session", "components/Settings/RolesPanel.tsx"),
            ("User access is unavailable for this session", "components/Settings/UserAssignmentsPanel.tsx"),
            ("Failed to load roles", "utils/rbacPresentation.ts"),
            ("Failed to load user assignments", "utils/rbacPresentation.ts"),
        ):
            with self.subTest(label=label):
                self.assertIn(label, readable)
                self.assertIn(label, " ".join((frontend / source).read_text().split()))
        for boundary in (
            "active licence is not an administrator grant",
            "licence and runtime status",
            "do not buy a second licence or reset the data directory",
            "independent administrator recovery path",
            "next sign-in replaces manually assigned roles",
            "#automatic-role-assignment-via-oidc",
            "runtime's `rbac` capability and this session's permission",
            "missing menu entry alone does not tell you which check failed",
            "do not remove proxy role gating or widen an agent's token",
            "not proof that no roles or users exist",
            "Do not create or delete access records to test it",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, readable)

    def test_rbac_unconfirmed_change_guidance_uses_existing_read_controls(self):
        guide = (DOCS / "RBAC.md").read_text()
        section = guide.split("### A role or user change could not be confirmed\n", 1)[1].split("\n---", 1)[0]
        readable = " ".join(section.replace("**", "").split())
        frontend = ROOT / "frontend-modern/src/components/Settings"
        for label, panel, reader in (
            ("Check saved roles", "RolesPanel.tsx", "state.loadRoles()"),
            ("Check saved access", "UserAssignmentsPanel.tsx", "state.loadData()"),
        ):
            with self.subTest(label=label):
                self.assertIn(label, readable)
                source = (frontend / panel).read_text()
                self.assertIn(f"'{label}'", source)
                self.assertIn(f"actionOnClick={{() => void {reader}}}", source)
        for boundary in (
            "after Pulse has saved a role or removed access",
            "Do not repeat the Save or Delete",
            "working administrator session open",
            "identity and complete role list",
            "do not repeat the change",
            "access remains unknown, not empty or unchanged",
            "Do not grant broader roles, widen a token or disable authentication",
            "not that the intended user's access works",
            "#automatic-role-assignment-via-oidc",
            "does not disable the upstream identity-provider account",
            "tokens, cookies and full responses private",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, readable)
        self.assertIn("#a-role-or-user-change-could-not-be-confirmed", guide)
        self.assertNotIn("```", section, "recovery must not introduce a mutation recipe")

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
                            # Every guide preserves the error in a private response file.
                            responses = list((Path(temporary) / ".config/pulse").glob("api-response.*"))
                            self.assertEqual(len(responses), 1)
                            output = responses[0].read_bytes()
                            self.assertNotIn(b'{"fixture":true}', result.stdout)
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

    def test_rbac_guidance_preserves_private_and_uncertain_change_boundaries(self):
        guide = (DOCS / "RBAC.md").read_text()
        readable = " ".join(guide.replace("**", "").split())
        for boundary in ("in the same Bash session", "not an installed Pulse command",
                         "owner-only file", "redacted", "five-second", "twenty-second",
                         "not as a top-to-bottom script", "administrator recovery path",
                         "empty role list, removes access", "does not follow redirects or retry",
                         "timeout can follow an applied change", "read before writing again",
                         "HTTP success", "upstream IdP account"):
            self.assertIn(boundary, readable)

    def test_role_replacement_guidance_preserves_the_current_policy(self):
        guide = (DOCS / "RBAC.md").read_text()
        readable = " ".join(guide.replace("**", "").split())
        for boundary in (
            "Role PUT replaces the complete policy",
            "existing role from `GET /api/admin/roles`",
            "exact ID, `parentId`, `priority`",
            "each permission's `effect` and `conditions`",
            "Missing `effect` means `allow`, not “keep deny”",
            "lowercase `allow` or `deny`",
            "Stop on failed reads or missing/null `permissions`",
            "Coordinate writers",
            "Re-read the list after saving, not just the write response",
            "examples use the new `alert-manager`",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, readable)
        update = guide.split("### Updating a Role\n", 1)[1].split("### Deleting a Role\n", 1)[0]
        self.assertIn("**Full replacement.** [Preserve saved fields](#before-using-the-api).", update)

        # The live tenant store, not the legacy file manager, owns this policy.
        provider = (ROOT / "internal/api/access_tenant_provider.go").read_text()
        self.assertIn("auth.NewSQLiteManager(auth.SQLiteManagerConfig{", provider)
        handler = (ROOT / "internal/api/access_control_handlers.go").read_text()
        put = handler.split("case http.MethodPut:", 1)[1].split("case http.MethodDelete:", 1)[0]
        self.assertIn("var role auth.Role", put)
        self.assertIn("role.ID = roleID", put)
        self.assertIn("manager.SaveRole(role)", put)
        store = (ROOT / "pkg/auth/sqlite_manager.go").read_text()
        save = store.split("func (m *SQLiteManager) SaveRoleWithContext(", 1)[1].split("func (m *SQLiteManager) getRoleUnsafe(", 1)[0]
        for field in ("name", "description", "parent_id", "priority"):
            self.assertIn(f"{field} = excluded.{field}", save)
        self.assertIn('DELETE FROM rbac_permissions WHERE role_id = ?', save)
        self.assertIn("effect := perm.GetEffect()", save)
        self.assertIn("json.Marshal(perm.Conditions)", save)
        role = (ROOT / "pkg/auth/rbac.go").read_text()
        self.assertIn('if p.Effect == "" {\n\t\treturn EffectAllow', role)
        evaluator = (ROOT / "pkg/auth/policy_evaluator.go").read_text()
        self.assertIn("if len(perm.Conditions) == 0 {\n\t\treturn true", evaluator)

    def test_rbac_operations_keep_private_success_bodies_in_new_owner_only_files(self):
        steps = recipes("RBAC")
        self.assertEqual(len(steps), len(EXPECTED["RBAC"]))
        with private_recording_server() as (port, requests), tempfile.TemporaryDirectory(prefix="rbac docs ") as temporary:
            home = Path(temporary)
            for header in (f"X-API-Token: {TEST_TOKEN}", f"Authorization: Bearer {TEST_TOKEN}"):
                for step, (method, path, _) in zip(steps, EXPECTED["RBAC"]):
                    with self.subTest(method=method, path=path, header=header.split(":")[0]):
                        before = len(requests)
                        result = exercise_curl(self, home, header, port,
                                               executable_recipe("RBAC", step), private_response=True)
                        self.assertEqual(result.returncode, 0, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1)
                        self.assertEqual((requests[-1][2], requests[-1][0]), (method, path))
                        self.assertIn(b"HTTP 200", result.stdout)
            self.assertEqual(len(list((home / ".config/pulse").glob("api-response.*"))), 17)

    def test_rbac_operations_keep_private_http_errors_and_fail_without_retry(self):
        for status in (400, 401, 402, 403, 500, 503):
            with private_recording_server(status) as (port, requests):
                for step in recipes("RBAC"):
                    with self.subTest(status=status, step=step), tempfile.TemporaryDirectory() as temporary:
                        before = len(requests)
                        result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port,
                                               executable_recipe("RBAC", step), private_response=True)
                        self.assertEqual(result.returncode, 22, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1)
                        self.assertIn(f"HTTP {status}".encode(), result.stdout)

    def test_rbac_operations_refuse_redirect_success_and_uncertain_transfer_retry(self):
        for status, partial, expected_exit in ((302, False, 1), (200, True, 18)):
            with private_recording_server(status, partial) as (port, requests):
                for step in recipes("RBAC"):
                    with self.subTest(status=status, partial=partial, step=step), tempfile.TemporaryDirectory() as temporary:
                        before = len(requests)
                        result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port,
                                               executable_recipe("RBAC", step), private_response=True)
                        self.assertEqual(result.returncode, expected_exit, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1, "an uncertain mutation must not be repeated or redirected")

    def test_org_feature_flag_setup_uses_the_installed_deployment(self):
        guide = (DOCS / "MULTI_TENANT.md").read_text()
        quick_start = guide.split("## Quick Start\n", 1)[1].split("## Concepts", 1)[0]
        flag_error = guide.split('### "Multi-tenant functionality', 1)[1].split("\n### ", 1)[0]
        for entry in (quick_start, flag_error):
            self.assertIn("#applying-the-feature-flag", entry)
            self.assertNotIn("in your environment and restart Pulse", entry)
        setup = guide.split("## Applying the feature flag\n", 1)[1].split("\n## ", 1)[0]
        readable = " ".join(setup.replace("**", "").split())
        for boundary in (
            "existing deployment's environment configuration", "Docker or Compose",
            "A restart alone does not apply a changed container environment.",
            "same pinned image, persistent data mount, credentials and other settings",
            "systemd", "unit or drop-in changed", "Kubernetes", "existing workload controller",
            "not its agents", "monitoring and alerts are unavailable",
            "working administrator recovery path", "not a licence or membership grant",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, readable)
        self.assertIn("DEPLOYMENT_MODELS.md#updates-by-model", setup)
        self.assertIn("CONFIGURATION.md#common-overrides-environment-variables", setup)
        self.assertNotIn("```", setup, "do not add an unqualified deployment mutation command")

    def test_org_guidance_preserves_private_bounded_reads_and_session_mutations(self):
        guide = (DOCS / "MULTI_TENANT.md").read_text()
        for boundary in ("in the same Bash session", "not an installed Pulse command",
                         "five-second", "twenty-second", "does not follow redirects or retry",
                         "owner-only file", "redacted", "partial file", "not a complete list",
                         "HTTP 401/402/403", "do not", "widen a token",
                         "mutations token-authenticated", "signed-in UI", "CSRF"):
            self.assertIn(boundary, guide)
        for step in recipes("MULTI_TENANT"):
            self.assertRegex(step, r"^pulse_api GET /api/orgs/")
            self.assertNotIn("curl ", step)

    def test_org_reads_keep_private_success_bodies_in_new_owner_only_files(self):
        steps = recipes("MULTI_TENANT")
        self.assertEqual(len(steps), len(EXPECTED["MULTI_TENANT"]))
        with private_recording_server() as (port, requests), tempfile.TemporaryDirectory(prefix="org docs ") as temporary:
            home = Path(temporary)
            for header in (f"X-API-Token: {TEST_TOKEN}", f"Authorization: Bearer {TEST_TOKEN}"):
                for step, (method, path, _) in zip(steps, EXPECTED["MULTI_TENANT"]):
                    with self.subTest(path=path, header=header.split(":")[0]):
                        before = len(requests)
                        result = exercise_curl(self, home, header, port,
                                               executable_recipe("MULTI_TENANT", step), private_response=True)
                        self.assertEqual(result.returncode, 0, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1)
                        self.assertEqual((requests[-1][2], requests[-1][0]), (method, path))
                        self.assertEqual(requests[-1][3], b"", "organization reads send no mutation body")
                        self.assertIn(b"HTTP 200", result.stdout)
            # Four reads retain four new files without overwriting the old response.
            self.assertEqual(len(list((home / ".config/pulse").glob("api-response.*"))), 5)

    def test_org_reads_keep_private_http_errors_and_fail_without_retry(self):
        for status in (400, 401, 402, 403, 500, 503):
            with private_recording_server(status) as (port, requests):
                for step in recipes("MULTI_TENANT"):
                    with self.subTest(status=status, step=step), tempfile.TemporaryDirectory() as temporary:
                        before = len(requests)
                        result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port,
                                               executable_recipe("MULTI_TENANT", step), private_response=True)
                        self.assertEqual(result.returncode, 22, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1)
                        self.assertIn(f"HTTP {status}".encode(), result.stdout)

    def test_org_reads_refuse_redirect_success_and_incomplete_transfer_retry(self):
        for status, partial, expected_exit in ((302, False, 1), (200, True, 18)):
            with private_recording_server(status, partial) as (port, requests):
                for step in recipes("MULTI_TENANT"):
                    with self.subTest(status=status, partial=partial, step=step), tempfile.TemporaryDirectory() as temporary:
                        before = len(requests)
                        result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port,
                                               executable_recipe("MULTI_TENANT", step), private_response=True)
                        self.assertEqual(result.returncode, expected_exit, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1, "do not follow redirects or retry partial reads")


if __name__ == "__main__":
    unittest.main()
