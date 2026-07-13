#!/usr/bin/env python3
"""Deterministic tests for the Pulse Intelligence aggregate release gate."""

from __future__ import annotations

import json
import hashlib
import re
import tempfile
import unittest
from pathlib import Path

import pulse_intelligence_gate as gate


SHA = "1" * 40
NEXT_SHA = "2" * 40
ANCESTOR_SHA = "3" * 40
MOBILE_SHA = "4" * 40


def write_json(path: Path, payload: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")


def create_rg12_artifact(root: Path, audited_sha: str) -> dict[str, str]:
    action_ids = ["approve-action", "deny-action"]
    environment = {
        "core_sha": audited_sha,
        "mobile_sha": MOBILE_SHA,
        "action_ids": action_ids,
        "revoked_barrier_action_id": "revoked-barrier",
        "temporary_token_record_id": "temporary-token",
        "credential_material_in_bundle": False,
    }
    write_json(root / "environment.json", environment)
    write_json(
        root / "correlation.json",
        {
            "core_sha": audited_sha,
            "mobile_sha": MOBILE_SHA,
            "action_ids": action_ids,
            "token_record_id": "temporary-token",
            "target_type": "test",
            "executor": "none",
            "physical_proof_sequence": list(gate.RG12_PROOF_NAMES),
        },
    )

    def decision(action_id: str, state: str, outcome: str) -> dict[str, object]:
        return {
            "actionId": action_id,
            "state": state,
            "execute_endpoint_called": False,
            "approval": {"outcome": outcome},
            "audit": {"request": {"params": {"target_type": "test", "executor": "none"}}},
        }

    write_json(
        root / "canonical-action-api.json",
        {
            "approve_http_status": 200,
            "deny_http_status": 200,
            "matching_pending_after_count": 0,
            "approve": decision(action_ids[0], "approved", "approved"),
            "deny": decision(action_ids[1], "rejected", "rejected"),
        },
    )
    write_json(
        root / "action-store-observations.json",
        {
            "records": [
                {"id": action_ids[0], "state": "approved", "target_type": "test", "executor": "none"},
                {"id": action_ids[1], "state": "rejected", "target_type": "test", "executor": "none"},
                {"id": "revoked-barrier", "state": "pending_approval", "target_type": "test", "executor": "none"},
            ],
            "dispatch_attempts": 0,
            "dispatch_outbox": 0,
            "dispatch_receipts": 0,
        },
    )
    barrier = {"id": "revoked-barrier", "state": "pending_approval", "decision_revision": 0}
    write_json(
        root / "revocation-negative-barrier.json",
        {
            "token_delete_http_status": 204,
            "post_revoke_pending_http_status": 401,
            "post_revoke_decision_http_status": 401,
            "admin_bypass_enabled": False,
            "before": [barrier],
            "after": [barrier],
            "triple_zero": {field: 0 for field in gate.TRIPLE_ZERO_FIELDS},
            "transport_residue": {"dispatch_outbox": 0, "dispatch_receipts": 0},
        },
    )
    write_json(
        root / "cleanup.json",
        {
            "relay_after": [
                {"surface": "instances", "temp_count": 0},
                {"surface": "device_tokens", "temp_count": 0},
                {"surface": "connection_log", "temp_count": 0},
                {"surface": "original_preserved", "temp_count": 1},
            ],
            "local_action_store_after": [[{"temp_action_audits": 0}], [{"temp_events": 0}]],
            "temporary_api_token_present_after": False,
            "local_environment_restored_from_pre_run_backup": True,
            "temporary_license_activation_files_present_after": False,
            "ipad_local_pairing_removed": True,
        },
    )

    digest_records: list[dict[str, object]] = []
    for proof_name in gate.RG12_PROOF_NAMES:
        proof_dir = root / "mobile-proofs" / proof_name
        write_json(
            proof_dir / "test-summary.json",
            {"result": "Passed", "totalTestCount": 1, "failedTests": []},
        )
        (proof_dir / "attachments").mkdir(parents=True)
        (proof_dir / "attachments" / "proof.png").write_bytes(b"png")
        source = root.parent / f"{proof_name}.xcresult"
        source.mkdir()
        (source / "member.bin").write_bytes(proof_name.encode())
        member_digest = hashlib.sha256(proof_name.encode()).hexdigest()
        line = f"{member_digest}  member.bin\n"
        digest_records.append(
            {
                "proof": proof_name,
                "source": str(source),
                "file_count": 1,
                "aggregate_sha256": hashlib.sha256(line.encode()).hexdigest(),
                "members": [{"path": "member.bin", "sha256": member_digest}],
            }
        )
    write_json(root / "xcresult-digests.json", digest_records)
    files = sorted(candidate for candidate in root.rglob("*") if candidate.is_file())
    lines = [f"{hashlib.sha256(candidate.read_bytes()).hexdigest()}  {candidate.relative_to(root)}" for candidate in files]
    manifest = root / "SHA256SUMS"
    manifest.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return {
        "contract": gate.RG12_ARTIFACT_CONTRACT,
        "path": str(root),
        "sha256sums_sha256": hashlib.sha256(manifest.read_bytes()).hexdigest(),
        "mobile_git_sha": MOBILE_SHA,
    }


def valid_matrix() -> dict[str, object]:
    gates: list[dict[str, object]] = []
    for gate_id in gate.GATE_IDS:
        gates.append(
            {
                "id": gate_id,
                "title": f"Test gate {gate_id}",
                "owner": gate.APPROVED_OWNERS[gate_id],
                "required_evidence_tier": "integration",
                "requires_triple_zero": True,
                "allows_ancestor_provenance": gate_id == "RG-08",
                "non_mutating_proof_commands": [
                    {"id": f"{gate_id.lower()}-proof", "argv": ["go", "test", "./internal/example"]}
                ],
                "mutation_gated_commands": [],
            }
        )
    return {
        "schema_version": 3,
        "matrix_id": "pulse-intelligence-rg-01-rg-12",
        "program": "pulse-intelligence",
        "result_semantics": {
            "PASS": "External evidence satisfies the gate at the audited Git SHA.",
            "FAIL": "External evidence bound to the audited Git SHA records a failed proof or an invalid binding.",
            "NOT_RUN": "No qualifying external evidence exists for the gate at the audited Git SHA.",
        },
        "evidence_tiers": list(gate.EVIDENCE_TIERS),
        "triple_zero_semantics": dict(gate.TRIPLE_ZERO_SEMANTICS),
        "gates": gates,
    }


def evidence_for(matrix: dict[str, object], audited_sha: str) -> dict[str, object]:
    records: list[dict[str, object]] = []
    kind_for_tier = {
        "unit": "source",
        "integration": "automated-test",
        "browser": "browser-artifact",
        "real-lab": "real-lab-artifact",
        "physical-device": "physical-device-artifact",
        "live-relay": "live-relay-artifact",
    }
    for item in matrix["gates"]:
        record: dict[str, object] = {
            "id": f"{item['id'].lower()}-evidence",
            "gate_id": item["id"],
            "result": "PASS",
            "tier": item["required_evidence_tier"],
            "kind": kind_for_tier[item["required_evidence_tier"]],
            "git_sha": audited_sha,
            "command_id": item["non_mutating_proof_commands"][0]["id"],
        }
        if item["requires_triple_zero"]:
            record["triple_zero"] = {
                "persistence_writes": 0,
                "dispatch_attempts": 0,
                "external_mutations": 0,
            }
        records.append(record)
    return {
        "schema_version": 2,
        "matrix_id": matrix["matrix_id"],
        "audited_git_sha": audited_sha,
        "records": records,
    }


class PulseIntelligenceGateTest(unittest.TestCase):
    def test_malformed_matrix_is_rejected(self) -> None:
        payload = valid_matrix()
        del payload["result_semantics"]
        with self.assertRaisesRegex(gate.MatrixError, "missing fields: result_semantics"):
            gate.validate_matrix(payload)

    def test_duplicate_gate_is_rejected(self) -> None:
        payload = valid_matrix()
        payload["gates"][1]["id"] = "RG-01"
        payload["gates"][1]["owner"] = "task-01"
        with self.assertRaisesRegex(gate.MatrixError, "duplicate gate ids"):
            gate.validate_matrix(payload)

    def test_wrong_owner_is_rejected(self) -> None:
        payload = valid_matrix()
        payload["gates"][4]["owner"] = "task-12"
        with self.assertRaisesRegex(gate.MatrixError, "RG-05 owner must be 'task-05'"):
            gate.validate_matrix(payload)

    def test_over_tier_evidence_is_rejected(self) -> None:
        matrix = gate.validate_matrix(valid_matrix())
        payload = evidence_for(matrix, SHA)
        payload["records"][0]["kind"] = "mock"
        payload["records"][0]["tier"] = "real-lab"
        with self.assertRaisesRegex(gate.MatrixError, "overstates 'mock' as 'real-lab'"):
            gate.validate_evidence(payload, matrix)

    def test_external_exact_sha_evidence_can_certify_go(self) -> None:
        matrix = gate.validate_matrix(valid_matrix())
        evidence = gate.validate_evidence(evidence_for(matrix, SHA), matrix)
        evaluation = gate.evaluate_matrix(matrix, SHA, evidence)
        self.assertEqual(evaluation.verdict, "GO")
        self.assertEqual(set(evaluation.gate_results.values()), {"PASS"})

    def test_committing_matrix_does_not_inherently_stale_it(self) -> None:
        matrix = gate.validate_matrix(valid_matrix())
        self.assertNotIn("evaluation_git_sha", matrix)
        for audited_sha in (SHA, NEXT_SHA):
            with self.subTest(audited_sha=audited_sha):
                evidence = gate.validate_evidence(evidence_for(matrix, audited_sha), matrix)
                self.assertEqual(gate.evaluate_matrix(matrix, audited_sha, evidence).verdict, "GO")

    def test_ancestor_evidence_is_visible_but_non_qualifying(self) -> None:
        matrix = gate.validate_matrix(valid_matrix())
        payload = evidence_for(matrix, SHA)
        payload["records"][7]["git_sha"] = ANCESTOR_SHA
        evidence = gate.validate_evidence(payload, matrix)
        evaluation = gate.evaluate_matrix(
            matrix,
            SHA,
            evidence,
            is_ancestor=lambda older, newer: (older, newer) == (ANCESTOR_SHA, SHA),
        )
        self.assertEqual(evaluation.gate_results["RG-08"], "NOT_RUN")
        self.assertEqual(evaluation.verdict, "NO-GO")
        self.assertTrue(any("RG-08 provenance" in detail and "does not certify" in detail for detail in evaluation.details))

    def test_wrong_sha_external_evidence_fails(self) -> None:
        matrix = gate.validate_matrix(valid_matrix())
        payload = evidence_for(matrix, SHA)
        payload["records"][0]["git_sha"] = NEXT_SHA
        evidence = gate.validate_evidence(payload, matrix)
        evaluation = gate.evaluate_matrix(matrix, SHA, evidence, is_ancestor=lambda _older, _newer: False)
        self.assertEqual(evaluation.gate_results["RG-01"], "FAIL")
        self.assertTrue(any("bound to wrong SHA" in detail for detail in evaluation.details))

    def test_wrong_external_document_binding_fails(self) -> None:
        matrix = gate.validate_matrix(valid_matrix())
        evidence = gate.validate_evidence(evidence_for(matrix, NEXT_SHA), matrix)
        evaluation = gate.evaluate_matrix(matrix, SHA, evidence)
        self.assertEqual(set(evaluation.gate_results.values()), {"FAIL"})
        self.assertIn("not audited SHA", evaluation.details[0])

    def test_missing_external_evidence_is_honest_not_run(self) -> None:
        matrix = gate.validate_matrix(valid_matrix())
        evaluation = gate.evaluate_matrix(matrix, SHA, None)
        self.assertEqual(evaluation.verdict, "NO-GO")
        self.assertEqual(set(evaluation.gate_results.values()), {"NOT_RUN"})
        self.assertEqual(evaluation.details, ())

    def test_missing_triple_zero_fails_applicable_gate(self) -> None:
        matrix = gate.validate_matrix(valid_matrix())
        payload = evidence_for(matrix, SHA)
        del payload["records"][0]["triple_zero"]
        evidence = gate.validate_evidence(payload, matrix)
        evaluation = gate.evaluate_matrix(matrix, SHA, evidence)
        self.assertEqual(evaluation.gate_results["RG-01"], "FAIL")
        self.assertTrue(any("missing required triple-zero" in detail for detail in evaluation.details))

    def test_live_relay_rg12_requires_semantically_valid_sealed_artifact(self) -> None:
        payload = valid_matrix()
        rg12 = payload["gates"][11]
        rg12["required_evidence_tier"] = "live-relay"
        rg12["artifact_contract"] = gate.RG12_ARTIFACT_CONTRACT
        matrix = gate.validate_matrix(payload)
        evidence_payload = evidence_for(matrix, SHA)
        evidence_payload["records"][11]["tier"] = "live-relay"
        evidence_payload["records"][11]["kind"] = "live-relay-artifact"

        missing = gate.validate_evidence(evidence_payload, matrix)
        missing_evaluation = gate.evaluate_matrix(matrix, SHA, missing)
        self.assertEqual(missing_evaluation.gate_results["RG-12"], "FAIL")
        self.assertTrue(any("missing the required sealed artifact descriptor" in detail for detail in missing_evaluation.details))

        with tempfile.TemporaryDirectory() as temp_dir:
            artifact = create_rg12_artifact(Path(temp_dir) / "sealed", SHA)
            evidence_payload["records"][11]["artifact"] = artifact
            evidence = gate.validate_evidence(evidence_payload, matrix)
            evaluation = gate.evaluate_matrix(matrix, SHA, evidence)
            self.assertEqual(evaluation.gate_results["RG-12"], "PASS")
            self.assertEqual(evaluation.verdict, "GO")

    def test_rg12_artifact_checksum_tampering_fails_closed(self) -> None:
        payload = valid_matrix()
        rg12 = payload["gates"][11]
        rg12["required_evidence_tier"] = "live-relay"
        rg12["artifact_contract"] = gate.RG12_ARTIFACT_CONTRACT
        matrix = gate.validate_matrix(payload)
        evidence_payload = evidence_for(matrix, SHA)
        evidence_payload["records"][11]["tier"] = "live-relay"
        evidence_payload["records"][11]["kind"] = "live-relay-artifact"
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir) / "sealed"
            evidence_payload["records"][11]["artifact"] = create_rg12_artifact(root, SHA)
            (root / "cleanup.json").write_text("{}\n", encoding="utf-8")
            evidence = gate.validate_evidence(evidence_payload, matrix)
            evaluation = gate.evaluate_matrix(matrix, SHA, evidence)
            self.assertEqual(evaluation.gate_results["RG-12"], "FAIL")
            self.assertTrue(any("checksum mismatch" in detail for detail in evaluation.details))

    def test_repository_matrix_schema_is_deterministic_and_requirement_only(self) -> None:
        matrix_path = Path(__file__).resolve().parents[2] / "docs/release-control/v6/internal/pulse-intelligence-release-gate.json"
        matrix_text = matrix_path.read_text(encoding="utf-8")
        raw = json.loads(matrix_text)
        first = gate.validate_matrix(raw)
        second = gate.validate_matrix(json.loads(json.dumps(raw, sort_keys=True)))
        self.assertEqual(first, second)
        self.assertNotIn("evaluation_git_sha", first)
        self.assertNotIn("overall_result", first)
        self.assertEqual(first["triple_zero_semantics"], gate.TRIPLE_ZERO_SEMANTICS)
        self.assertIsNone(re.search(r"\b[0-9a-f]{40}\b", matrix_text))
        for item in first["gates"]:
            self.assertNotIn("result", item)
            self.assertNotIn("evidence", item)
        self.assertEqual(first["gates"][11]["artifact_contract"], gate.RG12_ARTIFACT_CONTRACT)


if __name__ == "__main__":
    unittest.main()
