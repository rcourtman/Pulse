import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


INTERNAL_DIR = Path(__file__).resolve().parent
RELEASE_CONTROL_DIR = INTERNAL_DIR.parent
for extra_dir in (INTERNAL_DIR, RELEASE_CONTROL_DIR):
    if str(extra_dir) not in sys.path:
        sys.path.insert(0, str(extra_dir))

from repo_file_io import strip_local_git_env
from verify_commit_slice import main, repo_relative_path


class VerifyCommitSliceTest(unittest.TestCase):
    def git(self, repo_root: Path, *args: str, check: bool = True) -> subprocess.CompletedProcess:
        # Scrub the full hook environment: with only GIT_INDEX_FILE removed, a
        # pre-commit run from a linked worktree exports an absolute GIT_DIR and
        # "git init" here re-initializes the REAL repository as bare.
        env = strip_local_git_env(os.environ.copy())
        return subprocess.run(
            ["git", *args],
            cwd=repo_root,
            check=check,
            capture_output=True,
            text=True,
            env=env,
        )

    def init_repo(self, repo_root: Path) -> None:
        self.git(repo_root, "init")
        self.git(repo_root, "config", "user.name", "Pulse Test")
        self.git(repo_root, "config", "user.email", "pulse-test@example.com")

    def test_repo_relative_path_normalizes_absolute_paths(self) -> None:
        repo_root = Path("/tmp/pulse")
        with patch("verify_commit_slice.REPO_ROOT", repo_root):
            self.assertEqual(repo_relative_path(repo_root / "docs/file.txt"), "docs/file.txt")

    def test_main_stages_requested_paths_in_isolated_index(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            hook = repo_root / ".husky" / "pre-commit"
            tracked = repo_root / "tracked.txt"
            explicit = repo_root / "explicit.txt"
            ignored = repo_root / "ignored.txt"

            hook.parent.mkdir(parents=True, exist_ok=True)
            tracked.write_text("tracked\n", encoding="utf-8")
            explicit.write_text("explicit\n", encoding="utf-8")
            ignored.write_text("ignored\n", encoding="utf-8")
            (repo_root / ".gitignore").write_text("ignored.txt\n", encoding="utf-8")
            hook.write_text(
                "#!/usr/bin/env bash\n"
                "set -euo pipefail\n"
                "git diff --cached --name-only | sort > slice.out\n",
                encoding="utf-8",
            )
            hook.chmod(0o755)

            self.init_repo(repo_root)
            self.git(repo_root, "add", ".gitignore", "tracked.txt", ".husky/pre-commit")
            self.git(repo_root, "commit", "-m", "initial")

            tracked.write_text("tracked updated\n", encoding="utf-8")

            with patch("verify_commit_slice.REPO_ROOT", repo_root), patch(
                "verify_commit_slice.HOOK_PATH", hook
            ):
                self.assertEqual(
                    main(["--add-updated", "explicit.txt", "ignored.txt"]),
                    0,
                )

            self.assertEqual(
                (repo_root / "slice.out").read_text(encoding="utf-8").splitlines(),
                ["explicit.txt", "ignored.txt", "tracked.txt"],
            )
            live_index = self.git(repo_root, "diff", "--cached", "--name-only").stdout.strip()
            self.assertEqual(live_index, "")

    def test_main_propagates_hook_failure(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            hook = repo_root / ".husky" / "pre-commit"
            tracked = repo_root / "tracked.txt"

            hook.parent.mkdir(parents=True, exist_ok=True)
            tracked.write_text("tracked\n", encoding="utf-8")
            hook.write_text("#!/usr/bin/env bash\nexit 7\n", encoding="utf-8")
            hook.chmod(0o755)

            self.init_repo(repo_root)
            self.git(repo_root, "add", "tracked.txt", ".husky/pre-commit")
            self.git(repo_root, "commit", "--no-verify", "-m", "initial")

            tracked.write_text("tracked updated\n", encoding="utf-8")

            with patch("verify_commit_slice.REPO_ROOT", repo_root), patch(
                "verify_commit_slice.HOOK_PATH", hook
            ):
                self.assertEqual(main(["--add-updated"]), 7)

    def test_scratch_git_init_under_worktree_hook_env_leaves_hook_repo_intact(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            base = Path(tmpdir)
            canary = base / "canary"
            linked_worktree = base / "canary-worktree"
            scratch = base / "scratch"
            canary.mkdir()
            scratch.mkdir()

            self.init_repo(canary)
            (canary / "tracked.txt").write_text("tracked\n", encoding="utf-8")
            self.git(canary, "add", "tracked.txt")
            self.git(canary, "commit", "--no-verify", "-m", "initial")
            self.git(canary, "worktree", "add", str(linked_worktree), "-b", "hook-branch")

            git_dir = self.git(
                linked_worktree, "rev-parse", "--path-format=absolute", "--git-dir"
            ).stdout.strip()
            # A pre-commit run from a linked worktree exports exactly this
            # shape: absolute GIT_DIR plus GIT_INDEX_FILE, no GIT_WORK_TREE.
            # (With GIT_WORK_TREE also set, "git init" keeps bare=false and
            # the corruption does not reproduce.)
            hook_env = {
                "GIT_DIR": git_dir,
                "GIT_INDEX_FILE": str(Path(git_dir) / "index"),
            }

            with patch.dict(os.environ, hook_env, clear=False):
                self.git(scratch, "init")

            self.assertTrue((scratch / ".git").is_dir())
            config_text = (canary / ".git" / "config").read_text(encoding="utf-8")
            self.assertNotIn("bare = true", config_text)
            self.git(canary, "status")
            self.git(linked_worktree, "status")


if __name__ == "__main__":
    unittest.main()
