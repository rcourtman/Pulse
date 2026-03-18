#!/usr/bin/env python3
"""Guard the RC-to-GA promotion policy across docs and release workflows."""

from __future__ import annotations

import os
import re
import subprocess
import unittest
import json

import record_rc_to_ga_blocked as blocked_record
from release_promotion_policy_support import (
    promotion_metadata_envelope,
    slice_requires_staged_governance_inputs,
    staged_governance_input_errors,
)
from repo_file_io import REPO_ROOT, git_env, read_repo_text

USE_STAGED_GOVERNANCE = os.environ.get("PULSE_READ_STAGED_GOVERNANCE") == "1"


def read(rel: str) -> str:
    return read_repo_text(
        rel,
        staged=USE_STAGED_GOVERNANCE,
        strict_staged=USE_STAGED_GOVERNANCE,
    )


def read_json(rel: str) -> dict:
    return json.loads(read(rel))


def normalize_ws(text: str) -> str:
    return " ".join(text.split())


def staged_files() -> tuple[str, ...]:
    result = subprocess.run(
        ["git", "diff", "--cached", "--name-only"],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=True,
        env=git_env(),
    )
    return tuple(line for line in result.stdout.splitlines() if line.strip())


STAGED_FILES = staged_files() if USE_STAGED_GOVERNANCE else ()
REQUIRES_STAGED_GOVERNANCE_INPUTS = slice_requires_staged_governance_inputs(STAGED_FILES)
STAGED_GOVERNANCE_INPUT_ERRORS = (
    tuple(staged_governance_input_errors(use_staged_governance=True))
    if REQUIRES_STAGED_GOVERNANCE_INPUTS
    else ()
)


