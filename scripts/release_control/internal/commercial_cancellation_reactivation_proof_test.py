from __future__ import annotations

import contextlib
import io
import json
import tempfile
import unittest
from pathlib import Path

import commercial_cancellation_reactivation_proof as proof


def run_main(argv: list[str]) -> tuple[int, str]:
    buffer = io.StringIO()
    with contextlib.redirect_stdout(buffer):
        exit_code = proof.main(argv)
    return exit_code, buffer.getvalue()


class CommercialCancellationReactivationProofTest(unittest.TestCase):
    def test_build_command_specs_uses_expected_directories(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            pulse_dir = Path(tmp) / "pulse"
            pulse_dir.mkdir()
            frontend_dir = pulse_dir / "frontend-modern"
            frontend_dir.mkdir()
            pulse_pro_dir = Path(tmp) / "pulse-pro" / "license-server"
            pulse_pro_dir.mkdir(parents=True)
            args = proof.parse_args(
                [
                    "--pulse-dir",
                    str(pulse_dir),
                    "--pulse-pro-license-server-dir",
                    str(pulse_pro_dir),
                ]
            )
            specs = proof.build_command_specs(args)
            self.assertEqual(len(specs), 4)
            self.assertEqual(specs[0].cwd, str(pulse_dir.resolve()))
            self.assertEqual(specs[2].cwd, str(frontend_dir.resolve()))
            self.assertEqual(specs[3].cwd, str(pulse_pro_dir.resolve()))

    def test_run_command_success(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            result = proof.run_command(
                proof.CommandSpec(
                    name="ok",
                    cwd=tmp,
                    command=["python3", "-c", "print('all good')"],
                )
            )
            self.assertTrue(result.ok)
            self.assertEqual(result.exit_code, 0)
            self.assertEqual(result.detail, "all good")

    def test_run_command_failure_uses_stderr_summary(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            result = proof.run_command(
                proof.CommandSpec(
                    name="fail",
                    cwd=tmp,
                    command=["python3", "-c", "import sys; sys.stderr.write('boom\\n'); sys.exit(2)"],
                )
            )
            self.assertFalse(result.ok)
            self.assertEqual(result.exit_code, 2)
            self.assertEqual(result.detail, "boom")

    def test_render_markdown_report(self) -> None:
        report = proof.render_markdown_report(
            "Commercial Cancellation/Reactivation Automated Proof",
            [
                proof.CommandResult(
                    name="pass-spec",
                    cwd="/tmp/pulse",
                    command=["go", "test", "./..."],
                    ok=True,
                    exit_code=0,
                    detail="ok",
                ),
                proof.CommandResult(
                    name="fail-spec",
                    cwd="/tmp/pulse-pro/license-server",
                    command=["go", "test", "."],
                    ok=False,
                    exit_code=1,
                    detail="FAIL",
                ),
            ],
        )
        self.assertIn("# Commercial Cancellation/Reactivation Automated Proof", report)
        self.assertIn("`PASS` `pass-spec`", report)
        self.assertIn("`FAIL` `fail-spec`", report)
        self.assertIn("## Manual Follow-up", report)

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
                exit_code, output = run_main(
                    [
                        "--report-out",
                        str(report_path),
                        "--json",
                    ]
                )
                self.assertEqual(exit_code, 0)
                payload = json.loads(output)
                self.assertEqual(payload[0]["name"], "fake")
                self.assertIn("Manual Follow-up", report_path.read_text(encoding="utf-8"))
        finally:
            proof.run_proof = original_run_proof  # type: ignore[assignment]


if __name__ == "__main__":
    unittest.main()
