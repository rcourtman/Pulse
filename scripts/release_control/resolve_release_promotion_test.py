#!/usr/bin/env python3
"""Unit tests for shared release-promotion metadata resolution."""

from __future__ import annotations

from pathlib import Path
import unittest
from unittest.mock import patch
import json
import subprocess

import resolve_release_promotion as resolver

REPO_ROOT = Path(__file__).resolve().parents[2]


class ResolveReleasePromotionTest(unittest.TestCase):
    def test_candidate_clock_uses_exact_release_publication(self) -> None:
        payload = dict(tagName="v6.5.0-rc.2", isDraft=False,
                       isPrerelease=True, publishedAt="2026-09-05T00:00:00Z")
        with patch.object(resolver.subprocess, "run") as run:
            run.return_value.stdout = json.dumps(payload)
            self.assertEqual(resolver.release_published_unix("v6.5.0-rc.2"),
                             1788566400)
            self.assertEqual(run.call_args.args[0][:4],
                             ["gh", "release", "view", "v6.5.0-rc.2"])
            self.assertEqual(run.call_count, 1)

    def test_candidate_clock_fails_closed_without_publication(self) -> None:
        payload = dict(tagName="v6.5.0-rc.2", isDraft=False,
                       isPrerelease=True, publishedAt="2026-09-05T00:00:00Z")
        for change in (dict(isDraft=True), dict(isPrerelease=False),
                       dict(publishedAt=None), dict(tagName="v6.5.0-rc.1"),
                       dict(publishedAt="2026-09-05T00:00:00")):
            with self.subTest(change=change), patch.object(resolver.subprocess, "run") as run:
                run.return_value.stdout = json.dumps(payload | change)
                with self.assertRaises(ValueError):
                    resolver.release_published_unix("v6.5.0-rc.2")
        with patch.object(resolver.subprocess, "run", side_effect=subprocess.CalledProcessError(1, "gh")):
            with self.assertRaises(subprocess.CalledProcessError):
                resolver.release_published_unix("v6.5.0-rc.2")

    def test_published_release_versions_use_explicit_maturity_stages(self) -> None:
        for version, expected in (
            ("6.5.0-alpha.1", "alpha"),
            ("6.5.0-beta.2", "beta"),
            ("6.5.0-rc.3", "rc"),
            ("6.5.0", "stable"),
        ):
            with self.subTest(version=version):
                self.assertEqual(resolver.release_stage(version), expected)

        for unsupported in ("6.5.0-dev", "6.5.0-preview.1", "6.5.0-beta.0"):
            with self.subTest(unsupported=unsupported):
                with self.assertRaisesRegex(ValueError, "Unsupported release version"):
                    resolver.release_stage(unsupported)

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
        self.assertEqual(metadata["rollback_command"], "sudo /bin/update --version v5.1.14")
        self.assertEqual(metadata["promoted_from_tag"], "")
        self.assertEqual(metadata["soak_hours"], "")
        self.assertEqual(metadata["release_stage"], "rc")

    def test_beta_uses_the_published_prerelease_path(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.5.0-beta.1",
            promoted_from_tag_input="",
            rollback_version_input="6.4.1",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=False,
            hotfix_reason_input="",
            release_notes_input="",
            tag_exists_fn=lambda tag: tag == "v6.4.1",
        )
        self.assertEqual(metadata["release_stage"], "beta")
        self.assertEqual(metadata["promotion_mode"], "prerelease")
        self.assertEqual(metadata["promoted_from_tag"], "")

    def test_first_rc_has_no_observation_window_to_wait_for(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.4.0-rc.1",
            promoted_from_tag_input="",
            rollback_version_input="6.3.2",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=False,
            hotfix_reason_input="",
            release_notes_input="",
            enforce_prerelease_observation_window=True,
            list_published_prereleases_fn=lambda: [
                ("v6.3.0-rc.8", 100),
                ("v6.5.0-rc.1", 200),
            ],
            tag_exists_fn=lambda tag: tag == "v6.3.2",
            now_unix_fn=lambda: 10_000,
        )
        self.assertEqual(metadata["previous_prerelease_tag"], "")
        self.assertEqual(metadata["prerelease_observation_hours"], "")

    def test_same_version_rc_is_rejected_inside_observation_window(self) -> None:
        with self.assertRaisesRegex(
            ValueError,
            "after only 23 hours of public observation.*require 24 hours",
        ):
            resolver.resolve_metadata(
                version="6.4.0-rc.13",
                promoted_from_tag_input="",
                rollback_version_input="6.3.2",
                ga_date_input="",
                v5_eos_date_input="",
                hotfix_exception=False,
                hotfix_reason_input="",
                release_notes_input="",
                enforce_prerelease_observation_window=True,
                list_published_prereleases_fn=lambda: [
                    ("v6.4.0-rc.11", 100),
                    ("v6.4.0-rc.12", 200),
                ],
                tag_exists_fn=lambda tag: tag == "v6.3.2",
                now_unix_fn=lambda: 200 + (23 * 3600) + 3599,
            )

    def test_same_version_rc_is_allowed_after_observation_window(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.4.0-rc.13",
            promoted_from_tag_input="",
            rollback_version_input="6.3.2",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=False,
            hotfix_reason_input="",
            release_notes_input="",
            enforce_prerelease_observation_window=True,
            list_published_prereleases_fn=lambda: [
                ("v6.3.0-rc.8", 300),
                ("v6.4.0-rc.12", 200),
            ],
            tag_exists_fn=lambda tag: tag == "v6.3.2",
            now_unix_fn=lambda: 200 + (24 * 3600),
        )
        self.assertEqual(metadata["previous_prerelease_tag"], "v6.4.0-rc.12")
        self.assertEqual(metadata["prerelease_observation_hours"], "24")

    def test_observation_window_is_scoped_to_the_same_maturity_stage(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.5.0-rc.1",
            promoted_from_tag_input="",
            rollback_version_input="6.4.1",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=False,
            hotfix_reason_input="",
            release_notes_input="",
            enforce_prerelease_observation_window=True,
            list_published_prereleases_fn=lambda: [
                ("v6.5.0-alpha.2", 100),
                ("v6.5.0-beta.4", 200),
            ],
            tag_exists_fn=lambda tag: tag == "v6.4.1",
            now_unix_fn=lambda: 201,
        )
        self.assertEqual(metadata["release_stage"], "rc")
        self.assertEqual(metadata["previous_prerelease_tag"], "")

    def test_rehearsal_does_not_query_or_enforce_publication_window(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.4.0-rc.13",
            promoted_from_tag_input="",
            rollback_version_input="6.3.2",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=False,
            hotfix_reason_input="",
            release_notes_input="",
            enforce_prerelease_observation_window=False,
            list_published_prereleases_fn=lambda: self.fail("publication lookup should not run"),
            tag_exists_fn=lambda tag: tag == "v6.3.2",
        )
        self.assertEqual(metadata["previous_prerelease_tag"], "")

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
        self.assertEqual(metadata["rollback_command"], "sudo /bin/update --version v6.0.4")

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
            release_published_unix_fn=lambda tag: 100,
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
            release_published_unix_fn=lambda tag: 100,
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

    def test_v612_owner_exception_allows_disclosed_emergency_patch(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.1.2",
            promoted_from_tag_input="",
            rollback_version_input="v6.1.1",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=True,
            hotfix_reason_input="Active customer-impact fixes.",
            release_notes_input=(
                "Windows Unified Agent binaries are not Authenticode-signed for v6.1.2."
            ),
            unsigned_windows_exception=True,
            unsigned_windows_reason_input=(
                "SignPath company verification is still processing; the release owner accepts "
                "unsigned Windows binaries for v6.1.2."
            ),
            list_stable_tags_fn=lambda: ["v6.1.1", "v6.1.0"],
            list_same_version_rc_tags_fn=lambda version: [],
            changed_paths_fn=lambda tag: ["scripts/install.sh"],
            tag_exists_fn=lambda tag: tag == "v6.1.1",
            tag_commit_fn=lambda tag: "v611-commit",
            head_descends_from_fn=lambda commit: commit == "v611-commit",
        )

        self.assertEqual(metadata["promotion_mode"], "emergency-stable-patch")
        self.assertEqual(metadata["rollback_tag"], "v6.1.1")
        self.assertEqual(metadata["require_windows_signing"], "false")
        self.assertEqual(metadata["unsigned_windows_exception"], "true")

    def test_v620_owner_exception_allows_disclosed_stable_promotion(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.2.0",
            promoted_from_tag_input="v6.2.0-rc.11",
            rollback_version_input="v6.1.2",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=True,
            hotfix_reason_input="Owner waived the remaining prerelease soak.",
            release_notes_input=(
                "Windows Unified Agent binaries are not Authenticode-signed for v6.2.0."
            ),
            unsigned_windows_exception=True,
            unsigned_windows_reason_input=(
                "The release certificate CSR remains pending; the release owner accepts "
                "unsigned Windows binaries for v6.2.0."
            ),
            tag_exists_fn=lambda tag: tag in {"v6.2.0-rc.11", "v6.1.2"},
            tag_commit_fn=lambda tag: "rc11-commit",
            head_descends_from_fn=lambda commit: commit == "rc11-commit",
            release_published_unix_fn=lambda tag: 100,
            now_unix_fn=lambda: 100 + (13 * 3600),
        )

        self.assertEqual(metadata["promotion_mode"], "stable-rc-promotion")
        self.assertEqual(metadata["rollback_tag"], "v6.1.2")
        self.assertEqual(metadata["require_windows_signing"], "false")
        self.assertEqual(metadata["unsigned_windows_exception"], "true")

    def test_v621_owner_exception_allows_disclosed_emergency_patch(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.2.1",
            promoted_from_tag_input="",
            rollback_version_input="v6.2.0",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=True,
            hotfix_reason_input="Active customer-impact fixes.",
            release_notes_input=(
                "Windows Unified Agent binaries are not Authenticode-signed for v6.2.1."
            ),
            unsigned_windows_exception=True,
            unsigned_windows_reason_input=(
                "The release certificate CSR remains pending; the release owner accepts "
                "unsigned Windows binaries for v6.2.1."
            ),
            list_stable_tags_fn=lambda: ["v6.2.0", "v6.1.2"],
            list_same_version_rc_tags_fn=lambda version: [],
            changed_paths_fn=lambda tag: ["scripts/install.sh"],
            tag_exists_fn=lambda tag: tag == "v6.2.0",
            tag_commit_fn=lambda tag: "v620-commit",
            head_descends_from_fn=lambda commit: commit == "v620-commit",
        )

        self.assertEqual(metadata["promotion_mode"], "emergency-stable-patch")
        self.assertEqual(metadata["rollback_tag"], "v6.2.0")
        self.assertEqual(metadata["require_windows_signing"], "false")
        self.assertEqual(metadata["unsigned_windows_exception"], "true")

    def test_v630_owner_exception_allows_disclosed_stable_promotion(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.3.0",
            promoted_from_tag_input="v6.3.0-rc.6",
            rollback_version_input="v6.2.1",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=True,
            hotfix_reason_input="Owner accepted the shortened prerelease soak.",
            release_notes_input=(
                "Windows Unified Agent binaries are not Authenticode-signed for v6.3.0."
            ),
            unsigned_windows_exception=True,
            unsigned_windows_reason_input=(
                "Windows signing is not yet available; the release owner accepts unsigned "
                "Windows binaries for v6.3.0."
            ),
            tag_exists_fn=lambda tag: tag in {"v6.3.0-rc.6", "v6.2.1"},
            tag_commit_fn=lambda tag: "rc6-commit",
            head_descends_from_fn=lambda commit: commit == "rc6-commit",
            release_published_unix_fn=lambda tag: 100,
            now_unix_fn=lambda: 100 + (24 * 3600),
        )

        self.assertEqual(metadata["promotion_mode"], "stable-rc-promotion")
        self.assertEqual(metadata["rollback_tag"], "v6.2.1")
        self.assertEqual(metadata["require_windows_signing"], "false")
        self.assertEqual(metadata["unsigned_windows_exception"], "true")

    def test_v631_owner_exception_allows_disclosed_emergency_patch(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.3.1",
            promoted_from_tag_input="",
            rollback_version_input="v6.3.0",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=True,
            hotfix_reason_input="Active customer-impact fixes.",
            release_notes_input=(
                "Windows Unified Agent binaries are not Authenticode-signed for v6.3.1."
            ),
            unsigned_windows_exception=True,
            unsigned_windows_reason_input=(
                "The SignPath production certificate remains CSR pending; the release owner "
                "accepts unsigned Windows binaries for v6.3.1."
            ),
            list_stable_tags_fn=lambda: ["v6.3.0", "v6.2.1"],
            list_same_version_rc_tags_fn=lambda version: [],
            changed_paths_fn=lambda tag: ["scripts/install.sh"],
            tag_exists_fn=lambda tag: tag == "v6.3.0",
            tag_commit_fn=lambda tag: "v630-commit",
            head_descends_from_fn=lambda commit: commit == "v630-commit",
        )

        self.assertEqual(metadata["promotion_mode"], "emergency-stable-patch")
        self.assertEqual(metadata["rollback_tag"], "v6.3.0")
        self.assertEqual(metadata["require_windows_signing"], "false")
        self.assertEqual(metadata["unsigned_windows_exception"], "true")

    def test_v632_standing_unavailable_policy_allows_disclosed_emergency_patch(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.3.2",
            promoted_from_tag_input="",
            rollback_version_input="v6.3.1",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=True,
            hotfix_reason_input=(
                "Metrics memory growth can wedge stable and disabled offline policies emit alert noise."
            ),
            release_notes_input=(
                "Windows Unified Agent binaries are not Authenticode-signed for v6.3.2."
            ),
            unsigned_windows_exception=False,
            unsigned_windows_reason_input="",
            list_stable_tags_fn=lambda: ["v6.3.1", "v6.3.0"],
            list_same_version_rc_tags_fn=lambda version: [],
            changed_paths_fn=lambda tag: ["internal/monitoring/metrics_history.go"],
            tag_exists_fn=lambda tag: tag == "v6.3.1",
            tag_commit_fn=lambda tag: "v631-commit",
            head_descends_from_fn=lambda commit: commit == "v631-commit",
        )

        self.assertEqual(metadata["promotion_mode"], "emergency-stable-patch")
        self.assertEqual(metadata["rollback_tag"], "v6.3.1")
        self.assertEqual(metadata["require_windows_signing"], "false")
        self.assertEqual(metadata["unsigned_windows_exception"], "true")
        self.assertIn("until availability is explicitly restored", metadata["unsigned_windows_reason"])

    def test_future_stable_release_stays_unsigned_until_authenticode_is_restored(self) -> None:
        metadata = resolver.resolve_metadata(
            version="6.4.0",
            promoted_from_tag_input="v6.4.0-rc.1",
            rollback_version_input="v6.3.2",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=True,
            hotfix_reason_input="Release owner approved stable promotion.",
            release_notes_input=(
                "Windows Unified Agent binaries are not Authenticode-signed for v6.4.0."
            ),
            list_stable_tags_fn=lambda: ["v6.3.2", "v6.3.1"],
            tag_exists_fn=lambda tag: tag in {"v6.4.0-rc.1", "v6.3.2"},
            tag_commit_fn=lambda tag: "rc-commit" if tag == "v6.4.0-rc.1" else "v632-commit",
            head_descends_from_fn=lambda commit: True,
            release_published_unix_fn=lambda tag: 100,
            now_unix_fn=lambda: 100 + (73 * 3600),
        )

        self.assertEqual(metadata["require_windows_signing"], "false")
        self.assertEqual(metadata["unsigned_windows_exception"], "true")

    def test_unsigned_windows_exception_is_rejected_for_other_stable_versions(self) -> None:
        with self.assertRaisesRegex(
            ValueError,
            "approved only for stable v6.1.0, v6.1.1, v6.1.2, v6.2.0, v6.2.1, v6.3.0, v6.3.1, or v6.3.2",
        ):
            resolver.resolve_metadata(
                version="6.4.0",
                promoted_from_tag_input="",
                rollback_version_input="v6.3.2",
                ga_date_input="",
                v5_eos_date_input="",
                hotfix_exception=True,
                hotfix_reason_input="Emergency patch.",
                release_notes_input="Windows binaries are not Authenticode-signed.",
                unsigned_windows_exception=True,
                unsigned_windows_reason_input="Not approved for this version.",
                windows_authenticode_available=True,
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
            "release_published_unix_fn": lambda tag: 100,
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
                release_published_unix_fn=lambda tag: 100,
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
            release_published_unix_fn=lambda tag: 100,
            now_unix_fn=lambda: 100 + (163 * 3600),
        )

        self.assertEqual(metadata["promoted_from_tag"], "v6.0.0-rc.7")
        self.assertEqual(metadata["rollback_tag"], "v5.1.35")
        self.assertEqual(metadata["rollback_command"], "sudo /bin/update --version v5.1.35")
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
        self.assertEqual(metadata["rollback_command"], "sudo /bin/update --version v6.0.1")
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
                release_published_unix_fn=lambda tag: 100,
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
                release_published_unix_fn=lambda tag: 100,
                now_unix_fn=lambda: 100 + (2 * 3600),
            )


