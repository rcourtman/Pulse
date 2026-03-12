import os
import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from repo_file_io import load_repo_json, missing_staged_repo_paths, read_repo_text


class RepoFileIoTest(unittest.TestCase):
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

    def test_read_repo_text_and_load_repo_json_can_read_staged_content(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            rel = "docs/release-control/v6/status.json"
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
            rel = "docs/release-control/v6/RC_TO_GA_REHEARSAL_TEMPLATE.md"
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
            staged_rel = "docs/release-control/v6/PRE_RELEASE_CHECKLIST.md"
            missing_rel = "docs/release-control/v6/RC_TO_GA_REHEARSAL_TEMPLATE.md"
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


if __name__ == "__main__":
    unittest.main()
