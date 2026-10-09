#!/usr/bin/env python3
"""Verify safe existing-view guidance, not polling, routing or native recovery."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "### Current, retained and unavailable disk readings\n"
ANCHOR = "#current-retained-and-unavailable-disk-readings"


class GuestDiskReadingDocsTest(unittest.TestCase):
    def section(self):
        guide = (ROOT / "docs/VM_DISK_MONITORING.md").read_text(encoding="utf-8")
        self.assertEqual(guide.count(HEADING), 1)
        return " ".join(guide.split(HEADING, 1)[1].split("\n### ", 1)[0].split())

    def test_explanations_use_existing_labels_without_claiming_freshness(self):
        text = self.section()
        row = (ROOT / "frontend-modern/src/components/Workloads/GuestRow.tsx").read_text()
        drawer = (ROOT / "frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx").read_text()
        presentation = (ROOT / "frontend-modern/src/utils/workloadGuestPresentation.ts").read_text()
        for label in ("Prior", "N/A", "Last known", "Unavailable"):
            with self.subTest(label=label):
                self.assertIn(label, text)
                self.assertIn(f"'{label}'", row)
        self.assertIn("Filesystems → Status", text)
        self.assertIn("label: 'Filesystems'", drawer)
        self.assertIn("makeDetailRow('Status'", drawer)
        self.assertIn("Using last known disk stats", text)
        self.assertIn("Using last known disk stats", presentation)
        for distinction in ("not current free space", "unknown, not zero",
                            "not proof that the disk is full", "freshness unknown",
                            "A cooldown ending does not establish a successful new read"):
            self.assertIn(distinction, text)

    def test_inventory_workflow_does_not_confuse_selection_with_coverage(self):
        text = self.section()
        for distinction in ("not a monitoring-coverage check", "clear numeric disk conditions",
                            "existing name search or unfiltered inventory",
                            "same guest's explanation and filesystem rows",
                            "not evidence of low usage, deletion or healthy monitoring",
                            "neither changes access nor stops API polling",
                            "keep intentionally restricted targets restricted"):
            self.assertIn(distinction, text)

    def test_source_and_recovery_boundaries_remain_distinct(self):
        text = self.section()
        for distinction in ("existing linked Pulse Agent", "stale agent",
                            "same guest identity and filesystem",
                            "does not prove that the QEMU-only path recovered",
                            "without installing another agent", "normally collected observations",
                            "Do not run Diagnostics, guest-agent probes", "backup or a restart",
                            "Do not bypass a guest-read pause or widen token access",
                            "None of these display states proves thaw", "Backup safety"):
            self.assertIn(distinction, text)
        self.assertNotIn("```", text, "use existing observations, not an active command")

    def test_existing_entry_points_reach_the_guidance_and_mirrors_match(self):
        guide = (ROOT / "docs/VM_DISK_MONITORING.md").read_text()
        diagnosis = guide.split("### A missing reading is not an installation diagnosis\n", 1)[1]
        self.assertIn(f"[Check the reading's context]({ANCHOR})", diagnosis)
        trouble = (ROOT / "docs/TROUBLESHOOTING.md").read_text()
        dash_help = trouble.split('#### VMs show "-" for disk usage\n', 1)[1].split("\n#### ", 1)[0]
        self.assertIn(f"VM_DISK_MONITORING.md{ANCHOR}", dash_help)
        self.assertIn("A filtered list or disk sort is not proof of complete monitoring", dash_help)
        for name in ("VM_DISK_MONITORING.md", "TROUBLESHOOTING.md"):
            self.assertEqual((ROOT / "docs" / name).read_bytes(),
                             (ROOT / "frontend-modern/public/docs" / name).read_bytes())


if __name__ == "__main__":
    unittest.main()
