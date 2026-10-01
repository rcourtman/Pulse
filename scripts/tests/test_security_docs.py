#!/usr/bin/env python3
"""Execute the canonical security guide with synthetic secrets and loopback.

Run via pulse-worker-source-proof. The fixture checks copied requests and private
files, not a deployed Pulse instance; existing Go tests check transfer authority,
encryption and lockout handlers.
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

from test_api_auth_docs import ROOT, TEST_TOKEN, exercise_curl, recording_server


DOC = ROOT / "SECURITY.md"
TEST_PASSPHRASE = "synthetic-security-export-passphrase"
EXPECTED = (
    ("GET", "/api/state/summary", None),
    ("POST", "/api/config/export", {"passphrase": TEST_PASSPHRASE}),
    ("GET", "/api/monitoring/scheduler/health", None),
    ("POST", "/api/security/reset-lockout", {"identifier": "username"}),
    ("POST", "/api/security/reset-lockout", {"identifier": "198.51.100.100"}),
)


def blocks() -> list[str]:
    return re.findall(r"```bash\n(.*?)```", DOC.read_text(encoding="utf-8"), re.DOTALL)


def recipes() -> list[str]:
    return [step for block in blocks() for step in re.split(r"\n(?=# )", block)
            if "curl " in step]


def private_export_request(home: Path):
    request = home / ".config/pulse/export-request.json"
    request.parent.mkdir(parents=True, exist_ok=True)
    request.write_text(json.dumps({"passphrase": TEST_PASSPHRASE}))
    request.chmod(0o600)


class SecurityDocsTest(unittest.TestCase):
    def test_shipped_copy_matches_canonical_guide(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/SECURITY.md").read_bytes())

    def test_copied_requests_never_expose_credentials_or_weaken_tls(self):
        requests = recipes()
        self.assertEqual(len(requests), len(EXPECTED))
        for request in requests:
            self.assertNotRegex(request, r"(?:\b\w*TOKEN=|X-API-Token:|Authorization:|Bearer\s|--cookie\b)")
            self.assertNotRegex(request, r"(?:--insecure|--verbose|--trace\S*|--location|\s-k\b)")
            self.assertIn('curl --disable --fail-with-body --header "@$HOME/.config/pulse/api-header"', request)
            if "/api/config/export" in request:
                self.assertIn('--data-binary "@$HOME/.config/pulse/export-request.json"', request)
                self.assertNotIn('"passphrase":', request)
            elif "--request POST" in request:
                self.assertIn("--data-binary @-", request)
                self.assertIn("<<'JSON'", request)
        shell = "\n".join(blocks())
        self.assertNotRegex(shell, r"(?:--token\s|PULSE_AUTH_PASS=|API_TOKEN[S]?=|curl[^\n]*\|)")
        self.assertNotIn("rm -rf", shell)
        self.assertNotIn("authorized_keys", shell)

    def test_export_preparation_protects_new_and_existing_files_without_erasing_them(self):
        preparation = next(block for block in blocks() if 'vi "$HOME/.config/pulse/export-request.json"' in block)
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            tools = home / "tools"
            tools.mkdir()
            editor = tools / "vi"
            editor.write_text('#!/bin/sh\n[ "$#" = 1 ] && [ "$1" = "$HOME/.config/pulse/export-request.json" ]\n')
            editor.chmod(0o700)
            env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}")
            request = home / ".config/pulse/export-request.json"
            for existing in (False, True):
                with self.subTest(existing=existing):
                    if existing:
                        private_export_request(home)
                        request.chmod(0o644)
                        request.parent.chmod(0o755)
                    subprocess.run(["bash", "-eu", "-c", preparation], env=env, check=True, timeout=10)
                    self.assertEqual(stat.S_IMODE(request.stat().st_mode), 0o600)
                    self.assertEqual(stat.S_IMODE(request.parent.stat().st_mode), 0o700)
                    self.assertEqual(request.read_text(), json.dumps({"passphrase": TEST_PASSPHRASE}) if existing else "")

    def test_exact_methods_paths_and_bodies_with_both_supported_headers(self):
        steps = recipes()
        with recording_server() as (port, requests):
            for key, value in (("X-API-Token", TEST_TOKEN), ("Authorization", "Bearer " + TEST_TOKEN)):
                for step, (method, path, body) in zip(steps, EXPECTED):
                    with self.subTest(header=key, method=method, path=path), tempfile.TemporaryDirectory() as temporary:
                        home = Path(temporary)
                        private_export_request(home)
                        result = exercise_curl(self, home, f"{key}: {value}", port, step)
                        self.assertEqual(result.returncode, 0, result.stderr.decode())
                        self.assertEqual(requests[-1][0], path)
                        self.assertEqual(requests[-1][1][key], value)
                        self.assertNotIn("X-Curlrc-Injected", requests[-1][1])
                        self.assertEqual(requests[-1][2], method)
                        self.assertEqual(json.loads(requests[-1][3]) if body is not None else None, body)
                        self.assertNotIn(TEST_PASSPHRASE.encode(), result.stdout + result.stderr)
                        self.assertNotIn(TEST_PASSPHRASE, (home / "argv.json").read_text())
                        if path == "/api/config/export":
                            self.check_export_files(home, result, success=True)

    def check_export_files(self, home: Path, result, *, success: bool):
        directories = list(home.glob("pulse-export.*"))
        self.assertEqual(len(directories), 1)
        directory = directories[0]
        output = directory / "config-export.json"
        self.assertEqual(stat.S_IMODE(directory.stat().st_mode), 0o700)
        self.assertEqual(stat.S_IMODE(output.stat().st_mode), 0o600)
        # A recording server does not prove encryption. This only checks the
        # exact response is saved privately, with truthful HTTP success/failure.
        self.assertEqual(output.read_text(), '{"fixture":true}\n')
        self.assertNotIn(TEST_TOKEN, output.read_text())
        self.assertNotIn(TEST_PASSPHRASE, output.read_text())
        if success:
            self.assertIn(b"Export response saved", result.stdout)
        else:
            self.assertNotIn(b"Export response saved", result.stdout)
            self.assertIn(b"Export failed; do not import", result.stderr)

    def test_unauthorised_and_wrong_scope_responses_never_report_success(self):
        for status in (401, 403, 500):
            with self.subTest(status=status), recording_server(status) as (port, requests):
                for step, (_, path, _) in zip(recipes(), EXPECTED):
                    with self.subTest(path=path), tempfile.TemporaryDirectory() as temporary:
                        home = Path(temporary)
                        private_export_request(home)
                        result = exercise_curl(self, home, f"X-API-Token: {TEST_TOKEN}", port, step)
                        self.assertNotEqual(result.returncode, 0)
                        self.assertEqual(requests[-1][0], path)
                        if path == "/api/config/export":
                            self.check_export_files(home, result, success=False)

    def test_guidance_retains_authority_and_credential_lifecycle_boundaries(self):
        guide = " ".join(DOC.read_text().split())
        for boundary in ("monitoring:read", "settings:read", "settings:write", "CSRF",
                         "organization-bound", "direct loopback", "export only",
                         "12 characters", "public", "separate from the export",
                         "entire configuration bundle", "31 March 2027", "no longer sold",
                         "does not reset a password", "deleting `.env` is not a universal reset",
                         "fingerprint", "Revoking the public key", "privileged inspection"):
            self.assertTrue(boundary in guide, f"missing safety boundary: {boundary}")
        self.assertIn("docs/UNIFIED_AGENT.md#private-file-installation-linux-macos-and-nas", guide)
        self.assertIn("docs/TROUBLESHOOTING.md#i-forgot-my-password", guide)
        self.assertNotIn("### What's **Not** Encrypted", guide)


if __name__ == "__main__":
    unittest.main()
