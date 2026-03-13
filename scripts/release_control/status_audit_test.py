from __future__ import annotations

import os
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from status_audit import (
    RC_READY_ASSERTIONS_BLOCKER,
    RC_RELEASE_GATES_BLOCKER,
    RELEASE_GATES_BLOCKER,
    RELEASE_READY_ASSERTIONS_BLOCKER,
    REPO_READY_BLOCKER,
    audit_status_payload,
    parse_args,
    render_pretty,
)


REPO_READY_RULE = "all lanes target-met and evidence-present plus all repo-ready assertions passed"
RC_READY_RULE = (
    "repo_ready plus all rc-ready assertions passed plus zero rc-ready open_decisions plus all rc-ready release_gates passed"
)
RELEASE_READY_RULE = (
    "rc_ready plus all release-ready assertions passed plus zero release-ready open_decisions plus all release-ready release_gates passed"
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
    assertion_id: str = "RA2",
    lane_id: str = "L1",
    gate_id: str = "g1",
    evidence_path: str = "docs/hybrid_test.go",
    blocking_level: str = "rc-ready",
    subsystem_ids: list[str] | None = None,
) -> dict[str, object]:
    return {
        "id": assertion_id,
        "summary": "Hybrid release assertion",
        "kind": "journey",
        "blocking_level": blocking_level,
        "proof_type": "hybrid",
        "lane_ids": [lane_id],
        "subsystem_ids": list(subsystem_ids or []),
        "release_gate_ids": [gate_id],
        "evidence": [{"repo": "pulse", "path": evidence_path, "kind": "file"}],
    }


