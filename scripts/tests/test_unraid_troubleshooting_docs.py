#!/usr/bin/env python3
"""Keep Unraid help distinct from MD spares and unsafe array-repair advice.

These checks guard the instructions and shipped mirror, not an installed array's
recovery. Source-bound collector/assessment fixtures and browser proof are retained
separately when the guidance changes.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "#### Unraid reports missing disks in empty slots\n"
ANCHOR = "TROUBLESHOOTING.md#unraid-reports-missing-disks-in-empty-slots"


def section(text):
    return " ".join(text.split(HEADING, 1)[1].split("\n#### ", 1)[0].split())


class UnraidTroubleshootingDocsTest(unittest.TestCase):
    def setUp(self):
        self.source = (ROOT / "docs/TROUBLESHOOTING.md").read_text()
        self.guide = section(self.source)

    def test_existing_symptom_entry_reaches_the_distinct_unraid_workflow(self):
        self.assertIn(ANCHOR, self.source.split("### Monitoring Data", 1)[0])
        self.assertIn("Unraid parity is not Linux MD RAID", self.source)
        self.assertIn("do not rebuild or change disk assignments", self.source)

    def test_empty_slot_evidence_does_not_hide_assigned_missing_members(self):
        for phrase in (
            "`DISK_NP`, no device or size", "`fsType=auto`",
            "transport-only identity such as `ata-_`", "not assignment evidence",
            "`DISK_NP_MISSING`, even without a device or identity",
            "assigned-but-absent member", "Do not discard it as an empty slot",
            "real device, model/serial, filesystem or size",
            "Assignment evidence remains", "`auto` alone is not a real filesystem",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, self.guide)

    def test_parity_and_unknowns_are_not_misrepresented_as_recovery(self):
        for phrase in (
            "Single online parity disk and five data disks",
            "`numProtected` counts online parity devices: **1**",
            "does not prove complete protection", "not a healthy result",
            "`STARTED` alone does not establish array health",
            "Pulse server and Unraid Unified Agent versions separate",
            "updating the server does not establish that the agent updated",
            "agent collection, server ingestion and storage-health assessment",
            "an earlier preview retest failed", "installation has recovered",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, self.guide)

    def test_no_replay_array_change_or_private_payload_is_requested(self):
        self.assertNotIn("```", self.source.split(HEADING, 1)[1].split("\n#### ", 1)[0])
        for phrase in (
            "existing", "No new command, Diagnostics run or array operation",
            "rather than suppressing the check or changing the array",
            "Do not unassign a disk, rebuild, start a parity check",
            "restart or re-enrol the agent", "enable Debug just to diagnose",
            "genuine missing, disabled, invalid or unavailable parity member",
            "already available versions", "original time", "redacted alert text",
            "Keep disk serials, paths and raw agent payloads private",
            "consistent aliases", "leave unknowns unknown", "without repeating a failed",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, self.guide)

    def test_shipped_help_matches_the_source(self):
        self.assertEqual((ROOT / "docs/TROUBLESHOOTING.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes())


if __name__ == "__main__":
    unittest.main()
