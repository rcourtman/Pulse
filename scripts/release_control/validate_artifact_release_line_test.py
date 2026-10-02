#!/usr/bin/env python3
"""Unit tests for artifact release-line validation."""

from __future__ import annotations

import unittest

import validate_artifact_release_line as validator


class ValidateArtifactReleaseLineTest(unittest.TestCase):
    def test_published_646_packet_uses_its_selected_line_not_main(self) -> None:
        # This is the real control-plane lookup used by convergence, not an
        # injected branch answer. The frozen tag need not be an ancestor of main.
        source = "1abc97cbc1fd7aefa052b4607a531af3e3b97eb6"
        fetched = []
        refs = {"origin/release/v6.4": "line", "origin/main": "main"}
        result = validator.validate_artifact_release_line(
            tag="v6.4.6-rc.1", purpose="floating-tag promotion",
            fetch_refs_fn=fetched.append,
            tag_exists_fn=lambda tag: tag == "v6.4.6-rc.1",
            tag_commit_fn=lambda tag: source,
            ref_commit_fn=refs.__getitem__,
            ref_is_ancestor_fn=lambda a, b: (a, b) == (source, "line"),
        )
        self.assertEqual(fetched, ["release/v6.4"])
        self.assertEqual(result["required_branch"], "release/v6.4")
        with self.assertRaisesRegex(ValueError, "not reachable from origin/release/v6.4"):
            validator.validate_artifact_release_line(
                tag="v6.4.6-rc.1", purpose="floating-tag promotion",
                fetch_refs_fn=lambda branch: None,
                tag_exists_fn=lambda tag: True,
                tag_commit_fn=lambda tag: "unrelated",
                ref_commit_fn=refs.__getitem__,
                ref_is_ancestor_fn=lambda a, b: False,
            )

    def validate(
        self,
        *,
        tag: str,
        existing_tags: set[str],
        commits: dict[str, str],
        ancestors: set[tuple[str, str]],
        prerelease_tags: tuple[str, ...] = (),
        anticipated_source_sha: str = "",
    ) -> dict[str, str]:
        return validator.validate_artifact_release_line(
            tag=tag,
            purpose="test publish",
            anticipated_source_sha=anticipated_source_sha,
            branch_for_version_fn=lambda version: "pulse/v6-release",
            fetch_refs_fn=lambda required_branch: None,
            tag_exists_fn=lambda candidate: candidate in existing_tags,
            tag_commit_fn=lambda candidate: commits[candidate],
            ref_commit_fn=lambda ref: commits[ref],
            ref_is_ancestor_fn=lambda ancestor, descendant: (ancestor, descendant) in ancestors,
            prerelease_tags_fn=lambda version: prerelease_tags,
        )

    def test_stable_ga_requires_matching_prerelease_lineage(self) -> None:
        result = self.validate(
            tag="v6.0.0",
            existing_tags={"v6.0.0", "v6.0.0-rc.7"},
            commits={
                "v6.0.0": "ga",
                "v6.0.0-rc.7": "rc7",
                "origin/pulse/v6-release": "branch",
            },
            ancestors={("ga", "branch"), ("rc7", "ga")},
            prerelease_tags=("v6.0.0-rc.7",),
        )

        self.assertEqual(result["lineage"], "promoted_prerelease")
        self.assertEqual(result["lineage_tag"], "v6.0.0-rc.7")

    def test_stable_patch_can_follow_previous_stable_without_fabricated_rc(self) -> None:
        result = self.validate(
            tag="v6.0.1",
            existing_tags={"v6.0.0", "v6.0.1"},
            commits={
                "v6.0.0": "ga",
                "v6.0.1": "patch",
                "origin/pulse/v6-release": "branch",
            },
            ancestors={("patch", "branch"), ("ga", "patch")},
        )

        self.assertEqual(result["lineage"], "stable_patch")
        self.assertEqual(result["lineage_tag"], "v6.0.0")

    def test_prerelease_support_tag_does_not_require_stable_lineage(self) -> None:
        result = self.validate(
            tag="v6.0.4-rc.2",
            existing_tags={"v6.0.4-rc.2"},
            commits={
                "v6.0.4-rc.2": "support-rc",
                "origin/pulse/v6-release": "branch",
            },
            ancestors={("support-rc", "branch")},
        )

        self.assertEqual(result["lineage"], "prerelease")
        self.assertEqual(result["lineage_tag"], "")

    def test_stable_patch_rejects_without_previous_stable_or_rc(self) -> None:
        with self.assertRaisesRegex(ValueError, "previous stable tag v6.0.0"):
            self.validate(
                tag="v6.0.1",
                existing_tags={"v6.0.1"},
                commits={
                    "v6.0.1": "patch",
                    "origin/pulse/v6-release": "branch",
                },
                ancestors={("patch", "branch")},
            )

    def test_cross_line_tag_is_rejected_before_lineage(self) -> None:
        with self.assertRaisesRegex(ValueError, "Refusing test publish"):
            self.validate(
                tag="v6.0.1",
                existing_tags={"v6.0.0", "v6.0.1"},
                commits={
                    "v6.0.0": "ga",
                    "v6.0.1": "patch",
                    "origin/pulse/v6-release": "branch",
                },
                ancestors={("ga", "patch")},
            )

    def test_anticipated_prerelease_staging_accepts_exact_branch_commit(self) -> None:
        source_sha = "a" * 40
        result = self.validate(
            tag="v6.0.4-rc.3",
            existing_tags=set(),
            commits={
                source_sha: source_sha,
                "origin/pulse/v6-release": "branch",
            },
            ancestors={(source_sha, "branch")},
            anticipated_source_sha=source_sha,
        )

        self.assertEqual(result["lineage"], "prerelease")

    def test_anticipated_staging_rejects_existing_tag_at_another_commit(self) -> None:
        source_sha = "b" * 40
        with self.assertRaisesRegex(ValueError, "not anticipated source"):
            self.validate(
                tag="v6.0.4-rc.3",
                existing_tags={"v6.0.4-rc.3"},
                commits={
                    source_sha: source_sha,
                    "v6.0.4-rc.3": "other",
                    "origin/pulse/v6-release": "branch",
                },
                ancestors=set(),
                anticipated_source_sha=source_sha,
            )

    def test_anticipated_staging_rejects_non_exact_sha(self) -> None:
        with self.assertRaisesRegex(ValueError, "exact lowercase 40-character commit"):
            self.validate(
                tag="v6.0.4-rc.3",
                existing_tags=set(),
                commits={"origin/pulse/v6-release": "branch"},
                ancestors=set(),
                anticipated_source_sha="main",
            )


if __name__ == "__main__":
    unittest.main()