def base_payload(
    *,
    lane_status: str = "target-met",
    current_score: int = 8,
    completion_state: str | None = None,
    completion_summary: str | None = None,
    completion_tracking: list[dict[str, str]] | None = None,
    release_gate_status: str = "passed",
    readiness_assertions: list[dict[str, object]] | None = None,
    lane_followups: list[dict[str, object]] | None = None,
    open_decisions: list[dict[str, object]] | None = None,
) -> dict[str, object]:
    resolved_completion_state = completion_state
    if resolved_completion_state is None:
        resolved_completion_state = "complete" if lane_status == "target-met" else "open"
    resolved_completion_summary = completion_summary
    if resolved_completion_summary is None:
        if resolved_completion_state == "complete":
            resolved_completion_summary = "Lane reached a coherent and complete stop point."
        elif resolved_completion_state == "bounded-residual":
            resolved_completion_summary = (
                "Lane reached the current governed floor and has normalized residual work."
            )
        else:
            resolved_completion_summary = "Lane still requires additional in-scope work."
    resolved_completion_tracking = list(completion_tracking or [])
    return {
        "version": "6.0",
        "updated_at": "2026-03-12",
        "execution_model": "direct-repo-sessions",
        "source_of_truth_file": "docs/release-control/v6/SOURCE_OF_TRUTH.md",
        "readiness": {
            "repo_ready_rule": REPO_READY_RULE,
            "rc_ready_rule": RC_READY_RULE,
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
                "completion": {
                    "state": resolved_completion_state,
                    "summary": resolved_completion_summary,
                    "tracking": resolved_completion_tracking,
                },
                "subsystems": [],
                "evidence": [{"repo": "pulse", "path": "docs/lane-proof.md", "kind": "file"}],
            }
        ],
        "readiness_assertions": readiness_assertions
        or [
            automated_assertion(),
            hybrid_assertion(),
            hybrid_assertion(assertion_id="RA3", gate_id="g2", blocking_level="release-ready"),
        ],
        "release_gates": [
            {
                "id": "g1",
                "summary": "Need release verification",
                "owner": "project-owner",
                "blocking_level": "rc-ready",
                "minimum_evidence_tier": "local-rehearsal",
                "status": release_gate_status,
                "verification_doc": "docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md",
                "lane_ids": ["L1"],
                "evidence": [
                    {
                        "repo": "pulse",
                        "path": "docs/lane-proof.md",
                        "kind": "file",
                        "evidence_tier": "local-rehearsal",
                    }
                ],
            },
            {
                "id": "g2",
                "summary": "Need GA promotion verification",
                "owner": "project-owner",
                "blocking_level": "release-ready",
                "minimum_evidence_tier": "local-rehearsal",
                "status": "passed",
                "verification_doc": "docs/release-control/v6/HIGH_RISK_RELEASE_VERIFICATION_MATRIX.md",
                "lane_ids": ["L1"],
                "evidence": [
                    {
                        "repo": "pulse",
                        "path": "docs/lane-proof.md",
                        "kind": "file",
                        "evidence_tier": "local-rehearsal",
                    }
                ],
            }
        ],
        "lane_followups": lane_followups or [],
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
            self.assertTrue(report["summary"]["rc_ready"])
            self.assertTrue(report["summary"]["release_ready"])
            self.assertEqual(report["readiness"]["rc_blockers"], [])
            self.assertEqual(report["readiness"]["release_blockers"], [])
            self.assertEqual(
                report["readiness"]["rc_blocker_details"],
                {"assertions": [], "open_decisions": [], "release_gates": []},
            )
            self.assertEqual(
                report["readiness"]["release_blocker_details"],
                {"assertions": [], "open_decisions": [], "release_gates": []},
            )
            self.assertEqual(
                report["control_plane"]["active_target"]["blocking_levels"],
                [],
            )
            self.assertEqual(
                report["readiness"]["current_target_blockers"],
                {"assertions": [], "open_decisions": [], "release_gates": []},
            )
            self.assertEqual(report["readiness"]["current_target_workstreams"], [])
            self.assertEqual(report["lanes"][0]["derived_status"], "target-met")
            self.assertEqual(report["lanes"][0]["completion_state"], "complete")
            self.assertEqual(report["readiness_assertions"][0]["proof_command_count"], 1)

    def test_rc_ready_gate_pending_blocks_rc_ready_and_release_ready(self) -> None:
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
            self.assertFalse(report["summary"]["rc_ready"])
            self.assertFalse(report["summary"]["release_ready"])
            self.assertEqual(
                report["readiness"]["rc_blockers"],
                [RC_READY_ASSERTIONS_BLOCKER, RC_RELEASE_GATES_BLOCKER],
            )
            self.assertEqual(
                report["readiness"]["release_blockers"],
                [RC_READY_ASSERTIONS_BLOCKER, RC_RELEASE_GATES_BLOCKER],
            )
            self.assertEqual(report["readiness_assertions"][1]["derived_status"], "gates-pending")
            self.assertEqual(
                [item["id"] for item in report["readiness"]["rc_blocker_details"]["assertions"]],
                ["RA2"],
            )
            self.assertEqual(
                [item["id"] for item in report["readiness"]["rc_blocker_details"]["release_gates"]],
                ["g1"],
            )
            self.assertEqual(
                [item["id"] for item in report["readiness"]["release_blocker_details"]["assertions"]],
                ["RA2"],
            )
            self.assertEqual(
                [item["id"] for item in report["readiness"]["release_blocker_details"]["release_gates"]],
                ["g1"],
            )
            self.assertEqual(
                report["readiness"]["current_target_blockers"],
                {"assertions": [], "open_decisions": [], "release_gates": []},
            )
            self.assertEqual(report["readiness"]["current_target_workstreams"], [])

    def test_release_ready_gate_pending_blocks_only_release_ready(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload()
            payload["release_gates"][1]["status"] = "pending"

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertEqual(report["errors"], [])
            self.assertTrue(report["summary"]["repo_ready"])
            self.assertTrue(report["summary"]["rc_ready"])
            self.assertFalse(report["summary"]["release_ready"])
            self.assertEqual(report["readiness"]["rc_blockers"], [])
            self.assertEqual(
                report["readiness"]["release_blockers"],
                [RELEASE_READY_ASSERTIONS_BLOCKER, RELEASE_GATES_BLOCKER],
            )
            self.assertEqual(report["readiness_assertions"][2]["derived_status"], "gates-pending")
            self.assertEqual(
                report["readiness"]["rc_blocker_details"],
                {"assertions": [], "open_decisions": [], "release_gates": []},
            )
            self.assertEqual(
                [item["id"] for item in report["readiness"]["release_blocker_details"]["assertions"]],
                ["RA3"],
            )
            self.assertEqual(
                [item["id"] for item in report["readiness"]["release_blocker_details"]["release_gates"]],
                ["g2"],
            )
            self.assertEqual(
                report["readiness"]["current_target_blockers"],
                {"assertions": [], "open_decisions": [], "release_gates": []},
            )
            self.assertEqual(report["readiness"]["current_target_workstreams"], [])

    def test_passed_gate_without_required_evidence_tier_stays_blocking(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload()
            payload["release_gates"][0]["minimum_evidence_tier"] = "real-external-e2e"

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertEqual(report["errors"], [])
            self.assertFalse(report["summary"]["rc_ready"])
            self.assertFalse(report["summary"]["release_ready"])
            self.assertEqual(report["release_gates"][0]["status"], "passed")
            self.assertEqual(report["release_gates"][0]["effective_status"], "threshold-unmet")
            self.assertEqual(report["release_gates"][0]["highest_evidence_tier"], "local-rehearsal")
            self.assertEqual(report["summary"]["overclosed_release_gate_count"], 1)
            self.assertEqual(
                [item["id"] for item in report["readiness"]["overclosed_release_gates"]],
                ["g1"],
            )
            self.assertEqual(report["readiness_assertions"][1]["derived_status"], "gates-pending")
            self.assertEqual(
                [item["id"] for item in report["readiness"]["rc_blocker_details"]["release_gates"]],
                ["g1"],
            )
            pretty = render_pretty(report)
            self.assertIn("overclosed_release_gates=1", pretty)
            self.assertIn("overclosed_release_gates:", pretty)
            self.assertIn("raw_status=passed", pretty)
            self.assertIn("effective=threshold-unmet", pretty)
            self.assertIn("min_tier=real-external-e2e", pretty)

    def test_current_target_blockers_include_subsystem_contract_context(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            rules = [
                {
                    "id": "frontend-primitives",
                    "lane": "L1",
                    "contract": "docs/release-control/v6/subsystems/frontend-primitives.md",
                    "verification": {
                        "allow_same_subsystem_tests": False,
                        "test_prefixes": [],
                        "exact_files": [],
                        "require_explicit_path_policy_coverage": False,
                        "path_policies": [],
                    },
                }
            ]

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=rules,
            ):
                payload = base_payload(
                    release_gate_status="pending",
                    readiness_assertions=[
                        automated_assertion(),
                        hybrid_assertion(subsystem_ids=["frontend-primitives"]),
                        hybrid_assertion(assertion_id="RA3", gate_id="g2", blocking_level="release-ready"),
                    ],
                )
                payload["lanes"][0]["subsystems"] = ["frontend-primitives"]
                report = audit_status_payload(
                    payload
                )

            self.assertEqual(report["errors"], [])
            self.assertEqual(
                report["readiness"]["current_target_blockers"],
                {"assertions": [], "open_decisions": [], "release_gates": []},
            )
            self.assertEqual(report["readiness"]["current_target_workstreams"], [])
            pretty = render_pretty(report)
            self.assertNotIn("current_target_workstreams:", pretty)
            self.assertNotIn("active target proof scope could not be derived", pretty)

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
                            hybrid_assertion(assertion_id="RA3", gate_id="g2", blocking_level="release-ready"),
                        ]
                    )
                )

            self.assertFalse(report["summary"]["repo_ready"])
            self.assertFalse(report["summary"]["rc_ready"])
            self.assertFalse(report["summary"]["release_ready"])
            self.assertEqual(report["readiness_assertions"][0]["derived_status"], "evidence-missing")
            self.assertEqual(report["readiness"]["rc_blockers"], [REPO_READY_BLOCKER])
            self.assertEqual(report["readiness"]["release_blockers"], [REPO_READY_BLOCKER])
            self.assertEqual(
                report["readiness"]["rc_blocker_details"],
                {"assertions": [], "open_decisions": [], "release_gates": []},
            )
            self.assertEqual(
                report["readiness"]["release_blocker_details"],
                {"assertions": [], "open_decisions": [], "release_gates": []},
            )

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

    def test_python_test_files_count_as_executable_proof_artifacts(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.py")
            write_file(pulse, "docs/hybrid_test.py")

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(
                    base_payload(
                        readiness_assertions=[
                            automated_assertion(evidence_path="docs/proof_test.py"),
                            hybrid_assertion(evidence_path="docs/hybrid_test.py"),
                        ]
                    )
                )

            self.assertEqual(report["errors"], [])

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
            self.assertIn("control_plane: profile=v6 target=v6-rc-stabilization", pretty)
            self.assertIn("scope: control_plane=pulse active_repos=pulse", pretty)
            self.assertIn("rc_ready=False", pretty)
            self.assertIn("completion=complete", pretty)
            self.assertIn("proof_commands=1", pretty)
            self.assertIn("overclosed_release_gates=0", pretty)
            self.assertIn("release_gates:", pretty)
            self.assertIn("effective=pending", pretty)
            self.assertNotIn("lane_residuals:", pretty)
            self.assertNotIn("overclosed_release_gates:", pretty)
            self.assertIn("current_target_blockers:", pretty)
            self.assertNotIn("current_target_workstreams:", pretty)
            self.assertIn("rc_blockers:", pretty)
            self.assertIn("rc_blocker_details:", pretty)
            self.assertIn("release_blockers:", pretty)
            self.assertIn("release_blocker_details:", pretty)
            self.assertIn(RC_RELEASE_GATES_BLOCKER, pretty)
            self.assertNotIn("target_blocking_levels=", pretty)
            self.assertNotIn("active target proof scope could not be derived", pretty)

    def test_stable_version_on_rc_hold_emits_warning(self) -> None:
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
                    base_payload(release_gate_status="pending"),
                    current_version="6.0.0",
                )

            self.assertIn(
                "VERSION is a stable release string while the active target is still a non-GA v6-rc-stabilization and release_ready is false; the repo is carrying a GA candidate on an RC-held line.",
                report["warnings"],
            )
            pretty = render_pretty(report)
            self.assertIn("current_version=6.0.0", pretty)
            self.assertIn("repo is carrying a GA candidate on an RC-held line", pretty)

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
                        "blocking_level": "rc-ready",
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

    def test_partial_lane_at_target_requires_bounded_residual_completion(self) -> None:
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
                        lane_status="partial",
                        current_score=8,
                        completion_state="open",
                    )
                )

            self.assertIn(
                "lanes[0] partial lanes that already meet target_score must use completion.state='bounded-residual'",
                report["errors"],
            )

    def test_bounded_residual_lane_requires_known_tracking_reference(self) -> None:
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
                        lane_status="partial",
                        current_score=8,
                        completion_state="bounded-residual",
                        completion_tracking=[{"kind": "target", "id": "missing-target"}],
                    )
                )

            self.assertIn(
                "lanes[L1].completion.tracking references unknown target 'missing-target'",
                report["errors"],
            )

    def test_bounded_residual_lane_rejects_unrelated_release_gate_reference(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload(
                lane_status="partial",
                current_score=8,
                completion_state="bounded-residual",
                completion_tracking=[{"kind": "release-gate", "id": "g1"}],
            )
            payload["lanes"].append(
                {
                    "id": "L2",
                    "name": "Lane 2",
                    "target_score": 8,
                    "current_score": 8,
                    "status": "partial",
                    "completion": {
                        "state": "bounded-residual",
                        "summary": "Second lane still has governed residual work.",
                        "tracking": [{"kind": "release-gate", "id": "g2"}],
                    },
                    "subsystems": [],
                    "evidence": [{"repo": "pulse", "path": "docs/lane-proof.md", "kind": "file"}],
                }
            )
            payload["release_gates"][0]["lane_ids"] = ["L2"]
            payload["release_gates"][1]["lane_ids"] = ["L2"]
            payload["priority_engine"]["floor_rule"]["release_critical_lanes"] = ["L1", "L2"]

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertIn(
                "lanes[L1].completion.tracking release gate 'g1' does not reference that lane",
                report["errors"],
            )

    def test_bounded_residual_lane_rejects_unrelated_open_decision_reference(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload(
                lane_status="partial",
                current_score=8,
                completion_state="bounded-residual",
                completion_tracking=[{"kind": "open-decision", "id": "d1"}],
                open_decisions=[
                    {
                        "id": "d1",
                        "summary": "Open decision owned by another lane",
                        "owner": "project-owner",
                        "blocking_level": "rc-ready",
                        "status": "open",
                        "opened_at": "2026-03-12",
                        "lane_ids": ["L2"],
                        "subsystem_ids": [],
                    }
                ],
            )
            payload["lanes"].append(
                {
                    "id": "L2",
                    "name": "Lane 2",
                    "target_score": 8,
                    "current_score": 4,
                    "status": "partial",
                    "completion": {
                        "state": "open",
                        "summary": "Second lane is still open.",
                        "tracking": [],
                    },
                    "subsystems": [],
                    "evidence": [{"repo": "pulse", "path": "docs/lane-proof.md", "kind": "file"}],
                }
            )
            payload["priority_engine"]["floor_rule"]["release_critical_lanes"] = ["L1", "L2"]

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertIn(
                "lanes[L1].completion.tracking open decision 'd1' does not reference that lane",
                report["errors"],
            )

    def test_bounded_residual_lane_rejects_unrelated_assertion_reference(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload(
                lane_status="partial",
                current_score=8,
                completion_state="bounded-residual",
                completion_tracking=[{"kind": "readiness-assertion", "id": "RA2"}],
                readiness_assertions=[
                    automated_assertion(),
                    hybrid_assertion(assertion_id="RA2", lane_id="L2"),
                    hybrid_assertion(assertion_id="RA3", lane_id="L2", gate_id="g2", blocking_level="release-ready"),
                ],
            )
            payload["lanes"].append(
                {
                    "id": "L2",
                    "name": "Lane 2",
                    "target_score": 8,
                    "current_score": 8,
                    "status": "partial",
                    "completion": {
                        "state": "bounded-residual",
                        "summary": "Second lane still has governed residual work.",
                        "tracking": [{"kind": "release-gate", "id": "g1"}],
                    },
                    "subsystems": [],
                    "evidence": [{"repo": "pulse", "path": "docs/lane-proof.md", "kind": "file"}],
                }
            )
            payload["release_gates"][0]["lane_ids"] = ["L2"]
            payload["release_gates"][1]["lane_ids"] = ["L2"]
            payload["priority_engine"]["floor_rule"]["release_critical_lanes"] = ["L1", "L2"]

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertIn(
                "lanes[L1].completion.tracking readiness assertion 'RA2' does not reference that lane",
                report["errors"],
            )

    def test_bounded_residual_lane_rejects_resolved_tracking_reference(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload(
                lane_status="partial",
                current_score=8,
                completion_state="bounded-residual",
                completion_tracking=[{"kind": "release-gate", "id": "g1"}],
                release_gate_status="passed",
            )

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertIn(
                "lanes[L1].completion.tracking release gate 'g1' is already resolved and cannot keep a bounded residual open",
                report["errors"],
            )

    def test_bounded_residual_lane_accepts_lane_followup_tracking(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload(
                lane_status="partial",
                current_score=8,
                completion_state="bounded-residual",
                completion_summary="Lane still has a named non-blocking same-lane follow-up.",
                completion_tracking=[{"kind": "lane-followup", "id": "mobile-post-rc-hardening"}],
                lane_followups=[
                    {
                        "id": "mobile-post-rc-hardening",
                        "summary": "Track post-RC hardening for the lane.",
                        "owner": "project-owner",
                        "status": "planned",
                        "recorded_at": "2026-03-13",
                        "lane_ids": ["L1"],
                        "subsystem_ids": [],
                    }
                ],
            )

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertEqual(report["errors"], [])
            self.assertEqual(
                report["readiness"]["lane_residuals"][0]["tracking"],
                ["lane-followup:mobile-post-rc-hardening[planned]"],
            )

    def test_bounded_residual_lane_rejects_resolved_lane_followup_reference(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload(
                lane_status="partial",
                current_score=8,
                completion_state="bounded-residual",
                completion_tracking=[{"kind": "lane-followup", "id": "mobile-post-rc-hardening"}],
                lane_followups=[
                    {
                        "id": "mobile-post-rc-hardening",
                        "summary": "Track post-RC hardening for the lane.",
                        "owner": "project-owner",
                        "status": "done",
                        "recorded_at": "2026-03-13",
                        "lane_ids": ["L1"],
                        "subsystem_ids": [],
                    }
                ],
            )

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertIn(
                "lanes[L1].completion.tracking lane followup 'mobile-post-rc-hardening' is already resolved and cannot keep a bounded residual open",
                report["errors"],
            )

    def test_lane_followup_must_be_referenced_by_bounded_residual_lane(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload(
                lane_status="partial",
                current_score=8,
                completion_state="bounded-residual",
                completion_tracking=[{"kind": "release-gate", "id": "g1"}],
                lane_followups=[
                    {
                        "id": "mobile-post-rc-hardening",
                        "summary": "Track post-RC hardening for the lane.",
                        "owner": "project-owner",
                        "status": "planned",
                        "recorded_at": "2026-03-13",
                        "lane_ids": ["L1"],
                        "subsystem_ids": [],
                    }
                ],
            )

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertIn(
                "lane_followups[mobile-post-rc-hardening] is not referenced by any bounded-residual lane completion.tracking",
                report["errors"],
            )

    def test_bounded_residual_lanes_appear_in_readiness_summary_and_pretty_output(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload(
                lane_status="partial",
                current_score=8,
                completion_state="bounded-residual",
                completion_summary="Lane still has a governed residual.",
                completion_tracking=[{"kind": "lane-followup", "id": "lane-1-followup"}],
                lane_followups=[
                    {
                        "id": "lane-1-followup",
                        "summary": "Track the remaining named lane follow-up.",
                        "owner": "project-owner",
                        "status": "planned",
                        "recorded_at": "2026-03-13",
                        "lane_ids": ["L1"],
                        "subsystem_ids": [],
                    }
                ],
            )

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertEqual(report["errors"], [])
            self.assertEqual(
                report["readiness"]["lane_residuals"],
                [
                    {
                        "lane_id": "L1",
                        "lane_name": "Lane 1",
                        "summary": "Lane still has a governed residual.",
                        "tracking": ["lane-followup:lane-1-followup[planned]"],
                        "tracking_details": [
                            {
                                "kind": "lane-followup",
                                "id": "lane-1-followup",
                                "status": "planned",
                                "resolved": False,
                                "summary": "Track the remaining named lane follow-up.",
                            }
                        ],
                        "unresolved_tracking_count": 1,
                        "repo_ids": ["pulse"],
                        "subsystem_ids": [],
                    }
                ],
            )
            pretty = render_pretty(report)
            self.assertIn("lane_residuals:", pretty)
            self.assertIn("L1 unresolved=1 tracking=lane-followup:lane-1-followup[planned]", pretty)
            self.assertIn("Lane still has a governed residual.", pretty)

    def test_target_only_bounded_residual_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload(
                lane_status="partial",
                current_score=8,
                completion_state="bounded-residual",
                completion_summary="Lane is at floor but only broader GA work is currently tracked.",
                completion_tracking=[{"kind": "target", "id": "v6-rc-stabilization"}],
            )

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertIn(
                "lanes[0].completion.tracking kind 'target' is not supported",
                report["errors"],
            )

    def test_mixed_target_and_concrete_bounded_residual_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse = Path(tmp) / "pulse"
            pulse.mkdir()
            write_file(pulse, "docs/lane-proof.md")
            write_file(pulse, "docs/proof_test.go")
            write_file(pulse, "docs/hybrid_test.go")

            payload = base_payload(
                lane_status="partial",
                current_score=8,
                completion_state="bounded-residual",
                completion_summary="Lane is at floor and has both a concrete gate and a leftover broad target fallback.",
                completion_tracking=[
                    {"kind": "release-gate", "id": "g1"},
                    {"kind": "target", "id": "v6-rc-stabilization"},
                ],
                release_gate_status="pending",
            )

            with mock.patch.dict(os.environ, {"PULSE_REPO_ROOT_PULSE": str(pulse)}, clear=False), mock.patch(
                "status_audit.load_subsystem_rules",
                return_value=[],
            ):
                report = audit_status_payload(payload)

            self.assertIn(
                "lanes[0].completion.tracking kind 'target' is not supported",
                report["errors"],
            )


if __name__ == "__main__":
    unittest.main()
