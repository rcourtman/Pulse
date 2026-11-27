#!/usr/bin/env python3
"""Check the copyable HTTPS drop-in without installing or starting any unit."""

import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]


def https_drop_in():
    guide = (ROOT / "docs/CONFIGURATION.md").read_text(encoding="utf-8")
    _, _, section = guide.partition("### HTTPS with systemd\n")
    section = section.split("\n### ", 1)[0]
    matches = re.findall(r"^\x60\x60\x60ini\n(.*?)^\x60\x60\x60$", section, re.M | re.S)
    if len(matches) != 1:
        raise AssertionError("expected one copyable systemd HTTPS drop-in")
    return matches[0]


class ServerHttpsDocsTest(unittest.TestCase):
    def test_drop_in_contains_all_three_service_environment_assignments(self):
        header, *assignments = https_drop_in().splitlines()
        self.assertEqual(header, "[Service]")
        variables = {}
        for assignment in assignments:
            match = re.fullmatch(r'Environment="([A-Z_]+)=([^"]+)"', assignment)
            self.assertIsNotNone(match, assignment)
            name, value = match.groups()
            self.assertNotIn(name, variables)
            variables[name] = value
        self.assertEqual(set(variables), {"HTTPS_ENABLED", "TLS_CERT_FILE", "TLS_KEY_FILE"})
        self.assertEqual(variables["HTTPS_ENABLED"], "true")
        for key in ("TLS_CERT_FILE", "TLS_KEY_FILE"):
            self.assertTrue(Path(variables[key]).is_absolute(), variables[key])

    @unittest.skipUnless(shutil.which("systemd-analyze"), "systemd parser unavailable")
    def test_systemd_accepts_the_example_as_a_drop_in(self):
        with tempfile.TemporaryDirectory(prefix="pulse-https-docs-") as directory:
            root = Path(directory)
            unit = root / "pulse-https-example.service"
            unit.write_text(
                "[Unit]\nDefaultDependencies=no\n[Service]\nExecStart=/bin/true\n",
                encoding="utf-8",
            )
            drop_ins = root / (unit.name + ".d")
            drop_ins.mkdir()
            (drop_ins / "override.conf").write_text(https_drop_in(), encoding="utf-8")
            result = subprocess.run(
                ["systemd-analyze", "verify", "--man=no", "--generators=no", str(unit)],
                env={**os.environ, "SYSTEMD_UNIT_PATH": directory},
                capture_output=True, text=True, timeout=10,
            )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stderr, "", result.stderr)

    def test_reverse_proxy_help_matches_security_overview_and_cors_labels(self):
        canonical = ROOT / "docs/REVERSE_PROXY.md"
        shipped = ROOT / "frontend-modern/public/docs/REVERSE_PROXY.md"
        self.assertEqual(canonical.read_bytes(), shipped.read_bytes())
        text = " ".join(canonical.read_text(encoding="utf-8").split())
        self.assertIn(
            "**Security Posture** card under **Settings → Security → Security Overview** (`/settings/security-overview`) shows **HTTPS** as **HTTP only**",
            text,
        )
        self.assertIn(
            "**Settings → System → Network → CORS Allowed Origins** (`/settings/system-network`)",
            text,
        )
        self.assertNotIn("Settings → System → Network → Allowed Origins", text)
        posture = (ROOT / "frontend-modern/src/utils/securityScorePresentation.ts").read_text(encoding="utf-8")
        self.assertIn("label: 'HTTPS'", posture)
        self.assertIn("'HTTP only'", posture)
        network = (ROOT / "frontend-modern/src/components/Settings/NetworkBoundarySettingsSection.tsx").read_text(encoding="utf-8")
        self.assertIn("CORS Allowed Origins", network)

    def test_reverse_proxy_help_matches_loopback_and_credential_origin_runtime(self):
        canonical = ROOT / "docs/REVERSE_PROXY.md"
        shipped = ROOT / "frontend-modern/public/docs/REVERSE_PROXY.md"
        self.assertEqual(canonical.read_bytes(), shipped.read_bytes())
        text = " ".join(canonical.read_text(encoding="utf-8").split())
        for phrase in (
            "Direct loopback trust requires a loopback peer",
            "no reverse-proxy forwarding headers",
            "`localhost`, `*.localhost` or a loopback IP",
            "same-origin or non-browser request",
            "Authorization: Basic",
            "X-API-Token",
            "Authorization: Bearer",
            "403 `CSRF origin validation failed`",
            "exact listed origin in `ALLOWED_ORIGINS`",
            "Never strip `Origin`, `Referer` or `Sec-Fetch-Site` headers",
            "Non-browser API clients using Basic authentication do not receive browser session cookies",
            "**403 CSRF origin validation failed**:",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)

        loopback = (ROOT / "internal/api/loopback_trust.go").read_text(encoding="utf-8")
        self.assertIn("func isDirectLoopbackRequest(req *http.Request) bool", loopback)
        self.assertIn("isLocalRequestHost(req.Host)", loopback)
        router = (ROOT / "internal/api/router.go").read_text(encoding="utf-8")
        self.assertIn("!originMayUseCredentials && isCrossSiteBasicAuthWrite(req)", router)
        self.assertIn("!originHasCORSGrant && isCrossSiteAPITokenWrite(req)", router)
        basic = (ROOT / "internal/api/basic_auth_session.go").read_text(encoding="utf-8")
        self.assertIn("func issueBasicBrowserSession(", basic)


if __name__ == "__main__":
    unittest.main()
