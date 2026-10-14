#!/usr/bin/env python3
"""Preserve spare-aware RAID help without implying storage recovery.

Browser verification covers navigation and the shipped renderer separately.
These checks guard the documented distinctions and avoid unsafe recipes.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "#### RAID array is degraded with a spare\n"


class RAIDTroubleshootingDocsTest(unittest.TestCase):
    def section(self):
        guide = (ROOT / "docs/TROUBLESHOOTING.md").read_text(encoding="utf-8")
        self.assertEqual(guide.count(HEADING), 1)
        return " ".join(guide.split(HEADING, 1)[1].split("\n#### ", 1)[0].split())

    def test_required_members_are_not_spare_inclusive_totals(self):
        text = self.section()
        for distinction in (
            "same array's **existing** native status",
            "observation time",
            "**Raid Devices** is the required member count",
            "**Total Devices** can include a spare",
            "Active **4**, Total **5**, Working **5**, Spare **1**, Failed **0**",
            "Raid Devices **4**, Active **3**",
            "Do not subtract the spare from the required count",
            "Missing counts, an old reading or different array identities",
        ):
            with self.subTest(distinction=distinction):
                self.assertIn(distinction, text)

    def test_correction_is_not_every_alert_or_installed_recovery(self):
        text = self.section()
        for limit in (
            "Pulse **v6.5.0**",
            "reports from older collectors",
            "does not establish that every RAID alert has this cause",
            "an affected installation has recovered",
            "data integrity, backup completeness or restore readiness",
            "Failed devices, a degraded/unavailable state, or recovery/reshape",
        ):
            with self.subTest(limit=limit):
                self.assertIn(limit, text)

    def test_diagnosis_does_not_require_mutation_or_raw_public_output(self):
        text = self.section()
        for boundary in (
            "independent monitoring",
            "Keep RAID alerts enabled",
            "do not remove a spare",
            "fail a member",
            "assemble/rebuild an array",
            "start a scrub",
            "reboot or change collection settings",
            "No new `mdadm` command, Diagnostics run or induced failure",
            "reported as unknown",
            "consistent private aliases",
            "UUIDs, serial numbers, private paths and full command output",
            "Do not repeat an array operation",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)
        self.assertNotIn("```", text, "existing observations, not a command recipe")

    def test_entrypoint_and_shipped_copy_match(self):
        source = (ROOT / "docs/TROUBLESHOOTING.md").read_bytes()
        self.assertEqual(source, (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes())
        self.assertIn(b"[required members, active members and spares](TROUBLESHOOTING.md#raid-array-is-degraded-with-a-spare)", source)


if __name__ == "__main__":
    unittest.main()
