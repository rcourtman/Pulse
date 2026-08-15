#!/usr/bin/env python3

from __future__ import annotations

from io import StringIO
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

from browser_verification_guard import (
    RECEIPT_PATH,
    formatting_only_paths,
    frontend_runtime_paths,
    main,
    validate_receipt,
)
from format_staged_frontend_test import REAL_PRETTIER
from repo_file_io import strip_local_git_env


BASE_SHA = "a" * 40
CHANGED_PATH = "frontend-modern/src/components/Example.tsx"
CONTENT_SHA = "c" * 64
REPO_ROOT = Path(__file__).resolve().parents[2]


def valid_receipt() -> dict:
    return {
        "version": 1,
        "base_sha": BASE_SHA,
        "verified_at": "2026-08-02T20:15:00Z",
        "result": "passed",
        "changed_paths": [CHANGED_PATH],
        "content_sha256": {CHANGED_PATH: CONTENT_SHA},
        "routes": ["/proxmox/overview"],
        "viewports": [
            {"width": 1280, "height": 800},
            {"width": 390, "height": 844},
        ],
        "states": ["toolbar closed", "View menu open", "Columns disclosure open"],
        "interactions": ["Open View, open Columns, dismiss with Escape"],
    }


class BrowserVerificationGuardTest(unittest.TestCase):
    def test_pre_commit_formats_frontend_before_validating_receipt_hashes(self) -> None:
        hook = (REPO_ROOT / ".husky" / "pre-commit").read_text(encoding="utf-8")

        formatter = hook.index("python3 scripts/release_control/format_staged_frontend.py")
        guard = hook.index("python3 scripts/release_control/browser_verification_guard.py")

        self.assertLess(formatter, guard)

    def test_blocks_frontend_change_when_receipt_is_not_in_commit(self) -> None:
        with (
            patch("sys.stdin", StringIO(CHANGED_PATH + "\n")),
            patch("sys.stderr", new=StringIO()),
        ):
            self.assertEqual(main(["--files-from-stdin"]), 1)

    def test_frontend_runtime_paths_exclude_tests_and_receipt(self) -> None:
        self.assertEqual(
            frontend_runtime_paths(
                [
                    CHANGED_PATH,
                    "frontend-modern/src/components/Example.test.tsx",
                    "frontend-modern/src/components/__tests__/Example.tsx",
                    RECEIPT_PATH,
                    "frontend-modern/index.html",
                    "internal/api/server.go",
                ]
            ),
            ["frontend-modern/index.html", CHANGED_PATH],
        )

    def test_accepts_receipt_bound_to_changed_paths_and_two_viewports(self) -> None:
        self.assertEqual(
            validate_receipt(
                valid_receipt(),
                changed_paths=[CHANGED_PATH],
                expected_base=BASE_SHA,
                expected_content_sha256={CHANGED_PATH: CONTENT_SHA},
            ),
            [],
        )

    def test_rejects_stale_base_and_incomplete_changed_path_coverage(self) -> None:
        receipt = valid_receipt()
        receipt["base_sha"] = "b" * 40
        receipt["changed_paths"] = ["frontend-modern/src/components/Other.tsx"]

        errors = validate_receipt(
            receipt,
            changed_paths=[CHANGED_PATH],
            expected_base=BASE_SHA,
            expected_content_sha256={CHANGED_PATH: CONTENT_SHA},
        )

        self.assertTrue(any("base_sha" in error for error in errors))
        self.assertTrue(any("changed_paths" in error for error in errors))

    def test_rejects_dom_only_proof_without_states_interactions_or_narrow_viewport(self) -> None:
        receipt = valid_receipt()
        receipt["viewports"] = [{"width": 1280, "height": 800}]
        receipt["states"] = []
        receipt["interactions"] = []

        errors = validate_receipt(
            receipt,
            changed_paths=[CHANGED_PATH],
            expected_base=BASE_SHA,
            expected_content_sha256={CHANGED_PATH: CONTENT_SHA},
        )

        self.assertTrue(any("narrow width" in error for error in errors))
        self.assertTrue(any("states" in error for error in errors))
        self.assertTrue(any("interactions" in error for error in errors))

    def test_rejects_source_content_changed_after_browser_verification(self) -> None:
        receipt = valid_receipt()

        errors = validate_receipt(
            receipt,
            changed_paths=[CHANGED_PATH],
            expected_base=BASE_SHA,
            expected_content_sha256={CHANGED_PATH: "d" * 64},
        )

        self.assertTrue(any("content_sha256" in error for error in errors))


class FormattingOnlyExemptionTest(unittest.TestCase):
    """A prettier sweep has no visual delta, but a real edit must still block."""

    def build_repo(self, tmpdir: str, old: str, new: str) -> Path:
        repo_root = Path(tmpdir)
        source = repo_root / CHANGED_PATH
        source.parent.mkdir(parents=True)
        source.write_text(old, encoding="utf-8")
        env = strip_local_git_env(os.environ.copy())

        def git(*args: str) -> None:
            subprocess.run(
                ["git", *args], cwd=repo_root, check=True, capture_output=True, env=env
            )

        git("init")
        git("add", CHANGED_PATH)
        git("-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-m", "seed")
        source.write_text(new, encoding="utf-8")
        git("add", CHANGED_PATH)
        return repo_root

    def resolve(self, repo_root: Path) -> set[str]:
        # Under the pre-commit hook, GIT_DIR and GIT_INDEX_FILE are exported
        # and would point this temp repo's plumbing at the real repository.
        with patch.dict("os.environ", strip_local_git_env(os.environ.copy()), clear=True):
            with patch("browser_verification_guard.REPO_ROOT", repo_root):
                return formatting_only_paths([CHANGED_PATH], commit=None, repo_root=repo_root)

    @unittest.skipUnless(REAL_PRETTIER.exists(), "prettier not installed under frontend-modern")
    def test_exempts_a_pure_reformat(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = self.build_repo(tmpdir, "const x = {a:1}\n", "const x = { a: 1 };\n")
            with patch.dict(
                "os.environ", {"PULSE_PRETTIER_BIN": str(REAL_PRETTIER)}, clear=False
            ):
                self.assertEqual(self.resolve(repo_root), {CHANGED_PATH})

    @unittest.skipUnless(REAL_PRETTIER.exists(), "prettier not installed under frontend-modern")
    def test_still_requires_proof_when_a_reformat_also_changes_a_value(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            # Formatted exactly as prettier would, but 1 became 2. The guard
            # must not treat a semantic edit as a cosmetic one.
            repo_root = self.build_repo(tmpdir, "const x = {a:1}\n", "const x = { a: 2 };\n")
            with patch.dict(
                "os.environ", {"PULSE_PRETTIER_BIN": str(REAL_PRETTIER)}, clear=False
            ):
                self.assertEqual(self.resolve(repo_root), set())

    def test_fails_closed_when_prettier_is_unavailable(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_root = self.build_repo(tmpdir, "const x = {a:1}\n", "const x = { a: 1 };\n")
            with patch.dict(
                "os.environ",
                {"PULSE_PRETTIER_BIN": str(repo_root / "missing-prettier")},
                clear=False,
            ):
                self.assertEqual(self.resolve(repo_root), set())


if __name__ == "__main__":
    unittest.main()
