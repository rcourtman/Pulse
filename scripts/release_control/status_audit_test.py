import os
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from status_audit import audit_status_payload


class StatusAuditTest(unittest.TestCase):
    def test_audit_status_payload_derives_gap_and_evidence_health(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pulse = root / "pulse"
            pulse.mkdir()
            (pulse / "docs").mkdir()
            (pulse / "docs" / "proof.md").write_text("proof", encoding="utf-8")

            payload = {
                "version": "6.0",
                "updated_at": "2026-03-11",
                "execution_model": "direct-repo-sessions",
                "source_of_truth_file": "docs/release-control/v6/SOURCE_OF_TRUTH.md",
                "scope": {
                    "active_repos": ["pulse"],
                    "ignored_repos": [],
                },
                "source_precedence": [
                    "docs/release-control/v6/SOURCE_OF_TRUTH.md",
                    "docs/release-control/v6/status.json",
                    "docs/release-control/v6/status.schema.json",
                    "docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md",
                    "docs/release-control/v6/subsystems/registry.json",
                ],
                "priority_engine": {
                    "formula": "gap-first",
                    "floor_rule": {
                        "release_critical_lanes": ["L1"],
                        "minimum_score": 6,
                    },
                    "weights": {
                        "gap_multiplier": 4,
                        "criticality_range": "0-5",
                        "staleness_range": "0-3",
                        "dependency_range": "0-3",
                    },
                },
                "evidence_reference_policy": {
                    "format": "repo-qualified-relative-paths",
                    "allowed_kinds": ["file", "dir"],
                    "absolute_paths_forbidden": True,
                    "local_repo": "pulse",
                },
                "lanes": [
                    {
                        "id": "L1",
                        "name": "Lane 1",
                        "target_score": 8,
                        "current_score": 6,
                        "status": "partial",
                        "subsystems": [],
                        "evidence": [
                            {"repo": "pulse", "path": "docs/proof.md", "kind": "file"},
                        ],
                    }
                ],
                "open_decisions": [
                    {
                        "id": "d1",
                        "summary": "Need a thing",
                        "owner": "project-owner",
                        "status": "open",
                        "opened_at": "2026-03-11",
                        "lane_ids": ["L1"],
                    }
                ],
                "resolved_decisions": [
                    {
                        "id": "r1",
                        "summary": "Locked a thing",
                        "kind": "governance",
                        "decided_at": "2026-03-10",
                        "lane_ids": ["L1"],
                    }
                ],
            }

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertEqual(report["errors"], [])
            self.assertEqual(report["summary"]["lane_count"], 1)
            self.assertEqual(report["summary"]["resolved_decision_count"], 1)
            self.assertEqual(report["lanes"][0]["gap"], 2.0)
            self.assertFalse(report["lanes"][0]["at_target"])
            self.assertTrue(report["lanes"][0]["all_evidence_present"])
            self.assertEqual(report["lanes"][0]["derived_status"], "behind-target")

    def test_audit_status_payload_reports_missing_cross_repo_evidence(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pulse = root / "pulse"
            pulse.mkdir()
            pulse_pro = root / "pulse-pro"
            pulse_pro.mkdir()

            payload = {
                "version": "6.0",
                "updated_at": "2026-03-11",
                "execution_model": "direct-repo-sessions",
                "source_of_truth_file": "docs/release-control/v6/SOURCE_OF_TRUTH.md",
                "scope": {
                    "active_repos": ["pulse", "pulse-pro"],
                    "ignored_repos": [],
                },
                "source_precedence": [
                    "docs/release-control/v6/SOURCE_OF_TRUTH.md",
                    "docs/release-control/v6/status.json",
                    "docs/release-control/v6/status.schema.json",
                    "docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md",
                    "docs/release-control/v6/subsystems/registry.json",
                ],
                "priority_engine": {
                    "formula": "gap-first",
                    "floor_rule": {
                        "release_critical_lanes": ["L2"],
                        "minimum_score": 6,
                    },
                    "weights": {
                        "gap_multiplier": 4,
                        "criticality_range": "0-5",
                        "staleness_range": "0-3",
                        "dependency_range": "0-3",
                    },
                },
                "evidence_reference_policy": {
                    "format": "repo-qualified-relative-paths",
                    "allowed_kinds": ["file", "dir"],
                    "absolute_paths_forbidden": True,
                    "local_repo": "pulse",
                },
                "lanes": [
                    {
                        "id": "L2",
                        "name": "Lane 2",
                        "target_score": 9,
                        "current_score": 9,
                        "status": "target-met",
                        "subsystems": [],
                        "evidence": [
                            {"repo": "pulse-pro", "path": "missing.md", "kind": "file"},
                        ],
                    }
                ],
                "open_decisions": [
                    {
                        "id": "d1",
                        "summary": "Need a thing",
                        "owner": "project-owner",
                        "status": "open",
                        "opened_at": "2026-03-11",
                        "lane_ids": ["L2"],
                    }
                ],
                "resolved_decisions": [
                    {
                        "id": "r1",
                        "summary": "Locked a thing",
                        "kind": "governance",
                        "decided_at": "2026-03-10",
                        "lane_ids": ["L2"],
                    }
                ],
            }

            with mock.patch.dict(
                os.environ,
                {
                    "PULSE_REPO_ROOT_PULSE": str(pulse),
                    "PULSE_REPO_ROOT_PULSE_PRO": str(pulse_pro),
                },
                clear=False,
            ), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertTrue(report["errors"])
            self.assertIn("missing evidence pulse-pro:missing.md", "\n".join(report["errors"]))
            self.assertEqual(report["lanes"][0]["missing_evidence"], ["pulse-pro:missing.md"])
            self.assertEqual(report["lanes"][0]["derived_status"], "evidence-missing")

    def test_audit_status_payload_cross_checks_lane_subsystems_against_registry(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pulse = root / "pulse"
            pulse.mkdir()
            (pulse / "docs").mkdir()
            (pulse / "docs" / "proof.md").write_text("proof", encoding="utf-8")

            payload = {
                "version": "6.0",
                "updated_at": "2026-03-11",
                "execution_model": "direct-repo-sessions",
                "source_of_truth_file": "docs/release-control/v6/SOURCE_OF_TRUTH.md",
                "scope": {
                    "active_repos": ["pulse"],
                    "ignored_repos": [],
                },
                "source_precedence": [
                    "docs/release-control/v6/SOURCE_OF_TRUTH.md",
                    "docs/release-control/v6/status.json",
                    "docs/release-control/v6/status.schema.json",
                    "docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md",
                    "docs/release-control/v6/subsystems/registry.json",
                ],
                "priority_engine": {
                    "formula": "gap-first",
                    "floor_rule": {
                        "release_critical_lanes": ["L6"],
                        "minimum_score": 6,
                    },
                    "weights": {
                        "gap_multiplier": 4,
                        "criticality_range": "0-5",
                        "staleness_range": "0-3",
                        "dependency_range": "0-3",
                    },
                },
                "evidence_reference_policy": {
                    "format": "repo-qualified-relative-paths",
                    "allowed_kinds": ["file", "dir"],
                    "absolute_paths_forbidden": True,
                    "local_repo": "pulse",
                },
                "lanes": [
                    {
                        "id": "L6",
                        "name": "Architecture coherence",
                        "target_score": 8,
                        "current_score": 7,
                        "status": "partial",
                        "subsystems": ["monitoring"],
                        "evidence": [
                            {"repo": "pulse", "path": "docs/proof.md", "kind": "file"},
                        ],
                    }
                ],
                "open_decisions": [],
                "resolved_decisions": [],
            }

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[
                    {"id": "alerts", "lane": "L6"},
                    {"id": "monitoring", "lane": "L6"},
                ],
            ):
                report = audit_status_payload(payload)

            self.assertTrue(report["errors"])
            self.assertIn(
                "lanes[L6].subsystems = ['monitoring'], want ['alerts', 'monitoring'] from subsystem registry",
                "\n".join(report["errors"]),
            )


if __name__ == "__main__":
    unittest.main()
