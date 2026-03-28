#!/usr/bin/env python3
"""Tests for the relay registration/reconnect proof wrapper."""

from __future__ import annotations

import sys
import unittest
from pathlib import Path


INTERNAL_DIR = Path(__file__).resolve().parent
if str(INTERNAL_DIR) not in sys.path:
    sys.path.insert(0, str(INTERNAL_DIR))

import relay_registration_reconnect_drain_proof as proof


class RelayRegistrationReconnectDrainProofTest(unittest.TestCase):
    def test_default_paths_resolve_from_internal_script_location(self) -> None:
        self.assertEqual(proof.default_pulse_dir(), Path(proof.__file__).resolve().parents[3])
        self.assertEqual(
            proof.default_pulse_mobile_dir(),
            proof.default_pulse_dir().parent / "pulse-mobile",
        )

    def test_build_command_specs_are_sorted_and_cover_expected_workspaces(self) -> None:
        args = proof.parse_args([])
        specs = proof.build_command_specs(args)
        self.assertEqual([spec.name for spec in specs], sorted(spec.name for spec in specs))
        self.assertIn("relay-managed-runtime", [spec.name for spec in specs])
        self.assertTrue(any(Path(spec.cwd) == proof.default_pulse_dir() / "frontend-modern" for spec in specs))
        self.assertTrue(any(Path(spec.cwd) == proof.default_pulse_mobile_dir() for spec in specs))
        self.assertTrue(any(Path(spec.cwd) == proof.default_pulse_dir() for spec in specs))
        mobile_spec = next(spec for spec in specs if spec.name == "relay-mobile-client")
        self.assertEqual(
            mobile_spec.command[:4],
            ["npx", "jest", "--runInBand", "--detectOpenHandles"],
        )


if __name__ == "__main__":
    unittest.main()
