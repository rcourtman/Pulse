#!/usr/bin/env python3
"""Keep guest-address help on existing views, without changing guest networking.

These are documentation/source checks, not a routing test or collector fix.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "#### Primary IP shows a Podman or Docker bridge address\n"


class GuestNetworkTroubleshootingDocsTest(unittest.TestCase):
    def section(self):
        guide = (ROOT / "docs/TROUBLESHOOTING.md").read_text(encoding="utf-8")
        self.assertEqual(guide.count(HEADING), 1)
        return " ".join(guide.split(HEADING, 1)[1].split("\n#### ", 1)[0].split())

    def test_selected_address_is_not_default_route_or_collection_recovery(self):
        text = self.section()
        for distinction in (
            "not proof of the guest's default route or reachability",
            "agentless LXC",
            "podman0",
            "Proxmox API and Pulse agent use different collection paths",
            "does not establish that the API-only view is correct",
            "existing Proxmox network configuration",
            "These views do not test routing or connectivity",
        ):
            with self.subTest(distinction=distinction):
                self.assertIn(distinction, text)

    def test_workaround_labels_and_interface_association_exist_in_product(self):
        text = self.section()
        row = (ROOT / "frontend-modern/src/components/Workloads/GuestRowCells.tsx").read_text()
        drawer = (ROOT / "frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx").read_text()
        for label in ("Workloads", "Network Interfaces", "Other IPs"):
            self.assertIn(label, text)
        self.assertIn("Network Interfaces", row)
        self.assertIn("props.networkInterfaces", row)
        self.assertIn("iface.name", row)
        self.assertIn("iface.addresses", row)
        self.assertIn("'Other IPs', props.ipAddresses.slice(1)", drawer)
        self.assertIn("when supplied", text)
        self.assertIn("keep that association unknown", text)

    def test_no_new_probe_network_change_or_public_identity_dump(self):
        text = self.section()
        for boundary in (
            "Do not remove a working bridge",
            "renumber the guest",
            "reinstall or add an agent",
            "recreate the monitored connection",
            "run a guest-agent probe",
            "relationship already observed",
            "Keep actual addresses, MACs and hostnames private",
            "consistent aliases",
            "a raw network or configuration dump is not needed",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)
        self.assertNotIn("```", text, "use existing observations, not a copied command")

    def test_shipped_help_is_byte_identical(self):
        self.assertEqual(
            (ROOT / "docs/TROUBLESHOOTING.md").read_bytes(),
            (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes(),
        )


if __name__ == "__main__":
    unittest.main()
