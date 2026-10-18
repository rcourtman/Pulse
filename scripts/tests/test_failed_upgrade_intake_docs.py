#!/usr/bin/env python3
"""Guard passive failed-upgrade intake, not installer or workload recovery."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]


class FailedUpgradeIntakeDocsTest(unittest.TestCase):
    def guidance(self):
        text = (ROOT / "docs/ISSUE_TRIAGE.md").read_text()
        start = "An interrupted upgrade needs a timeline, not one inferred version."
        self.assertEqual(text.count(start), 1)
        return " ".join(text.split(start, 1)[1].split("\nThe optional", 1)[0].split())

    def test_shipped_triage_guide_matches_source(self):
        self.assertEqual((ROOT / "docs/ISSUE_TRIAGE.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/ISSUE_TRIAGE.md").read_bytes())

    def test_attempt_failure_and_recovery_are_separate_observations(self):
        guide = self.guidance()
        for distinction in (
            "starting version, intended version or asset, route, original time and timezone",
            "last recorded updater or installer result, what stopped",
            "version running at failure separate from the version observed after recovery",
            "accept unknowns and use evidence already supplied anywhere in the thread",
            "Version labels are intake metadata, not proof of the failing executable",
        ):
            with self.subTest(distinction=distinction):
                self.assertIn(distinction, guide)

    def test_signature_and_later_boot_do_not_establish_completion_or_stop_cause(self):
        guide = self.guidance()
        for limit in (
            "artifact authenticity, not completed installation, successful rollback",
            "`Broken pipe` alone does not establish why a service or its host stopped",
            "A later boot marks a new run, not the stop's time or cause",
            "Distinguish Pulse stopping from its whole LXC, VM or host stopping",
        ):
            with self.subTest(limit=limit):
                self.assertIn(limit, guide)

    def test_collection_does_not_replay_incident_or_destroy_recovery_evidence(self):
        guide = self.guidance()
        for boundary in (
            "existing host stop record only when available and consequential",
            "Ask for only the missing distinction, not full journals",
            "another version already supplied",
            "Do not rerun the update, reboot, restore, run Diagnostics or delete rollback backups",
            "keep backups private and preserve the original evidence",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, guide)


if __name__ == "__main__":
    unittest.main()
