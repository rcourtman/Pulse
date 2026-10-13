#!/usr/bin/env python3
"""Verify safe existing-view guidance, not polling, routing or native recovery."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "### Current, retained and unavailable disk readings\n"
ANCHOR = "#current-retained-and-unavailable-disk-readings"


def assert_disk_reason_wiring(test, row, presentation):
    # An unavailable caption is optional presentation, not the reading's state
    # or explanation. Bind the actual disk cell (not another metric's N/A) to
    # its reason, and keep the current/retained/unavailable producer distinct.
    disk_cell = row.split("{/* Disk */}", 1)[1].split("{/* IP Address", 1)[0]
    test.assertIn("title={diskReadStatus()?.message}", disk_cell)
    test.assertIn("title={getDiskStatusTooltip()}", disk_cell)
    test.assertIn("{status().message}", disk_cell)
    test.assertIn("state: reason.startsWith('prev-') ? 'last-known' : 'unavailable'", presentation)
    test.assertIn("message: getWorkloadGuestDiskStatusMessage(reason)", presentation)


class GuestDiskReadingDocsTest(unittest.TestCase):
    def section(self):
        guide = (ROOT / "docs/VM_DISK_MONITORING.md").read_text(encoding="utf-8")
        self.assertEqual(guide.count(HEADING), 1)
        return " ".join(guide.split(HEADING, 1)[1].split("\n### ", 1)[0].split())

    def test_explanations_distinguish_reading_states_without_requiring_an_empty_caption(self):
        text = self.section()
        row = (ROOT / "frontend-modern/src/components/Workloads/GuestRow.tsx").read_text()
        drawer = (ROOT / "frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx").read_text()
        presentation = (ROOT / "frontend-modern/src/utils/workloadGuestPresentation.ts").read_text()
        for label in ("Prior", "Last known", "No permission", "Agent error", "Denied", "Error"):
            with self.subTest(label=label):
                self.assertIn(label, text)
                self.assertIn(f"'{label}'", row)
        for empty_state in ("Usage unavailable", "a dash"):
            self.assertIn(empty_state, text)
        assert_disk_reason_wiring(self, row, presentation)
        for distinction in ("select a visible disk reading indicator", "same guest's details",
                            "phone or with a keyboard", "rather than relying on a hover tooltip",
                            "A missing caption does not establish freshness",
                            "same guest's explanation before treating it as an agent setup problem"):
            self.assertIn(distinction, text)
        self.assertIn("Filesystems → Status", text)
        self.assertIn("label: 'Filesystems'", drawer)
        self.assertIn("makeDetailRow('Status'", drawer)
        self.assertIn("Using last known disk stats", text)
        self.assertIn("Using last known disk stats", presentation)
        for distinction in ("not current free space", "unknown, not zero",
                            "not proof that the disk is full", "freshness unknown",
                            "A cooldown ending does not establish a successful new read"):
            self.assertIn(distinction, text)

    def test_reason_contract_accepts_caption_removal_not_reason_removal(self):
        row = (ROOT / "frontend-modern/src/components/Workloads/GuestRow.tsx").read_text()
        presentation = (ROOT / "frontend-modern/src/utils/workloadGuestPresentation.ts").read_text()
        # A future caption-only edit must not require false documentation or
        # pass just because another metric still contains the old label.
        assert_disk_reason_wiring(self, row.replace("'N/A'", "null").replace("'Unavailable'", "null"),
                                 presentation)
        for required in ("title={diskReadStatus()?.message}", "title={getDiskStatusTooltip()}",
                         "{status().message}"):
            with self.subTest(missing=required), self.assertRaises(AssertionError):
                assert_disk_reason_wiring(self, row.replace(required, ""), presentation)
        for required in ("state: reason.startsWith('prev-') ? 'last-known' : 'unavailable'",
                         "message: getWorkloadGuestDiskStatusMessage(reason)"):
            with self.subTest(missing=required), self.assertRaises(AssertionError):
                assert_disk_reason_wiring(self, row, presentation.replace(required, ""))

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
