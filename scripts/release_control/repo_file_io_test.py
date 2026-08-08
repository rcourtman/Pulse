import os
import json
import re
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from repo_file_io import (
    canonical_repo_id,
    canonical_workspace_repos_root,
    execution_repo_root,
    git_env,
    load_repo_json,
    missing_staged_repo_paths,
    read_repo_text,
    repo_root_context,
    shared_repo_root,
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

    def test_staged_reads_preserve_standalone_temporary_index(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            rel = "docs/release-control/v6/internal/status.json"
            path = repo_root / rel
            path.parent.mkdir(parents=True, exist_ok=True)

            self.git(repo_root, "init")
            path.write_text('{"version": "head"}\n', encoding="utf-8")
            self.git(repo_root, "add", rel)
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

            with tempfile.TemporaryDirectory() as index_tmpdir:
                index_path = Path(index_tmpdir) / "copied-index"
                env = {"PATH": os.environ.get("PATH", ""), "GIT_INDEX_FILE": str(index_path)}
                path.write_text('{"version": "temporary-index"}\n', encoding="utf-8")
                subprocess.run(
                    ["git", "read-tree", "HEAD"],
                    cwd=repo_root,
                    check=True,
                    capture_output=True,
                    text=True,
                    env=env,
                )
                subprocess.run(
                    ["git", "add", rel],
                    cwd=repo_root,
                    check=True,
                    capture_output=True,
                    text=True,
                    env=env,
                )
                path.write_text('{"version": "working-tree"}\n', encoding="utf-8")

                with (
                    patch("repo_file_io.REPO_ROOT", repo_root),
                    patch("repo_file_io.DEFAULT_REPO_ROOT", repo_root),
                    patch.dict(os.environ, env, clear=True),
                ):
                    self.assertEqual(read_repo_text(rel), '{"version": "working-tree"}\n')
                    self.assertEqual(read_repo_text(rel, staged=True), '{"version": "temporary-index"}\n')
                    self.assertEqual(load_repo_json(rel, staged=True), {"version": "temporary-index"})

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

    def test_linked_worktree_reads_ignore_conflicting_primary_checkout_state(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            workspace = Path(tmpdir) / "workspace"
            repo_root = workspace / "repos" / "pulse"
            linked_worktree = workspace / ".worktrees" / "pulse-release-control-isolation"
            rel = "docs/release-control/v6/internal/status.json"
            primary_path = repo_root / rel
            repo_root.mkdir(parents=True)
            primary_path.parent.mkdir(parents=True)

            self.git(repo_root, "init")
            primary_path.write_text('{"version": "head"}\n', encoding="utf-8")
            self.git(repo_root, "add", rel)
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

            primary_path.unlink()
            linked_path = linked_worktree / rel
            linked_path.write_text('{"version": "linked-index"}\n', encoding="utf-8")
            self.git(linked_worktree, "add", rel)
            linked_path.write_text('{"version": "linked-worktree"}\n', encoding="utf-8")

            with (
                patch("repo_file_io.REPO_ROOT", linked_worktree),
                patch("repo_file_io.DEFAULT_REPO_ROOT", linked_worktree),
                patch.dict(os.environ, self.hook_env_for_worktree(linked_worktree), clear=False),
            ):
                self.assertEqual(read_repo_text(rel), '{"version": "linked-worktree"}\n')
                self.assertEqual(read_repo_text(rel, staged=True), '{"version": "linked-index"}\n')
                self.assertEqual(load_repo_json(rel, staged=True), {"version": "linked-index"})

            self.assertFalse(primary_path.exists())

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

    def test_repo_root_context_separates_linked_worktree_from_shared_identity(self) -> None:
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

            context = repo_root_context(linked_worktree)

            self.assertEqual(execution_repo_root(linked_worktree), linked_worktree.resolve())
            self.assertEqual(shared_repo_root(linked_worktree), repo_root.resolve())
            self.assertEqual(context.execution_root, linked_worktree.resolve())
            self.assertEqual(context.shared_repo_root, repo_root.resolve())
            self.assertEqual(context.repo_id, "pulse")
            self.assertEqual(context.workspace_repos_root, (workspace / "repos").resolve())
            self.assertEqual(canonical_repo_id(linked_worktree), "pulse")
            self.assertEqual(canonical_workspace_repos_root(linked_worktree), (workspace / "repos").resolve())

            primary_context = repo_root_context(repo_root)
            self.assertEqual(primary_context.execution_root, repo_root.resolve())
            self.assertEqual(primary_context.shared_repo_root, repo_root.resolve())
            self.assertEqual(primary_context.repo_id, "pulse")
            self.assertEqual(primary_context.workspace_repos_root, (workspace / "repos").resolve())

    def test_scratch_git_init_tests_scrub_env_through_shared_helper(self) -> None:
        # Running "git init" in a scratch directory while the pre-commit hook
        # environment from a linked worktree (absolute GIT_DIR et al.) is still
        # exported re-initializes the REAL repository with core.bare=true,
        # breaking git for every checkout. Two rounds of per-file fixes each
        # missed a straggler, so every release-control test that creates
        # scratch repos must route its git env through strip_local_git_env.
        release_control_dir = Path(__file__).resolve().parent
        scratch_init = re.compile(r"\.git\([^)]*\"init\"|\[\s*\"git\",\s*\"init\"")
        offenders = []
        for test_file in sorted(release_control_dir.rglob("*_test.py")):
            source = test_file.read_text(encoding="utf-8")
            if not scratch_init.search(source):
                continue
            if "strip_local_git_env" not in source:
                offenders.append(test_file.relative_to(release_control_dir).as_posix())
        self.assertEqual(
            offenders,
            [],
            "these test files run scratch 'git init' without scrubbing the "
            "inherited hook git env via repo_file_io.strip_local_git_env",
        )


if __name__ == "__main__":
    unittest.main()
