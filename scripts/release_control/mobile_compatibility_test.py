#!/usr/bin/env python3
"""Unit tests for provider/consumer Pulse Mobile compatibility checks."""

from __future__ import annotations

import copy
import unittest

from generate_mobile_compatibility import REPO_ROOT, load_manifest
from mobile_compatibility import compare_contracts, load_consumer


MOBILE_REPO = REPO_ROOT.parent / "pulse-mobile"


class MobileCompatibilityTest(unittest.TestCase):
    def setUp(self) -> None:
        if not (MOBILE_REPO / "config/mobile-api-surface.json").is_file():
            self.skipTest(f"Pulse Mobile checkout not available at {MOBILE_REPO}")
        self.provider, _ = load_manifest(REPO_ROOT)
        self.consumer = load_consumer(MOBILE_REPO)

    def test_checked_in_contracts_are_compatible(self) -> None:
        self.assertEqual(compare_contracts(self.provider, self.consumer), [])

    def test_removed_mobile_route_is_rejected(self) -> None:
        provider = copy.deepcopy(self.provider)
        provider["routes"] = [
            route for route in provider["routes"] if route["id"] != "attention-list"
        ]
        self.assertTrue(
            any(
                "removed mobile route 'attention-list'" in error
                for error in compare_contracts(provider, self.consumer)
            )
        )

    def test_response_field_type_change_is_rejected(self) -> None:
        provider = copy.deepcopy(self.provider)
        route = next(route for route in provider["routes"] if route["id"] == "attention-list")
        field = next(field for field in route["response_fields"] if field["path"] == "data")
        field["type"] = "object"
        self.assertTrue(
            any(
                "field 'data' changed type" in error
                for error in compare_contracts(provider, self.consumer)
            )
        )

    def test_pairing_and_push_drift_are_rejected(self) -> None:
        provider = copy.deepcopy(self.provider)
        provider["pairing"]["host"] = "pair"
        provider["push"]["actions"] = [
            action for action in provider["push"]["actions"] if action["value"] != "decide_action"
        ]
        errors = compare_contracts(provider, self.consumer)
        self.assertTrue(any("pairing host" in error for error in errors))
        self.assertTrue(any("removed action 'decide_action'" in error for error in errors))


if __name__ == "__main__":
    unittest.main()
