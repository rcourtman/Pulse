import io
import unittest
from contextlib import redirect_stderr, redirect_stdout
from unittest.mock import patch

from staged_commit_shape_guard import (
    advanced_lane_ids,
    format_combined_errors,
    lane_progress_shape_errors,
    main,
    staged_promotion_proof_errors,
)


class StagedCommitShapeGuardTest(unittest.TestCase):
    def test_advanced_lane_ids_detects_score_or_completion_progress(self) -> None:
        previous = {
            "lanes": [
                {
                    "id": "L15",
                    "current_score": 6,
                    "target_score": 8,
                    "status": "partial",
                    "completion": {"state": "open"},
                }
            ]
        }
        staged = {
            "lanes": [
                {
                    "id": "L15",
                    "current_score": 7,
                    "target_score": 8,
                    "status": "partial",
                    "completion": {"state": "open"},
                },
                {
                    "id": "L16",
                    "current_score": 8,
                    "target_score": 8,
                    "status": "target-met",
                    "completion": {"state": "complete"},
                },
            ]
        }
        self.assertEqual(advanced_lane_ids(previous, staged), ["L15"])

    def test_staged_promotion_proof_errors_prefix_shared_messages(self) -> None:
        with patch(
            "staged_commit_shape_guard.staged_governance_input_errors",
            return_value=["stage the canonical promotion proof inputs:\n- docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md"],
        ), patch(
            "staged_commit_shape_guard.slice_requires_staged_governance_inputs",
            return_value=True,
        ):
            self.assertEqual(
                staged_promotion_proof_errors([".github/workflows/release-dry-run.yml"]),
                [
                    "BLOCKED: staged promotion proof inputs are incomplete.\n\n"
                    "Required staged promotion inputs:\n"
                    "- stage the canonical promotion proof inputs:\n"
                    "  - docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md"
                ],
            )

    def test_staged_promotion_proof_errors_skip_unrelated_slices(self) -> None:
        with patch(
            "staged_commit_shape_guard.slice_requires_staged_governance_inputs",
            return_value=False,
        ), patch(
            "staged_commit_shape_guard.staged_governance_input_errors",
        ) as staged_governance_input_errors:
            self.assertEqual(
                staged_promotion_proof_errors(["scripts/release_control/verify_commit_slice.py"]),
                [],
            )
            staged_governance_input_errors.assert_not_called()

    def test_format_combined_errors_joins_blocks_with_blank_line(self) -> None:
        self.assertEqual(
            format_combined_errors(["first block", "second block"]),
            "first block\n\nsecond block",
        )

    def test_lane_progress_shape_errors_blocks_support_only_lane_advancement(self) -> None:
        staged_status = {
            "lanes": [
                {
                    "id": "L15",
                    "name": "Storage and recovery",
                    "current_score": 7,
                    "target_score": 8,
                    "status": "partial",
                    "subsystems": ["storage-recovery"],
                    "completion": {"state": "open"},
                }
            ]
        }
        previous_status = {
            "lanes": [
                {
                    "id": "L15",
                    "name": "Storage and recovery",
                    "current_score": 6,
                    "target_score": 8,
                    "status": "partial",
                    "subsystems": ["storage-recovery"],
                    "completion": {"state": "open"},
                }
            ]
        }
        registry_payload = {
            "shared_ownerships": [],
            "subsystems": [],
        }
        subsystem_rules = [
            {
                "id": "storage-recovery",
                "owned_prefixes": ["frontend-modern/src/components/Recovery/"],
                "owned_files": ["frontend-modern/src/components/Storage/Storage.tsx"],
            }
        ]
        with (
            patch("staged_commit_shape_guard.load_staged_status_payload", return_value=staged_status),
            patch("staged_commit_shape_guard.load_head_status_payload", return_value=previous_status),
            patch("staged_commit_shape_guard.load_staged_registry_payload", return_value=registry_payload),
            patch("staged_commit_shape_guard.load_subsystem_rules", return_value=subsystem_rules),
        ):
            errors = lane_progress_shape_errors(
                [
                    "docs/release-control/v6/internal/status.json",
                    "docs/release-control/v6/internal/subsystems/storage-recovery.md",
                    "scripts/release_control/subsystem_lookup_test.py",
                ]
            )
        self.assertEqual(len(errors), 1)
        self.assertIn("support-only proof or guardrail changes", errors[0])
        self.assertIn("L15 Storage and recovery", errors[0])

    def test_lane_progress_shape_errors_allows_lane_advancement_with_runtime_change(self) -> None:
        staged_status = {
            "lanes": [
                {
                    "id": "L15",
                    "name": "Storage and recovery",
                    "current_score": 7,
                    "target_score": 8,
                    "status": "partial",
                    "subsystems": ["storage-recovery"],
                    "completion": {"state": "open"},
                }
            ]
        }
        previous_status = {
            "lanes": [
                {
                    "id": "L15",
                    "name": "Storage and recovery",
                    "current_score": 6,
                    "target_score": 8,
                    "status": "partial",
                    "subsystems": ["storage-recovery"],
                    "completion": {"state": "open"},
                }
            ]
        }
        registry_payload = {
            "shared_ownerships": [],
            "subsystems": [],
        }
        subsystem_rules = [
            {
                "id": "storage-recovery",
                "owned_prefixes": ["frontend-modern/src/components/Recovery/"],
                "owned_files": ["frontend-modern/src/components/Storage/Storage.tsx"],
            }
        ]
        with (
            patch("staged_commit_shape_guard.load_staged_status_payload", return_value=staged_status),
            patch("staged_commit_shape_guard.load_head_status_payload", return_value=previous_status),
            patch("staged_commit_shape_guard.load_staged_registry_payload", return_value=registry_payload),
            patch("staged_commit_shape_guard.load_subsystem_rules", return_value=subsystem_rules),
        ):
            self.assertEqual(
                lane_progress_shape_errors(
                    [
                        "docs/release-control/v6/internal/status.json",
                        "frontend-modern/src/components/Recovery/Recovery.tsx",
                    ]
                ),
                [],
            )

    def test_main_prints_combined_errors(self) -> None:
        stdout = io.StringIO()
        stderr = io.StringIO()
        with (
            patch("staged_commit_shape_guard.git_staged_files", return_value=["runtime/file.go"]),
            patch(
                "staged_commit_shape_guard.canonical_commit_shape_errors",
                return_value=["BLOCKED: canonical issue"],
            ),
            patch(
                "staged_commit_shape_guard.lane_progress_shape_errors",
                return_value=["BLOCKED: lane progress issue"],
            ),
            patch(
                "staged_commit_shape_guard.staged_promotion_proof_errors",
                return_value=["BLOCKED: promotion proof issue"],
            ),
            redirect_stdout(stdout),
            redirect_stderr(stderr),
        ):
            self.assertEqual(main(), 1)

        self.assertEqual(stdout.getvalue(), "")
        self.assertEqual(
            stderr.getvalue(),
            "BLOCKED: canonical issue\n\nBLOCKED: lane progress issue\n\nBLOCKED: promotion proof issue\n",
        )

    def test_main_reports_success_when_no_errors(self) -> None:
        stdout = io.StringIO()
        stderr = io.StringIO()
        with (
            patch("staged_commit_shape_guard.git_staged_files", return_value=[]),
            patch("staged_commit_shape_guard.canonical_commit_shape_errors", return_value=[]),
            patch("staged_commit_shape_guard.lane_progress_shape_errors", return_value=[]),
            patch("staged_commit_shape_guard.staged_promotion_proof_errors", return_value=[]),
            redirect_stdout(stdout),
            redirect_stderr(stderr),
        ):
            self.assertEqual(main(), 0)

        self.assertEqual(stdout.getvalue(), "Staged commit shape guard passed.\n")
        self.assertEqual(stderr.getvalue(), "")


if __name__ == "__main__":
    unittest.main()
