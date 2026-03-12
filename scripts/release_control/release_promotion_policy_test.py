#!/usr/bin/env python3
"""Guard the RC-to-GA promotion policy across docs and release workflows."""

from __future__ import annotations

from pathlib import Path
import unittest


REPO_ROOT = Path(__file__).resolve().parents[2]


def read(rel: str) -> str:
    return (REPO_ROOT / rel).read_text(encoding="utf-8")


class ReleasePromotionPolicyTest(unittest.TestCase):
    def test_release_promotion_policy_requires_live_rc_and_v5_policy(self) -> None:
        content = read("docs/release-control/v6/RELEASE_PROMOTION_POLICY.md")
        self.assertIn("Every candidate intended for broad customer use must ship to `rc`", content)
        self.assertIn("live run of the release pipeline for the RC tag itself", content)
        self.assertIn("do not promote to `stable` until the active control-plane target", content)
        self.assertIn("A live release-pipeline exercise already completed for the promoted RC", content)
        self.assertIn("maintenance-only window lasts 90 calendar days", content)
        self.assertIn("critical security issues", content)
        self.assertIn("After the 90-day window ends, v5 may continue running", content)

    def test_pre_release_checklist_tracks_rc_to_ga_gate_inputs(self) -> None:
        content = read("docs/release-control/v6/PRE_RELEASE_CHECKLIST.md")
        self.assertIn("release pipeline has already been exercised on a real RC tag", content)
        self.assertIn("90-day v5 maintenance-only policy", content)
        self.assertIn("rc-to-ga-promotion-readiness", content)

    def test_release_notes_publish_v5_support_transition(self) -> None:
        content = read("docs/releases/RELEASE_NOTES_v6.md")
        self.assertIn("## Pulse v5 Support Transition", content)
        self.assertIn("maintenance-only support immediately", content)
        self.assertIn("90 calendar days from the v6 GA date", content)
        self.assertIn("After the 90-day window ends, v5 may continue running", content)

    def test_release_workflow_enforces_rc_lineage_and_soak(self) -> None:
        content = read(".github/workflows/create-release.yml")
        self.assertIn('REQUIRED_BRANCH="pulse/v6"', content)
        self.assertIn('REQUIRED_BRANCH="main"', content)
        self.assertIn("Stable promotions require promoted_from_tag", content)
        self.assertIn("rollback_version is required for every release", content)
        self.assertIn("minimum is 72 hours unless hotfix_exception is true", content)

    def test_release_artifact_workflows_refuse_stable_without_matching_rc(self) -> None:
        publish = read(".github/workflows/publish-docker.yml")
        promote = read(".github/workflows/promote-floating-tags.yml")
        self.assertIn("does not descend from any matching RC tag", publish)
        self.assertIn("does not descend from any matching RC tag", promote)


if __name__ == "__main__":
    unittest.main()
