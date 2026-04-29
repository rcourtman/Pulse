from __future__ import annotations

import contextlib
import io
import json
import sys
import tempfile
import unittest
from pathlib import Path


INTERNAL_DIR = Path(__file__).resolve().parent
if str(INTERNAL_DIR) not in sys.path:
    sys.path.insert(0, str(INTERNAL_DIR))

import paid_feature_claims_proof as proof


def run_main(argv: list[str]) -> tuple[int, str]:
    buffer = io.StringIO()
    with contextlib.redirect_stdout(buffer):
        exit_code = proof.main(argv)
    return exit_code, buffer.getvalue()


class PaidFeatureClaimsProofTest(unittest.TestCase):
    def test_default_paths_resolve_from_internal_script_location(self) -> None:
        self.assertEqual(proof.default_pulse_dir(), Path(proof.__file__).resolve().parents[3])
        self.assertEqual(
            proof.default_pulse_pro_license_server_dir(),
            proof.default_pulse_dir().parent / "pulse-pro" / "license-server",
        )
        self.assertEqual(
            proof.default_pulse_pro_relay_server_dir(),
            proof.default_pulse_dir().parent / "pulse-pro" / "relay-server",
        )

    def test_build_command_specs_cover_paid_claim_layers(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse_dir = Path(tmp) / "pulse"
            pulse_dir.mkdir()
            frontend_dir = pulse_dir / "frontend-modern"
            frontend_dir.mkdir()
            pulse_pro_license_server_dir = Path(tmp) / "pulse-pro" / "license-server"
            pulse_pro_license_server_dir.mkdir(parents=True)
            pulse_pro_relay_server_dir = Path(tmp) / "pulse-pro" / "relay-server"
            pulse_pro_relay_server_dir.mkdir(parents=True)

            args = proof.parse_args(
                [
                    "--pulse-dir",
                    str(pulse_dir),
                    "--pulse-pro-license-server-dir",
                    str(pulse_pro_license_server_dir),
                    "--pulse-pro-relay-server-dir",
                    str(pulse_pro_relay_server_dir),
                ]
            )
            specs = proof.build_command_specs(args)

            self.assertEqual(len(specs), 5)
            self.assertEqual(specs[0].cwd, str(pulse_dir.resolve()))
            self.assertEqual(specs[2].cwd, str(frontend_dir.resolve()))
            self.assertEqual(specs[3].cwd, str(pulse_pro_license_server_dir.resolve()))
            self.assertEqual(specs[4].cwd, str(pulse_pro_relay_server_dir.resolve()))
            names = {spec.name for spec in specs}
            self.assertIn("pulse-licensing-paid-feature-contract", names)
            self.assertIn("pulse-api-history-entitlement-enforcement", names)
            self.assertIn("pulse-frontend-paid-claim-contract", names)
            self.assertIn("pulse-pro-public-pricing-and-checkout-contract", names)
            self.assertIn("pulse-pro-relay-entitlement-runtime", names)

    def test_run_command_success_and_failure(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            ok = proof.run_command(
                proof.CommandSpec(
                    name="ok",
                    cwd=tmp,
                    command=["python3", "-c", "print('paid proof ok')"],
                )
            )
            self.assertTrue(ok.ok)
            self.assertEqual(ok.detail, "paid proof ok")

            failed = proof.run_command(
                proof.CommandSpec(
                    name="fail",
                    cwd=tmp,
                    command=["python3", "-c", "import sys; sys.stderr.write('boom\\n'); sys.exit(3)"],
                )
            )
            self.assertFalse(failed.ok)
            self.assertEqual(failed.exit_code, 3)
            self.assertEqual(failed.detail, "boom")

    def test_summarize_output_prefers_success_summary_over_expected_error_logs(self) -> None:
        stdout = """
        stderr | expected store fallback test
        Error: API error

         Test Files  7 passed (7)
              Tests  116 passed (116)
        """
        stderr = """
        at runSuite (node_modules/@vitest/runner/dist/chunk-hooks.js:1729:8)
        """

        self.assertEqual(proof.summarize_output(stdout, stderr), "Tests  116 passed (116)")

    def test_main_json_and_report(self) -> None:
        original_run_proof = proof.run_proof
        try:
            proof.run_proof = lambda _args: [  # type: ignore[assignment]
                proof.CommandResult(
                    name="fake",
                    cwd="/tmp",
                    command=["echo", "fake"],
                    ok=True,
                    exit_code=0,
                    detail="pass",
                )
            ]
            with tempfile.TemporaryDirectory() as tmp:
                report_path = Path(tmp) / "report.md"
                exit_code, output = run_main(["--report-out", str(report_path), "--json"])
                self.assertEqual(exit_code, 0)
                self.assertEqual(json.loads(output)[0]["name"], "fake")
                report = report_path.read_text(encoding="utf-8")
                self.assertIn("Paid Feature Claim Automated Proof", report)
                self.assertIn("self-hosted paid claims", report)
        finally:
            proof.run_proof = original_run_proof  # type: ignore[assignment]


if __name__ == "__main__":
    unittest.main()
