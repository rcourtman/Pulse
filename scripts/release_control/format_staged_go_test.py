import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from format_staged_go import format_staged_go_files


class FormatStagedGoTest(unittest.TestCase):
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

    def test_formats_staged_go_and_syncs_clean_worktree(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            go_file = repo_root / "main.go"
            go_file.write_text("package main\nfunc main(){println(\"x\")}\n", encoding="utf-8")

            self.git(repo_root, "init")
            self.git(repo_root, "add", "main.go")

            with patch("format_staged_go.REPO_ROOT", repo_root):
                exit_code = format_staged_go_files()

            self.assertEqual(exit_code, 0)
            self.assertEqual(
                go_file.read_text(encoding="utf-8"),
                "package main\n\nfunc main() { println(\"x\") }\n",
            )
            staged = self.git(repo_root, "show", ":main.go").stdout
            self.assertEqual(staged, go_file.read_text(encoding="utf-8"))

    def test_formats_staged_go_without_overwriting_unstaged_worktree_changes(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            go_file = repo_root / "main.go"
            go_file.write_text("package main\nfunc main(){println(\"x\")}\n", encoding="utf-8")

            self.git(repo_root, "init")
            self.git(repo_root, "add", "main.go")
            go_file.write_text(
                "package main\nfunc main(){println(\"x\")}\n// unstaged worktree edit\n",
                encoding="utf-8",
            )

            with patch("format_staged_go.REPO_ROOT", repo_root):
                exit_code = format_staged_go_files()

            self.assertEqual(exit_code, 0)
            self.assertEqual(
                go_file.read_text(encoding="utf-8"),
                "package main\nfunc main(){println(\"x\")}\n// unstaged worktree edit\n",
            )
            staged = self.git(repo_root, "show", ":main.go").stdout
            self.assertEqual(staged, "package main\n\nfunc main() { println(\"x\") }\n")

    def test_patched_repo_root_ignores_inherited_alternate_index(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            go_file = repo_root / "main.go"
            go_file.write_text("package main\nfunc main(){println(\"x\")}\n", encoding="utf-8")

            self.git(repo_root, "init")
            self.git(repo_root, "add", "main.go")

            fake_index = repo_root / "foreign.index"
            fake_index.write_text("not a git index", encoding="utf-8")

            with patch.dict("os.environ", {"GIT_INDEX_FILE": str(fake_index)}, clear=False):
                with patch("format_staged_go.REPO_ROOT", repo_root):
                    exit_code = format_staged_go_files()

            self.assertEqual(exit_code, 0)
            staged = self.git(repo_root, "show", ":main.go").stdout
            self.assertEqual(staged, "package main\n\nfunc main() { println(\"x\") }\n")


if __name__ == "__main__":
    unittest.main()
