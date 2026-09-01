#!/usr/bin/env python3
"""Tests for the injection-safe GitHub output writer."""

from __future__ import annotations

import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
MODULE_PATH = REPO_ROOT / "scripts" / "write_github_output.py"
SPEC = importlib.util.spec_from_file_location("write_github_output", MODULE_PATH)
assert SPEC and SPEC.loader
write_github_output = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = write_github_output
SPEC.loader.exec_module(write_github_output)


class WriteGitHubOutputTest(unittest.TestCase):
    def test_arbitrary_lines_remain_inside_one_collision_free_record(self) -> None:
        tokens = iter(("collision", "safe"))
        value = "v1\npulse_output_collision\ninjected=surprise"
        with tempfile.TemporaryDirectory() as temporary_directory:
            output = Path(temporary_directory) / "output"
            write_github_output.append_output(
                output, "release", value, lambda: next(tokens)
            )
            self.assertEqual(
                output.read_text(encoding="utf-8"),
                "release<<pulse_output_safe\n"
                f"{value}\n"
                "pulse_output_safe\n",
            )

    def test_rejects_names_that_can_create_another_command(self) -> None:
        with tempfile.TemporaryDirectory() as temporary_directory:
            with self.assertRaises(ValueError):
                write_github_output.append_output(
                    Path(temporary_directory) / "output",
                    "release\ninjected",
                    "v1",
                )


if __name__ == "__main__":
    unittest.main()
