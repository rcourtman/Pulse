import os
import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from repo_file_io import (
    canonical_repo_id,
    canonical_repo_root,
    canonical_workspace_repos_root,
    git_env,
    load_repo_json,
    missing_staged_repo_paths,
    read_repo_text,
    strip_local_git_env,
)


class RepoFileIoTest(unittest.TestCase):
    def git(self, repo_root: Path, *args: str) -> subprocess.CompletedProcess:
        env = os.environ.copy()
        strip_local_git_env(env)
        return subprocess.run(
            ["git", *args],
            cwd=repo_root,
            check=True,
            capture_output=True,
            text=True,
            env=env,
        )

    def git_stdout(self, repo_root: Path, *args: str) -> str:
        return self.git(repo_root, *args).stdout.strip()

    def hook_env_for_worktree(self, worktree_root: Path) -> dict[str, str]:
        git_dir = self.git_stdout(worktree_root, "rev-parse", "--path-format=absolute", "--git-dir")
        common_dir = self.git_stdout(worktree_root, "rev-parse", "--path-format=absolute", "--git-common-dir")
        work_tree = self.git_stdout(worktree_root, "rev-parse", "--show-toplevel")
        return {
            "GIT_DIR": git_dir,
            "GIT_WORK_TREE": work_tree,
            "GIT_INDEX_FILE": str(Path(git_dir) / "index"),
            "GIT_COMMON_DIR": common_dir,
        }

    def test_read_repo_text_and_load_repo_json_can_read_staged_content(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            rel = "docs/release-control/v6/internal/status.json"
            path = repo_root / rel
            path.parent.mkdir(parents=True, exist_ok=True)

            self.git(repo_root, "init")
            path.write_text('{"version": "staged"}\n', encoding="utf-8")
            self.git(repo_root, "add", rel)
            path.write_text('{"version": "working-tree"}\n', encoding="utf-8")

            with patch("repo_file_io.REPO_ROOT", repo_root):
                self.assertEqual(read_repo_text(rel), '{"version": "working-tree"}\n')
                self.assertEqual(read_repo_text(rel, staged=True), '{"version": "staged"}\n')
                self.assertEqual(load_repo_json(rel, staged=True), {"version": "staged"})

    def test_strict_staged_read_rejects_worktree_only_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            rel = "docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md"
            path = repo_root / rel
            path.parent.mkdir(parents=True, exist_ok=True)

            self.git(repo_root, "init")
            path.write_text("working-tree-only\n", encoding="utf-8")

            with patch("repo_file_io.REPO_ROOT", repo_root):
                self.assertEqual(read_repo_text(rel, staged=True), "working-tree-only\n")
                with self.assertRaisesRegex(FileNotFoundError, "missing staged index entry"):
                    read_repo_text(rel, staged=True, strict_staged=True)

    def test_missing_staged_repo_paths_lists_unstaged_inputs(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            staged_rel = "docs/release-control/v6/internal/PRE_RELEASE_CHECKLIST.md"
            missing_rel = "docs/release-control/v6/internal/RC_TO_GA_REHEARSAL_TEMPLATE.md"
            staged_path = repo_root / staged_rel
            missing_path = repo_root / missing_rel
            staged_path.parent.mkdir(parents=True, exist_ok=True)
            missing_path.parent.mkdir(parents=True, exist_ok=True)

            self.git(repo_root, "init")
            staged_path.write_text("staged\n", encoding="utf-8")
            self.git(repo_root, "add", staged_rel)
            missing_path.write_text("worktree-only\n", encoding="utf-8")

            with patch("repo_file_io.REPO_ROOT", repo_root):
                self.assertEqual(missing_staged_repo_paths([staged_rel, missing_rel]), [missing_rel])

    def test_git_env_preserves_local_hook_env_and_scrubs_other_repos(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir) / "workspace"
            local_repo = workspace / "repos" / "pulse"
            other_local_repo = workspace / ".worktrees" / "pulse-first-session-onboarding-parity"
            sibling_repo = workspace / "repos" / "pulse-mobile"
            local_repo.mkdir(parents=True)
            other_local_repo.mkdir(parents=True)
            sibling_repo.mkdir(parents=True)

            self.git(local_repo, "init")
            leaked_env = {
                "PATH": os.environ.get("PATH", ""),
                "GIT_DIR": str(local_repo / ".git"),
                "GIT_WORK_TREE": str(local_repo),
                "GIT_INDEX_FILE": str(local_repo / ".git" / "index"),
                "GIT_COMMON_DIR": str(local_repo / ".git"),
                "GIT_PREFIX": "docs/",
            }

            with patch.dict(os.environ, leaked_env, clear=True):
                local_env = git_env(local_repo, local_repo_root=local_repo)
                sibling_env = git_env(sibling_repo, local_repo_root=local_repo)
                mismatched_local_env = git_env(other_local_repo, local_repo_root=other_local_repo)

            self.assertEqual(local_env["GIT_DIR"], leaked_env["GIT_DIR"])
            self.assertEqual(local_env["GIT_WORK_TREE"], leaked_env["GIT_WORK_TREE"])
            self.assertEqual(local_env["GIT_INDEX_FILE"], leaked_env["GIT_INDEX_FILE"])
            self.assertEqual(sibling_env["PATH"], leaked_env["PATH"])
            self.assertNotIn("GIT_DIR", sibling_env)
            self.assertNotIn("GIT_WORK_TREE", sibling_env)
            self.assertNotIn("GIT_INDEX_FILE", sibling_env)
            self.assertNotIn("GIT_COMMON_DIR", sibling_env)
            self.assertNotIn("GIT_PREFIX", sibling_env)
            self.assertEqual(mismatched_local_env["PATH"], leaked_env["PATH"])
            self.assertNotIn("GIT_DIR", mismatched_local_env)
            self.assertNotIn("GIT_WORK_TREE", mismatched_local_env)
            self.assertNotIn("GIT_INDEX_FILE", mismatched_local_env)
            self.assertNotIn("GIT_COMMON_DIR", mismatched_local_env)
            self.assertNotIn("GIT_PREFIX", mismatched_local_env)

    def test_git_env_scrubs_unproven_linked_worktree_index_env(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir) / "workspace"
            repo_root = workspace / "repos" / "pulse"
            linked_worktree = workspace / ".worktrees" / "pulse-first-session-onboarding-parity"
            repo_root.mkdir(parents=True)
            linked_worktree.parent.mkdir(parents=True)

            self.git(repo_root, "init")
            (repo_root / "README.md").write_text("pulse\n", encoding="utf-8")
            self.git(repo_root, "add", "README.md")
            self.git(
                repo_root,
                "-c",
                "user.name=Pulse Test",
                "-c",
                "user.email=pulse-test@example.invalid",
                "commit",
                "-m",
                "initial",
            )
            self.git(repo_root, "worktree", "add", "--detach", str(linked_worktree), "HEAD")

            hook_env = self.hook_env_for_worktree(linked_worktree)
            hook_env.pop("GIT_WORK_TREE")

            with patch.dict(os.environ, hook_env, clear=True):
                env = git_env(linked_worktree, local_repo_root=linked_worktree)

            self.assertNotIn("GIT_DIR", env)
            self.assertNotIn("GIT_WORK_TREE", env)
            self.assertNotIn("GIT_INDEX_FILE", env)
            self.assertNotIn("GIT_COMMON_DIR", env)

    def test_canonical_repo_identity_uses_git_common_dir_for_linked_worktree(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir) / "workspace"
            repo_root = workspace / "repos" / "pulse"
            linked_worktree = workspace / ".worktrees" / "pulse-first-session-onboarding-parity"
            repo_root.mkdir(parents=True)
            linked_worktree.parent.mkdir(parents=True)

            self.git(repo_root, "init")
            (repo_root / "README.md").write_text("pulse\n", encoding="utf-8")
            self.git(repo_root, "add", "README.md")
            self.git(
                repo_root,
                "-c",
                "user.name=Pulse Test",
                "-c",
                "user.email=pulse-test@example.invalid",
                "commit",
                "-m",
                "initial",
            )
            self.git(repo_root, "worktree", "add", "--detach", str(linked_worktree), "HEAD")

            self.assertEqual(canonical_repo_root(linked_worktree), repo_root.resolve())
            self.assertEqual(canonical_repo_id(linked_worktree), "pulse")
            self.assertEqual(canonical_workspace_repos_root(linked_worktree), (workspace / "repos").resolve())


if __name__ == "__main__":
    unittest.main()
