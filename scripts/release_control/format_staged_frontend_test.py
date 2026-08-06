import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import format_staged_frontend
from format_staged_frontend import format_staged_frontend_files
from repo_file_io import strip_local_git_env


def _resolve_real_prettier() -> Path:
    # Linked worktrees have no node_modules of their own. Without this the
    # tests below silently skip in exactly the checkouts where the formatter's
    # worktree fallback matters.
    for root in format_staged_frontend.prettier_search_roots():
        candidate = root / "frontend-modern" / "node_modules" / ".bin" / "prettier"
        if candidate.exists():
            return candidate
    return (
        Path(format_staged_frontend.DEFAULT_REPO_ROOT)
        / "frontend-modern"
        / "node_modules"
        / ".bin"
        / "prettier"
    )


REAL_PRETTIER = _resolve_real_prettier()


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

    def test_linked_worktree_falls_back_to_primary_checkout_prettier(self) -> None:
        # Regression: prettier resolved only under REPO_ROOT, so every frontend
        # commit made from a Claude or Codex worktree silently skipped
        # formatting and let drift accumulate in already-committed files.
        with tempfile.TemporaryDirectory() as tmpdir:
            primary = Path(tmpdir) / "primary"
            primary.mkdir()
            self.git(primary, "init")
            (primary / "seed.txt").write_text("seed\n", encoding="utf-8")
            self.git(primary, "add", "seed.txt")
            self.git(
                primary,
                "-c",
                "user.email=test@example.com",
                "-c",
                "user.name=test",
                "commit",
                "-m",
                "seed",
            )

            installed = primary / "frontend-modern" / "node_modules" / ".bin" / "prettier"
            installed.parent.mkdir(parents=True)
            installed.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
            installed.chmod(0o755)

            linked = Path(tmpdir) / "linked"
            self.git(primary, "worktree", "add", str(linked))
            self.assertFalse((linked / "frontend-modern" / "node_modules").exists())

            with patch.dict("os.environ", {}, clear=False):
                os.environ.pop("PULSE_PRETTIER_BIN", None)
                with patch("format_staged_frontend.REPO_ROOT", linked):
                    resolved = format_staged_frontend.prettier_bin()

            # --git-common-dir comes back resolved, so compare resolved paths:
            # on macOS the temp dir is /var -> /private/var.
            self.assertIsNotNone(resolved)
            self.assertEqual(resolved.resolve(), installed.resolve())


if __name__ == "__main__":
    unittest.main()
