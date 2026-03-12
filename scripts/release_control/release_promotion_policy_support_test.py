import unittest
from unittest.mock import patch

from release_promotion_policy_support import (
    slice_requires_staged_governance_inputs,
    staged_governance_input_errors,
)


class ReleasePromotionPolicySupportTest(unittest.TestCase):
    def test_slice_requires_staged_governance_inputs_for_promotion_paths(self) -> None:
        self.assertTrue(
            slice_requires_staged_governance_inputs(
                [
                    ".github/workflows/release-dry-run.yml",
                    "scripts/other_helper.py",
                ]
            )
        )

    def test_slice_skips_staged_governance_inputs_for_unrelated_paths(self) -> None:
        self.assertFalse(
            slice_requires_staged_governance_inputs(
                [
                    "scripts/release_control/release_promotion_policy_support.py",
                    "scripts/release_control/verify_commit_slice.py",
                    "scripts/release_control/verify_commit_slice_test.py",
                ]
            )
        )

    def test_returns_no_errors_when_not_using_staged_governance(self) -> None:
        self.assertEqual(staged_governance_input_errors(use_staged_governance=False), [])

    def test_reports_missing_template_and_missing_checklist_update(self) -> None:
        with (
            patch(
                "release_promotion_policy_support.missing_staged_repo_paths",
                return_value=["docs/release-control/v6/RC_TO_GA_REHEARSAL_TEMPLATE.md"],
            ),
            patch(
                "release_promotion_policy_support.read_repo_text",
                return_value="# Pulse v6 Pre-Release Checklist\n",
            ),
        ):
            self.assertEqual(
                staged_governance_input_errors(use_staged_governance=True),
                [
                    "stage the canonical promotion proof inputs:\n- docs/release-control/v6/RC_TO_GA_REHEARSAL_TEMPLATE.md",
                    "stage the updated docs/release-control/v6/PRE_RELEASE_CHECKLIST.md that records the rc-to-ga-rehearsal-summary gate input",
                ],
            )

    def test_skips_checklist_read_when_checklist_itself_is_unstaged(self) -> None:
        with (
            patch(
                "release_promotion_policy_support.missing_staged_repo_paths",
                return_value=["docs/release-control/v6/PRE_RELEASE_CHECKLIST.md"],
            ),
            patch("release_promotion_policy_support.read_repo_text") as read_repo_text,
        ):
            self.assertEqual(
                staged_governance_input_errors(use_staged_governance=True),
                [
                    "stage the canonical promotion proof inputs:\n- docs/release-control/v6/PRE_RELEASE_CHECKLIST.md",
                    "stage the updated docs/release-control/v6/PRE_RELEASE_CHECKLIST.md that records the rc-to-ga-rehearsal-summary gate input",
                ],
            )
            read_repo_text.assert_not_called()


if __name__ == "__main__":
    unittest.main()
