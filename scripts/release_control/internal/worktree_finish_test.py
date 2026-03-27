import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from worktree_finish import commits_ahead_of_base, parse_args, render_pretty


class WorktreeFinishTest(unittest.TestCase):
    def test_parse_args_accepts_write_and_base_branch(self) -> None:
        args = parse_args(["--base-branch", "pulse/v6", "--write", "--pretty"])
        self.assertEqual(args.base_branch, "pulse/v6")
        self.assertTrue(args.write)
        self.assertTrue(args.pretty)

    def test_commits_ahead_of_base_splits_rev_list_output(self) -> None:
        with patch(
            "worktree_finish.git",
            return_value=type("Result", (), {"stdout": "abc123\ndef456\n"})(),
        ):
            self.assertEqual(
                commits_ahead_of_base(
                    repo_root=Path("/Volumes/Development/pulse/repos/pulse"),
                    base_branch="pulse/v6",
                ),
                ["abc123", "def456"],
            )

    def test_render_pretty_includes_commit_count_and_errors(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            worktree = Path(tmp) / "source"
            base = Path(tmp) / "base"
            output = render_pretty(
                current_worktree=worktree,
                current_branch_name="pulse/claude-code/lane-l15",
                base_branch="pulse/v6",
                base_worktree=base,
                commits=["abc123"],
                errors=["base worktree is dirty"],
                wrote=False,
            )
        self.assertIn("commit_count=1", output)
        self.assertIn("base worktree is dirty", output)


if __name__ == "__main__":
    unittest.main()
