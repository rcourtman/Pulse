#!/usr/bin/env python3
"""Guard public capability claims and complete, scan-local link validation."""

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

    def test_rejects_removed_truenas_rest_probe(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            note = root / "note.md"
            note.write_text(
                "Test with https://nas.local/api/v2.0/system/info.\n",
                encoding="utf-8",
            )
            with mock.patch.object(public_docs, "ROOT", root):
                errors = public_docs.check_public_claims([note])

        self.assertEqual(len(errors), 1)
        self.assertIn("TrueNAS 26 removed", errors[0])


class PublicDocsLinksTest(unittest.TestCase):
    def setUp(self) -> None:
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.root_patch = mock.patch.object(public_docs, "ROOT", self.root)
        self.root_patch.start()
        self.addCleanup(self.root_patch.stop)

    def document(self, name: str, text: str) -> Path:
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def test_repeated_links_scan_each_target_once_without_hiding_any_errors(self) -> None:
        target = self.document("guide.md", "# Shared\n# Shared\n")
        first = self.document(
            "first.md", "[one](guide.md#shared) [two](guide.md#shared-1) "
            "[bad](guide.md#missing)\n",
        )
        second = self.document(
            "nested/second.md", "[same](../guide.md#shared) "
            "[bad](../guide.md#missing)\n",
        )
        with mock.patch.object(public_docs, "anchors_for", wraps=public_docs.anchors_for) as scan:
            errors = public_docs.check_links([first, second])
        self.assertEqual(errors, [
            "first.md: missing anchor #missing in guide.md",
            "nested/second.md: missing anchor #missing in guide.md",
        ])
        scan.assert_called_once_with(target)

    def test_empty_anchor_set_is_cached_and_each_link_still_fails(self) -> None:
        target = self.document("empty.md", "There are no headings.\n")
        source = self.document("source.md", "[one](empty.md#one) [two](empty.md#two)\n")
        with mock.patch.object(public_docs, "anchors_for", wraps=public_docs.anchors_for) as scan:
            errors = public_docs.check_links([source])
        self.assertEqual(errors, [
            "source.md: missing anchor #one in empty.md",
            "source.md: missing anchor #two in empty.md",
        ])
        scan.assert_called_once_with(target)

    def test_distinct_targets_with_the_same_filename_do_not_share_anchors(self) -> None:
        first = self.document("first/guide.md", "# First\n")
        second = self.document("second/guide.md", "# Second\n")
        source = self.document(
            "source.md", "[one](first/guide.md#first) [two](second/guide.md#second) "
            "[bad](second/guide.md#first)\n",
        )
        with mock.patch.object(public_docs, "anchors_for", wraps=public_docs.anchors_for) as scan:
            errors = public_docs.check_links([source])
        self.assertEqual(errors, ["source.md: missing anchor #first in second/guide.md"])
        self.assertEqual(scan.call_args_list, [mock.call(first), mock.call(second)])

    def test_next_check_reads_changed_headings_instead_of_reusing_an_old_scan(self) -> None:
        target = self.document("guide.md", "# Before\n")
        source = self.document("source.md", "[one](guide.md#before) [two](guide.md#before)\n")
        with mock.patch.object(public_docs, "anchors_for", wraps=public_docs.anchors_for) as scan:
            self.assertEqual(public_docs.check_links([source]), [])
            target.write_text("# After\n", encoding="utf-8")
            errors = public_docs.check_links([source])
        self.assertEqual(errors, ["source.md: missing anchor #before in guide.md"] * 2)
        self.assertEqual(scan.call_args_list, [mock.call(target), mock.call(target)])

    def test_self_links_decoding_and_heading_suffixes_keep_their_existing_rules(self) -> None:
        source = self.document(
            "source.md", "# Café\n# Café\n[one](#caf%C3%A9) [two](#café-1) [bad](#café-2)\n",
        )
        with mock.patch.object(public_docs, "anchors_for", wraps=public_docs.anchors_for) as scan:
            errors = public_docs.check_links([source])
        self.assertEqual(errors, ["source.md: missing anchor #café-2 in source.md"])
        scan.assert_called_once_with(source)

    def test_missing_files_still_fail_without_scanning_external_or_nonmarkdown_targets(self) -> None:
        self.document("image.svg", "<svg/>\n")
        source = self.document(
            "source.md", "[missing](missing.md#one) [external](https://example.invalid/#one) "
            "![image](image.svg#one) [self](#)\n",
        )
        with mock.patch.object(public_docs, "anchors_for", wraps=public_docs.anchors_for) as scan:
            errors = public_docs.check_links([source])
        self.assertEqual(errors, ["source.md: missing local target missing.md#one"])
        scan.assert_not_called()


if __name__ == "__main__":
    unittest.main()
