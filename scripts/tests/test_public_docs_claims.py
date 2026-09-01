#!/usr/bin/env python3
"""Guard public capability claims that can reverse the product contract."""

from __future__ import annotations

import importlib.util
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "check_public_docs.py"
SPEC = importlib.util.spec_from_file_location("check_public_docs", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
public_docs = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = public_docs
SPEC.loader.exec_module(public_docs)


class PublicDocsClaimsTest(unittest.TestCase):
    def test_release_notes_are_in_the_public_claim_surface(self) -> None:
        files = public_docs.public_markdown_files()

        self.assertIn(ROOT / "docs/releases/RELEASE_NOTES_v6.4.0-rc.7.md", files)

    def test_rejects_reversed_dead_man_direction(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            note = root / "note.md"
            note.write_text(
                "Dead-man checks can notify when an expected external signal "
                "stops arriving.\n",
                encoding="utf-8",
            )
            with mock.patch.object(public_docs, "ROOT", root):
                errors = public_docs.check_public_claims([note])

        self.assertEqual(len(errors), 1)
        self.assertIn("dead-man direction is reversed", errors[0])

    def test_accepts_outbound_watchdog_description(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            note = root / "note.md"
            note.write_text(
                "Pulse sends its own health signal to a watchdog on another host.\n",
                encoding="utf-8",
            )
            with mock.patch.object(public_docs, "ROOT", root):
                errors = public_docs.check_public_claims([note])

        self.assertEqual(errors, [])


if __name__ == "__main__":
    unittest.main()
