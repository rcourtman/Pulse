#!/usr/bin/env python3
"""Tests for the mobile relay auth/approvals proof wrapper."""

from __future__ import annotations

import unittest

import mobile_relay_auth_approvals_proof as proof


class MobileRelayAuthApprovalsProofTest(unittest.TestCase):
    def test_build_command_specs_are_sorted_and_cross_repo(self) -> None:
        args = proof.parse_args([])
        specs = proof.build_command_specs(args)
        self.assertEqual([spec.name for spec in specs], sorted(spec.name for spec in specs))
        self.assertTrue(all(spec.cwd.endswith(("pulse-mobile", "pulse-enterprise")) for spec in specs))
        self.assertTrue(any(spec.cwd.endswith("pulse-enterprise") for spec in specs))
        self.assertTrue(any(spec.cwd.endswith("pulse-mobile") for spec in specs))


if __name__ == "__main__":
    unittest.main()
