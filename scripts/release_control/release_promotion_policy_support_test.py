import unittest
from unittest.mock import patch

from release_promotion_policy_support import (
    missing_workflow_dispatch_inputs,
    parse_workflow_dispatch_inputs,
    slice_requires_staged_governance_inputs,
    staged_governance_input_errors,
)


class ReleasePromotionPolicySupportTest(unittest.TestCase):
    def test_parse_workflow_dispatch_inputs_reads_top_level_inputs(self) -> None:
        content = """
name: Example
on:
  workflow_dispatch:
    inputs:
      version:
        required: true
        type: string
      promoted_from_tag:
        required: false
        type: string
      ga_date:
        required: false
        type: string
jobs:
  example:
    runs-on: ubuntu-latest
"""
        self.assertEqual(
            parse_workflow_dispatch_inputs(content),
            ("version", "promoted_from_tag", "ga_date"),
        )

    def test_missing_workflow_dispatch_inputs_reports_missing_keys(self) -> None:
        with (
            patch(
                "release_promotion_policy_support.branch_workflow_text",
                return_value="""
on:
  workflow_dispatch:
    inputs:
      version:
        type: string
      note:
        type: string
""",
            ),
            patch(
                "release_promotion_policy_support.origin_default_branch",
                return_value="main",
            ),
        ):
            self.assertEqual(
                missing_workflow_dispatch_inputs(
                    workflow_path=".github/workflows/release-dry-run.yml",
                    required_inputs=("version", "promoted_from_tag", "ga_date"),
                ),
                ("main", ("promoted_from_tag", "ga_date")),
            )

    def test_slice_requires_staged_governance_inputs_for_promotion_paths(self) -> None:
        self.assertTrue(
            slice_requires_staged_governance_inputs(
                [
                    ".github/workflows/release-dry-run.yml",
                    "scripts/other_helper.py",
                ]
            )
        )
        self.assertTrue(
            slice_requires_staged_governance_inputs(
                [
                    "scripts/check-workflow-dispatch-inputs.py",
                ]
            )
        )
        self.assertTrue(
            slice_requires_staged_governance_inputs(
                [
                    "scripts/trigger-release-dry-run.sh",
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
                return_value=["docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md"],
            ),
            patch(
                "release_promotion_policy_support.read_repo_text",
                return_value="# Pulse v6 Pre-Release Checklist\n",
            ),
        ):
            self.assertEqual(
                staged_governance_input_errors(use_staged_governance=True),
                [
                    "stage the canonical promotion proof inputs:\n- docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md",
                    "stage the updated docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md that records the rc-to-ga-rehearsal-summary gate input",
                    "stage the updated docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md that records the canonical promotion metadata envelope",
                    "stage the updated docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md that preserves the artifact-backed promotion metadata record shape",
                ],
            )

    def test_skips_checklist_read_when_checklist_itself_is_unstaged(self) -> None:
        with (
            patch(
                "release_promotion_policy_support.missing_staged_repo_paths",
                return_value=["docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md"],
            ),
            patch(
                "release_promotion_policy_support.read_repo_text",
                return_value="# Pulse v6 RC-to-GA Rehearsal Record\n",
            ) as read_repo_text,
        ):
            self.assertEqual(
                staged_governance_input_errors(use_staged_governance=True),
                [
                    "stage the canonical promotion proof inputs:\n- docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md",
                    "stage the updated docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md that records the rc-to-ga-rehearsal-summary gate input",
                    "stage the updated docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md that records the canonical promotion metadata envelope",
                    "stage the updated docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md that preserves the artifact-backed promotion metadata record shape",
                ],
            )
            read_repo_text.assert_called_once_with(
                "docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md",
                staged=True,
                strict_staged=True,
            )

    def test_reports_missing_checklist_and_template_contract_language(self) -> None:
        with patch(
            "release_promotion_policy_support.missing_staged_repo_paths",
            return_value=[],
        ), patch(
            "release_promotion_policy_support.read_repo_text",
            side_effect=[
                "# Pulse v6 Pre-Release Checklist\n- rc-to-ga-rehearsal-summary\n",
                "# Pulse v6 RC-to-GA Rehearsal Record\n",
            ],
        ):
            self.assertEqual(
                staged_governance_input_errors(use_staged_governance=True),
                [
                    "stage the updated docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md that records the canonical promotion metadata envelope",
                    "stage the updated docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md that preserves the artifact-backed promotion metadata record shape",
                ],
            )


if __name__ == "__main__":
    unittest.main()
