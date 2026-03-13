#!/usr/bin/env python3
"""Tests for the live commercial cancellation/reactivation rehearsal wrapper."""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from unittest import mock

import commercial_cancellation_reactivation_rehearsal as rehearsal


class BuildCommandTests(unittest.TestCase):
    def test_run_rehearsal_uses_expected_commands(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            integration = root / "tests" / "integration"
            integration.mkdir(parents=True)
            args = rehearsal.parse_args(
                [
                    "--pulse-dir",
                    str(root),
                    "--integration-dir",
                    str(integration),
                    "--project",
                    "chromium",
                ]
            )
            recorded: list[tuple[str, Path, list[str]]] = []

            def fake_run(name: str, cwd: Path, command: list[str]) -> rehearsal.CommandResult:
                recorded.append((name, cwd, command))
                return rehearsal.CommandResult(
                    name=name,
                    cwd=str(cwd),
                    command=command,
                    ok=True,
                    exit_code=0,
                    detail="pass",
                )

            with mock.patch.object(rehearsal, "run_command", side_effect=fake_run):
                results = rehearsal.run_rehearsal(args)

            self.assertEqual(len(results), 2)
            self.assertEqual(recorded[0][0], "commercial-cancellation-automated-proof-floor")
            self.assertEqual(
                recorded[0][2],
                [
                    "python3",
                    "scripts/release_control/commercial_cancellation_reactivation_proof.py",
                    "--json",
                ],
            )
            self.assertEqual(recorded[1][0], "commercial-cancellation-playwright-live-journey")
            self.assertEqual(
                recorded[1][2],
                [
                    "npm",
                    "test",
                    "--",
                    "tests/14-commercial-cancellation-reactivation.spec.ts",
                    "--project=chromium",
                ],
            )


class ReportTests(unittest.TestCase):
    def test_render_markdown_report_mentions_live_journey(self) -> None:
        report = rehearsal.render_markdown_report(
            "Commercial Cancellation/Reactivation Rehearsal",
            [
                rehearsal.CommandResult(
                    name="commercial-cancellation-playwright-live-journey",
                    cwd="/tmp/integration",
                    command=["npm", "test"],
                    ok=True,
                    exit_code=0,
                    detail="pass",
                )
            ],
        )
        self.assertIn("commercial-cancellation-playwright-live-journey", report)
        self.assertIn("14-commercial-cancellation-reactivation.spec.ts", report)


if __name__ == "__main__":
    unittest.main()
