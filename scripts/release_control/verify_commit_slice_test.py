import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from verify_commit_slice import main, repo_relative_path


class VerifyCommitSliceTest(unittest.TestCase):
    def git(self, repo_root: Path, *args: str, check: bool = True) -> subprocess.CompletedProcess:
        env = os.environ.copy()
        env.pop("GIT_INDEX_FILE", None)
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


if __name__ == "__main__":
    unittest.main()
