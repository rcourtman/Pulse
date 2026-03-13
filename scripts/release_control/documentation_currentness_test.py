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
STATUS = load_json("docs/release-control/v6/status.json")
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
        "docs/release-control/CONTROL_PLANE.md",
        "docs/release-control/control_plane.json",
        "docs/release-control/control_plane.schema.json",
        "docs/release-control/v6/README.md",
        "docs/release-control/v6/PRE_RELEASE_CHECKLIST.md",
        "docs/release-control/v6/RELEASE_PROMOTION_POLICY.md",
        "docs/release-control/v6/V5_MAINTENANCE_SUPPORT_POLICY.md",
        "docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md",
        "docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md",
        "docs/release-control/v6/SOURCE_OF_TRUTH.md",
        "docs/release-control/v6/status.json",
        "docs/release-control/v6/status.schema.json",
        "docs/release-control/v6/subsystems/registry.json",
        "docs/release-control/v6/subsystems/registry.schema.json",
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
        content = read("docs/release-control/CONTROL_PLANE.md")
        self.assertIn(f"`{ACTIVE_TARGET_ID}` is the current active engineering target.", content)
        inactive_target_id = "v6-ga-promotion" if ACTIVE_TARGET_ID != "v6-ga-promotion" else "v6-rc-stabilization"
        self.assertNotIn(f"`{inactive_target_id}` is the current active engineering target.", content)

    def test_source_of_truth_keeps_supporting_docs_as_evidence_only(self) -> None:
        content = read("docs/release-control/v6/SOURCE_OF_TRUTH.md")
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


if __name__ == "__main__":
    unittest.main()
