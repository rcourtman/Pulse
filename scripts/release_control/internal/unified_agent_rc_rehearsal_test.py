from __future__ import annotations

import contextlib
import io
import json
import tempfile
import threading
import unittest
from dataclasses import asdict
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from unified_agent_rc_rehearsal import (
    agent_binary_asset_name,
    check_update_info,
    main,
    render_markdown_report,
    release_asset_url,
    run_rehearsal,
    summarize_http_error_body,
)


def run_main(argv: list[str]) -> int:
    with contextlib.redirect_stdout(io.StringIO()):
        return main(argv)


class _FixtureHandler(BaseHTTPRequestHandler):
    routes: dict[str, tuple[int, bytes, dict[str, str]]] = {}
    request_headers: dict[str, dict[str, str]] = {}

    def do_GET(self) -> None:  # noqa: N802
        self.request_headers[self.path] = {
            key.lower(): value for key, value in self.headers.items()
        }
        status, body, headers = self.routes.get(self.path, (404, b"missing", {}))
        self.send_response(status)
        for key, value in headers.items():
            self.send_header(key, value)
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format: str, *args: object) -> None:  # noqa: A003
        return


class UnifiedAgentRCRehearsalTest(unittest.TestCase):
    def setUp(self) -> None:
        self.server = ThreadingHTTPServer(("127.0.0.1", 0), _FixtureHandler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.base_url = f"http://127.0.0.1:{self.server.server_port}"

    def tearDown(self) -> None:
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)

    def set_routes(self, routes: dict[str, tuple[int, bytes, dict[str, str]]]) -> None:
        _FixtureHandler.routes = routes
        _FixtureHandler.request_headers = {}

    def test_release_asset_url(self) -> None:
        got = release_asset_url("https://example.invalid/releases/download/", "6.0.0-rc.1", "install.sh")
        self.assertEqual(got, "https://example.invalid/releases/download/v6.0.0-rc.1/install.sh")

    def test_agent_binary_asset_name_windows(self) -> None:
        self.assertEqual(agent_binary_asset_name("windows-amd64"), "pulse-agent-windows-amd64.exe")
        self.assertEqual(agent_binary_asset_name("linux-amd64"), "pulse-agent-linux-amd64")

    def test_run_rehearsal_passes_with_matching_assets(self) -> None:
        install = b"#!/bin/sh\necho install\n"
        ps1 = b"Write-Output 'install'\n"
        binary = b"pulse-agent-binary"
        checksum = __import__("hashlib").sha256(binary).hexdigest()

        self.set_routes(
            {
                "/pulse/api/agent/version": (
                    200,
                    json.dumps({"version": "6.0.0-rc.1"}).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/pulse/install.sh": (200, install, {}),
                "/pulse/install.ps1": (200, ps1, {}),
                "/pulse/download/pulse-agent?arch=linux-amd64": (
                    200,
                    binary,
                    {"X-Checksum-Sha256": checksum, "X-Served-From": "github-fallback"},
                ),
                "/releases/v6.0.0-rc.1/install.sh": (200, install, {}),
                "/releases/v6.0.0-rc.1/install.ps1": (200, ps1, {}),
                "/releases/v6.0.0-rc.1/pulse-agent-linux-amd64": (200, binary, {}),
            }
        )

        args = type(
            "Args",
            (),
            {
                "base_url": f"{self.base_url}/pulse",
                "expected_version": "6.0.0-rc.1",
                "release_base_url": f"{self.base_url}/releases",
                "arch": ["linux-amd64"],
                "update_info_dir": None,
                "expected_updated_from": None,
                "timeout": 5.0,
                "api_token": None,
                "bearer_token": None,
                "cookie": None,
                "expected_active_agents": None,
                "expected_agent_name": [],
                "expected_online_agents": None,
                "json": False,
            },
        )()
        results = run_rehearsal(args)
        self.assertTrue(all(result.ok for result in results), [asdict(result) for result in results])

    def test_run_rehearsal_fails_on_binary_checksum_header_mismatch(self) -> None:
        install = b"#!/bin/sh\n"
        self.set_routes(
            {
                "/pulse/api/agent/version": (
                    200,
                    json.dumps({"version": "6.0.0-rc.1"}).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/pulse/install.sh": (200, install, {}),
                "/pulse/install.ps1": (200, b"ps1", {}),
                "/pulse/download/pulse-agent?arch=linux-amd64": (
                    200,
                    b"binary",
                    {"X-Checksum-Sha256": "bad"},
                ),
                "/releases/v6.0.0-rc.1/install.sh": (200, install, {}),
                "/releases/v6.0.0-rc.1/install.ps1": (200, b"ps1", {}),
                "/releases/v6.0.0-rc.1/pulse-agent-linux-amd64": (200, b"binary", {}),
            }
        )

        exit_code = run_main(
            [
                "--base-url",
                f"{self.base_url}/pulse",
                "--expected-version",
                "6.0.0-rc.1",
                "--release-base-url",
                f"{self.base_url}/releases",
                "--arch",
                "linux-amd64",
                "--json",
            ]
        )
        self.assertEqual(exit_code, 1)

    def test_run_rehearsal_reports_missing_release_asset_without_crashing(self) -> None:
        install = b"#!/bin/sh\n"
        self.set_routes(
            {
                "/pulse/api/agent/version": (
                    200,
                    json.dumps({"version": "6.0.0-rc.1"}).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/pulse/install.sh": (200, install, {}),
                "/pulse/install.ps1": (200, b"ps1", {}),
                "/releases/v6.0.0-rc.1/install.ps1": (200, b"ps1", {}),
            }
        )

        exit_code = run_main(
            [
                "--base-url",
                f"{self.base_url}/pulse",
                "--expected-version",
                "6.0.0-rc.1",
                "--release-base-url",
                f"{self.base_url}/releases",
                "--json",
            ]
        )
        self.assertEqual(exit_code, 1)

    def test_summarize_http_error_body_omits_html_page(self) -> None:
        body = b"<!DOCTYPE html>\n<html><body>missing</body></html>"
        self.assertEqual(summarize_http_error_body(body), "<html error page omitted>")

    def test_check_update_info(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            info_path = Path(tmp) / ".pulse-update-info"
            info_path.write_text("5.1.14\n", encoding="utf-8")
            result = check_update_info(tmp, "5.1.14")
            self.assertTrue(result.ok)

    def test_check_update_info_requires_matching_content(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            info_path = Path(tmp) / ".pulse-update-info"
            info_path.write_text("5.1.13\n", encoding="utf-8")
            result = check_update_info(tmp, "5.1.14")
            self.assertFalse(result.ok)

    def test_render_markdown_report(self) -> None:
        report = render_markdown_report(
            title="Unified Agent RC Rehearsal",
            base_url="https://pulse.example.com",
            expected_version="6.0.0-rc.1",
            release_base_url="https://github.com/example/releases/download",
            results=[
                type("R", (), {"name": "agent-version-endpoint", "ok": True, "detail": "matched"})(),
                type("R", (), {"name": "agent-binary-linux-amd64", "ok": False, "detail": "checksum mismatch"})(),
            ],
        )
        self.assertIn("# Unified Agent RC Rehearsal", report)
        self.assertIn("`PASS` `agent-version-endpoint`", report)
        self.assertIn("`FAIL` `agent-binary-linux-amd64`", report)
        self.assertIn("## Manual Follow-up", report)

    def test_main_writes_report(self) -> None:
        install = b"#!/bin/sh\necho install\n"
        ps1 = b"Write-Output 'install'\n"
        self.set_routes(
            {
                "/pulse/api/agent/version": (
                    200,
                    json.dumps({"version": "6.0.0-rc.1"}).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/pulse/install.sh": (200, install, {}),
                "/pulse/install.ps1": (200, ps1, {}),
                "/releases/v6.0.0-rc.1/install.sh": (200, install, {}),
                "/releases/v6.0.0-rc.1/install.ps1": (200, ps1, {}),
            }
        )
        with tempfile.TemporaryDirectory() as tmp:
            report_path = Path(tmp) / "report.md"
            exit_code = main(
                [
                    "--base-url",
                    f"{self.base_url}/pulse",
                    "--expected-version",
                    "6.0.0-rc.1",
                    "--release-base-url",
                    f"{self.base_url}/releases",
                    "--report-out",
                    str(report_path),
                ]
            )
            self.assertEqual(exit_code, 0)
            content = report_path.read_text(encoding="utf-8")
            self.assertIn("# Unified Agent RC Rehearsal", content)
            self.assertIn("`PASS` `install-sh-asset`", content)

    def test_run_rehearsal_verifies_active_agent_accounting_via_api_token(self) -> None:
        install = b"#!/bin/sh\necho install\n"
        ps1 = b"Write-Output 'install'\n"
        self.set_routes(
            {
                "/pulse/api/agent/version": (
                    200,
                    json.dumps({"version": "6.0.0-rc.1"}).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/pulse/install.sh": (200, install, {}),
                "/pulse/install.ps1": (200, ps1, {}),
                "/pulse/api/license/entitlements": (
                    200,
                    json.dumps(
                        {
                            "limits": [
                                {"key": "max_agents", "limit": 10, "current": 3, "state": "ok"}
                            ]
                        }
                    ).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/pulse/api/license/agent-ledger": (
                    200,
                    json.dumps(
                        {
                            "agents": [{"name": "a"}, {"name": "b"}, {"name": "c"}],
                            "total": 3,
                            "limit": 10,
                        }
                    ).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/releases/v6.0.0-rc.1/install.sh": (200, install, {}),
                "/releases/v6.0.0-rc.1/install.ps1": (200, ps1, {}),
            }
        )

        exit_code = run_main(
            [
                "--base-url",
                f"{self.base_url}/pulse",
                "--expected-version",
                "6.0.0-rc.1",
                "--release-base-url",
                f"{self.base_url}/releases",
                "--api-token",
                "token-123",
                "--expected-active-agents",
                "3",
                "--json",
            ]
        )
        self.assertEqual(exit_code, 0)
        self.assertEqual(
            _FixtureHandler.request_headers["/pulse/api/license/entitlements"].get("x-api-token"),
            "token-123",
        )
        self.assertEqual(
            _FixtureHandler.request_headers["/pulse/api/license/agent-ledger"].get("x-api-token"),
            "token-123",
        )

    def test_run_rehearsal_fails_when_active_agent_accounting_mismatches(self) -> None:
        install = b"#!/bin/sh\necho install\n"
        ps1 = b"Write-Output 'install'\n"
        self.set_routes(
            {
                "/pulse/api/agent/version": (
                    200,
                    json.dumps({"version": "6.0.0-rc.1"}).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/pulse/install.sh": (200, install, {}),
                "/pulse/install.ps1": (200, ps1, {}),
                "/pulse/api/license/entitlements": (
                    200,
                    json.dumps(
                        {
                            "limits": [
                                {"key": "max_agents", "limit": 10, "current": 2, "state": "ok"}
                            ]
                        }
                    ).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/pulse/api/license/agent-ledger": (
                    200,
                    json.dumps(
                        {
                            "agents": [{"name": "a"}, {"name": "b"}, {"name": "c"}],
                            "total": 3,
                            "limit": 10,
                        }
                    ).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/releases/v6.0.0-rc.1/install.sh": (200, install, {}),
                "/releases/v6.0.0-rc.1/install.ps1": (200, ps1, {}),
            }
        )

        exit_code = run_main(
            [
                "--base-url",
                f"{self.base_url}/pulse",
                "--expected-version",
                "6.0.0-rc.1",
                "--release-base-url",
                f"{self.base_url}/releases",
                "--api-token",
                "token-123",
                "--expected-active-agents",
                "3",
                "--json",
            ]
        )
        self.assertEqual(exit_code, 1)

    def test_run_rehearsal_requires_auth_for_active_agent_accounting(self) -> None:
        install = b"#!/bin/sh\necho install\n"
        ps1 = b"Write-Output 'install'\n"
        self.set_routes(
            {
                "/pulse/api/agent/version": (
                    200,
                    json.dumps({"version": "6.0.0-rc.1"}).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/pulse/install.sh": (200, install, {}),
                "/pulse/install.ps1": (200, ps1, {}),
                "/releases/v6.0.0-rc.1/install.sh": (200, install, {}),
                "/releases/v6.0.0-rc.1/install.ps1": (200, ps1, {}),
            }
        )

        exit_code = run_main(
            [
                "--base-url",
                f"{self.base_url}/pulse",
                "--expected-version",
                "6.0.0-rc.1",
                "--release-base-url",
                f"{self.base_url}/releases",
                "--expected-active-agents",
                "3",
                "--json",
            ]
        )
        self.assertEqual(exit_code, 1)

    def test_run_rehearsal_verifies_expected_agent_name_and_online_status(self) -> None:
        install = b"#!/bin/sh\necho install\n"
        ps1 = b"Write-Output 'install'\n"
        self.set_routes(
            {
                "/pulse/api/agent/version": (
                    200,
                    json.dumps({"version": "6.0.0-rc.1"}).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/pulse/install.sh": (200, install, {}),
                "/pulse/install.ps1": (200, ps1, {}),
                "/pulse/api/license/agent-ledger": (
                    200,
                    json.dumps(
                        {
                            "agents": [
                                {
                                    "name": "workstation-01",
                                    "type": "agent",
                                    "status": "online",
                                    "last_seen": "2026-03-12T12:00:00Z",
                                    "source": "agent",
                                }
                            ],
                            "total": 1,
                            "limit": 10,
                        }
                    ).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/releases/v6.0.0-rc.1/install.sh": (200, install, {}),
                "/releases/v6.0.0-rc.1/install.ps1": (200, ps1, {}),
            }
        )

        exit_code = run_main(
            [
                "--base-url",
                f"{self.base_url}/pulse",
                "--expected-version",
                "6.0.0-rc.1",
                "--release-base-url",
                f"{self.base_url}/releases",
                "--api-token",
                "token-123",
                "--expected-agent-name",
                "workstation-01",
                "--expected-online-agents",
                "1",
                "--json",
            ]
        )
        self.assertEqual(exit_code, 0)

    def test_run_rehearsal_fails_when_expected_agent_name_is_duplicated(self) -> None:
        install = b"#!/bin/sh\necho install\n"
        ps1 = b"Write-Output 'install'\n"
        self.set_routes(
            {
                "/pulse/api/agent/version": (
                    200,
                    json.dumps({"version": "6.0.0-rc.1"}).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/pulse/install.sh": (200, install, {}),
                "/pulse/install.ps1": (200, ps1, {}),
                "/pulse/api/license/agent-ledger": (
                    200,
                    json.dumps(
                        {
                            "agents": [
                                {"name": "workstation-01", "status": "online"},
                                {"name": "workstation-01", "status": "online"},
                            ],
                            "total": 2,
                            "limit": 10,
                        }
                    ).encode("utf-8"),
                    {"Content-Type": "application/json"},
                ),
                "/releases/v6.0.0-rc.1/install.sh": (200, install, {}),
                "/releases/v6.0.0-rc.1/install.ps1": (200, ps1, {}),
            }
        )

        exit_code = run_main(
            [
                "--base-url",
                f"{self.base_url}/pulse",
                "--expected-version",
                "6.0.0-rc.1",
                "--release-base-url",
                f"{self.base_url}/releases",
                "--api-token",
                "token-123",
                "--expected-agent-name",
                "workstation-01",
                "--json",
            ]
        )
        self.assertEqual(exit_code, 1)


if __name__ == "__main__":
    unittest.main()
