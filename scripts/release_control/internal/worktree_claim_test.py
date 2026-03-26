import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from worktree_claim import (
    branch_path_slug,
    build_branch_name,
    build_worktree_path,
    parse_args,
    parse_worktree_list,
    validate_worktree_target,
)


class WorktreeClaimTest(unittest.TestCase):
    def test_parse_args_accepts_write_and_overrides(self) -> None:
        args = parse_args(
            [
                "--kind",
                "lane",
                "--id",
                "L15",
                "--summary",
                "Tighten storage recovery coherence.",
                "--agent-id",
                "claude-code",
                "--branch",
                "pulse/claude-code/lane-l15",
                "--path",
                "/tmp/pulse-l15",
                "--write",
                "--pretty",
            ]
        )
        self.assertEqual(args.kind, "lane")
        self.assertEqual(args.id, "L15")
        self.assertEqual(args.branch, "pulse/claude-code/lane-l15")
        self.assertEqual(args.path, "/tmp/pulse-l15")
        self.assertTrue(args.write)
        self.assertTrue(args.pretty)

    def test_build_branch_name_uses_pulse_prefix(self) -> None:
        self.assertEqual(
            build_branch_name(agent_id="Claude Code", work_kind="lane", work_id="L15"),
            "pulse/claude-code/lane-l15",
        )

    def test_build_worktree_path_uses_workspace_root(self) -> None:
        path = build_worktree_path(
            repo_root=Path("/Volumes/Development/pulse/repos/pulse"),
            branch_name="pulse/claude-code/lane-l15",
        )
        self.assertEqual(
            path,
            Path("/Volumes/Development/pulse/worktrees/pulse/claude-code__lane-l15"),
        )

    def test_branch_path_slug_drops_prefix_and_replaces_slashes(self) -> None:
        self.assertEqual(branch_path_slug("pulse/codex-gpt5/lane-l16"), "codex-gpt5__lane-l16")

    def test_parse_worktree_list_reads_porcelain_records(self) -> None:
        entries = parse_worktree_list(
            "worktree /tmp/pulse-main\nHEAD abc123\nbranch refs/heads/pulse/v6\n\n"
            "worktree /tmp/pulse-l15\nHEAD def456\nbranch refs/heads/pulse/claude/lane-l15\n\n"
        )
        self.assertEqual(len(entries), 2)
        self.assertEqual(entries[1]["worktree"], "/tmp/pulse-l15")
        self.assertEqual(entries[1]["branch"], "refs/heads/pulse/claude/lane-l15")

    def test_validate_worktree_target_rejects_existing_branch_or_path(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            existing = Path(tmp) / "existing"
            existing.mkdir()
            with patch(
                "worktree_claim.list_worktrees",
                return_value=[
                    {
                        "worktree": str(existing),
                        "branch": "refs/heads/pulse/claude-code/lane-l15",
                    }
                ],
            ):
                errors = validate_worktree_target(
                    repo_root=Path("/Volumes/Development/pulse/repos/pulse"),
                    branch_name="pulse/claude-code/lane-l15",
                    path=existing,
                )
        self.assertEqual(
            errors,
            [
                f"worktree path already exists in git worktree list: {existing.resolve()}",
                "branch already checked out in another worktree: pulse/claude-code/lane-l15",
                f"worktree path already exists on disk: {existing.resolve()}",
            ],
        )


if __name__ == "__main__":
    unittest.main()
