#!/usr/bin/env python3
"""Tests for RC-to-GA rehearsal record generation."""

from __future__ import annotations

import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import record_rc_to_ga_rehearsal as mod


SUMMARY = """# RC-to-GA Rehearsal Summary

- Workflow run: https://github.com/rcourtman/Pulse/actions/runs/12345
- Branch: main
- Version: 6.0.0
- Tag: v6.0.0
- Channel under rehearsal: stable
- Promoted from RC: v6.0.0-rc.1
- Rollback target: v5.1.23
- RC soak hours at rehearsal time: 96
- Planned v5 EOS date: 2026-06-11
- Hotfix exception: false
- Operator note: dry-run from test
"""


class RecordRcToGaRehearsalTest(unittest.TestCase):
    def test_parse_summary_markdown(self) -> None:
        parsed = mod.parse_summary_markdown(SUMMARY)
        self.assertEqual(parsed["workflow_run"], "https://github.com/rcourtman/Pulse/actions/runs/12345")
        self.assertEqual(parsed["promoted_from_rc"], "v6.0.0-rc.1")
        self.assertEqual(parsed["planned_v5_eos_date"], "2026-06-11")

    def test_render_record_contains_required_fields(self) -> None:
        rendered = mod.render_record(
            record_date="2026-03-12",
            result="pass",
            rollback_command="./scripts/install.sh --version v5.1.23",
            run_metadata={"headSha": "abc123", "url": "https://github.com/rcourtman/Pulse/actions/runs/12345"},
            summary_metadata=mod.parse_summary_markdown(SUMMARY),
            summary_source="/tmp/rc-to-ga-rehearsal-summary.md",
            ga_date="2026-03-15",
            v5_eos_date="2026-06-13",
            follow_ups=["Publish GA release notes with the EOS date."],
            notes=["No unexpected prompts during the rehearsal."],
            summary_markdown=SUMMARY,
        )
        self.assertIn("GitHub Actions run URL: https://github.com/rcourtman/Pulse/actions/runs/12345", rendered)
        self.assertIn("Candidate stable tag: v6.0.0", rendered)
        self.assertIn("Promoted RC tag: v6.0.0-rc.1", rendered)
        self.assertIn("Exact rollback or reinstall command: `./scripts/install.sh --version v5.1.23`", rendered)
        self.assertIn("Exact GA date to publish: 2026-03-15", rendered)
        self.assertIn("Exact v5 end-of-support date to publish: 2026-06-13", rendered)
        self.assertIn("Publish GA release notes with the EOS date.", rendered)
        self.assertIn("```md", rendered)

    def test_main_writes_record_from_summary_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            summary_path = tmp_path / "summary.md"
            summary_path.write_text(SUMMARY, encoding="utf-8")
            output_path = tmp_path / "record.md"

            exit_code = mod.main(
                [
                    "--summary-file",
                    str(summary_path),
                    "--output",
                    str(output_path),
                    "--rollback-command",
                    "./scripts/install.sh --version v5.1.23",
                    "--ga-date",
                    "2026-03-15",
                ]
            )

            self.assertEqual(exit_code, 0)
            content = output_path.read_text(encoding="utf-8")
            self.assertIn("RC-to-GA Rehearsal Record", content)
            self.assertIn("v6.0.0-rc.1", content)
            self.assertIn("2026-03-15", content)

    def test_download_summary_artifact_reads_named_artifact(self) -> None:
        artifact_text = "# summary\n"

        def fake_run_gh(*args: str) -> str:
            if args[:3] == ("run", "download", "123"):
                dest = Path(args[-1])
                artifact_path = dest / "rc-to-ga-rehearsal-summary" / "rc-to-ga-rehearsal-summary.md"
                artifact_path.parent.mkdir(parents=True, exist_ok=True)
                artifact_path.write_text(artifact_text, encoding="utf-8")
                return ""
            raise AssertionError(f"unexpected gh args: {args}")

        with mock.patch.object(mod, "_run_gh", side_effect=fake_run_gh):
            content, source = mod.download_summary_artifact("123")

        self.assertEqual(content, artifact_text)
        self.assertTrue(source.endswith("rc-to-ga-rehearsal-summary.md"))

    def test_download_summary_artifact_raises_clear_error_when_artifact_missing(self) -> None:
        error = subprocess.CalledProcessError(
            1,
            ["gh", "run", "download", "123"],
            stderr="no valid artifacts found to download",
        )

        with mock.patch.object(mod, "_run_gh", side_effect=error):
            with self.assertRaisesRegex(FileNotFoundError, "may be missing or expired"):
                mod.download_summary_artifact("123")

    def test_main_normalizes_relative_summary_file_and_validates_dates(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            summary_path = tmp_path / "summary.md"
            summary_path.write_text(SUMMARY, encoding="utf-8")
            output_path = tmp_path / "record.md"

            with mock.patch.object(mod, "REPO_ROOT", tmp_path):
                exit_code = mod.main(
                    [
                        "--summary-file",
                        "summary.md",
                        "--output",
                        str(output_path),
                        "--rollback-command",
                        "./scripts/install.sh --version v5.1.23",
                        "--ga-date",
                        "2026-03-15",
                        "--v5-eos-date",
                        "2026-06-13",
                        "--record-date",
                        "2026-03-12",
                    ]
                )

            self.assertEqual(exit_code, 0)
            self.assertIn("2026-06-13", output_path.read_text(encoding="utf-8"))

    def test_main_raises_clear_error_for_missing_summary_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            with mock.patch.object(mod, "REPO_ROOT", tmp_path):
                with self.assertRaisesRegex(FileNotFoundError, "summary file does not exist"):
                    mod.main(
                        [
                            "--summary-file",
                            "missing.md",
                            "--output",
                            str(tmp_path / "record.md"),
                            "--rollback-command",
                            "./scripts/install.sh --version v5.1.23",
                        ]
                    )


if __name__ == "__main__":
    unittest.main()
