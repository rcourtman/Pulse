import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import format_staged_frontend
from format_staged_frontend import format_staged_frontend_files
from repo_file_io import strip_local_git_env


REAL_PRETTIER = (
    Path(format_staged_frontend.DEFAULT_REPO_ROOT)
    / "frontend-modern"
    / "node_modules"
    / ".bin"
    / "prettier"
)


class FormatStagedFrontendTest(unittest.TestCase):
    def git(self, repo_root: Path, *args: str) -> subprocess.CompletedProcess:
        # Scrub the full hook environment: with only GIT_INDEX_FILE removed, a
        # pre-commit run from a linked worktree exports an absolute GIT_DIR and
        # "git init" here re-initializes the REAL repository as bare.
        env = strip_local_git_env(os.environ.copy())
        return subprocess.run(
            ["git", *args],
            cwd=repo_root,
            check=True,
            capture_output=True,
            text=True,
            env=env,
        )

    def seed_repo(self, repo_root: Path, source: str) -> Path:
        ts_file = repo_root / "frontend-modern" / "src" / "sample.ts"
        ts_file.parent.mkdir(parents=True)
        ts_file.write_text(source, encoding="utf-8")
        self.git(repo_root, "init")
        self.git(repo_root, "add", "frontend-modern/src/sample.ts")
        return ts_file

    def test_skips_gracefully_when_prettier_is_unavailable(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            unformatted = "const x = {a:1}\n"
            ts_file = self.seed_repo(repo_root, unformatted)

            with patch.dict(
                "os.environ",
                {"PULSE_PRETTIER_BIN": str(repo_root / "missing-prettier")},
                clear=False,
            ):
                with patch("format_staged_frontend.REPO_ROOT", repo_root):
                    exit_code = format_staged_frontend_files()

            self.assertEqual(exit_code, 0)
            self.assertEqual(ts_file.read_text(encoding="utf-8"), unformatted)
            staged = self.git(repo_root, "show", ":frontend-modern/src/sample.ts").stdout
            self.assertEqual(staged, unformatted)

    @unittest.skipUnless(REAL_PRETTIER.exists(), "prettier not installed under frontend-modern")
    def test_formats_staged_frontend_and_syncs_clean_worktree(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            ts_file = self.seed_repo(repo_root, "const x = {a:1}\n")

            with patch.dict(
                "os.environ", {"PULSE_PRETTIER_BIN": str(REAL_PRETTIER)}, clear=False
            ):
                with patch("format_staged_frontend.REPO_ROOT", repo_root):
                    exit_code = format_staged_frontend_files()

            self.assertEqual(exit_code, 0)
            self.assertEqual(ts_file.read_text(encoding="utf-8"), "const x = { a: 1 };\n")
            staged = self.git(repo_root, "show", ":frontend-modern/src/sample.ts").stdout
            self.assertEqual(staged, ts_file.read_text(encoding="utf-8"))

    @unittest.skipUnless(REAL_PRETTIER.exists(), "prettier not installed under frontend-modern")
    def test_formats_staged_frontend_without_overwriting_unstaged_worktree_changes(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = Path(tmpdir)
            ts_file = self.seed_repo(repo_root, "const x = {a:1}\n")
            ts_file.write_text("const x = {a:1}\n// unstaged worktree edit\n", encoding="utf-8")

            with patch.dict(
                "os.environ", {"PULSE_PRETTIER_BIN": str(REAL_PRETTIER)}, clear=False
            ):
                with patch("format_staged_frontend.REPO_ROOT", repo_root):
                    exit_code = format_staged_frontend_files()

            self.assertEqual(exit_code, 0)
            self.assertEqual(
                ts_file.read_text(encoding="utf-8"),
                "const x = {a:1}\n// unstaged worktree edit\n",
            )
            staged = self.git(repo_root, "show", ":frontend-modern/src/sample.ts").stdout
            self.assertEqual(staged, "const x = { a: 1 };\n")


if __name__ == "__main__":
    unittest.main()
