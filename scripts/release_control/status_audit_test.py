from __future__ import annotations

import os
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from status_audit import (
    READINESS_ASSERTIONS_BLOCKER,
    RELEASE_GATES_BLOCKER,
    audit_status_payload,
    parse_args,
    render_pretty,
)


REPO_READY_RULE = "all lanes target-met and evidence-present plus all repo-ready assertions passed"
RELEASE_READY_RULE = (
    "repo_ready plus all release-ready assertions passed plus zero open_decisions plus all release_gates passed"
)


def write_file(root: Path, rel: str, content: str = "proof\n") -> None:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def automated_assertion(
    *,
    lane_id: str = "L1",
    evidence_path: str = "docs/proof_test.go",
    proof_commands: list[dict[str, object]] | None = None,
) -> dict[str, object]:
    return {
        "id": "RA1",
        "summary": "Core readiness assertion",
        "kind": "invariant",
        "blocking_level": "repo-ready",
        "proof_type": "automated",
        "lane_ids": [lane_id],
        "subsystem_ids": [],
        "release_gate_ids": [],
        "proof_commands": proof_commands
        if proof_commands is not None
        else [{"id": "ra1", "run": ["python3", "-c", "print('ok')"]}],
        "evidence": [{"repo": "pulse", "path": evidence_path, "kind": "file"}],
    }


def hybrid_assertion(
    *,
    lane_id: str = "L1",
    gate_id: str = "g1",
    evidence_path: str = "docs/hybrid_test.go",
) -> dict[str, object]:
    return {
        "id": "RA2",
        "summary": "Hybrid release assertion",
        "kind": "journey",
        "blocking_level": "release-ready",
        "proof_type": "hybrid",
        "lane_ids": [lane_id],
        "subsystem_ids": [],
        "release_gate_ids": [gate_id],
        "evidence": [{"repo": "pulse", "path": evidence_path, "kind": "file"}],
    }


def base_payload(
    *,
    lane_status: str = "target-met",
    current_score: int = 8,
    release_gate_status: str = "passed",
    readiness_assertions: list[dict[str, object]] | None = None,
    open_decisions: list[dict[str, object]] | None = None,
) -> dict[str, object]:
    return {
        "version": "6.0",
        "updated_at": "2026-03-12",
        "execution_model": "direct-repo-sessions",
        "source_of_truth_file": "docs/release-control/v6/SOURCE_OF_TRUTH.md",
        "readiness": {
            "repo_ready_rule": REPO_READY_RULE,
            "release_ready_rule": RELEASE_READY_RULE,
        },
        "scope": {
            "active_repos": ["pulse"],
            "control_plane_repo": "pulse",
            "ignored_repos": [],
            "repo_catalog": [
                {
                    "id": "pulse",
                    "purpose": "Core repo and control plane.",
                    "visibility": "public",
                }
            ],
        },
        "source_precedence": [
            "docs/release-control/v6/SOURCE_OF_TRUTH.md",
            "docs/release-control/v6/status.json",
            "docs/release-control/v6/status.schema.json",
            "docs/release-control/v6/CANONICAL_DEVELOPMENT_PROTOCOL.md",
            "docs/release-control/v6/subsystems/registry.json",
            "docs/release-control/v6/subsystems/registry.schema.json",
        ],
        "priority_engine": {
            "formula": "gap-first",
            "floor_rule": {
                "release_critical_lanes": ["L1"],
                "minimum_score": 6,
            },
            "weights": {
                "gap_multiplier": 4,
                "blocker_bonus": 8,
                "criticality_range": "0-5",
                "staleness_range": "0-3",
                "dependency_range": "0-3",
            },
        },
        "evidence_reference_policy": {
            "format": "repo-qualified-relative-paths",
            "allowed_kinds": ["dir", "file"],
            "absolute_paths_forbidden": True,
            "local_repo": "pulse",
        },
        "lanes": [
            {
                "id": "L1",
                "name": "Lane 1",
                "target_score": 8,
                "current_score": current_score,
                "status": lane_status,
                "subsystems": [],
                "evidence": [{"repo": "pulse", "path": "docs/lane-proof.md", "kind": "file"}],
            }
        ],
        "readiness_assertions": readiness_assertions or [automated_assertion(), hybrid_assertion()],
        "release_gates": [
            {
                "id": "g1",
                "summary": "Need release verification",
                "owner": "project-owner",
                "status": release_gate_status,
                "verification_doc": "docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md",
                "lane_ids": ["L1"],
            }
        ],
        "open_decisions": open_decisions or [],
        "resolved_decisions": [],
    }


