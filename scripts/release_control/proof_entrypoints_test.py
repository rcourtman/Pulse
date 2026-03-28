from __future__ import annotations

import subprocess
import unittest
from pathlib import Path


RELEASE_CONTROL_DIR = Path(__file__).resolve().parent


class ProofEntrypointsTest(unittest.TestCase):
    def test_documented_release_control_entrypoints_exist_and_show_help(self) -> None:
        entrypoints = [
            "commercial_cancellation_reactivation_proof.py",
            "commercial_cancellation_reactivation_rehearsal.py",
            "mobile_relay_auth_approvals_proof.py",
            "relay_registration_reconnect_drain_proof.py",
        ]
        for entrypoint in entrypoints:
            with self.subTest(entrypoint=entrypoint):
                path = RELEASE_CONTROL_DIR / entrypoint
                self.assertTrue(path.is_file())
                result = subprocess.run(
                    ["python3", str(path), "--help"],
                    cwd=RELEASE_CONTROL_DIR.parent.parent,
                    capture_output=True,
                    text=True,
                    check=False,
                )
                self.assertEqual(result.returncode, 0)
                self.assertIn("usage:", result.stdout)
