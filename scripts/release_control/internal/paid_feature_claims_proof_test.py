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


def write_public_copy_fixture(root: Path) -> tuple[Path, Path]:
    pulse_dir = root / "pulse"
    pulse_pro_dir = root / "pulse-pro"
    (pulse_dir / "docs").mkdir(parents=True)
    (pulse_dir / "frontend-modern" / "src" / "utils").mkdir(parents=True)
    (pulse_pro_dir / "landing-page").mkdir(parents=True)
    (pulse_pro_dir / "license-server").mkdir(parents=True)
    (pulse_pro_dir / "relay-server").mkdir(parents=True)

    (pulse_dir / "docs" / "PULSE_PRO.md").write_text(
        """
        monitored-system volume is no longer the paid gate.
        Relay gives 14-day history.
        Pro gives 90-day history.
        Alert-triggered root-cause analysis.
        Safe remediation workflows.
        """,
        encoding="utf-8",
    )
    (pulse_dir / "docs" / "AI.md").write_text(
        """
        Pulse Patrol is available to everyone on the Community plan with BYOK.
        Pro adds alert-triggered root-cause analysis and safe remediation workflows.
        Community and Relay installs can still run scheduled Patrol findings with BYOK.
        """,
        encoding="utf-8",
    )
    (pulse_dir / "docs" / "UPGRADE_v6.md").write_text(
        """
        monitoring stays available across Community, Relay, and Pro.
        Relay raises history to 14 days.
        Pro raises it to 90 days.
        Self-hosted v6 does not expose a general in-app trial.
        """,
        encoding="utf-8",
    )
    (pulse_dir / "frontend-modern" / "src" / "utils" / "selfHostedPlans.ts").write_text(
        """
        Self-hosted Pulse includes core monitoring for free.
        Remote web access, mobile pairing, and push.
        14-day metric history.
        Root-cause analysis, safe remediation workflows, 90-day history.
        admin/reporting extras.
        """,
        encoding="utf-8",
    )
    (pulse_pro_dir / "landing-page" / "index.html").write_text(
        """
        Community keeps core monitoring free.
        mobile app pairing, push notifications, and 14-day history.
        root-cause analysis, safe remediation workflows, and 90-day history.
        Do Relay or Pro charge by server?
        Existing Pulse Pro and legacy Pro+ holders keep their paid runtime access.
        """,
        encoding="utf-8",
    )
    (pulse_pro_dir / "license-server" / "public_pricing.go").write_text(
        """
        Community keeps core monitoring free.
        mobile app pairing, push notifications, and 14-day history.
        root-cause analysis, safe remediation workflows, team controls, and 90-day history.
        not as the self-hosted paid gate.
        """,
        encoding="utf-8",
    )
    return pulse_dir, pulse_pro_dir


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

    def test_public_copy_audit_passes_for_current_claim_floor(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse_dir, pulse_pro_dir = write_public_copy_fixture(Path(tmp))
            args = proof.parse_args(
                [
                    "--pulse-dir",
                    str(pulse_dir),
                    "--pulse-pro-license-server-dir",
                    str(pulse_pro_dir / "license-server"),
                    "--pulse-pro-relay-server-dir",
                    str(pulse_pro_dir / "relay-server"),
                ]
            )

            result = proof.run_public_paid_claim_copy_audit(args)

            self.assertTrue(result.ok)
            self.assertEqual(result.detail, "checked 6 public paid-claim files")

    def test_public_copy_audit_rejects_stale_limit_and_trial_claims(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse_dir, pulse_pro_dir = write_public_copy_fixture(Path(tmp))
            plans = pulse_dir / "frontend-modern" / "src" / "utils" / "selfHostedPlans.ts"
            plans.write_text(
                plans.read_text(encoding="utf-8")
                + "\nUpgrade for higher limits and start a 14-day Pro trial.\n",
                encoding="utf-8",
            )
            args = proof.parse_args(
                [
                    "--pulse-dir",
                    str(pulse_dir),
                    "--pulse-pro-license-server-dir",
                    str(pulse_pro_dir / "license-server"),
                    "--pulse-pro-relay-server-dir",
                    str(pulse_pro_dir / "relay-server"),
                ]
            )

            result = proof.run_public_paid_claim_copy_audit(args)

            self.assertFalse(result.ok)
            self.assertEqual(result.exit_code, 1)
            self.assertIn("higher monitoring limits", result.detail)
            self.assertIn("start trial CTA", result.detail)

    def test_run_proof_includes_public_copy_audit_before_command_specs(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse_dir, pulse_pro_dir = write_public_copy_fixture(Path(tmp))
            args = proof.parse_args(
                [
                    "--pulse-dir",
                    str(pulse_dir),
                    "--pulse-pro-license-server-dir",
                    str(pulse_pro_dir / "license-server"),
                    "--pulse-pro-relay-server-dir",
                    str(pulse_pro_dir / "relay-server"),
                ]
            )
            original_build_command_specs = proof.build_command_specs
            original_run_command = proof.run_command
            try:
                proof.build_command_specs = lambda _args: [  # type: ignore[assignment]
                    proof.CommandSpec(name="fake-command", cwd=str(pulse_dir), command=["true"])
                ]
                proof.run_command = lambda spec: proof.CommandResult(  # type: ignore[assignment]
                    name=spec.name,
                    cwd=spec.cwd,
                    command=spec.command,
                    ok=True,
                    exit_code=0,
                    detail="pass",
                )

                results = proof.run_proof(args)
            finally:
                proof.build_command_specs = original_build_command_specs  # type: ignore[assignment]
                proof.run_command = original_run_command  # type: ignore[assignment]

            self.assertEqual(
                [result.name for result in results],
                ["pulse-public-paid-claim-copy-audit", "fake-command"],
            )

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
