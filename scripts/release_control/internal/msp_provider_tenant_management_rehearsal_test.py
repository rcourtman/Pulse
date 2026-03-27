from __future__ import annotations

import contextlib
import io
import json
import socket
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import msp_provider_tenant_management_rehearsal as mod


def run_main(argv: list[str]) -> int:
    with contextlib.redirect_stdout(io.StringIO()):
        return mod.main(argv)


class MSPProviderTenantManagementRehearsalTest(unittest.TestCase):
    def test_run_rehearsal_full_flow(self) -> None:
        calls: list[tuple[str, str, dict | None]] = []
        tenants_state = [
            {"id": "t-1", "display_name": "Client One", "account_id": "acct_1", "plan_version": "msp_starter"},
            {"id": "t-2", "display_name": "Client Two", "account_id": "acct_1", "plan_version": "msp_starter"},
        ]
        members_state = [
            {"email": "owner@example.com", "role": "owner"},
        ]

        def fake_fetch_json(method: str, url: str, **kwargs):
            calls.append((method, url, kwargs.get("headers")))
            if url.endswith("/api/accounts/acct_1/tenants") and method == "GET":
                return 200, tenants_state
            if url.endswith("/api/accounts/acct_1/tenants") and method == "POST":
                payload = kwargs["json_body"]
                tenant = {
                    "id": "t-3",
                    "display_name": payload["display_name"],
                    "account_id": "acct_1",
                    "plan_version": "msp_starter",
                }
                tenants_state.append(tenant)
                return 201, tenant
            if url.endswith("/api/accounts/acct_1/members") and method == "POST":
                payload = kwargs["json_body"]
                members_state.append({"email": payload["email"], "role": payload["role"]})
                return 201, {"ok": True}
            if url.endswith("/api/accounts/acct_1/members") and method == "GET":
                return 200, members_state
            if url.endswith("/api/portal/dashboard?account_id=acct_1"):
                return 200, {
                    "account": {"kind": "msp"},
                    "summary": {"total": 3},
                }
            if url.endswith("/api/portal/workspaces/t-3?account_id=acct_1"):
                return 200, {
                    "account": {"kind": "msp"},
                    "workspace": {"display_name": "Client Three", "plan_version": "msp_starter"},
                }
            if url.endswith("/api/public/signup"):
                return 400, {"code": "tier_unavailable"}
            raise AssertionError(f"unexpected call: {method} {url}")

        with mock.patch.object(mod, "fetch_json", side_effect=fake_fetch_json):
            args = mod.parse_args(
                [
                    "--base-url",
                    "https://pulse.example.com",
                    "--account-id",
                    "acct_1",
                    "--bearer-token",
                    "token",
                    "--workspace-name",
                    "Client Three",
                    "--member-email",
                    "tech@example.com",
                    "--member-role",
                    "tech",
                    "--public-signup-email",
                    "public@example.com",
                    "--public-signup-org-name",
                    "Public Boundary",
                ]
            )
            results = mod.run_rehearsal(args)

        self.assertTrue(all(result.ok for result in results), [result.detail for result in results])
        headers = next(headers for method, url, headers in calls if method == "POST" and url.endswith("/api/accounts/acct_1/tenants"))
        self.assertEqual(headers["Authorization"], "Bearer token")

    def test_run_rehearsal_fails_when_created_workspace_drifts_plan(self) -> None:
        def fake_fetch_json(method: str, url: str, **kwargs):
            if url.endswith("/api/accounts/acct_1/tenants") and method == "GET":
                return 200, []
            if url.endswith("/api/accounts/acct_1/tenants") and method == "POST":
                return 201, {
                    "id": "t-3",
                    "display_name": "Client Three",
                    "account_id": "acct_1",
                    "plan_version": "cloud_starter",
                }
            if url.endswith("/api/accounts/acct_1/members"):
                return 200, []
            if url.endswith("/api/portal/dashboard?account_id=acct_1"):
                return 200, {"account": {"kind": "msp"}, "summary": {"total": 1}}
            if url.endswith("/api/public/signup"):
                return 400, {"code": "tier_unavailable"}
            raise AssertionError(f"unexpected call: {method} {url}")

        with mock.patch.object(mod, "fetch_json", side_effect=fake_fetch_json):
            exit_code = run_main(
                [
                    "--base-url",
                    "https://pulse.example.com",
                    "--account-id",
                    "acct_1",
                    "--api-token",
                    "token",
                    "--workspace-name",
                    "Client Three",
                ]
            )
        self.assertEqual(exit_code, 1)

    def test_main_returns_failure_when_workspace_create_raises_runtime_error(self) -> None:
        def fake_fetch_json(method: str, url: str, **kwargs):
            if url.endswith("/api/accounts/acct_1/tenants") and method == "GET":
                return 200, []
            if url.endswith("/api/accounts/acct_1/tenants") and method == "POST":
                raise RuntimeError("POST https://pulse.example.com/api/accounts/acct_1/tenants returned non-JSON body: internal error")
            if url.endswith("/api/accounts/acct_1/members") and method == "GET":
                return 200, []
            if url.endswith("/api/portal/dashboard?account_id=acct_1"):
                return 200, {"account": {"kind": "msp"}, "summary": {"total": 1}}
            raise AssertionError(f"unexpected call: {method} {url}")

        with mock.patch.object(mod, "fetch_json", side_effect=fake_fetch_json):
            exit_code = run_main(
                [
                    "--base-url",
                    "https://pulse.example.com",
                    "--account-id",
                    "acct_1",
                    "--api-token",
                    "token",
                    "--workspace-name",
                    "Client Three",
                ]
            )
        self.assertEqual(exit_code, 1)

    def test_render_markdown_report(self) -> None:
        report = mod.render_markdown_report(
            title="MSP Provider Tenant Management Rehearsal",
            base_url="https://pulse.example.com",
            account_id="acct_1",
            results=[
                mod.CheckResult(name="msp-tenant-list", ok=True, detail="tenant_count=2"),
                mod.CheckResult(name="public-cloud-boundary", ok=False, detail="status=500"),
            ],
        )
        self.assertIn("# MSP Provider Tenant Management Rehearsal", report)
        self.assertIn("`PASS` `msp-tenant-list`", report)
        self.assertIn("`FAIL` `public-cloud-boundary`", report)

    def test_main_writes_report(self) -> None:
        def fake_fetch_json(method: str, url: str, **kwargs):
            if url.endswith("/api/accounts/acct_1/tenants") and method == "GET":
                return 200, []
            if url.endswith("/api/accounts/acct_1/members") and method == "GET":
                return 200, []
            if url.endswith("/api/portal/dashboard?account_id=acct_1"):
                return 200, {"account": {"kind": "msp"}, "summary": {"total": 1}}
            raise AssertionError(f"unexpected call: {method} {url}")

        with tempfile.TemporaryDirectory() as tmp:
            report_path = Path(tmp) / "report.md"
            with mock.patch.object(mod, "fetch_json", side_effect=fake_fetch_json):
                exit_code = mod.main(
                    [
                        "--base-url",
                        "https://pulse.example.com",
                        "--account-id",
                        "acct_1",
                        "--api-token",
                        "token",
                        "--report-out",
                        str(report_path),
                    ]
                )
            self.assertEqual(exit_code, 0)
            self.assertIn("# MSP Provider Tenant Management Rehearsal", report_path.read_text(encoding="utf-8"))

    def test_main_returns_failure_on_timeout_without_traceback(self) -> None:
        with mock.patch.object(mod.request, "urlopen", side_effect=socket.timeout("timed out")):
            exit_code = run_main(
                [
                    "--base-url",
                    "https://pulse.example.com",
                    "--account-id",
                    "acct_1",
                    "--api-token",
                    "token",
                ]
            )
        self.assertEqual(exit_code, 1)


if __name__ == "__main__":
    unittest.main()
