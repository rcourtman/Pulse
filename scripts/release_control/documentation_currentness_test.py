#!/usr/bin/env python3
"""Guard the active v6 documentation surface against stale or legacy guidance drift."""

from __future__ import annotations

import json
import os
import unittest
from pathlib import Path

from repo_file_io import load_repo_json, read_repo_text


USE_STAGED_GOVERNANCE = os.environ.get("PULSE_READ_STAGED_GOVERNANCE") == "1"


def load_json(rel: str) -> dict:
    return load_repo_json(rel, staged=USE_STAGED_GOVERNANCE, strict_staged=USE_STAGED_GOVERNANCE)


def read(rel: str) -> str:
    return read_repo_text(rel, staged=USE_STAGED_GOVERNANCE, strict_staged=USE_STAGED_GOVERNANCE)


CONTROL_PLANE = load_json("docs/release-control/control_plane.json")
STATUS = load_json("docs/release-control/v6/internal/status.json")
ACTIVE_PROFILE_ID = str(CONTROL_PLANE["active_profile_id"])
ACTIVE_TARGET_ID = str(CONTROL_PLANE["active_target_id"])
ACTIVE_PROFILE = next(profile for profile in CONTROL_PLANE["profiles"] if profile["id"] == ACTIVE_PROFILE_ID)

LEGACY_NAME_MARKERS = (
    "LEGACY",
    "GUIDING_LIGHT",
    "_AUDIT_",
    "RETIREMENT_AUDIT",
    "V5_TO_V6",
)


def active_guidance_paths() -> tuple[str, ...]:
    paths = {
        "docs/release-control/internal/AGENT_VALUES.md",
        "docs/release-control/internal/CONTROL_PLANE.md",
        "docs/release-control/control_plane.json",
        "docs/release-control/control_plane.schema.json",
        "docs/release-control/v6/README.md",
        "docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md",
        "docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md",
        "docs/release-control/v6/internal/V5_MAINTENANCE_SUPPORT_POLICY.md",
        "docs/release-control/v6/internal/CANONICAL_DEVELOPMENT_PROTOCOL.md",
        "docs/release-control/v6/internal/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md",
        "docs/release-control/v6/internal/SOURCE_OF_TRUTH.md",
        "docs/release-control/v6/internal/status.json",
        "docs/release-control/v6/status.schema.json",
        "docs/release-control/v6/internal/subsystems/registry.json",
        "docs/release-control/v6/internal/subsystems/registry.schema.json",
    }
    for key in (
        "control_plane_doc",
        "control_plane_schema",
    ):
        paths.add(str(CONTROL_PLANE[key]))
    for key in (
        "source_of_truth",
        "status",
        "status_schema",
        "development_protocol",
        "high_risk_matrix",
        "registry",
        "registry_schema",
    ):
        paths.add(str(ACTIVE_PROFILE[key]))
    paths.update(str(path) for path in STATUS["source_precedence"])
    paths.add(str(STATUS["source_of_truth_file"]))
    return tuple(sorted(paths))


