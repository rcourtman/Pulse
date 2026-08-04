#!/usr/bin/env python3
"""Tests for live runtime release proof collection and verification."""

from __future__ import annotations

import argparse
import json
import threading
import unittest
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from unittest import mock

from live_runtime_proof import collect_receipt, evaluate_live_runtime, seal_receipt, verify_receipt


class _ProofHandler(BaseHTTPRequestHandler):
    authorization = "Bearer proof-token"

    def do_GET(self) -> None:  # noqa: N802 - stdlib handler API
        if self.headers.get("Authorization") != self.authorization:
            self.send_response(401)
            self.end_headers()
            return
        if self.path == "/api/version":
            self._json(
                {
                    "version": "6.2.0-rc.8",
                    "build": "proof-build",
                    "channel": "rc",
                    "isSourceBuild": False,
                    "isDevelopment": False,
                }
            )
            return
        if self.path == "/api/recovery/postures?page=1&limit=200":
            self._json(
                {
                    "data": [
                        {
                            "subjectResourceId": "vm-100",
                            "state": "protected",
                            "lastSuccessfulPointAt": "2026-08-04T10:00:00Z",
                        },
                        {
                            "subjectResourceId": "vm-101",
                            "state": "attention",
                            "lastSuccessfulPointAt": "2026-08-03T10:00:00Z",
                        },
                    ],
                    "meta": {"page": 1, "limit": 200, "total": 2, "totalPages": 1},
                }
            )
            return
        self.send_response(404)
        self.end_headers()

    def _json(self, payload: object) -> None:
        encoded = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def log_message(self, format: str, *args: object) -> None:
        return


class LiveRuntimeProofTest(unittest.TestCase):
    def test_evaluation_rejects_unknown_with_successful_point(self) -> None:
        observed, failures = evaluate_live_runtime(
            expected_version="6.2.0-rc.8",
            observed_version="6.2.0-rc.8",
            postures=[
                {
                    "subjectResourceId": "vm-100",
                    "state": "unknown",
                    "lastSuccessfulPointAt": "2026-08-04T10:00:00Z",
                }
            ],
            minimum_postures=1,
            minimum_successful_postures=1,
        )
        self.assertEqual(observed["unknownWithSuccessfulPointCount"], 1)
        self.assertTrue(any("remain unknown" in failure for failure in failures))

    def test_evaluation_rejects_version_mismatch_and_empty_dataset(self) -> None:
        _, failures = evaluate_live_runtime(
            expected_version="6.2.0-rc.8",
            observed_version="6.2.0-rc.7",
            postures=[],
            minimum_postures=1,
            minimum_successful_postures=1,
        )
        self.assertEqual(len(failures), 3)
        self.assertTrue(any("does not match" in failure for failure in failures))

    def test_collects_passing_receipt_from_running_target(self) -> None:
        server = ThreadingHTTPServer(("127.0.0.1", 0), _ProofHandler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)

        with mock.patch.dict(
            "os.environ", {"PULSE_PROOF_TEST_AUTH": _ProofHandler.authorization}
        ):
            receipt = collect_receipt(
                argparse.Namespace(
                    base_url=f"http://127.0.0.1:{server.server_port}",
                    expected_version="6.2.0-rc.8",
                    authorization_env="PULSE_PROOF_TEST_AUTH",
                    cookie_env="",
                    insecure=False,
                    timeout_seconds=2.0,
                    minimum_postures=2,
                    minimum_successful_postures=2,
                )
            )

        self.assertEqual(receipt["result"], "passed")
        self.assertEqual(receipt["observed"]["stateCounts"], {"attention": 1, "protected": 1})
        self.assertEqual(receipt["observed"]["unknownWithSuccessfulPointCount"], 0)
        self.assertEqual(len(receipt["observed"]["postureResponseSha256"]), 1)

    def test_verifier_rejects_tampering(self) -> None:
        receipt = self._passing_receipt()
        receipt["observed"]["unknownWithSuccessfulPointCount"] = 3
        errors = verify_receipt(
            receipt,
            expected_version="6.2.0-rc.8",
            expected_origin="https://pulse.example.test",
            max_age_seconds=3600,
            now=datetime(2026, 8, 4, 12, 30, tzinfo=timezone.utc),
        )
        self.assertIn("receiptSha256 does not match receipt content", errors)
        self.assertIn("receipt reports unknown postures with successful restore points", errors)

    def test_verifier_rejects_wrong_target_and_stale_receipt(self) -> None:
        receipt = self._passing_receipt()
        errors = verify_receipt(
            receipt,
            expected_version="6.2.0-rc.8",
            expected_origin="https://other.example.test",
            max_age_seconds=60,
            now=datetime(2026, 8, 4, 12, 30, tzinfo=timezone.utc),
        )
        self.assertIn("receipt target origin does not match verifier expectation", errors)
        self.assertTrue(any("exceeds maximum" in error for error in errors))

    def test_verifier_accepts_fresh_sealed_receipt(self) -> None:
        errors = verify_receipt(
            self._passing_receipt(),
            expected_version="v6.2.0-rc.8",
            expected_origin="https://pulse.example.test/",
            max_age_seconds=3600,
            now=datetime(2026, 8, 4, 12, 30, tzinfo=timezone.utc),
        )
        self.assertEqual(errors, [])

    @staticmethod
    def _passing_receipt() -> dict[str, object]:
        return seal_receipt(
            {
                "schemaVersion": 1,
                "proofType": "pulse-live-runtime",
                "assertion": "successful-restore-points-have-evaluated-posture",
                "result": "passed",
                "collectedAt": "2026-08-04T12:00:00Z",
                "target": {"origin": "https://pulse.example.test", "tlsVerified": True},
                "expected": {
                    "version": "6.2.0-rc.8",
                    "minimumPostures": 1,
                    "minimumSuccessfulPostures": 1,
                },
                "observed": {
                    "version": "6.2.0-rc.8",
                    "postureTotal": 2,
                    "stateCounts": {"attention": 1, "protected": 1},
                    "successfulPostureCount": 2,
                    "unknownWithSuccessfulPointCount": 0,
                    "unknownWithSuccessfulPointResourceIds": [],
                    "isSourceBuild": False,
                    "isDevelopment": False,
                    "versionResponseSha256": "a" * 64,
                    "postureResponseSha256": ["b" * 64],
                },
                "failures": [],
            }
        )


if __name__ == "__main__":
    unittest.main()
