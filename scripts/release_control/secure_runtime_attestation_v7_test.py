#!/usr/bin/env python3
"""Focused schema-v7 secure-runtime attestation contract tests."""

from __future__ import annotations

import copy
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import secure_runtime_attestation as v5
import secure_runtime_attestation_v6 as v6
import secure_runtime_attestation_v7 as v7


DIGEST = "1" * 64


def valid_receipt() -> dict:
    observations = {
        "rootful_docker_summary_migration": {
            "legacy_container_count": 2,
            "migrated_container_count": 2,
            "legacy_inventory_sha256": DIGEST,
            "migrated_inventory_sha256": DIGEST,
        },
        "rootful_docker_summary_restart": {
            "pre_restart_container_count": 2,
            "post_restart_container_count": 2,
            "pre_restart_inventory_sha256": DIGEST,
            "post_restart_inventory_sha256": DIGEST,
            "pre_restart_helper_pid": "100",
            "post_restart_helper_pid": "101",
            "report_stream_id": "docker-stream",
            "pre_restart_sequence": 4,
            "post_restart_sequence": 5,
        },
        "rootful_docker_helper_loss_recovery": {
            "pre_loss_container_count": 2,
            "recovered_container_count": 2,
            "pre_loss_inventory_sha256": DIGEST,
            "recovered_inventory_sha256": DIGEST,
            "report_stream_id": "docker-stream",
            "loss_sequence": 6,
            "recovery_sequence": 7,
        },
    }
    return {
        "rootful_container_runtime": "docker",
        "rootful_container_summary_qualified": True,
        "scenarios": [
            {"name": name, "evidence": {"observations": value}}
            for name, value in observations.items()
        ],
    }


class SecureRuntimeAttestationV7Test(unittest.TestCase):
    def test_v7_extends_without_mutating_v6(self) -> None:
        self.assertEqual(v6.RECEIPT_SCHEMA_VERSION, 6)
        self.assertEqual(len(v6.REQUIRED_SCENARIOS), 20)
        self.assertEqual(v7.RECEIPT_SCHEMA_VERSION, 7)
        self.assertEqual(len(v7.REQUIRED_SCENARIOS), 23)
        self.assertEqual(
            v7.REQUIRED_SCENARIOS[4:5],
            ("rootful_docker_summary_migration",),
        )
        self.assertEqual(
            v7.REQUIRED_SCENARIOS[9:11],
            ("rootful_docker_summary_restart", "rootful_docker_helper_loss_recovery"),
        )

    def test_correlated_rootful_docker_observations_pass(self) -> None:
        v7.verify_rootful_docker_observations(valid_receipt())

    def test_correlated_rootful_docker_observations_fail_closed(self) -> None:
        cases = {
            "zero inventory": ("rootful_docker_summary_migration", "legacy_container_count", 0),
            "migration digest mismatch": ("rootful_docker_summary_migration", "migrated_inventory_sha256", "2" * 64),
            "unchanged helper pid": ("rootful_docker_summary_restart", "post_restart_helper_pid", "100"),
            "restart sequence regression": ("rootful_docker_summary_restart", "post_restart_sequence", 4),
            "recovery stream changed": ("rootful_docker_helper_loss_recovery", "report_stream_id", "other-stream"),
            "recovery sequence regression": ("rootful_docker_helper_loss_recovery", "recovery_sequence", 6),
            "recovery digest mismatch": ("rootful_docker_helper_loss_recovery", "recovered_inventory_sha256", "2" * 64),
        }
        for label, (scenario, key, value) in cases.items():
            with self.subTest(label=label):
                receipt = copy.deepcopy(valid_receipt())
                for item in receipt["scenarios"]:
                    if item["name"] == scenario:
                        item["evidence"]["observations"][key] = value
                with self.assertRaises(v5.AttestationError):
                    v7.verify_rootful_docker_observations(receipt)

    def test_context_restores_v6_contract(self) -> None:
        with v7.v7_contract():
            self.assertEqual(v6.RECEIPT_SCHEMA_VERSION, 7)
            self.assertEqual(v6.ATTESTATION_TOOL_PATH, v7.ATTESTATION_TOOL_PATH)
        self.assertEqual(v6.RECEIPT_SCHEMA_VERSION, 6)
        self.assertEqual(v6.ATTESTATION_TOOL_PATH, "scripts/release_control/secure_runtime_attestation_v6.py")

    def test_create_attestation_rejects_receipt_swapped_after_base_validation(self) -> None:
        receipt = valid_receipt()
        receipt["schema_version"] = 7
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "receipt.json"
            path.write_text(json.dumps(receipt), encoding="utf-8")
            base_result = {"receipt": {"sha256": "0" * 64}}
            with mock.patch.object(v7, "_V6_CREATE_ATTESTATION", return_value=base_result):
                with self.assertRaises(v5.AttestationError):
                    v7.create_attestation(receipt_path=path)


if __name__ == "__main__":
    unittest.main()
