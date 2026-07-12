#!/usr/bin/env python3
"""Deterministic tests for the Pulse Intelligence aggregate release gate."""

from __future__ import annotations

import json
import re
import unittest
from pathlib import Path

import pulse_intelligence_gate as gate


SHA = "1" * 40
NEXT_SHA = "2" * 40
ANCESTOR_SHA = "3" * 40


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
        "schema_version": 2,
        "matrix_id": "pulse-intelligence-rg-01-rg-12",
        "program": "pulse-intelligence",
        "result_semantics": {
            "PASS": "External evidence satisfies the gate at the audited Git SHA.",
            "FAIL": "External evidence bound to the audited Git SHA records a failed proof or an invalid binding.",
            "NOT_RUN": "No qualifying external evidence exists for the gate at the audited Git SHA.",
        },
        "evidence_tiers": list(gate.EVIDENCE_TIERS),
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
                "unauthorized_mutations": 0,
                "transport_dispatches": 0,
                "authority_writes": 0,
            }
        records.append(record)
    return {
        "schema_version": 1,
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

    def test_repository_matrix_schema_is_deterministic_and_requirement_only(self) -> None:
        matrix_path = Path(__file__).resolve().parents[2] / "docs/release-control/v6/internal/pulse-intelligence-release-gate.json"
        matrix_text = matrix_path.read_text(encoding="utf-8")
        raw = json.loads(matrix_text)
        first = gate.validate_matrix(raw)
        second = gate.validate_matrix(json.loads(json.dumps(raw, sort_keys=True)))
        self.assertEqual(first, second)
        self.assertNotIn("evaluation_git_sha", first)
        self.assertNotIn("overall_result", first)
        self.assertIsNone(re.search(r"\b[0-9a-f]{40}\b", matrix_text))
        for item in first["gates"]:
            self.assertNotIn("result", item)
            self.assertNotIn("evidence", item)


if __name__ == "__main__":
    unittest.main()
