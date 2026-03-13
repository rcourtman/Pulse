#!/usr/bin/env python3
"""Tests for the relay registration/reconnect proof wrapper."""

from __future__ import annotations

import unittest

import relay_registration_reconnect_drain_proof as proof


class RelayRegistrationReconnectDrainProofTest(unittest.TestCase):
    def test_build_command_specs_are_sorted_and_cover_expected_workspaces(self) -> None:
        args = proof.parse_args([])
        specs = proof.build_command_specs(args)
        self.assertEqual([spec.name for spec in specs], sorted(spec.name for spec in specs))
        self.assertIn("relay-managed-runtime", [spec.name for spec in specs])
        self.assertTrue(any(spec.cwd.endswith("frontend-modern") for spec in specs))
        self.assertTrue(any(spec.cwd.endswith("pulse-mobile") for spec in specs))
        self.assertTrue(any(spec.cwd.endswith("pulse") for spec in specs))


if __name__ == "__main__":
    unittest.main()
