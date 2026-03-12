import io
import unittest
from contextlib import redirect_stderr, redirect_stdout
from unittest.mock import patch

from staged_commit_shape_guard import format_combined_errors, main, staged_promotion_proof_errors


class StagedCommitShapeGuardTest(unittest.TestCase):
    def test_staged_promotion_proof_errors_prefix_shared_messages(self) -> None:
        with patch(
            "staged_commit_shape_guard.staged_governance_input_errors",
            return_value=["stage the canonical promotion proof inputs:\n- docs/release-control/v6/RC_TO_GA_REHEARSAL_TEMPLATE.md"],
        ):
            self.assertEqual(
                staged_promotion_proof_errors(),
                [
                    "BLOCKED: staged promotion proof inputs are incomplete.\n\n"
                    "Required staged promotion inputs:\n"
                    "- stage the canonical promotion proof inputs:\n"
                    "  - docs/release-control/v6/RC_TO_GA_REHEARSAL_TEMPLATE.md"
                ],
            )

    def test_format_combined_errors_joins_blocks_with_blank_line(self) -> None:
        self.assertEqual(
            format_combined_errors(["first block", "second block"]),
            "first block\n\nsecond block",
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
            "BLOCKED: canonical issue\n\nBLOCKED: promotion proof issue\n",
        )

    def test_main_reports_success_when_no_errors(self) -> None:
        stdout = io.StringIO()
        stderr = io.StringIO()
        with (
            patch("staged_commit_shape_guard.git_staged_files", return_value=[]),
            patch("staged_commit_shape_guard.canonical_commit_shape_errors", return_value=[]),
            patch("staged_commit_shape_guard.staged_promotion_proof_errors", return_value=[]),
            redirect_stdout(stdout),
            redirect_stderr(stderr),
        ):
            self.assertEqual(main(), 0)

        self.assertEqual(stdout.getvalue(), "Staged commit shape guard passed.\n")
        self.assertEqual(stderr.getvalue(), "")


if __name__ == "__main__":
    unittest.main()
