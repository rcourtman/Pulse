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

    def test_missing_rollback_is_rejected_without_derivation(self) -> None:
        with self.assertRaisesRegex(
            ValueError,
            "rollback_version is required for every release rehearsal and promotion",
        ):
            resolver.resolve_metadata(
                version="6.0.5-rc.3",
                promoted_from_tag_input="",
                rollback_version_input="",
                ga_date_input="",
                v5_eos_date_input="",
                hotfix_exception=False,
                hotfix_reason_input="",
                release_notes_input="",
                tag_exists_fn=lambda tag: True,
            )

    def test_scheduled_rehearsal_derives_latest_preceding_stable_rollback(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.0.5-rc.3",
            promoted_from_tag_input="",
            rollback_version_input="",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=False,
            hotfix_reason_input="",
            release_notes_input="",
            derive_rollback_when_missing=True,
            list_stable_tags_fn=lambda: ["v5.1.35", "v6.0.1", "v6.0.4", "v6.0.2"],
            tag_exists_fn=lambda tag: True,
        )
        self.assertEqual(metadata["rollback_tag"], "v6.0.4")
        self.assertEqual(metadata["rollback_command"], "./scripts/install.sh --version v6.0.4")

    def test_explicit_rollback_input_wins_over_derivation(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.0.5-rc.3",
            promoted_from_tag_input="",
            rollback_version_input="6.0.3",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=False,
            hotfix_reason_input="",
            release_notes_input="",
            derive_rollback_when_missing=True,
            list_stable_tags_fn=lambda: ["v6.0.4"],
            tag_exists_fn=lambda tag: tag == "v6.0.3",
        )
        self.assertEqual(metadata["rollback_tag"], "v6.0.3")

    def test_derivation_never_selects_the_rehearsal_version_or_prereleases(self) -> None:
        self.assertEqual(
            resolver.derive_latest_stable_rollback_tag(
                "6.0.5",
                ["v6.0.5", "v6.0.5-rc.3", "v6.0.4", "v5.1.35"],
            ),
            "v6.0.4",
        )

    def test_derivation_requires_a_preceding_stable_tag(self) -> None:
        with self.assertRaisesRegex(ValueError, "no stable release tag precedes"):
            resolver.resolve_metadata(
                version="6.0.5-rc.3",
                promoted_from_tag_input="",
                rollback_version_input="",
                ga_date_input="",
                v5_eos_date_input="",
                hotfix_exception=False,
                hotfix_reason_input="",
                release_notes_input="",
                derive_rollback_when_missing=True,
                list_stable_tags_fn=lambda: [],
                tag_exists_fn=lambda tag: True,
            )

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
        self.assertEqual(metadata["require_windows_signing"], "true")
        self.assertEqual(metadata["unsigned_windows_exception"], "false")

    def test_v610_owner_exception_allows_disclosed_unsigned_windows_candidate(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.1.0",
            promoted_from_tag_input="v6.1.0-rc.4",
            rollback_version_input="v6.0.5",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=True,
            hotfix_reason_input="Owner waived the remaining prerelease soak.",
            release_notes_input=(
                "Windows Unified Agent binaries are not Authenticode-signed for v6.1.0."
            ),
            unsigned_windows_exception=True,
            unsigned_windows_reason_input=(
                "Release owner accepted the Windows unknown-publisher warning for v6.1.0."
            ),
            tag_exists_fn=lambda tag: tag in {"v6.1.0-rc.4", "v6.0.5"},
            tag_commit_fn=lambda tag: "rc4-commit",
            head_descends_from_fn=lambda commit: commit == "rc4-commit",
            tag_created_unix_fn=lambda tag: 100,
            now_unix_fn=lambda: 100 + (27 * 3600),
        )

        self.assertEqual(metadata["require_windows_signing"], "false")
        self.assertEqual(metadata["unsigned_windows_exception"], "true")
        self.assertEqual(
            metadata["unsigned_windows_reason"],
            "Release owner accepted the Windows unknown-publisher warning for v6.1.0.",
        )

    def test_v611_owner_exception_allows_disclosed_emergency_patch(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.1.1",
            promoted_from_tag_input="",
            rollback_version_input="v6.1.0",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=True,
            hotfix_reason_input="Active customer update harm.",
            release_notes_input=(
                "Windows Unified Agent binaries are not Authenticode-signed for v6.1.1."
            ),
            unsigned_windows_exception=True,
            unsigned_windows_reason_input=(
                "Release owner accepted the Windows unknown-publisher warning for v6.1.1."
            ),
            list_stable_tags_fn=lambda: ["v6.1.0", "v6.0.5"],
            list_same_version_rc_tags_fn=lambda version: [],
            changed_paths_fn=lambda tag: ["install.sh"],
            tag_exists_fn=lambda tag: tag == "v6.1.0",
            tag_commit_fn=lambda tag: "v610-commit",
            head_descends_from_fn=lambda commit: commit == "v610-commit",
        )

        self.assertEqual(metadata["promotion_mode"], "emergency-stable-patch")
        self.assertEqual(metadata["rollback_tag"], "v6.1.0")
        self.assertEqual(metadata["require_windows_signing"], "false")
        self.assertEqual(metadata["unsigned_windows_exception"], "true")

    def test_unsigned_windows_exception_is_rejected_for_other_stable_versions(self) -> None:
        with self.assertRaisesRegex(ValueError, "approved only for stable v6.1.0 or v6.1.1"):
            resolver.resolve_metadata(
                version="6.1.2",
                promoted_from_tag_input="",
                rollback_version_input="v6.1.1",
                ga_date_input="",
                v5_eos_date_input="",
                hotfix_exception=True,
                hotfix_reason_input="Emergency patch.",
                release_notes_input="Windows binaries are not Authenticode-signed.",
                unsigned_windows_exception=True,
                unsigned_windows_reason_input="Not approved for this version.",
                tag_exists_fn=lambda tag: True,
            )

    def test_unsigned_windows_exception_requires_reason_and_release_note_disclosure(self) -> None:
        common = {
            "version": "6.1.0",
            "promoted_from_tag_input": "v6.1.0-rc.4",
            "rollback_version_input": "v6.0.5",
            "ga_date_input": "",
            "v5_eos_date_input": "",
            "hotfix_exception": True,
            "hotfix_reason_input": "Owner waived the remaining prerelease soak.",
            "unsigned_windows_exception": True,
            "tag_exists_fn": lambda tag: True,
            "tag_commit_fn": lambda tag: "rc4-commit",
            "head_descends_from_fn": lambda commit: True,
            "tag_created_unix_fn": lambda tag: 100,
            "now_unix_fn": lambda: 100 + (27 * 3600),
        }
        with self.assertRaisesRegex(ValueError, "unsigned_windows_reason is required"):
            resolver.resolve_metadata(
                **common,
                release_notes_input="Windows binaries are not Authenticode-signed.",
                unsigned_windows_reason_input="",
            )
        with self.assertRaisesRegex(ValueError, "must disclose"):
            resolver.resolve_metadata(
                **common,
                release_notes_input="Windows agent details omitted.",
                unsigned_windows_reason_input="Owner accepted the warning.",
            )

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
            list_stable_tags_fn=lambda: ["v6.0.1"],
            list_same_version_rc_tags_fn=lambda version: ["v6.0.2-rc.1"],
            changed_paths_fn=lambda tag: ["internal/api/auth.go"],
            tag_exists_fn=lambda tag: tag == "v6.0.1",
            tag_commit_fn=lambda tag: "rollback-commit",
            head_descends_from_fn=lambda commit: commit == "rollback-commit",
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
        self.assertEqual(metadata["promotion_mode"], "emergency-stable-patch")

    def test_routine_stable_patch_can_omit_rc_ceremony(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.0.2",
            promoted_from_tag_input="",
            rollback_version_input="6.0.1",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=False,
            hotfix_reason_input="",
            release_notes_input="bounded customer fixes",
            list_stable_tags_fn=lambda: ["v5.1.35", "v6.0.1"],
            list_same_version_rc_tags_fn=lambda version: [],
            changed_paths_fn=lambda tag: ["frontend-modern/src/features/settings/Settings.tsx"],
            tag_exists_fn=lambda tag: tag == "v6.0.1",
            tag_commit_fn=lambda tag: "rollback-commit",
            head_descends_from_fn=lambda commit: commit == "rollback-commit",
        )

        self.assertEqual(metadata["promotion_mode"], "routine-stable-patch")
        self.assertEqual(metadata["is_stable_patch"], "true")
        self.assertEqual(metadata["rollback_tag"], "v6.0.1")
        self.assertEqual(metadata["hotfix_exception"], "false")

    def test_routine_stable_patch_requires_latest_stable_rollback(self) -> None:
        with self.assertRaisesRegex(ValueError, "latest preceding stable tag v6.0.1"):
            resolver.resolve_metadata(
                version="6.0.2",
                promoted_from_tag_input="",
                rollback_version_input="6.0.0",
                ga_date_input="",
                v5_eos_date_input="",
                hotfix_exception=False,
                hotfix_reason_input="",
                release_notes_input="bounded customer fixes",
                list_stable_tags_fn=lambda: ["v6.0.0", "v6.0.1"],
                list_same_version_rc_tags_fn=lambda version: [],
                changed_paths_fn=lambda tag: [],
                tag_exists_fn=lambda tag: True,
            )

    def test_routine_stable_patch_requires_rc_for_risk_changes(self) -> None:
        with self.assertRaisesRegex(ValueError, "RC-required runtime changes"):
            resolver.resolve_metadata(
                version="6.0.2",
                promoted_from_tag_input="",
                rollback_version_input="6.0.1",
                ga_date_input="",
                v5_eos_date_input="",
                hotfix_exception=False,
                hotfix_reason_input="",
                release_notes_input="authentication correction",
                list_stable_tags_fn=lambda: ["v6.0.1"],
                list_same_version_rc_tags_fn=lambda version: [],
                changed_paths_fn=lambda tag: ["internal/api/auth.go"],
                tag_exists_fn=lambda tag: True,
                tag_commit_fn=lambda tag: "rollback-commit",
                head_descends_from_fn=lambda commit: True,
            )

    def test_routine_stable_patch_requires_rc_when_candidate_exists(self) -> None:
        with self.assertRaisesRegex(ValueError, "same-version release candidates already exist"):
            resolver.resolve_metadata(
                version="6.0.2",
                promoted_from_tag_input="",
                rollback_version_input="6.0.1",
                ga_date_input="",
                v5_eos_date_input="",
                hotfix_exception=False,
                hotfix_reason_input="",
                release_notes_input="bounded customer fixes",
                list_stable_tags_fn=lambda: ["v6.0.1"],
                list_same_version_rc_tags_fn=lambda version: ["v6.0.2-rc.1"],
                changed_paths_fn=lambda tag: [],
                tag_exists_fn=lambda tag: True,
                tag_commit_fn=lambda tag: "rollback-commit",
                head_descends_from_fn=lambda commit: True,
            )

    def test_routine_patch_risk_classifier_covers_governed_categories(self) -> None:
        risks = resolver.classify_routine_patch_risks(
            [
                "internal/api/auth.go",
                "pkg/licensing/license.go",
                "internal/storage/schema.go",
                "internal/relay/client.go",
                "internal/updates/apply.go",
                "frontend-modern/src/App.tsx",
            ]
        )
        self.assertEqual(len(risks), 5)
        self.assertFalse(any("frontend-modern/src/App.tsx" in risk for risk in risks))

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
                list_stable_tags_fn=lambda: ["v6.0.1"],
                list_same_version_rc_tags_fn=lambda version: [],
                changed_paths_fn=lambda tag: [],
                tag_exists_fn=lambda tag: tag == "v6.0.1",
                tag_commit_fn=lambda tag: "rollback-commit",
                head_descends_from_fn=lambda commit: True,
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
