#!/usr/bin/env python3
"""Tests for the governed mobile release decision gate."""

from __future__ import annotations

import unittest

from mobile_release_gate import normalize_decision, validate_mobile_release_decision


class MobileReleaseGateTest(unittest.TestCase):
    def test_no_mobile_impact_allows_empty_evidence(self) -> None:
        self.assertEqual(validate_mobile_release_decision("no-mobile-impact", ""), [])

    def test_existing_build_compatibility_requires_evidence(self) -> None:
        errors = validate_mobile_release_decision("existing-mobile-build-compatible", "")
        self.assertEqual(len(errors), 1)
        self.assertIn("mobile_release_evidence is required", errors[0])

    def test_uploaded_candidate_requires_evidence(self) -> None:
        errors = validate_mobile_release_decision("mobile-candidate-uploaded", "")
        self.assertEqual(len(errors), 1)
        self.assertIn("mobile_release_evidence is required", errors[0])

    def test_uploaded_candidate_accepts_evidence(self) -> None:
        self.assertEqual(
            validate_mobile_release_decision("mobile-candidate-uploaded", "iOS build 6 uploaded to TestFlight"),
            [],
        )

    def test_mobile_candidate_required_blocks_release(self) -> None:
        errors = validate_mobile_release_decision("mobile-candidate-required", "")
        self.assertEqual(len(errors), 1)
        self.assertIn("blocking release state", errors[0])

    def test_unknown_decision_is_rejected(self) -> None:
        errors = validate_mobile_release_decision("ship-it", "proof")
        self.assertEqual(len(errors), 1)
        self.assertIn("unknown mobile_release_decision", errors[0])

    def test_underscore_aliases_normalize_to_canonical_decision(self) -> None:
        self.assertEqual(
            normalize_decision("existing_mobile_build_compatible"),
            "existing-mobile-build-compatible",
        )


if __name__ == "__main__":
    unittest.main()
