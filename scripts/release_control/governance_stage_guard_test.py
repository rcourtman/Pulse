import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from governance_stage_guard import (
    blocked_unstaged_governance_paths,
    is_worktree_sensitive_governance_path,
    unstaged_governance_paths,
)


class GovernanceStageGuardTest(unittest.TestCase):
    def git(self, repo_root: Path, *args: str) -> subprocess.CompletedProcess:
        env = os.environ.copy()
        env.pop("GIT_INDEX_FILE", None)
        return subprocess.run(
            ["git", *args],
            cwd=repo_root,
            check=True,
            capture_output=True,
            text=True,
            env=env,
        )

    def test_is_worktree_sensitive_governance_path_matches_expected_scope(self) -> None:
        self.assertTrue(is_worktree_sensitive_governance_path(".husky/pre-commit"))
        self.assertTrue(is_worktree_sensitive_governance_path("docs/release-control/control_plane.json"))
        self.assertTrue(is_worktree_sensitive_governance_path("internal/repoctl/canonical_development_protocol_test.go"))
        self.assertTrue(is_worktree_sensitive_governance_path("scripts/release_control/status_audit.py"))
        self.assertFalse(is_worktree_sensitive_governance_path(".github/workflows/canonical-governance.yml"))
        self.assertFalse(is_worktree_sensitive_governance_path("docs/release-control/CONTROL_PLANE.md"))
        self.assertFalse(is_worktree_sensitive_governance_path("docs/release-control/v6/status.json"))
        self.assertFalse(is_worktree_sensitive_governance_path("docs/release-control/v6/PRE_RELEASE_CHECKLIST.md"))
        self.assertFalse(
            is_worktree_sensitive_governance_path(
                "scripts/release_control/release_promotion_policy_test.py"
            )
        )
        self.assertFalse(is_worktree_sensitive_governance_path("internal/api/slo.go"))
        self.assertFalse(is_worktree_sensitive_governance_path("docs/API.md"))

    def test_blocked_unstaged_governance_paths_filters_to_governance_scope(self) -> None:
        unstaged = [
            ".husky/pre-commit",
            "docs/release-control/control_plane.json",
            "docs/release-control/v6/status.json",
            "docs/release-control/v6/PRE_RELEASE_CHECKLIST.md",
            "scripts/release_control/release_promotion_policy_test.py",
            "internal/api/slo.go",
            "scripts/release_control/status_audit.py",
        ]

        self.assertEqual(
            blocked_unstaged_governance_paths(unstaged),
            [
                ".husky/pre-commit",
                "docs/release-control/control_plane.json",
                "scripts/release_control/status_audit.py",
            ],
        )

    def test_unstaged_governance_paths_detects_real_unstaged_governance_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            rel = "docs/release-control/control_plane.json"
            path = repo_root / rel
            path.parent.mkdir(parents=True, exist_ok=True)

            self.git(repo_root, "init")

            path.write_text('{"version":"staged"}\n', encoding="utf-8")
            self.git(repo_root, "add", rel)
            path.write_text('{"version":"working-tree"}\n', encoding="utf-8")

            with patch("governance_stage_guard.REPO_ROOT", repo_root):
                self.assertEqual(unstaged_governance_paths(), [rel])

    def test_unstaged_governance_paths_ignores_unstaged_changes_outside_scope(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            rel = "docs/release-control/v6/status.json"
            path = repo_root / rel
            path.parent.mkdir(parents=True, exist_ok=True)

            self.git(repo_root, "init")

            path.write_text('{"version":"staged"}\n', encoding="utf-8")
            self.git(repo_root, "add", rel)
            path.write_text('{"version":"working-tree"}\n', encoding="utf-8")

            with patch("governance_stage_guard.REPO_ROOT", repo_root):
                self.assertEqual(unstaged_governance_paths(), [])


if __name__ == "__main__":
    unittest.main()
