import os
import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from repo_file_io import load_repo_json, read_repo_text


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


if __name__ == "__main__":
    unittest.main()
