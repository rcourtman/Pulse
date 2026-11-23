#!/usr/bin/env python3
"""Verify .husky/commit-msg enforces subject, body, and prose punctuation rules."""

from __future__ import annotations

from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
HOOK = ROOT / ".husky" / "commit-msg"


class CommitMsgHookTest(unittest.TestCase):
    def run_hook(self, message: str) -> subprocess.CompletedProcess[str]:
        with tempfile.NamedTemporaryFile("w", encoding="utf-8", delete=False) as handle:
            handle.write(message)
            path = Path(handle.name)
        try:
            return subprocess.run(
                ["sh", str(HOOK), str(path)],
                cwd=ROOT,
                text=True,
                capture_output=True,
                timeout=10,
                check=False,
            )
        finally:
            path.unlink(missing_ok=True)

    def test_accepts_clean_commit_message(self) -> None:
        result = self.run_hook(
            "docs: update installation guide\n\n"
            "Align the container setup steps with the current release image.\n"
        )
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_rejects_em_dash_in_subject(self) -> None:
        result = self.run_hook(
            "docs: update guide \u2014 clearer steps\n\n"
            "Clarify the container setup instructions.\n"
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("avoid em dashes", result.stderr)

    def test_rejects_semicolon_in_body(self) -> None:
        result = self.run_hook(
            "docs: update installation guide\n\n"
            "Clarify the container setup; remove stale flags.\n"
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("avoid semicolons", result.stderr)

    def test_allows_code_spans_fences_and_urls(self) -> None:
        result = self.run_hook(
            "fix: quote command arguments\n\n"
            "Handle `cmd1; cmd2` cleanly in shell examples and links like "
            "https://example.invalid/a;b.\n\n"
            "```sh\n"
            "echo one; echo two # \u2014 example\n"
            "```\n"
        )
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_still_requires_body_and_short_subject(self) -> None:
        no_body = self.run_hook("docs: short subject\n")
        self.assertNotEqual(no_body.returncode, 0)
        long_subject = self.run_hook("x" * 73 + "\n\nReasonable body.\n")
        self.assertNotEqual(long_subject.returncode, 0)


if __name__ == "__main__":
    unittest.main()
