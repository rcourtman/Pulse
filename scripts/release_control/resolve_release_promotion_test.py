#!/usr/bin/env python3
"""Unit tests for shared release-promotion metadata resolution."""

from __future__ import annotations

from pathlib import Path
import unittest

import resolve_release_promotion as resolver

REPO_ROOT = Path(__file__).resolve().parents[2]


class ResolveReleasePromotionTest(unittest.TestCase):
    def test_dev_prerelease_uses_prerelease_path(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.0.0-dev",
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
        self.assertEqual(metadata["promoted_from_tag"], "")
        self.assertEqual(metadata["soak_hours"], "")

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

    def test_current_stable_v6_packet_resolves_with_publish_dates(self) -> None:
        release_notes = (REPO_ROOT / "docs/releases/RELEASE_NOTES_v6.md").read_text(encoding="utf-8")
        metadata = resolver.resolve_metadata(
            version="6.0.0",
            promoted_from_tag_input="v6.0.0-rc.7",
            rollback_version_input="v5.1.35",
            ga_date_input="2026-07-04",
            v5_eos_date_input="2026-10-02",
            hotfix_exception=False,
            hotfix_reason_input="",
            release_notes_input=release_notes,
            tag_exists_fn=lambda tag: tag in {"v6.0.0-rc.7", "v5.1.35"},
            tag_commit_fn=lambda tag: "rc7-commit",
            head_descends_from_fn=lambda commit: commit == "rc7-commit",
            tag_created_unix_fn=lambda tag: 100,
            now_unix_fn=lambda: 100 + (163 * 3600),
        )

        self.assertEqual(metadata["promoted_from_tag"], "v6.0.0-rc.7")
        self.assertEqual(metadata["rollback_tag"], "v5.1.35")
        self.assertEqual(metadata["rollback_command"], "./scripts/install.sh --version v5.1.35")
        self.assertEqual(metadata["ga_date"], "2026-07-04")
        self.assertEqual(metadata["v5_eos_date"], "2026-10-02")

    def test_stable_patch_hotfix_can_omit_promoted_prerelease(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.0.2",
            promoted_from_tag_input="",
            rollback_version_input="6.0.1",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=True,
            hotfix_reason_input="Patch release for v6.0.1 agent upgrade recovery.",
            release_notes_input="",
            tag_exists_fn=lambda tag: tag == "v6.0.1",
        )

        self.assertEqual(metadata["promoted_from_tag"], "")
        self.assertEqual(metadata["rollback_tag"], "v6.0.1")
        self.assertEqual(metadata["rollback_command"], "./scripts/install.sh --version v6.0.1")
        self.assertEqual(metadata["hotfix_exception"], "true")
        self.assertEqual(
            metadata["hotfix_reason"],
            "Patch release for v6.0.1 agent upgrade recovery.",
        )
        self.assertEqual(metadata["soak_hours"], "")

    def test_stable_patch_without_promoted_tag_requires_hotfix_exception(self) -> None:
        with self.assertRaisesRegex(ValueError, "Stable promotion requires promoted_from_tag"):
            resolver.resolve_metadata(
                version="6.0.2",
                promoted_from_tag_input="",
                rollback_version_input="6.0.1",
                ga_date_input="",
                v5_eos_date_input="",
                hotfix_exception=False,
                hotfix_reason_input="",
                release_notes_input="",
                tag_exists_fn=lambda tag: tag == "v6.0.1",
            )

    def test_stable_patch_hotfix_without_promoted_tag_requires_reason(self) -> None:
        with self.assertRaisesRegex(ValueError, "hotfix_reason is required"):
            resolver.resolve_metadata(
                version="6.0.2",
                promoted_from_tag_input="",
                rollback_version_input="6.0.1",
                ga_date_input="",
                v5_eos_date_input="",
                hotfix_exception=True,
                hotfix_reason_input="",
                release_notes_input="",
                tag_exists_fn=lambda tag: tag == "v6.0.1",
            )

    def test_stable_hotfix_requires_reason(self) -> None:
        with self.assertRaisesRegex(ValueError, "hotfix_reason is required"):
            resolver.resolve_metadata(
                version="6.0.2",
                promoted_from_tag_input="6.0.2-rc.1",
                rollback_version_input="6.0.1",
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
        with self.assertRaisesRegex(ValueError, "hours of prerelease soak"):
            resolver.resolve_metadata(
                version="6.0.2",
                promoted_from_tag_input="6.0.2-rc.1",
                rollback_version_input="6.0.1",
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
