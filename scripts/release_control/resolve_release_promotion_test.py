#!/usr/bin/env python3
"""Unit tests for shared release-promotion metadata resolution."""

from __future__ import annotations

import unittest

import resolve_release_promotion as resolver


class ResolveReleasePromotionTest(unittest.TestCase):
    def test_prerelease_requires_explicit_stable_rollback(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.0.0-rc.2",
            promoted_from_tag_input="",
            rollback_version_input="5.1.14",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=False,
            hotfix_reason_input="",
            release_notes_input="",
            tag_exists_fn=lambda tag: tag == "v5.1.14",
        )
        self.assertEqual(metadata["rollback_tag"], "v5.1.14")
        self.assertEqual(metadata["rollback_command"], "./scripts/install.sh --version v5.1.14")
        self.assertEqual(metadata["promoted_from_tag"], "")
        self.assertEqual(metadata["soak_hours"], "")

    def test_stable_requires_matching_promoted_rc_and_soak(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.0.0",
            promoted_from_tag_input="6.0.0-rc.2",
            rollback_version_input="5.1.14",
            ga_date_input="2026-03-20",
            v5_eos_date_input="2026-06-18",
            hotfix_exception=False,
            hotfix_reason_input="",
            release_notes_input="maintenance-only support 2026-03-20 2026-06-18",
            tag_exists_fn=lambda tag: tag in {"v6.0.0-rc.2", "v5.1.14"},
            tag_commit_fn=lambda tag: "abc123",
            head_descends_from_fn=lambda commit: commit == "abc123",
            tag_created_unix_fn=lambda tag: 100,
            now_unix_fn=lambda: 100 + (73 * 3600),
        )
        self.assertEqual(metadata["promoted_from_tag"], "v6.0.0-rc.2")
        self.assertEqual(metadata["soak_hours"], "73")

    def test_stable_requires_release_notes_notice_when_supplied(self) -> None:
        with self.assertRaisesRegex(
            ValueError,
            "release_notes must include the Pulse v5 maintenance-only support notice",
        ):
            resolver.resolve_metadata(
                version="6.0.0",
                promoted_from_tag_input="6.0.0-rc.2",
                rollback_version_input="5.1.14",
                ga_date_input="2026-03-20",
                v5_eos_date_input="2026-06-18",
                hotfix_exception=False,
                hotfix_reason_input="",
                release_notes_input="missing notice 2026-03-20 2026-06-18",
                tag_exists_fn=lambda tag: True,
                tag_commit_fn=lambda tag: "abc123",
                head_descends_from_fn=lambda commit: True,
                tag_created_unix_fn=lambda tag: 100,
                now_unix_fn=lambda: 100 + (73 * 3600),
            )

    def test_stable_hotfix_requires_reason(self) -> None:
        with self.assertRaisesRegex(ValueError, "hotfix_reason is required"):
            resolver.resolve_metadata(
                version="6.0.1",
                promoted_from_tag_input="6.0.1-rc.1",
                rollback_version_input="6.0.0",
                ga_date_input="",
                v5_eos_date_input="",
                hotfix_exception=True,
                hotfix_reason_input="",
                release_notes_input="",
                tag_exists_fn=lambda tag: True,
                tag_commit_fn=lambda tag: "abc123",
                head_descends_from_fn=lambda commit: True,
                tag_created_unix_fn=lambda tag: 100,
                now_unix_fn=lambda: 100 + (2 * 3600),
            )

    def test_stable_rejects_short_soak_without_hotfix(self) -> None:
        with self.assertRaisesRegex(ValueError, "minimum is 72 hours unless hotfix_exception is true"):
            resolver.resolve_metadata(
                version="6.0.1",
                promoted_from_tag_input="6.0.1-rc.1",
                rollback_version_input="6.0.0",
                ga_date_input="",
                v5_eos_date_input="",
                hotfix_exception=False,
                hotfix_reason_input="",
                release_notes_input="",
                tag_exists_fn=lambda tag: True,
                tag_commit_fn=lambda tag: "abc123",
                head_descends_from_fn=lambda commit: True,
                tag_created_unix_fn=lambda tag: 100,
                now_unix_fn=lambda: 100 + (2 * 3600),
            )


if __name__ == "__main__":
    unittest.main()
