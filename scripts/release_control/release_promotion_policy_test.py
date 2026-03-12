#!/usr/bin/env python3
"""Guard the RC-to-GA promotion policy across docs and release workflows."""

from __future__ import annotations

import os
import unittest

from repo_file_io import read_repo_text

USE_STAGED_GOVERNANCE = os.environ.get("PULSE_READ_STAGED_GOVERNANCE") == "1"


def read(rel: str) -> str:
    return read_repo_text(rel, staged=USE_STAGED_GOVERNANCE)


class ReleasePromotionPolicyTest(unittest.TestCase):
    def test_release_promotion_policy_requires_live_rc_and_v5_policy(self) -> None:
        content = read("docs/release-control/v6/RELEASE_PROMOTION_POLICY.md")
        self.assertIn("Every candidate intended for broad customer use must ship to `rc`", content)
        self.assertIn("live run of the release pipeline for the RC tag itself", content)
        self.assertIn("do not promote to `stable` until the active control-plane target", content)
        self.assertIn("A live release-pipeline exercise already completed for the promoted RC", content)
        self.assertIn("maintenance-only window lasts 90 calendar days", content)
        self.assertIn("V5_MAINTENANCE_SUPPORT_POLICY.md", content)
        self.assertIn("Exact v6 GA and v5 end-of-support dates", content)

    def test_pre_release_checklist_tracks_rc_to_ga_gate_inputs(self) -> None:
        content = read("docs/release-control/v6/PRE_RELEASE_CHECKLIST.md")
        self.assertIn("release pipeline has already been exercised on a real RC tag", content)
        self.assertIn("V5_MAINTENANCE_SUPPORT_POLICY.md", content)
        self.assertIn("exact GA/EOS dates", content)
        self.assertIn("rc-to-ga-promotion-readiness", content)

    def test_v5_support_policy_and_release_notes_publish_exact_notice(self) -> None:
        policy = read("docs/release-control/v6/V5_MAINTENANCE_SUPPORT_POLICY.md")
        release_notes = read("docs/releases/RELEASE_NOTES_v6.md")
        self.assertIn("maintenance-only support immediately on the v6 GA date", policy)
        self.assertIn("90 calendar days from the v6 GA", policy)
        self.assertIn("pulse/v5-maintenance", policy)
        self.assertIn("[v5-eos-date]", release_notes)
        self.assertIn("Pulse v5 Support Transition", release_notes)
        self.assertIn("publish an explicit exception", release_notes)

    def test_release_workflow_enforces_rc_lineage_soak_and_v5_notice(self) -> None:
        content = read(".github/workflows/create-release.yml")
        self.assertIn('REQUIRED_BRANCH="pulse/v6"', content)
        self.assertIn('REQUIRED_BRANCH="main"', content)
        self.assertIn("Stable promotions require promoted_from_tag", content)
        self.assertIn("rollback_version is required for every release", content)
        self.assertIn("minimum is 72 hours unless hotfix_exception is true", content)
        self.assertIn("v5_eos_date", content)
        self.assertIn("Stable v6.0.0 requires v5_eos_date in YYYY-MM-DD form", content)
        self.assertIn("release_notes must include the Pulse v5 maintenance-only support notice", content)

    def test_release_artifact_workflows_refuse_stable_without_matching_rc(self) -> None:
        publish = read(".github/workflows/publish-docker.yml")
        promote = read(".github/workflows/promote-floating-tags.yml")
        self.assertIn("does not descend from any matching RC tag", publish)
        self.assertIn("does not descend from any matching RC tag", promote)


if __name__ == "__main__":
    unittest.main()