class DocumentationCurrentnessTest(unittest.TestCase):
    def test_active_guidance_paths_are_not_legacy_named_artifacts(self) -> None:
        for rel in active_guidance_paths():
            path = Path(rel)
            self.assertNotIn("/records/", rel)
            upper_name = path.name.upper()
            self.assertFalse(
                any(marker in upper_name for marker in LEGACY_NAME_MARKERS),
                msg=f"{rel} still looks like a legacy or historical artifact in the active guidance surface",
            )

    def test_control_plane_doc_reflects_current_active_target(self) -> None:
        content = read("docs/release-control/internal/CONTROL_PLANE.md")
        self.assertIn(f"`{ACTIVE_TARGET_ID}` is the current active engineering target.", content)
        inactive_target_id = "v6-ga-promotion" if ACTIVE_TARGET_ID != "v6-ga-promotion" else "v6-rc-stabilization"
        self.assertNotIn(f"`{inactive_target_id}` is the current active engineering target.", content)

    def test_source_of_truth_keeps_supporting_docs_as_evidence_only(self) -> None:
        content = read("docs/release-control/v6/internal/SOURCE_OF_TRUTH.md")
        self.assertIn("Supporting architecture and release docs are evidence only.", content)
        self.assertIn("override the files above.", content)

    def test_status_source_precedence_stays_on_current_canonical_docs(self) -> None:
        for rel in STATUS["source_precedence"]:
            self.assertTrue(rel.startswith("docs/release-control/v6/"))
            self.assertNotIn("/records/", rel)
            upper_name = Path(rel).name.upper()
            self.assertFalse(
                any(marker in upper_name for marker in LEGACY_NAME_MARKERS),
                msg=f"{rel} should not be part of source precedence while marked as historical",
            )

    def test_release_gates_have_high_risk_matrix_sections(self) -> None:
        matrix = read("docs/release-control/v6/internal/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md")
        for gate in STATUS["release_gates"]:
            gate_id = str(gate["id"])
            self.assertIn(
                f"## Gate: `{gate_id}`",
                matrix,
                msg=f"release gate {gate_id!r} points at the high-risk matrix but has no section",
            )

    def test_public_ai_autonomy_docs_keep_free_first_boundary(self) -> None:
        ai_doc = read("docs/AI.md")
        autonomy_doc = read("docs/AI_AUTONOMY.md")
        pulse_pro_doc = read("docs/PULSE_PRO.md")

        self.assertIn(
            "| **Ask before changes** | Investigates findings and proposes fixes. All fixes require approval before execution. | Pro / hosted Cloud |",
            ai_doc,
        )
        self.assertIn(
            "Community and Relay installs can still run scheduled Patrol findings with BYOK.",
            ai_doc,
        )
        self.assertNotIn(
            "| **Investigate** | Investigates findings and proposes fixes. All fixes require approval before execution. | Community (BYOK) |",
            ai_doc,
        )
        self.assertIn(
            "`approval`, `assisted`, and `full`: Require the `ai_autofix` capability",
            autonomy_doc,
        )
        self.assertNotIn("Upgrade to Assisted", autonomy_doc)
        self.assertIn(
            "| **Ask before changes** | Investigates findings and proposes fixes. All fixes require approval before execution. | Pro / hosted Cloud |",
            pulse_pro_doc,
        )
        self.assertNotIn(
            "| **Ask before changes** | Investigates findings and proposes fixes. All fixes require approval before execution. | Community / Relay |",
            pulse_pro_doc,
        )
        for rel in ("docs/AI.md", "docs/PULSE_PRO.md", "docs/FAQ.md", "docs/README.md"):
            content = read(rel)
            self.assertNotIn("alert-triggered root-cause analysis", content)
            self.assertNotIn("safe remediation workflows", content)

    def test_public_self_hosted_docs_avoid_unlimited_monitoring_claims(self) -> None:
        public_docs = (
            "README.md",
            "docs/PULSE_PRO.md",
            "docs/UPGRADE_v6.md",
            "docs/architecture/v6-pricing-and-tiering.md",
            "docs/releases/RELEASE_NOTES_v6.md",
            "docs/releases/RELEASE_NOTES_v6_RC2_DRAFT.md",
            "docs/releases/RELEASE_NOTES_v6_RC3_DRAFT.md",
            "docs/releases/V6_CHANGELOG.md",
            "docs/releases/V6_CHANGELOG_RC1.md",
            "docs/releases/V6_CHANGELOG_RC2_DRAFT.md",
            "docs/releases/V6_CHANGELOG_RC3_DRAFT.md",
            "docs/releases/V6_RC_OPERATOR_SUPPORT_PACK.md",
            "docs/releases/V6_RC2_OPERATOR_SUPPORT_PACK_DRAFT.md",
            "docs/releases/V6_RC3_OPERATOR_SUPPORT_PACK_DRAFT.md",
            "frontend-modern/public/docs/README.md",
        )
        forbidden_phrases = (
            "core monitoring unlimited",
            "core monitoring stays unlimited",
            "core monitoring remains unlimited",
            "monitoring unlimited",
            "unlimited core",
            "no-cap",
            "no cap",
            "remain uncapped",
            "remains uncapped",
            "| unlimited |",
        )
        for rel in public_docs:
            content = read(rel).lower()
            for phrase in forbidden_phrases:
                self.assertNotIn(phrase, content, msg=f"{rel} contains stale paid-cap wording: {phrase}")

    def test_current_agent_docs_point_to_infrastructure_install_flow(self) -> None:
        current_docs = (
            "README.md",
            "SECURITY.md",
            "docs/CENTRALIZED_MANAGEMENT.md",
            "docs/DOCKER.md",
            "docs/PBS.md",
            "docs/UNIFIED_AGENT.md",
            "docs/UPGRADE_v6.md",
            "docs/releases/RELEASE_NOTES_v6_RC1.md",
            "frontend-modern/public/docs/SECURITY.md",
        )
        retired_agent_settings_paths = (
            "Settings → Unified Agents",
            "Settings -> Unified Agents",
            "Settings → Agents → Installation commands",
            "Settings -> Agents -> Installation commands",
        )
        for rel in current_docs:
            content = read(rel)
            for retired_path in retired_agent_settings_paths:
                self.assertNotIn(retired_path, content, msg=f"{rel} still points at {retired_path}")

        root_readme = read("README.md")
        upgrade_doc = read("docs/UPGRADE_v6.md")
        unified_agent_doc = read("docs/UNIFIED_AGENT.md")
        self.assertIn("Settings → Infrastructure → Install on a host", root_readme)
        self.assertIn("v5-to-v6 agent upgrades", root_readme)
        self.assertIn("first installs and in-place agent upgrades", upgrade_doc)
        self.assertIn("supported v5-to-v6 agent upgrade path", unified_agent_doc)
        self.assertIn("Version and outdated", upgrade_doc)
        self.assertIn("only after the agent has successfully reported", upgrade_doc)

    def test_api_token_docs_explain_primary_token_and_revocation_safety(self) -> None:
        config_doc = read("docs/CONFIGURATION.md")
        public_config_doc = read("frontend-modern/public/docs/CONFIGURATION.md")

        expected_fragments = (
            "primary automation API token",
            "separate from your web login password",
            "Revoking a token is safe for Pulse itself",
            "immediately breaks any agent",
            "script, kiosk, or integration still using that token",
            "create and install a replacement token first",
        )
        for content in (config_doc, public_config_doc):
            for fragment in expected_fragments:
                self.assertIn(fragment, content)

    def test_webhook_docs_make_ntfy_service_picker_discoverable(self) -> None:
        webhook_doc = read("docs/WEBHOOKS.md")

        self.assertIn("Click the current service label (Generic by default) to open the service picker", webhook_doc)
        self.assertIn("choose **ntfy** in the service picker before entering the topic URL", webhook_doc)

    def test_alert_help_no_longer_documents_custom_threshold_overrides(self) -> None:
        alerts_help = read("frontend-modern/src/content/help/alerts.ts")

        self.assertNotIn("alerts.thresholds.perGuest", alerts_help)
        self.assertNotIn("Per-Guest Threshold Overrides", alerts_help)
        self.assertNotIn("custom threshold settings that override the defaults", alerts_help)


if __name__ == "__main__":
    unittest.main()
