import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from format_staged_go import format_staged_go_files


class FormatStagedGoTest(unittest.TestCase):
    def test_formats_staged_go_and_syncs_clean_worktree(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            go_file = repo_root / "main.go"
            go_file.write_text("package main\nfunc main(){println(\"x\")}\n", encoding="utf-8")

            subprocess.run(["git", "init"], cwd=repo_root, check=True, capture_output=True, text=True)
            subprocess.run(["git", "add", "main.go"], cwd=repo_root, check=True, capture_output=True, text=True)

            with patch("format_staged_go.REPO_ROOT", repo_root):
                exit_code = format_staged_go_files()

            self.assertEqual(exit_code, 0)
            self.assertEqual(
                go_file.read_text(encoding="utf-8"),
                "package main\n\nfunc main() { println(\"x\") }\n",
            )
            staged = subprocess.run(
                ["git", "show", ":main.go"],
                cwd=repo_root,
                check=True,
                capture_output=True,
                text=True,
            ).stdout
            self.assertEqual(staged, go_file.read_text(encoding="utf-8"))

    def test_formats_staged_go_without_overwriting_unstaged_worktree_changes(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            go_file = repo_root / "main.go"
            go_file.write_text("package main\nfunc main(){println(\"x\")}\n", encoding="utf-8")

            subprocess.run(["git", "init"], cwd=repo_root, check=True, capture_output=True, text=True)
            subprocess.run(["git", "add", "main.go"], cwd=repo_root, check=True, capture_output=True, text=True)
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
            staged = subprocess.run(
                ["git", "show", ":main.go"],
                cwd=repo_root,
                check=True,
                capture_output=True,
                text=True,
            ).stdout
            self.assertEqual(staged, "package main\n\nfunc main() { println(\"x\") }\n")


if __name__ == "__main__":
    unittest.main()
