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
    REQUIRED_STAGED_GOVERNANCE_INPUTS,
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
        strict_staged=USE_STAGED_GOVERNANCE and rel in REQUIRED_STAGED_GOVERNANCE_INPUTS,
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
        self.assertIn("live run of the release pipeline for the prerelease tag itself", content)
        self.assertIn("an accidental git tag by itself", content)
        self.assertIn("does not count as a shipped prerelease", content)
        self.assertIn("do not promote to `stable` until the active control-plane target", content)
        self.assertIn("A live release-pipeline exercise already completed for the promoted prerelease tag", content)
        self.assertIn("maintenance-only window lasts 90 calendar days", content)
        self.assertIn("V5_MAINTENANCE_SUPPORT_POLICY.md", content)
        self.assertIn("release notes may keep placeholder", content)
        self.assertIn("Exact v6 GA and v5 end-of-support dates locked before GA publish", content)
        self.assertIn("governed prerelease and stable release branches", content)

    def test_pre_release_checklist_tracks_rc_to_ga_gate_inputs(self) -> None:
        content = read("docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md")
        self.assertIn("release pipeline has already been exercised on a real prerelease tag", content)
        self.assertIn("not an accidental git tag", content)
        self.assertIn("V5_MAINTENANCE_SUPPORT_POLICY.md", content)
        self.assertIn("replace any placeholder GA notice dates", content)
        self.assertIn("rc-to-ga-rehearsal-summary", content)
        self.assertIn("rc-to-ga-promotion-readiness", content)
        self.assertIn("record_rc_to_ga_rehearsal.py --run-id <run-id>", content)
        self.assertIn("rc-to-ga-promotion-readiness-rehearsal-<record-date>.md", content)
        self.assertIn(promotion_metadata_envelope(), normalize_ws(content))

    def test_v5_support_policy_and_release_notes_publish_exact_notice(self) -> None:
        policy = read("docs/release-control/v6/internal/V5_MAINTENANCE_SUPPORT_POLICY.md")
        release_notes = read("docs/releases/RELEASE_NOTES_v6.md")
        self.assertIn("maintenance-only support immediately on the v6 GA date", policy)
        self.assertIn("90 calendar days from the v6 GA", policy)
        self.assertIn("pulse/v5-maintenance", policy)
        if "Pulse v5 Support Transition" in release_notes:
            self.assertIn("publish an explicit exception", release_notes)
            self.assertRegex(
                release_notes,
                re.compile(r"Pulse v5 entered maintenance-only support on `(?:\[v6-ga-date\]|\d{4}-\d{2}-\d{2})`\.")
            )
            self.assertRegex(
                release_notes,
                re.compile(r"existing v5 users until `(?:\[v5-eos-date\]|\d{4}-\d{2}-\d{2})`\.")
            )
        else:
            self.assertRegex(release_notes, re.compile(r"(Pre-Release Notes|Release Candidate Notes)"))
            self.assertRegex(release_notes, re.compile(r"(final GA release|stable `v6\.0\.0` release)"))
            self.assertNotIn("Pulse v5 Support Transition", release_notes)

    def test_release_notes_index_points_at_current_rc_packet(self) -> None:
        release_index = read("docs/RELEASE_NOTES.md")
        self.assertIn("docs/releases/RELEASE_NOTES_v6.md", release_index)
        self.assertIn("docs/releases/V6_CHANGELOG.md", release_index)
        self.assertIn("docs/UPGRADE_v6.md", release_index)
        self.assertIn("docs/releases/RELEASE_NOTES_v6_RC1.md", release_index)
        self.assertIn("docs/releases/V6_CHANGELOG_RC1.md", release_index)
        self.assertIn("docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md", release_index)
        self.assertIn("docs/releases/RELEASE_NOTES_v6_RC2_DRAFT.md", release_index)
        self.assertIn("docs/releases/V6_CHANGELOG_RC2_DRAFT.md", release_index)
        self.assertIn("docs/releases/V6_RC2_OPERATOR_SUPPORT_PACK_DRAFT.md", release_index)

    def test_operator_support_packs_keep_free_first_paid_continuity_wording(self) -> None:
        for rel in (
            "docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md",
            "docs/releases/V6_RC2_OPERATOR_SUPPORT_PACK_DRAFT.md",
        ):
            with self.subTest(rel=rel):
                support_pack = read(rel)
                self.assertIn(
                    "keep the current recurring price, with self-hosted monitoring and",
                    support_pack,
                )
                self.assertIn("child-resource volume not metered", support_pack)
                self.assertIn("core monitoring included", support_pack)
                self.assertNotIn("uncapped monitored-system plus guest", support_pack)
                self.assertNotIn("uncapped monitored-system and guest capacity", support_pack)
                self.assertNotIn("guest-capacity continuity", support_pack)
                self.assertNotIn("core monitoring unlimited", normalize_ws(support_pack))

    def test_stable_release_packet_keeps_infrastructure_as_current_landing_surface(self) -> None:
        stable_docs = {
            "release_notes": read("docs/releases/RELEASE_NOTES_v6.md"),
            "changelog": read("docs/releases/V6_CHANGELOG.md"),
            "operator_support_pack": read("docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md"),
        }

        for name, content in stable_docs.items():
            with self.subTest(name=name):
                normalized = normalize_ws(content)
                self.assertIn("Infrastructure", content)
                self.assertNotIn("default route lands on `Dashboard`", normalized)
                self.assertNotIn("around Dashboard, Infrastructure", normalized)
                self.assertNotIn(
                    "`Dashboard`, `Infrastructure`, `Workloads`, `Storage`, and `Recovery`",
                    content,
                )
                self.assertNotIn("- `Dashboard`", content)

        self.assertIn(
            "Authenticated users now land on `Infrastructure`",
            normalize_ws(stable_docs["release_notes"]),
        )
        self.assertIn("default route lands on `Infrastructure`", normalize_ws(stable_docs["changelog"]))
        self.assertIn(
            "Infrastructure as the default landing surface",
            stable_docs["operator_support_pack"],
        )

    def test_rc1_changelog_keeps_current_free_first_licensing_posture(self) -> None:
        changelog = read("docs/releases/V6_CHANGELOG_RC1.md")
        normalized = normalize_ws(changelog)
        self.assertIn("Pricing/limit note", changelog)
        self.assertIn("include core monitoring by default", normalized)
        self.assertIn("not a monitoring-volume paywall", normalized)
        self.assertNotIn(
            "monitored-system limits, commercial posture, and trial eligibility",
            changelog,
        )
        self.assertNotIn("Limits are applied to canonical top-level monitored systems", changelog)

    def test_version_file_matches_current_rc_packet(self) -> None:
        current_version = read("VERSION").strip()
        if current_version == "6.0.0":
            release_notes = read("docs/releases/RELEASE_NOTES_v6.md")
            changelog = read("docs/releases/V6_CHANGELOG.md")
            self.assertIn(f"Pulse v{current_version} Release Notes", release_notes)
            self.assertIn(f"`v{current_version}`", release_notes)
            self.assertIn(f"Pulse v{current_version}", changelog)
        else:
            self.assertEqual(current_version, "6.0.0-rc.2")

            release_notes = read("docs/releases/RELEASE_NOTES_v6_RC2_DRAFT.md")
            changelog = read("docs/releases/V6_CHANGELOG_RC2_DRAFT.md")
            operator_pack = read("docs/releases/V6_RC2_OPERATOR_SUPPORT_PACK_DRAFT.md")

            self.assertIn(f"Pulse v{current_version} Draft Release Notes", release_notes)
            self.assertIn(f"`v{current_version}`", release_notes)
            self.assertIn(f"Pulse v{current_version} Draft Changelog", changelog)
            self.assertIn(f"`v{current_version}`", operator_pack)

    def test_upgrade_guide_points_at_current_rc_support_pack(self) -> None:
        upgrade_guide = read("docs/UPGRADE_v6.md")
        current_version = read("VERSION").strip()
        self.assertIn("sudo /bin/update --version vX.Y.Z", upgrade_guide)
        self.assertIn("follow the signed server-installer flow in [INSTALL.md](INSTALL.md)", upgrade_guide)
        self.assertIn("the historical Pulse update signer was not recovered", normalize_ws(upgrade_guide))
        self.assertIn("manual reinstall or other explicit trust migration", normalize_ws(upgrade_guide))
        self.assertIn("### License and Entitlements", upgrade_guide)
        self.assertNotIn("### License, Trial, and Entitlements", upgrade_guide)
        self.assertIn("does not expose a general in-app trial, trial-return callback, or hosted AI quickstart", normalize_ws(upgrade_guide))
        self.assertIn(
            "Self-hosted monitoring and child-resource volume are not metered under the current v6 policy",
            upgrade_guide,
        )
        self.assertIn("monitored-system, guest, or child-resource volume cap", upgrade_guide)
        self.assertNotIn("uncapped monitored-system and guest capacity", upgrade_guide)
        self.assertNotIn("uncapped capacity automatically", upgrade_guide)
        self.assertNotIn("`POST /api/license/trial/start`", upgrade_guide)
        self.assertNotIn("signed activation token to `/auth/trial-activate`", upgrade_guide)
        self.assertNotIn("25 hosted Patrol", upgrade_guide)
        if current_version == "6.0.0":
            self.assertIn("docs/releases/RELEASE_NOTES_v6.md", upgrade_guide)
            self.assertIn("docs/releases/V6_CHANGELOG.md", upgrade_guide)
            self.assertNotIn("docs/releases/V6_RC2_OPERATOR_SUPPORT_PACK_DRAFT.md", upgrade_guide)
        else:
            self.assertIn("docs/releases/V6_RC2_OPERATOR_SUPPORT_PACK_DRAFT.md", upgrade_guide)
            self.assertNotIn("docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md", upgrade_guide)

    def test_prerelease_feedback_template_uses_generic_current_rc_wording(self) -> None:
        template = read(".github/ISSUE_TEMPLATE/v6_rc_feedback.yml")
        self.assertIn("placeholder: v6.0.0-rc.N", template)
        self.assertIn("placeholder: rcourtman/pulse:v6.0.0-rc.N or pulse-linux-amd64", template)
        self.assertIn("Upgrade to the current v6 RC build", template)
        self.assertNotIn("v6.0.0-rc.1", template)

    def test_demo_site_copy_points_at_current_release_packet_index(self) -> None:
        demo_copy = read("docs/releases/V6_RC_DEMO_SITE_COPY.md")
        self.assertIn("docs/RELEASE_NOTES.md", demo_copy)
        self.assertIn("Current RC packet: `docs/releases/`", demo_copy)
        self.assertNotIn("docs/releases/RELEASE_NOTES_v6.md", demo_copy)
        self.assertNotIn("docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md", demo_copy)

    def test_update_demo_server_workflow_uses_generic_tag_examples(self) -> None:
        workflow = read(".github/workflows/update-demo-server.yml")
        self.assertIn("v6.0.0-rc.N or v6.0.0", workflow)
        self.assertNotIn("v6.0.0-rc.1", workflow)

    def test_rehearsal_template_and_workflow_capture_ga_rehearsal_record(self) -> None:
        template = read("docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md")
        workflow = read(".github/workflows/release-dry-run.yml")
        release_workflow = read(".github/workflows/create-release.yml")
        dry_run_trigger = read("scripts/trigger-release-dry-run.sh")
        recorder = read("scripts/release_control/record_rc_to_ga_rehearsal.py")
        internal_recorder = read("scripts/release_control/internal/record_rc_to_ga_rehearsal.py")
        renderer = read("scripts/release_control/render_release_body.py")
        resolver = read("scripts/release_control/resolve_release_promotion.py")
        self.assertIn("GitHub Actions run URL", template)
        self.assertIn("Exact GA date to publish with GA", template)
        self.assertIn("record_rc_to_ga_rehearsal.py --run-id <run-id>", template)
        self.assertIn("rc-to-ga-promotion-readiness-rehearsal-<record-date>.md", template)
        self.assertIn(promotion_metadata_envelope(), normalize_ws(template))
        self.assertIn("rc-to-ga-rehearsal-summary", workflow)
        self.assertIn("record_rc_to_ga_rehearsal.py --run-id ${{ github.run_id }}", workflow)
        self.assertIn("rc-to-ga-promotion-readiness-rehearsal-<record-date>.md", workflow)
        self.assertIn("control_plane.py --branch-for-version", workflow)
        self.assertIn('git fetch --prune origin main "${REQUIRED_BRANCH}" --tags', workflow)
        self.assertIn("resolve_release_promotion.py", workflow)
        self.assertIn("- Rollback command:", workflow)
        self.assertIn("- Candidate stable tag:", workflow)
        self.assertIn("- Promotion channel:", workflow)
        self.assertIn("- Promoted prerelease tag:", workflow)
        self.assertIn("Prerelease soak hours at rehearsal time", workflow)
        self.assertIn("Planned GA date", workflow)
        self.assertIn("Planned v5 end-of-support date", workflow)
        self.assertIn("resolve_release_promotion.py", release_workflow)
        self.assertIn("render_release_body.py", release_workflow)
        self.assertIn("build_promotion_metadata_section", renderer)
        self.assertIn("default_output_path", internal_recorder)
        self.assertIn("output path already exists", internal_recorder)
        self.assertIn("default_output_path", recorder)
        self.assertIn("rollback_version is required for every release rehearsal and promotion", resolver)
        self.assertIn("Stable promotion requires promoted_from_tag", resolver)
        self.assertIn("Stable v6.0.0 requires ga_date in YYYY-MM-DD form", resolver)
        self.assertIn("release_notes must include the exact ga_date", resolver)
        self.assertIn("check-workflow-dispatch-inputs.py", dry_run_trigger)
        self.assertIn('--branch "$CURRENT_BRANCH"', dry_run_trigger)
        self.assertIn("release-dry-run.yml", dry_run_trigger)
        self.assertIn("gh workflow run release-dry-run.yml", dry_run_trigger)
        self.assertIn("Release Dry Run executes the selected remote ref", dry_run_trigger)
        self.assertNotIn("Continue anyway?", dry_run_trigger)
        self.assertIn('if [ "${REHEARSAL_CONCLUSION}" != "success" ]; then', workflow)
        self.assertIn("did not produce a valid promotion metadata envelope", workflow)
        self.assertIn("Do not use this artifact to clear", workflow)

    def test_release_workflow_enforces_rc_lineage_soak_and_v5_notice(self) -> None:
        content = read(".github/workflows/create-release.yml")
        helper = read("scripts/trigger-release.sh")
        renderer = read("scripts/release_control/render_release_body.py")
        policy = read("docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md")
        source_of_truth = read("docs/release-control/v6/internal/SOURCE_OF_TRUTH.md")
        runbook = read("docs/releases/V6_PRERELEASE_RUNBOOK.md")
        resolver = read("scripts/release_control/resolve_release_promotion.py")
        contract = read("docs/release-control/v6/internal/subsystems/deployment-installability.md")
        self.assertIn("It does not automatically check out or build `pulse-enterprise`.", runbook)
        self.assertIn("public `pulse-v...` release archives are OSS runtime artifacts", runbook)
        self.assertIn("`pulse-pro-v...` archives", runbook)
        self.assertIn("`bin/pulse --version` identifies `Pulse Pro (Enterprise)`", runbook)
        self.assertIn("Paid-user GA is part of that same release boundary", contract)
        self.assertIn(
            "the public Pulse release workflow builds OSS `pulse-v...` artifacts only",
            normalize_ws(contract),
        )
        self.assertIn("`pulse-pro-v...` archives identify `Pulse Pro (Enterprise)`", contract)
        self.assertIn("control_plane.py --branch-for-version", content)
        self.assertIn('git fetch --prune origin main "${REQUIRED_BRANCH}" --tags', content)
        self.assertIn('REQUIRED_BRANCH: ${{ steps.branch_policy.outputs.required_branch }}', content)
        self.assertIn("resolve_release_promotion.py", content)
        self.assertIn("render_release_body.py", content)
        self.assertIn("build_promotion_metadata_section", renderer)
        self.assertIn('gh workflow run publish-docker.yml --ref "${REQUIRED_BRANCH}"', content)
        self.assertIn('gh workflow run update-demo-server.yml --ref "${REQUIRED_BRANCH}"', content)
        self.assertIn("sanitize_release_notes", renderer)
        self.assertIn("Do not treat this as published", renderer)
        self.assertIn("_DRAFT.md", renderer)
        self.assertIn("rollback target and exact reinstall command recorded", policy)
        self.assertIn("rc-to-ga-rehearsal-summary", policy)
        self.assertIn("record_rc_to_ga_rehearsal.py --run-id <run-id>", policy)
        self.assertIn(promotion_metadata_envelope(), normalize_ws(policy))
        self.assertIn("recorded rollback target plus exact", source_of_truth)
        self.assertIn("hours of prerelease soak", resolver)
        self.assertIn("minimum is 72 hours unless hotfix_exception is true", resolver)
        self.assertIn("build_promotion_metadata_section", renderer)
        self.assertIn("Planned GA date", renderer)
        self.assertIn("Planned v5 end-of-support date", renderer)
        self.assertIn("historical_asset_backfill_only:", content)
        self.assertIn("Repair an already-published release packet in place without rebuilding binaries", content)
        self.assertIn("draft: ${{ github.event.inputs.draft_only == 'true' }}", content)
        self.assertIn('gh api "repos/${{ github.repository }}/releases?per_page=100" --paginate', content)
        self.assertIn('git push origin "refs/tags/${TAG}" --force', content)
        self.assertIn('Retargeting existing draft tag ${TAG}', content)
        self.assertIn('-F target_commitish="${HEAD_SHA}"', content)
        self.assertIn('historical_asset_backfill_only=${HISTORICAL_ASSET_BACKFILL_ONLY}', content)
        self.assertIn("if: ${{ needs.prepare.outputs.historical_asset_backfill_only != 'true' }}", content)
        self.assertIn("if: ${{ needs.prepare.outputs.historical_asset_backfill_only == 'true' }}", content)
        self.assertIn("issues: write", content)
        self.assertIn('./scripts/backfill-release-assets.sh --tag "${{ needs.prepare.outputs.tag }}" --repo "${{ github.repository }}"', content)
        self.assertIn('./scripts/validate-published-release.sh "${{ needs.prepare.outputs.tag }}" "${{ github.repository }}"', content)
        self.assertIn("PULSE_UPDATE_SIGNING_KEY: ${{ secrets.PULSE_UPDATE_SIGNING_KEY }}", content)
        self.assertIn("PULSE_UPDATE_SIGNING_PUBLIC_KEY: ${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}", content)
        self.assertIn("PULSE_UPDATE_SIGNING_PUBLIC_KEY=${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}", content)
        self.assertIn('SYFT_VERSION="1.42.4"', content)
        self.assertIn('SYFT_ARCHIVE="syft_${SYFT_VERSION}_linux_amd64.tar.gz"', content)
        self.assertIn('SYFT_SHA256="590650c2743b83f327d1bf9bec64f6f83b7fec504187bb84f500c862bf8f2a0f"', content)
        self.assertIn('install -m 0755 "${TMP_DIR}/syft" /usr/local/bin/syft', content)
        self.assertIn('gh release upload "${TAG}" release/*.sig --clobber', content)
        self.assertIn('gh release upload "${TAG}" release/*.sshsig --clobber', content)
        self.assertIn('gh release upload "${TAG}" release/*.sbom.spdx.json --clobber', content)
        self.assertIn("provenance: mode=max", content)
        self.assertIn("sbom: true", content)
        self.assertIn("id-token: write", content)
        self.assertIn("attestations: write", content)
        self.assertIn("uses: actions/attest@59d89421af93a897026c735860bf21b6eb4f7b26 # v4", content)
        self.assertIn("subject-path: release/*", content)
        build_script = read("scripts/build-release.sh")
        release_asset_helper = read("scripts/release_asset_common.sh")
        backfill_script = read("scripts/backfill-release-assets.sh")
        backfill_workflow = read(".github/workflows/backfill-release-assets.yml")
        self.assertIn('RELEASE_PACKET_SBOM="pulse-v${VERSION}-release.sbom.spdx.json"', build_script)
        self.assertIn('SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"', build_script)
        self.assertIn('PULSE_REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"', build_script)
        self.assertIn('cd "${PULSE_REPO_ROOT}"', build_script)
        self.assertIn('source "${SCRIPT_DIR}/release_asset_common.sh"', build_script)
        self.assertIn('pulse_release_prepare_signing_state "pulse-installer" "pulse-install"', build_script)
        self.assertIn('pulse_release_generate_packet_sbom "${RELEASE_DIR}" "${RELEASE_PACKET_SBOM}"', build_script)
        self.assertIn('pulse_release_write_checksums_and_signatures "${RELEASE_DIR}" "${checksum_files[@]}"', build_script)
        self.assertIn(': "${PULSE_SCRIPTS_DIR:=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"', release_asset_helper)
        self.assertIn(': "${PULSE_REPO_ROOT:=$(cd "${PULSE_SCRIPTS_DIR}/.." && pwd)}"', release_asset_helper)
        self.assertIn('go -C "${PULSE_REPO_ROOT}" run "${PULSE_SCRIPTS_DIR}/release_update_key.go" "$@"', release_asset_helper)
        self.assertIn('pulse_release_go_run_update_key public-key --private-key "${PULSE_UPDATE_SIGNING_KEY}"', release_asset_helper)
        self.assertIn(
            'pulse_release_go_run_update_key fingerprint --public-key "${PULSE_RELEASE_UPDATE_PUBLIC_KEY}"',
            release_asset_helper,
        )
        self.assertIn('pulse_release_go_run_update_key public-key-ssh --private-key "${PULSE_UPDATE_SIGNING_KEY}"', release_asset_helper)
        self.assertIn('pulse_release_go_run_update_key openssh-private-key --private-key "${PULSE_UPDATE_SIGNING_KEY}"', release_asset_helper)
        self.assertIn('pulse_release_go_run_update_key sign --private-key "${PULSE_UPDATE_SIGNING_KEY}" --file "${absolute_file}"', release_asset_helper)
        self.assertIn("PULSE_UPDATE_SIGNING_PUBLIC_KEY", release_asset_helper)
        self.assertIn("PULSE_UPDATE_SIGNING_PUBLIC_KEY_FINGERPRINT", release_asset_helper)
        self.assertIn(
            "Verified update signing public key fingerprint: ${PULSE_RELEASE_UPDATE_PUBLIC_KEY_FINGERPRINT}",
            release_asset_helper,
        )
        self.assertIn('"${resolved_tool}" "dir:${release_dir}" -o "spdx-json=${tmp_sbom}"', release_asset_helper)
        self.assertIn('find . -maxdepth 1 -type f \\( -name \'*.sig\' -o -name \'*.sshsig\' \\) -delete', release_asset_helper)
        self.assertIn('source "${SCRIPT_DIR}/release_asset_common.sh"', backfill_script)
        self.assertIn('gh release download "${TAG}" -R "${REPO}" --dir "${RELEASE_DIR}" --clobber', backfill_script)
        self.assertIn('pulse_release_generate_packet_sbom "${PAYLOAD_DIR}" "${RELEASE_PACKET_SBOM}"', backfill_script)
        self.assertIn('pulse_release_write_checksums_and_signatures "${RELEASE_DIR}" "${checksum_files[@]}"', backfill_script)
        self.assertIn('gh release upload "${TAG}" "${RELEASE_DIR}/${RELEASE_PACKET_SBOM}" --clobber', backfill_script)
        self.assertIn("name: Backfill Release Assets", backfill_workflow)
        self.assertIn("workflow_dispatch:", backfill_workflow)
        self.assertIn('SYFT_VERSION="1.42.4"', backfill_workflow)
        self.assertIn('./scripts/backfill-release-assets.sh --tag "${{ inputs.tag }}" --repo "${{ github.repository }}"', backfill_workflow)
        self.assertIn('./scripts/validate-published-release.sh "${{ inputs.tag }}" "${{ github.repository }}"', backfill_workflow)
        self.assertIn("PULSE_UPDATE_SIGNING_PUBLIC_KEY: ${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}", backfill_workflow)
        self.assertIn("id: license_key_cache", content)
        self.assertIn("PULSE_LICENSE_PUBLIC_KEY_SHA256=${{ steps.license_key_cache.outputs.sha256 }}", content)
        self.assertIn("pulse_license_public_key=${{ secrets.PULSE_LICENSE_PUBLIC_KEY }}", content)
        self.assertIn("pulse_update_signing_key=${{ secrets.PULSE_UPDATE_SIGNING_KEY }}", content)
        self.assertIn("--secret id=pulse_license_public_key,env=PULSE_LICENSE_PUBLIC_KEY", content)
        self.assertIn("--secret id=pulse_update_signing_key,env=PULSE_UPDATE_SIGNING_KEY", content)
        self.assertIn('--build-arg PULSE_LICENSE_PUBLIC_KEY_SHA256="${PULSE_LICENSE_PUBLIC_KEY_SHA256}"', content)
        self.assertNotIn("provenance: false", content)
        self.assertIn("Derived rollback command:", helper)
        self.assertIn("./scripts/install.sh --version", helper)
        self.assertIn("v6 GA date to publish with GA", helper)
        self.assertIn("-f ga_date", helper)
        self.assertIn("Planned GA date", renderer)
        self.assertIn("Planned v5 end-of-support date", renderer)
        self.assertIn("Stable v6.0.0 requires v5_eos_date in YYYY-MM-DD form", resolver)
        self.assertIn("release_notes must include the Pulse v5 maintenance-only support notice", resolver)
        dry_run_workflow = read(".github/workflows/release-dry-run.yml")
        self.assertIn("Required rollback stable version to rehearse", dry_run_workflow)
        self.assertIn("rollback_version:\n        description: 'Required rollback stable version to rehearse", dry_run_workflow)
        self.assertIn("required: true", dry_run_workflow)
        self.assertIn("Required: prior stable version to pin for rollback", content)
        self.assertIn("rollback_version:\n        description: 'Required: prior stable version to pin for rollback", content)
        self.assertIn("check-workflow-dispatch-inputs.py", helper)
        self.assertIn('--branch "$CURRENT_BRANCH"', helper)
        self.assertIn('--ref "$CURRENT_BRANCH"', helper)
        self.assertIn("Release automation executes the selected remote ref", helper)
        self.assertNotIn("Continue anyway?", helper)
        self.assertIn("Audit header composition", content)
        self.assertIn("run: npm --prefix frontend-modern run lint:headers", content)
        self.assertIn("pushed governed release-branch copy of `.github/workflows/release-dry-run.yml`", policy)
        self.assertIn("GitHub executes the selected remote ref", normalize_ws(policy))
        checklist = read("docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md")
        self.assertIn("pushed governed release-branch copy of `.github/workflows/release-dry-run.yml`", checklist)
        self.assertIn("workflow_dispatch", checklist)
        self.assertIn("selected remote ref", normalize_ws(checklist))
        self.assertIn("local rehearsal branch exactly matches `origin`", checklist)
        self.assertIn("derive the governed release branch from release-control metadata", checklist)
        template = read("docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md")
        self.assertIn("governed release line from `control_plane.json`", template)
        self.assertIn("pulse/v6-release", template)
        self.assertIn("record_rc_to_ga_rehearsal.py --run-id <run-id>", runbook)
        self.assertIn("rc-to-ga-promotion-readiness-rehearsal-<record-date>.md", runbook)
        self.assertIn("Existing unpublished draft releases for the same tag are updated in place", runbook)
        self.assertIn("Do not rewrite shipped RC notes in place", runbook)
        self.assertIn("`rc.1`, `rc.2`, and later prerelease", runbook)
        self.assertIn("The current RC release packet is prepared and internally linked", runbook)
        self.assertIn("operators know the update signer changed", normalize_ws(runbook))
        self.assertIn("manual reinstall or other explicit trust-migration path", normalize_ws(runbook))
        self.assertIn("points at the current in-repo draft packet", runbook)
        self.assertIn('export RC_VERSION="6.0.0-rc.2"', runbook)
        self.assertIn("printf '%s\\n' \"$RC_VERSION\" > VERSION", runbook)
        self.assertIn("markdown text from the current release-notes packet", runbook)
        self.assertIn("Keep the current release-notes, changelog, and operator-support packet in", runbook)
        self.assertIn("Published release bodies must also stay publication-safe", contract)
        self.assertIn("must state the continuity impact explicitly", normalize_ws(contract))
        self.assertIn(
            "append the standardized installation and promotion metadata sections exactly once",
            normalize_ws(contract),
        )

    def test_release_artifact_workflows_refuse_stable_without_matching_rc(self) -> None:
        publish = read(".github/workflows/publish-docker.yml")
        promote = read(".github/workflows/promote-floating-tags.yml")
        demo = read(".github/workflows/update-demo-server.yml")
        preview_deploy = read(".github/workflows/deploy-demo-server.yml")
        release_workflow = read(".github/workflows/create-release.yml")
        helm = read(".github/workflows/publish-helm-chart.yml")
        helm_pages = read(".github/workflows/helm-pages.yml")
        chart = read("deploy/helm/pulse/Chart.yaml")
        chart_sync = read("scripts/sync_chart_release_metadata.py")
        demo_smoke = read("scripts/demo_public_browser_smoke.cjs")
        runbook = read("docs/releases/V6_PRERELEASE_RUNBOOK.md")
        self.assertIn("control_plane.py --branch-for-version", publish)
        self.assertIn("provenance: mode=max", publish)
        self.assertIn("sbom: true", publish)
        self.assertIn("uses: actions/attest@59d89421af93a897026c735860bf21b6eb4f7b26 # v4", publish)
        self.assertIn("subject-name: docker.io/rcourtman/pulse", publish)
        self.assertIn("subject-name: ghcr.io/${{ github.repository_owner }}/pulse", publish)
        self.assertIn("subject-name: docker.io/rcourtman/pulse-agent", publish)
        self.assertIn("subject-name: ghcr.io/${{ github.repository_owner }}/pulse-agent", publish)
        self.assertIn("create-storage-record: false", publish)
        self.assertIn("id: license_key_cache", publish)
        self.assertIn("PULSE_LICENSE_PUBLIC_KEY_SHA256=${{ steps.license_key_cache.outputs.sha256 }}", publish)
        self.assertIn("pulse_license_public_key=${{ secrets.PULSE_LICENSE_PUBLIC_KEY }}", publish)
        self.assertIn("PULSE_UPDATE_SIGNING_PUBLIC_KEY=${{ vars.PULSE_UPDATE_SIGNING_PUBLIC_KEY }}", publish)
        self.assertNotIn("provenance: false", publish)
        self.assertIn("control_plane.py --branch-for-version", promote)
        self.assertIn("control_plane.py --branch-for-version", demo)
        self.assertIn("preview-v6", demo)
        self.assertIn("demo-preview-v6", demo)
        self.assertIn("demo-stable", demo)
        self.assertIn("Refusing prerelease tag", demo)
        self.assertIn("Refusing stable tag", demo)
        self.assertIn("latest published release for target", demo)
        self.assertIn('SERVICE_NAME="pulse-v6-preview"', demo)
        self.assertIn("Preview demo updates must not target the stable pulse service.", demo)
        self.assertIn("tailscale/github-action@4e4c49acaa9818630ce0bd7a564372c17e33fb4d # v2", demo)
        self.assertIn("TS_AUTHKEY", demo)
        self.assertIn("DEMO_EXPECTED_HOSTNAME", demo)
        self.assertIn("Verify target host identity", demo)
        self.assertIn("Demo environment points at host $REMOTE_HOSTNAME but expected $DEMO_EXPECTED_HOSTNAME.", demo)
        self.assertIn(".resources | if type == \"array\" then length else 0 end", demo)
        self.assertNotIn(".nodes | length", demo)
        self.assertIn("Mock resources detected", demo)
        self.assertIn("canonical mock resources are missing", demo)
        self.assertIn("Verify frontend parity", demo)
        self.assertIn("Verify public browser smoke", demo)
        self.assertIn("./scripts/run_demo_public_browser_smoke.sh", demo)
        self.assertIn("extract_entry_asset()", demo)
        self.assertIn(r'<script\b[^>]*\bsrc=\"(/assets/index-[^\"]*\.js)\"', demo)
        self.assertIn("Public demo is serving $PUBLIC_ASSET but the target service is serving $REMOTE_ASSET.", demo)
        self.assertIn('SERVICE_NAME="pulse-v6-preview"', preview_deploy)
        self.assertIn("Preview demo deployments must not target the stable pulse service.", preview_deploy)
        self.assertIn("DEMO_EXPECTED_HOSTNAME", preview_deploy)
        self.assertIn("Verify target host identity", preview_deploy)
        self.assertIn("Demo environment points at host $REMOTE_HOSTNAME but expected $DEMO_EXPECTED_HOSTNAME.", preview_deploy)
        self.assertIn("Verify frontend parity", preview_deploy)
        self.assertIn("Verify public browser smoke", preview_deploy)
        self.assertIn("./scripts/run_demo_public_browser_smoke.sh", preview_deploy)
        self.assertIn("extract_entry_asset()", preview_deploy)
        self.assertIn(r'<script\b[^>]*\bsrc=\"(/assets/index-[^\"]*\.js)\"', preview_deploy)
        self.assertIn("Public demo is serving $PUBLIC_ASSET but the build expected $EXPECTED_ASSET.", preview_deploy)
        self.assertIn("control_plane.py --branch-for-version", helm)
        self.assertIn("control_plane.py --branch-for-version", helm_pages)
        self.assertIn("does not descend from any matching prerelease tag", publish)
        self.assertIn("does not descend from any matching prerelease tag", promote)
        self.assertIn("Refusing cross-line Helm pages release", helm_pages)
        self.assertIn("Build local Pulse runtime image for Helm smoke", release_workflow)
        self.assertIn('kind load docker-image "${SMOKE_IMAGE_REPOSITORY}:${SMOKE_IMAGE_TAG}" --name pulse-test', release_workflow)
        self.assertIn('--set image.repository="${SMOKE_IMAGE_REPOSITORY}"', release_workflow)
        self.assertIn('--set image.pullPolicy=Never', release_workflow)
        self.assertIn("needs.helm_smoke.result == 'success'", release_workflow)
        self.assertIn('echo "required_branch=${REQUIRED_BRANCH}" >> "$GITHUB_OUTPUT"', helm_pages)
        self.assertIn('git checkout -B "$REQUIRED_BRANCH" "origin/$REQUIRED_BRANCH"', helm_pages)
        self.assertIn('git pull --rebase origin "$REQUIRED_BRANCH"', helm_pages)
        self.assertIn('git push origin HEAD:"$REQUIRED_BRANCH"', helm_pages)
        self.assertIn('HELM_DOCS_VERSION="1.14.2"', helm_pages)
        self.assertIn('HELM_DOCS_ARCHIVE="helm-docs_${HELM_DOCS_VERSION}_Linux_x86_64.tar.gz"', helm_pages)
        self.assertIn(
            'HELM_DOCS_SHA256="a8cf72ada34fad93285ba2a452b38bdc5bd52cc9a571236244ec31022928d6cc"',
            helm_pages,
        )
        self.assertIn('printf \'%s  %s\\n\' "$HELM_DOCS_SHA256" "$HELM_DOCS_ARCHIVE" | sha256sum --check --', helm_pages)
        self.assertNotIn("git pull --rebase origin main", helm_pages)
        self.assertNotIn("git push origin main", helm_pages)
        self.assertNotIn("kind load docker-image", helm_pages)
        self.assertIn("helm status pulse || true", helm_pages)
        self.assertIn("kubectl describe pods -A || true", helm_pages)
        self.assertIn("kubectl get events -A --sort-by=.lastTimestamp || kubectl get events -A || true", helm_pages)
        self.assertIn('TARGET="preview-v6"', release_workflow)
        self.assertIn('TARGET="stable"', release_workflow)
        self.assertIn('gh workflow run update-demo-server.yml --ref "${REQUIRED_BRANCH}" -f tag="${{ needs.prepare.outputs.tag }}" -f target="${TARGET}"', release_workflow)
        self.assertIn("sync_chart_release_metadata.py", helm)
        self.assertIn("sync_chart_release_metadata.py", helm_pages)
        self.assertIn("--chart deploy/helm/pulse/Chart.yaml", helm)
        self.assertIn("--chart deploy/helm/pulse/Chart.yaml", helm_pages)
        self.assertIn('git checkout --detach "refs/tags/${RELEASE_TAG}"', helm)
        self.assertNotIn("blob/main/docs/KUBERNETES.md", chart)
        self.assertNotIn("raw.githubusercontent.com/rcourtman/Pulse/main/docs/images/pulse-logo.svg", chart)
        self.assertIn("blob/{tag}/docs/KUBERNETES.md", chart_sync)
        self.assertIn("raw.githubusercontent.com/{repo}/{tag}/docs/images/pulse-logo.svg", chart_sync)
        self.assertIn("both stable and prerelease releases dispatch", runbook)
        self.assertIn("Release `6.0.0` from `pulse/v6-release`", runbook)
        self.assertIn("separate v6 preview demo environment", runbook)
        self.assertIn("preview-v6", runbook)
        self.assertIn(promotion_metadata_envelope(), normalize_ws(runbook))
        self.assertIn("waitUntil: 'domcontentloaded'", demo_smoke)
        self.assertIn("getByLabel('Username').waitFor({ state: 'visible', timeout: 120000 })", demo_smoke)
        self.assertIn("getByLabel('Password').waitFor({ state: 'visible', timeout: 120000 })", demo_smoke)
        self.assertIn("getByRole('button', { name: 'Sign in to Pulse' }).waitFor({ state: 'visible', timeout: 120000 })", demo_smoke)
        self.assertNotIn("waitUntil: 'networkidle'", demo_smoke)

    def test_blocked_record_tracks_current_target_and_candidate_version(self) -> None:
        blocked_record_surface = {
            "VERSION",
            "docs/release-control/control_plane.json",
            "docs/release-control/v6/internal/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md",
            "docs/release-control/v6/internal/records/rc-to-ga-promotion-readiness-blocked-2026-04-04.md",
            "scripts/release_control/record_rc_to_ga_blocked.py",
        }
        if USE_STAGED_GOVERNANCE and not any(path in blocked_record_surface for path in STAGED_FILES):
            self.skipTest("staged slice does not touch the blocked-record promotion surface")
        blocked = read("docs/release-control/v6/internal/records/rc-to-ga-promotion-readiness-blocked-2026-04-04.md")
        current_version = read("VERSION").strip()
        active_target_id = read_json("docs/release-control/control_plane.json")["active_target_id"]
        self.assertIn(f"VERSION={current_version}", blocked)
        self.assertIn("artifact-owned candidate stable tag", blocked)
        self.assertIn("artifact-owned promotion channel", blocked)
        self.assertIn("artifact-owned promoted prerelease tag", blocked)
        self.assertIn("artifact-owned rollback target", blocked)
        self.assertIn("Materialize the final rehearsal record from that artifact without", blocked)
        self.assertIn("hand-repairing any missing candidate tag, promoted prerelease tag, rollback", blocked)
        if active_target_id == "v6-ga-promotion":
            self.assertIn(
                f"The active control-plane target is `{active_target_id}`, so stable or GA",
                blocked,
            )
        else:
            self.assertIn(f"The active control-plane target is still `{active_target_id}`, not", blocked)
        matrix = read("docs/release-control/v6/internal/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md")
        self.assertIn(promotion_metadata_envelope(), normalize_ws(matrix))
        expected = blocked_record.build_blocked_record(record_date="2026-04-04")
        self.assertEqual(blocked, expected)


if __name__ == "__main__":
    unittest.main()
