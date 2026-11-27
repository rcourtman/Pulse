#!/usr/bin/env python3
"""Bind Proxmox node, vSphere datastore, Availability nav, and agent opt-out Help to live source.

Documentation consistency checks only, not live Proxmox, vSphere, or agent runtime tests.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]


def doc_text(rel: str) -> str:
    return (ROOT / rel).read_text(encoding="utf-8")


def norm(text: str) -> str:
    return " ".join(text.split())


class StaleReadingAndAgentHelpDocsTest(unittest.TestCase):
    def test_faq_documents_top_level_availability_navigation(self):
        faq = doc_text("docs/FAQ.md")
        section = norm(
            faq.split("### How is navigation organised in Pulse v6?\n", 1)[1].split(
                "\n### ", 1
            )[0]
        )
        for phrase in (
            "**Availability** (`/availability`)",
            "website, service, device, and TLS certificate checks",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, section)
        self.assertNotIn("Push checks", section)

        layout = doc_text("frontend-modern/src/AppLayout.tsx")
        for marker in (
            "id: 'availability'",
            "label: 'Availability'",
            "route: ROOT_AVAILABILITY_PATH",
            "tooltip: 'Website, service, device, and TLS certificate checks'",
        ):
            self.assertIn(marker, layout)

    def test_faq_documents_proxmox_node_reading_status_badges(self):
        faq = doc_text("docs/FAQ.md")
        section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**Proxmox nodes**",
            "beside the node name on both desktop and phones",
            "withholding live metrics and uptime when readings are not current",
            "**Offline** (a failed Proxmox read does not prove the host is powered off)",
            "**Stale · API error**",
            "**Stale · Degraded**",
            "**Stale** (Pulse has no recent Proxmox readings)",
            "**Degraded** (a node warning with current Proxmox readings)",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, section)

        table = doc_text("frontend-modern/src/features/proxmox/ProxmoxNodesTable.tsx")
        for marker in (
            "Offline in Pulse. A failed Proxmox read does not prove the host is powered off.",
            "label: 'Stale · API error'",
            "label: 'Stale · Degraded'",
            "Pulse has no recent Proxmox readings.",
            "label: 'Degraded'",
            "const showReadings = () => readingState().variant === 'success';",
        ):
            self.assertIn(marker, table)

    def test_faq_documents_vsphere_datastore_stale_readings_and_unknown_capacity(self):
        faq = doc_text("docs/FAQ.md")
        section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**vSphere datastores**",
            "**Last reported inaccessible**",
            "**Stale data** beside the name",
            "`vCenter has not updated this datastore recently.`",
            "Missing or unreadable capacity displays as a dash (`—`), not `0%`",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, section)

        table = doc_text("frontend-modern/src/features/vmware/VsphereDatastoresTable.tsx")
        model = doc_text("frontend-modern/src/features/vmware/vmwarePageModel.ts")
        history = doc_text(
            "frontend-modern/src/components/Infrastructure/resourceDetailDrawerMetricsHistoryModel.ts"
        )
        self.assertIn("Stale data", table)
        self.assertIn("PlatformTableMetricFallback", table)
        self.assertIn("'Last reported inaccessible'", model)
        self.assertIn(
            "`vCenter has not updated this ${isHost ? 'host' : 'datastore'} recently.`",
            history,
        )

    def test_unified_agent_aligns_external_probes_nav_and_proxmox_opt_out_preservation(
        self,
    ):
        agent_doc = doc_text("docs/UNIFIED_AGENT.md")
        probes = norm(
            agent_doc.split("## External Probes (Pro)\n", 1)[1].split("\n## ", 1)[0]
        )
        for phrase in (
            "Open **Availability** (`/availability`",
            "**Observation locations**",
            "**This Pulse server**",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, probes)

        auto_update = norm(
            agent_doc.split("## Auto-Update\n", 1)[1].split("\n## ", 1)[0]
        )
        for phrase in (
            "Current installers preserve the selected agent's installed Proxmox setup choice during `--update`",
            "`--enable-proxmox=false`",
            "Older installers (including v6.8.0 and v6.9.0)",
            "passing `--disable-proxmox` alongside `--update` remains safe across versions",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, auto_update)

        installer = doc_text("scripts/install.sh")
        for marker in (
            "EXEC_ARG_ITEMS+=(--enable-proxmox=false)",
            "recover_installed_proxmox_setup()",
            'ENABLE_PROXMOX="${ENABLE_PROXMOX:-false}"',
        ):
            self.assertIn(marker, installer)

    def test_shipped_public_docs_mirrors_match_byte_for_byte(self):
        for name in ("FAQ.md", "UNIFIED_AGENT.md"):
            with self.subTest(doc=name):
                self.assertEqual(
                    (ROOT / "docs" / name).read_bytes(),
                    (ROOT / "frontend-modern/public/docs" / name).read_bytes(),
                )


if __name__ == "__main__":
    unittest.main()
