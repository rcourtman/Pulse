#!/usr/bin/env python3
"""Keep provider connection help within the chosen network and access boundary.

Source/document consistency only: no setup script, provider call, ACL or guest
operation is executed. The normal documentation shards discover this suite.
"""

from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
HEADING = "## Connecting a client's Proxmox or PBS over your VPN\n"


def connection_help():
    text = (ROOT / "docs/MSP.md").read_text(encoding="utf-8")
    return text.split(HEADING, 1)[1].split("\n## ", 1)[0]


class MSPConnectionDocsTest(unittest.TestCase):
    def test_host_connectivity_is_not_runtime_or_collection_acceptance(self):
        text = " ".join(connection_help().split())
        for distinction in (
            "client workspace's Pulse runtime", "only a preliminary check",
            "different routes, DNS and certificate trust",
            "neither authenticated collection nor access to every cluster member",
            "do not expose it publicly", "ordinary polling for each intended member",
            "primary is not proof that all members or optional checks work",
            "unavailable readings distinct from measured zero",
        ):
            with self.subTest(distinction=distinction):
                self.assertIn(distinction, text)

    def test_first_seen_pin_is_not_independent_server_authentication(self):
        text = " ".join(connection_help().split())
        for boundary in (
            "independently trusted client console", "**runtime** needs",
            "matching endpoint hostname", "independently verified SHA-256",
            "first-seen pinning does not independently authenticate the server",
            "solely from the failing connection", "copy one member's pin",
            "disable verification",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)
        self.assertNotIn("Self-signed certificates are handled automatically", text)
        source = (ROOT / "internal/api/configapi/config_setup_handlers.go").read_text()
        selection = source.split("func selectAutoRegisterHost(", 1)[1].split("\nfunc ", 1)[0]
        self.assertIn("fetchTLSFingerprint(candidate)", selection)
        self.assertIn("return candidate, fingerprint, true, candidates, nil", selection)

    def test_generated_root_acls_and_optional_reads_are_not_required_baseline(self):
        text = " ".join(connection_help().split())
        for boundary in (
            "`PVEAuditor` provides baseline inventory and metrics",
            "intersection of the user and token permissions",
            "Keep privilege separation enabled", "do not modify the built-in role",
            "administrative change, not a read-only test", "ACLs at `/`",
            "`VM.GuestAgent.Audit`", "`VM.GuestAgent.FileRead`", "`VM.Monitor`",
            "existing deliberately scoped token can be entered directly",
            "never a command line or public report", "Do not rerun setup",
            "remain unavailable without granting more access",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)
        self.assertNotIn("`PVEAuditor` role is not sufficient on its own", text)
        source = (ROOT / "internal/api/configapi/setup_script_render.go").read_text()
        for marker in (
            "pveum aclmod / -user pulse-monitor@pve -role PVEAuditor",
            'pveum aclmod / -token "$PULSE_TOKEN_ID" -role PulseMonitor',
            'EXTRA_PRIVS+=("VM.GuestAgent.Audit")',
            'EXTRA_PRIVS+=("VM.GuestAgent.FileRead")',
            'EXTRA_PRIVS+=("VM.Monitor")',
        ):
            self.assertIn(marker, source)

    def test_existing_help_targets_safety_and_shipped_copy_remain_usable(self):
        section = connection_help()
        text = " ".join(section.split())
        self.assertIn("defer setup, live tests and guest-agent probes", text)
        self.assertIn("this connection workflow is not guest recovery", text)
        self.assertNotIn("```", section, "reuse existing guidance, not a new secret-bearing command")
        links = re.findall(r"\]\(([A-Z_]+\.md)#([a-z0-9-]+)\)", section)
        self.assertGreaterEqual(len(links), 4)
        for name, anchor in links:
            target = (ROOT / "docs" / name).read_text(encoding="utf-8")
            headings = re.findall(r"^#+ (.+)$", target, re.MULTILINE)
            anchors = [re.sub(r"[^\w -]", "", h).strip().lower().replace(" ", "-") for h in headings]
            with self.subTest(link=f"{name}#{anchor}"):
                self.assertIn(anchor, anchors)
        self.assertEqual(
            (ROOT / "docs/MSP.md").read_bytes(),
            (ROOT / "frontend-modern/public/docs/MSP.md").read_bytes(),
        )


if __name__ == "__main__":
    unittest.main()
