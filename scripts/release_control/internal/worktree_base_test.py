import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

from worktree_base import base_slug, canonical_base_worktree_path, parse_args


class WorktreeBaseTest(unittest.TestCase):
    def test_parse_args_accepts_write(self) -> None:
        args = parse_args(["--base-branch", "pulse/v6", "--write", "--pretty"])
        self.assertEqual(args.base_branch, "pulse/v6")
        self.assertTrue(args.write)
        self.assertTrue(args.pretty)

    def test_base_slug_replaces_slashes(self) -> None:
        self.assertEqual(base_slug("pulse/v6"), "base__pulse__v6")

    def test_canonical_base_worktree_path_uses_workspace_root(self) -> None:
        with patch(
            "worktree_base.canonical_worktrees_root",
            return_value=Path("/Volumes/Development/pulse/worktrees"),
        ), patch(
            "worktree_base.repo_root_context",
            return_value=SimpleNamespace(repo_id="pulse"),
        ):
            path = canonical_base_worktree_path(
                repo_root=Path("/isolated/worktrees/l1/pulse"),
                branch_name="pulse/v6",
            )
        self.assertEqual(path, Path("/Volumes/Development/pulse/worktrees/pulse/base__pulse__v6"))


if __name__ == "__main__":
    unittest.main()