class ReleasePromotionPolicyTest(unittest.TestCase):
    def setUp(self) -> None:
        if USE_STAGED_GOVERNANCE and not REQUIRES_STAGED_GOVERNANCE_INPUTS:
            self.skipTest("staged slice does not touch the promotion-proof surface")
        if (
            STAGED_GOVERNANCE_INPUT_ERRORS
            and self._testMethodName != "test_staged_governance_inputs_are_present"
        ):
            self.skipTest("staged governance inputs missing; see test_staged_governance_inputs_are_present")

    def test_staged_governance_inputs_are_present(self) -> None:
        if STAGED_GOVERNANCE_INPUT_ERRORS:
            self.fail(
                "staged promotion proof inputs are incomplete:\n- "
                + "\n- ".join(STAGED_GOVERNANCE_INPUT_ERRORS)
            )

    def test_release_promotion_policy_requires_live_rc_and_v5_policy(self) -> None:
        content = read("docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md")
        self.assertIn("Every candidate intended for broad customer use must ship to `rc`", content)
        self.assertIn("live run of the release pipeline for the RC tag itself", content)
        self.assertIn("an accidental git tag by itself", content)
        self.assertIn("does not count as a shipped RC", content)
        self.assertIn("do not promote to `stable` until the active control-plane target", content)
        self.assertIn("A live release-pipeline exercise already completed for the promoted RC", content)
        self.assertIn("maintenance-only window lasts 90 calendar days", content)
        self.assertIn("V5_MAINTENANCE_SUPPORT_POLICY.md", content)
        self.assertIn("release notes may keep placeholder", content)
        self.assertIn("Exact v6 GA and v5 end-of-support dates locked before GA publish", content)
        self.assertIn("governed prerelease and stable release branches", content)

    def test_pre_release_checklist_tracks_rc_to_ga_gate_inputs(self) -> None:
        content = read("docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md")
        self.assertIn("release pipeline has already been exercised on a real RC tag", content)
        self.assertIn("not an accidental git tag", content)
        self.assertIn("V5_MAINTENANCE_SUPPORT_POLICY.md", content)
        self.assertIn("replace any placeholder GA notice dates", content)
        self.assertIn("rc-to-ga-rehearsal-summary", content)
        self.assertIn("rc-to-ga-promotion-readiness", content)
        self.assertIn(promotion_metadata_envelope(), normalize_ws(content))

    def test_v5_support_policy_and_release_notes_publish_exact_notice(self) -> None:
        policy = read("docs/release-control/v6/internal/V5_MAINTENANCE_SUPPORT_POLICY.md")
        release_notes = read("docs/releases/RELEASE_NOTES_v6.md")
        self.assertIn("maintenance-only support immediately on the v6 GA date", policy)
        self.assertIn("90 calendar days from the v6 GA", policy)
        self.assertIn("pulse/v5-maintenance", policy)
        self.assertIn("Pulse v5 Support Transition", release_notes)
        self.assertIn("publish an explicit exception", release_notes)
        self.assertRegex(
            release_notes,
            re.compile(r"Pulse v5 entered maintenance-only support on `(?:\[v6-ga-date\]|\d{4}-\d{2}-\d{2})`\.")
        )
        self.assertRegex(
            release_notes,
            re.compile(r"existing v5 users until `(?:\[v5-eos-date\]|\d{4}-\d{2}-\d{2})`\.")
        )

    def test_rehearsal_template_and_workflow_capture_ga_rehearsal_record(self) -> None:
        template = read("docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md")
        workflow = read(".github/workflows/release-dry-run.yml")
        release_workflow = read(".github/workflows/create-release.yml")
        dry_run_trigger = read("scripts/trigger-release-dry-run.sh")
        resolver = read("scripts/release_control/resolve_release_promotion.py")
        self.assertIn("GitHub Actions run URL", template)
        self.assertIn("Exact GA date to publish with GA", template)
        self.assertIn(promotion_metadata_envelope(), normalize_ws(template))
        self.assertIn("rc-to-ga-rehearsal-summary", workflow)
        self.assertIn("control_plane.py --branch-for-version", workflow)
        self.assertIn("resolve_release_promotion.py", workflow)
        self.assertIn("- Rollback command:", workflow)
        self.assertIn("- Candidate stable tag:", workflow)
        self.assertIn("- Promotion channel:", workflow)
        self.assertIn("- Promoted RC tag:", workflow)
        self.assertIn("Planned GA date", workflow)
        self.assertIn("Planned v5 end-of-support date", workflow)
        self.assertIn("resolve_release_promotion.py", release_workflow)
        self.assertIn("- Rollback command:", release_workflow)
        self.assertIn("rollback_version is required for every release rehearsal and promotion", resolver)
        self.assertIn("Stable promotion requires promoted_from_tag", resolver)
        self.assertIn("Stable v6.0.0 requires ga_date in YYYY-MM-DD form", resolver)
        self.assertIn("release_notes must include the exact ga_date", resolver)
        self.assertIn("check-workflow-dispatch-inputs.py", dry_run_trigger)
        self.assertIn("release-dry-run.yml", dry_run_trigger)
        self.assertIn("gh workflow run release-dry-run.yml", dry_run_trigger)

    def test_release_workflow_enforces_rc_lineage_soak_and_v5_notice(self) -> None:
        content = read(".github/workflows/create-release.yml")
        helper = read("scripts/trigger-release.sh")
        policy = read("docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md")
        source_of_truth = read("docs/release-control/v6/internal/SOURCE_OF_TRUTH.md")
        resolver = read("scripts/release_control/resolve_release_promotion.py")
        self.assertIn("control_plane.py --branch-for-version", content)
        self.assertIn("resolve_release_promotion.py", content)
        self.assertIn("Rollback command:", content)
        self.assertIn("rollback target and exact reinstall command recorded", policy)
        self.assertIn("rc-to-ga-rehearsal-summary", policy)
        self.assertIn(promotion_metadata_envelope(), normalize_ws(policy))
        self.assertIn("recorded rollback target plus exact", source_of_truth)
        self.assertIn("minimum is 72 hours unless hotfix_exception is true", resolver)
        self.assertIn("- Candidate stable tag:", content)
        self.assertIn("- Promotion channel:", content)
        self.assertIn("- Promoted RC tag:", content)
        self.assertIn("ga_date", content)
        self.assertIn("v5_eos_date", content)
        self.assertIn("Derived rollback command:", helper)
        self.assertIn("./scripts/install.sh --version", helper)
        self.assertIn("v6 GA date to publish with GA", helper)
        self.assertIn("-f ga_date", helper)
        self.assertIn("Planned GA date", content)
        self.assertIn("Planned v5 end-of-support date", content)
        self.assertIn("Stable v6.0.0 requires v5_eos_date in YYYY-MM-DD form", resolver)
        self.assertIn("release_notes must include the Pulse v5 maintenance-only support notice", resolver)
        self.assertIn("check-workflow-dispatch-inputs.py", helper)
        self.assertIn(
            "default branch copy of `.github/workflows/release-dry-run.yml`",
            policy,
        )
        self.assertIn(
            "GitHub validates dispatch inputs against the default branch",
            normalize_ws(policy),
        )
        checklist = read("docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md")
        self.assertIn("default-branch copy of `.github/workflows/release-dry-run.yml`", checklist)
        self.assertIn("workflow_dispatch", checklist)

    def test_release_artifact_workflows_refuse_stable_without_matching_rc(self) -> None:
        publish = read(".github/workflows/publish-docker.yml")
        promote = read(".github/workflows/promote-floating-tags.yml")
        demo = read(".github/workflows/update-demo-server.yml")
        helm = read(".github/workflows/publish-helm-chart.yml")
        runbook = read("docs/releases/V6_PRERELEASE_RUNBOOK.md")
        self.assertIn("control_plane.py --branch-for-version", publish)
        self.assertIn("control_plane.py --branch-for-version", promote)
        self.assertIn("control_plane.py --branch-for-version", demo)
        self.assertIn("control_plane.py --branch-for-version", helm)
        self.assertIn("does not descend from any matching RC tag", publish)
        self.assertIn("does not descend from any matching RC tag", promote)
        self.assertIn("both stable and prerelease releases dispatch", runbook)
        self.assertIn("Release `6.0.0` from `pulse/v6`", runbook)
        self.assertIn(promotion_metadata_envelope(), normalize_ws(runbook))

    def test_blocked_record_tracks_current_target_and_candidate_version(self) -> None:
        blocked = read("docs/release-control/v6/internal/records/rc-to-ga-promotion-readiness-blocked-2026-03-13.md")
        self.assertIn("exact derived rollback command", blocked)
        self.assertIn("rollback target and exact derived", blocked)
        self.assertIn("artifact-owned candidate stable tag", blocked)
        self.assertIn("artifact-owned promotion channel", blocked)
        self.assertIn("artifact-owned promoted RC tag", blocked)
        self.assertIn("artifact-owned rollback target", blocked)
        self.assertIn("Materialize the final rehearsal record from that artifact without", blocked)
        self.assertIn("hand-repairing any missing candidate tag, promoted RC tag, rollback", blocked)
        matrix = read("docs/release-control/v6/internal/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md")
        self.assertIn(promotion_metadata_envelope(), normalize_ws(matrix))
        expected = blocked_record.build_blocked_record(record_date="2026-03-13")
        self.assertEqual(blocked, expected)


if __name__ == "__main__":
    unittest.main()