class StatusAuditTest(unittest.TestCase):
    def test_parse_args_accepts_staged_flag(self) -> None:
        args = parse_args(["--check", "--staged"])
        self.assertTrue(args.check)
        self.assertTrue(args.staged)

    def test_audit_status_payload_derives_repo_and_release_readiness(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(base_payload())

            self.assertEqual(report["errors"], [])
            self.assertTrue(report["summary"]["repo_ready"])
            self.assertTrue(report["summary"]["release_ready"])
            self.assertEqual(report["readiness"]["release_blockers"], [])
            self.assertEqual(report["lanes"][0]["derived_status"], "target-met")
            self.assertEqual(report["readiness_assertions"][0]["proof_command_count"], 1)

    def test_release_gate_pending_blocks_release_ready_and_hybrid_assertion(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(base_payload(release_gate_status="pending"))

            self.assertEqual(report["errors"], [])
            self.assertTrue(report["summary"]["repo_ready"])
            self.assertFalse(report["summary"]["release_ready"])
            self.assertEqual(report["readiness"]["release_blockers"], [READINESS_ASSERTIONS_BLOCKER, RELEASE_GATES_BLOCKER])
            self.assertEqual(report["readiness_assertions"][1]["derived_status"], "gates-pending")

    def test_repo_ready_assertion_failure_blocks_repo_ready(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/hybrid_test.go")

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(
                    base_payload(
                        readiness_assertions=[
                            automated_assertion(evidence_path="docs/missing_test.go"),
                            hybrid_assertion(),
                        ]
                    )
                )

            self.assertFalse(report["summary"]["repo_ready"])
            self.assertFalse(report["summary"]["release_ready"])
            self.assertEqual(report["readiness_assertions"][0]["derived_status"], "evidence-missing")
            self.assertIn(READINESS_ASSERTIONS_BLOCKER, report["readiness"]["release_blockers"])

    def test_automated_assertion_requires_proof_commands(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(
                    base_payload(
                        readiness_assertions=[
                            automated_assertion(proof_commands=[]),
                            hybrid_assertion(),
                        ]
                    )
                )

            self.assertIn(
                "readiness_assertions[0] proof_type 'automated' must declare proof_commands",
                report["errors"],
            )

    def test_automated_and_hybrid_assertions_require_executable_proof_artifact(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof.md")
            write_file(pulse, "docs/hybrid.md")

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(
                    base_payload(
                        readiness_assertions=[
                            automated_assertion(evidence_path="docs/proof.md"),
                            hybrid_assertion(evidence_path="docs/hybrid.md"),
                        ]
                    )
                )

            self.assertIn(
                "readiness_assertions[0] proof_type 'automated' must include at least one executable proof artifact",
                report["errors"],
            )
            self.assertIn(
                "readiness_assertions[1] proof_type 'hybrid' must include at least one executable proof artifact",
                report["errors"],
            )

    def test_render_pretty_includes_proof_command_counts_and_release_blockers(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(base_payload(release_gate_status="pending"))

            pretty = render_pretty(report)
            self.assertIn("scope: control_plane=pulse active_repos=pulse", pretty)
            self.assertIn("proof_commands=1", pretty)
            self.assertIn("release_gates:", pretty)
            self.assertIn("release_blockers:", pretty)
            self.assertIn(RELEASE_GATES_BLOCKER, pretty)

    def test_open_decisions_and_release_gates_derive_repo_scope_from_lane_evidence(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            pulse_pro = Path(tmp) / "pulse-pro"
            pulse_pro.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse_pro, "docs/cross-repo-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload(
                open_decisions=[
                    {
                        "id": "d1",
                        "summary": "Cross-repo decision",
                        "owner": "project-owner",
                        "status": "open",
                        "opened_at": "2026-03-12",
                        "lane_ids": ["L1"],
                        "subsystem_ids": [],
                    }
                ]
            )
            payload["scope"] = {
                "active_repos": ["pulse", "pulse-pro"],
                "control_plane_repo": "pulse",
                "ignored_repos": [],
                "repo_catalog": [
                    {
                        "id": "pulse",
                        "purpose": "Core repo and control plane.",
                        "visibility": "public",
                    },
                    {
                        "id": "pulse-pro",
                        "purpose": "Commercial operations repo.",
                        "visibility": "private",
                    },
                ],
            }
            payload["lanes"][0]["evidence"] = [
                {"repo": "pulse", "path": "docs/lane-proof.md", "kind": "file"},
                {"repo": "pulse-pro", "path": "docs/cross-repo-proof.md", "kind": "file"},
            ]

            env = {
                "PULSE_REPO_ROOT_PULSE": str(pulse),
                "PULSE_REPO_ROOT_PULSE_PRO": str(pulse_pro),
            }
            with mock.patch.dict(os.environ, env, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertEqual(report["errors"], [])
            self.assertEqual(report["lanes"][0]["repo_ids"], ["pulse", "pulse-pro"])
            self.assertEqual(report["open_decisions"][0]["repo_ids"], ["pulse", "pulse-pro"])
            self.assertEqual(report["release_gates"][0]["repo_ids"], ["pulse", "pulse-pro"])


if __name__ == "__main__":
    unittest.main()
