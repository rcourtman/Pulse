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

    def test_code_comments_do_not_create_recovery_destinations_or_shift_suffixes(self) -> None:
        self.document(
            "guide.md", "# Recovery\n```sh\n# Recovery\n# Reset lockout\n```\n# Recovery\n",
        )
        source = self.document(
            "source.md", "[first](guide.md#recovery) [second](guide.md#recovery-1) "
            "[phantom](guide.md#recovery-2) [comment](guide.md#reset-lockout)\n",
        )

        self.assertEqual(public_docs.check_links([source]), [
            "source.md: missing anchor #recovery-2 in guide.md",
            "source.md: missing anchor #reset-lockout in guide.md",
        ])

    def test_fenced_markdown_and_html_examples_are_not_live_links(self) -> None:
        source = self.document(
            "source.md", "# Present\n```markdown\n[example](absent.md#one)\n"
            '<a href="also-absent.md#two">example</a>\n```\n'
            "~~~html\n<img src='absent.png'>\n~~~\n"
            "[real](#present) [broken](#missing)\n",
        )

        self.assertEqual(public_docs.check_links([source]), [
            "source.md: missing anchor #missing in source.md",
        ])

    def test_closing_fence_needs_matching_character_and_enough_markers(self) -> None:
        for opening, short, wrong, closing in [
            ("````sh", "```", "~~~~", "`````"),
            ("~~~~sh", "~~~", "````", "~~~~~"),
        ]:
            with self.subTest(opening=opening):
                guide = self.document(
                    "guide.md", f"{opening}\n# Hidden\n{short}\n# Still hidden\n"
                    f"{wrong}\n# Also hidden\n{closing}\n# Visible\n",
                )
                self.assertEqual(public_docs.anchors_for(guide), {"visible"})

    def test_closing_fence_cannot_carry_an_info_string(self) -> None:
        guide = self.document(
            "guide.md", "```sh\n# Hidden\n```not-a-close\n# Still hidden\n```\n# Visible\n",
        )
        self.assertEqual(public_docs.anchors_for(guide), {"visible"})

    def test_fences_allow_up_to_three_spaces_and_trailing_closing_whitespace(self) -> None:
        for indent in range(4):
            with self.subTest(indent=indent):
                guide = self.document(
                    "guide.md", f"{' ' * indent}~~~sh\n# Hidden\n"
                    f"{' ' * indent}~~~ \t\n# Visible\n",
                )
                self.assertEqual(public_docs.anchors_for(guide), {"visible"})

    def test_unclosed_fences_hide_examples_through_the_end_of_the_file(self) -> None:
        source = self.document(
            "source.md", "# Present\n```sh\n# Not a heading\n[example](absent.md)\n",
        )
        self.assertEqual(public_docs.anchors_for(source), {"present"})
        self.assertEqual(public_docs.check_links([source]), [])

    def test_backticks_in_backtick_info_do_not_open_a_fence(self) -> None:
        guide = self.document("guide.md", "```not`a-fence\n# Visible\n")
        self.assertEqual(public_docs.anchors_for(guide), {"visible"})

    def test_closed_example_does_not_join_prose_into_an_accidental_link(self) -> None:
        source = self.document(
            "source.md", "[example]\n```markdown\nanything\n```\n(absent.md)\n",
        )
        self.assertEqual(public_docs.check_links([source]), [])


if __name__ == "__main__":
    unittest.main()
