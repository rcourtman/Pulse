from __future__ import annotations

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
    release_asset_url,
    run_rehearsal,
)


class _FixtureHandler(BaseHTTPRequestHandler):
    routes: dict[str, tuple[int, bytes, dict[str, str]]] = {}

    def do_GET(self) -> None:  # noqa: N802
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

        exit_code = main(
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


if __name__ == "__main__":
    unittest.main()
