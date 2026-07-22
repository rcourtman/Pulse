#!/usr/bin/env python3
"""Unit tests for the canonical Pulse Mobile contract generator."""

from __future__ import annotations

import copy
import unittest

from generate_mobile_compatibility import (
    ManifestError,
    REPO_ROOT,
    generated_outputs,
    load_manifest,
    validate_manifest,
)


class MobileCompatibilityGeneratorTest(unittest.TestCase):
    def setUp(self) -> None:
        self.manifest, _ = load_manifest(REPO_ROOT)

    def test_manifest_renders_backend_projections(self) -> None:
        outputs = generated_outputs(REPO_ROOT, None)
        rendered = "\n".join(outputs.values())
        self.assertIn("relayMobileRouteAttentionList", rendered)
        self.assertIn("PushActionDecideAction", rendered)
        self.assertIn('onboardingSchemaVersion = "pulse-mobile-onboarding-v1"', rendered)

    def test_duplicate_route_ids_fail_closed(self) -> None:
        payload = copy.deepcopy(self.manifest)
        payload["routes"][1]["id"] = payload["routes"][0]["id"]
        with self.assertRaisesRegex(ManifestError, "duplicate route id"):
            validate_manifest(payload)

    def test_unknown_scope_fails_closed(self) -> None:
        payload = copy.deepcopy(self.manifest)
        payload["routes"][0]["required_scope"] = "mobile:anything"
        with self.assertRaisesRegex(ManifestError, "unsupported scope"):
            validate_manifest(payload)

    def test_invalid_mobile_impact_fails_closed(self) -> None:
        payload = copy.deepcopy(self.manifest)
        payload["routes"][0]["mobile_impact"] = "maybe-mobile"
        with self.assertRaisesRegex(ManifestError, "invalid mobile_impact"):
            validate_manifest(payload)


if __name__ == "__main__":
    unittest.main()