class ReleaseTrainPromotionTest(unittest.TestCase):
    """The release train: a stable ships its soaked candidate, and minors soak a week."""

    def promote(self, version: str, **overrides):
        promoted = f"{version}-rc.1"
        arguments = dict(
            version=version,
            promoted_from_tag_input=promoted,
            rollback_version_input="6.4.1",
            ga_date_input="",
            v5_eos_date_input="",
            hotfix_exception=False,
            hotfix_reason_input="",
            release_notes_input="",
            tag_exists_fn=lambda tag: tag in {f"v{promoted}", "v6.4.1"},
            tag_commit_fn=lambda tag: "abc123",
            head_descends_from_fn=lambda commit: commit == "abc123",
            release_published_unix_fn=lambda tag: 100,
            now_unix_fn=lambda: 100 + (168 * 3600),
            changed_paths_fn=lambda base_tag: [
                "VERSION",
                "deploy/helm/pulse/Chart.yaml",
                "docs/RELEASE_NOTES.md",
                "docs/releases/RELEASE_NOTES_v6.5.0.md",
                "docs/release-control/v6/internal/status.json",
            ],
        )
        arguments.update(overrides)
        return resolver.resolve_metadata(**arguments)

    def test_release_metadata_paths_are_the_only_allowed_drift(self) -> None:
        for path in (
            "VERSION",
            "deploy/helm/pulse/Chart.yaml",
            "deploy/helm/pulse/README.md",
            "docker-compose.yml",
            "docs/RELEASE_NOTES.md",
            "docs/UPGRADE_v6.md",
            "frontend-modern/public/docs/UPGRADE_v6.md",
            "docs/releases/V6_CHANGELOG_v6.5.0.md",
            "docs/release-control/v6/internal/records/v6.5.0-ga.md",
            "docs/release-control/v6/internal/status.json",
            "docs/release-control/v6/internal/subsystems/deployment-installability.md",
        ):
            with self.subTest(path=path):
                self.assertIsNotNone(resolver.RELEASE_METADATA_PATH_RE.match(path))
        for path in (
            "internal/api/router.go",
            "frontend-modern/src/App.tsx",
            ".github/workflows/create-release.yml",
            "docs/TRUENAS.md",
            "docs/release-control/v6/internal/RELEASE_PROMOTION_POLICY.md",
            "scripts/install.sh",
        ):
            with self.subTest(path=path):
                self.assertIsNone(resolver.RELEASE_METADATA_PATH_RE.match(path))

    def test_minor_promotion_ships_exactly_the_soaked_candidate(self) -> None:
        metadata = self.promote("6.5.0")
        self.assertEqual(metadata["promoted_from_tag"], "v6.5.0-rc.1")
        self.assertEqual(metadata["soak_hours"], "168")

    def test_content_the_candidate_never_soaked_is_refused(self) -> None:
        drift = [
            "VERSION",
            "internal/api/router.go",
            "frontend-modern/src/features/proxmox/ProxmoxBackupServersTable.tsx",
        ]
        with self.assertRaisesRegex(ValueError, "never soaked.*2 paths beyond release metadata"):
            self.promote("6.5.0", changed_paths_fn=lambda base_tag: drift)
        with self.assertRaisesRegex(ValueError, "never soaked"):
            self.promote("6.5.1", changed_paths_fn=lambda base_tag: drift)

    def test_hotfix_exception_still_requires_a_reason_for_drift(self) -> None:
        drift = ["VERSION", "internal/api/router.go"]
        with self.assertRaisesRegex(ValueError, "hotfix_reason is required"):
            self.promote("6.5.1", hotfix_exception=True, changed_paths_fn=lambda base_tag: drift)
        metadata = self.promote(
            "6.5.1",
            hotfix_exception=True,
            hotfix_reason_input="Active customer harm: agents cannot re-enrol after upgrade.",
            changed_paths_fn=lambda base_tag: drift,
            now_unix_fn=lambda: 100 + (2 * 3600),
        )
        self.assertEqual(metadata["hotfix_exception"], "true")

    def test_repaired_candidate_restarts_full_publication_soak(self) -> None:
        for version, hours in (("6.5.0", 168), ("6.5.1", 72)):
            tag = f"v{version}-rc.2"
            observed = []
            def publication(candidate):
                observed.append(candidate)
                return 200
            arguments = dict(
                promoted_from_tag_input=tag,
                tag_exists_fn=lambda candidate: candidate in {tag, "v6.4.1"},
                release_published_unix_fn=publication,
            )
            with self.subTest(version=version):
                with self.assertRaisesRegex(ValueError, "hours"):
                    self.promote(version, now_unix_fn=lambda: 200 + hours * 3600 - 1,
                                 **arguments)
                metadata = self.promote(
                    version, now_unix_fn=lambda: 200 + hours * 3600, **arguments)
                self.assertEqual(metadata["soak_hours"], str(hours))
                self.assertEqual(observed, [tag, tag])

    def test_minor_releases_soak_seven_days_and_patches_seventy_two_hours(self) -> None:
        with self.assertRaisesRegex(ValueError, "release train requires 168 hours"):
            self.promote("6.5.0", now_unix_fn=lambda: 100 + (100 * 3600))
        metadata = self.promote("6.5.1", now_unix_fn=lambda: 100 + (73 * 3600))
        self.assertEqual(metadata["soak_hours"], "73")
        with self.assertRaisesRegex(ValueError, "minimum is 72 hours"):
            self.promote("6.5.1", now_unix_fn=lambda: 100 + (71 * 3600))


if __name__ == "__main__":
    unittest.main()
