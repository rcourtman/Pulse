#!/usr/bin/env python3
"""Unit tests for artifact release-line validation."""

from __future__ import annotations

import unittest

import validate_artifact_release_line as validator


class ValidateArtifactReleaseLineTest(unittest.TestCase):
    def validate(
        self,
        *,
        tag: str,
        existing_tags: set[str],
        commits: dict[str, str],
        ancestors: set[tuple[str, str]],
        prerelease_tags: tuple[str, ...] = (),
    ) -> dict[str, str]:
        return validator.validate_artifact_release_line(
            tag=tag,
            purpose="test publish",
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
            tag="v6.0.4-rc.1",
            existing_tags={"v6.0.4-rc.1"},
            commits={
                "v6.0.4-rc.1": "support-rc",
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


if __name__ == "__main__":
    unittest.main()
