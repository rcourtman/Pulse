import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from governance_stage_guard import (
    blocked_partially_staged_paths,
    is_worktree_sensitive_governance_path,
    partially_staged_governance_paths,
)


class GovernanceStageGuardTest(unittest.TestCase):
    def test_is_worktree_sensitive_governance_path_matches_expected_scope(self) -> None:
        self.assertTrue(is_worktree_sensitive_governance_path(".husky/pre-commit"))
        self.assertTrue(is_worktree_sensitive_governance_path(".github/workflows/canonical-governance.yml"))
        self.assertTrue(is_worktree_sensitive_governance_path("docs/release-control/v6/status.json"))
        self.assertTrue(is_worktree_sensitive_governance_path("internal/repoctl/canonical_development_protocol_test.go"))
        self.assertTrue(is_worktree_sensitive_governance_path("scripts/release_control/status_audit.py"))
        self.assertFalse(is_worktree_sensitive_governance_path("internal/api/slo.go"))
        self.assertFalse(is_worktree_sensitive_governance_path("docs/API.md"))

    def test_blocked_partially_staged_paths_filters_to_partial_governance_files(self) -> None:
        staged = [
            ".husky/pre-commit",
            "docs/release-control/v6/status.json",
            "internal/api/slo.go",
            "scripts/release_control/status_audit.py",
        ]
        unstaged = [
            "docs/release-control/v6/status.json",
            "internal/api/slo.go",
            "scripts/release_control/status_audit.py",
        ]

        self.assertEqual(
            blocked_partially_staged_paths(staged, unstaged),
            [
                "docs/release-control/v6/status.json",
                "scripts/release_control/status_audit.py",
            ],
        )

    def test_partially_staged_governance_paths_detects_real_partial_stage(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            rel = "docs/release-control/v6/status.json"
            path = repo_root / rel
            path.parent.mkdir(parents=True, exist_ok=True)

            subprocess.run(["git", "init"], cwd=repo_root, check=True, capture_output=True, text=True)

            path.write_text('{"version":"staged"}\n', encoding="utf-8")
            subprocess.run(["git", "add", rel], cwd=repo_root, check=True, capture_output=True, text=True)
            path.write_text('{"version":"working-tree"}\n', encoding="utf-8")

            with patch("governance_stage_guard.REPO_ROOT", repo_root):
                self.assertEqual(partially_staged_governance_paths(), [rel])

    def test_partially_staged_governance_paths_ignores_partial_stage_outside_scope(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            rel = "internal/api/slo.go"
            path = repo_root / rel
            path.parent.mkdir(parents=True, exist_ok=True)

            subprocess.run(["git", "init"], cwd=repo_root, check=True, capture_output=True, text=True)

            path.write_text("package api\n", encoding="utf-8")
            subprocess.run(["git", "add", rel], cwd=repo_root, check=True, capture_output=True, text=True)
            path.write_text("package api\n\nvar x = 1\n", encoding="utf-8")

            with patch("governance_stage_guard.REPO_ROOT", repo_root):
                self.assertEqual(partially_staged_governance_paths(), [])


if __name__ == "__main__":
    unittest.main()
