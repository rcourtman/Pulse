#!/usr/bin/env python3

from __future__ import annotations

import hashlib
from io import StringIO
import json
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
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


OTHER_PATH = "frontend-modern/src/components/Other.tsx"


class IntegrationRangeTest(unittest.TestCase):
    """--base validates a merged integration range, not one tip receipt.

    The maintainer coordinator merges reviewed candidates onto a main that
    keeps moving, so the range tip is a merge whose parent no lane receipt can
    name. Each frontend file must still ship with content a browser pass
    verified, recorded by a non-merge commit bound to its own parent.
    """

    def setUp(self) -> None:
        self.tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmpdir.cleanup)
        self.repo_root = Path(self.tmpdir.name)
        self.env = strip_local_git_env(os.environ.copy())
        self.env["PULSE_PRETTIER_BIN"] = str(self.repo_root / "missing-prettier")
        self.git("init", "--quiet", "--initial-branch=main")
        self.write(CHANGED_PATH, "export const a = 0;\n")
        self.write(OTHER_PATH, "export const b = 0;\n")
        self.base = self.commit("seed")

    def git(self, *args: str) -> str:
        return subprocess.run(
            ["git", "-c", "user.email=t@example.com", "-c", "user.name=t", *args],
            cwd=self.repo_root,
            check=True,
            capture_output=True,
            text=True,
            env=self.env,
        ).stdout.strip()

    def write(self, path: str, text: str) -> None:
        target = self.repo_root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text, encoding="utf-8")

    def commit(self, message: str) -> str:
        self.git("add", "-A")
        self.git("commit", "--quiet", "--allow-empty", "-m", message)
        return self.git("rev-parse", "HEAD")

    def write_receipt(self, paths: list[str], *, base: str | None = None) -> None:
        receipt = valid_receipt()
        receipt["base_sha"] = base or self.git("rev-parse", "HEAD")
        receipt["changed_paths"] = paths
        receipt["content_sha256"] = {
            path: hashlib.sha256((self.repo_root / path).read_bytes()).hexdigest()
            for path in paths
        }
        self.write(RECEIPT_PATH, json.dumps(receipt, indent=2) + "\n")

    def verified_change(self, path: str, text: str, message: str) -> str:
        self.write(path, text)
        self.write_receipt([path])
        return self.commit(message)

    def run_range(self, *extra: str) -> tuple[int, str]:
        stderr = StringIO()
        with (
            patch.dict("os.environ", self.env, clear=True),
            patch("browser_verification_guard.REPO_ROOT", self.repo_root),
            patch("sys.stdout", new=StringIO()),
            patch("sys.stderr", new=stderr),
        ):
            status = main(["--base", self.base, "--commit", self.git("rev-parse", "HEAD"), *extra])
        return status, stderr.getvalue()

    def merge_advanced_main_into_candidate(self) -> None:
        # main advances with its own verified frontend change while the
        # candidate is verified against the older base, then the coordinator
        # merges the candidate onto the newer main.
        self.git("checkout", "--quiet", "-b", "candidate", self.base)
        self.verified_change(CHANGED_PATH, "export const a = 1;\n", "candidate")
        self.git("checkout", "--quiet", "main")
        self.verified_change(OTHER_PATH, "export const b = 1;\n", "upstream")
        # Both sides edited the shared receipt; the resolution keeps one side.
        subprocess.run(
            ["git", "-c", "user.email=t@example.com", "-c", "user.name=t",
             "merge", "--quiet", "--no-ff", "candidate", "-m", "integrate"],
            cwd=self.repo_root, capture_output=True, env=self.env,
        )
        self.git("checkout", "--quiet", "--theirs", RECEIPT_PATH)
        self.commit("integrate candidate")
        self.assertEqual(len(self.git("log", "-1", "--format=%P").split()), 2)

    def test_accepts_merge_of_verified_candidate_onto_advanced_main(self) -> None:
        self.merge_advanced_main_into_candidate()
        self.assertEqual(self.run_range(), (0, ""))

    def test_accepts_receipt_recorded_in_a_later_commit(self) -> None:
        self.write(CHANGED_PATH, "export const a = 1;\n")
        self.commit("frontend change")
        self.write_receipt([CHANGED_PATH])
        self.commit("record proof")
        self.assertEqual(self.run_range()[0], 0)

    def run_workflow_browser_step(self, *, base: str | None = None) -> subprocess.CompletedProcess:
        workflow = (REPO_ROOT / ".github/workflows/canonical-governance.yml").read_text()
        step = workflow.split("      - name: Validate final frontend browser evidence\n", 1)[1]
        step = step.split("\n      - name:", 1)[0]
        command = textwrap.dedent(step.split("        run: |\n", 1)[1])
        for name in ("browser_verification_guard.py", "format_staged_frontend.py"):
            self.write("scripts/release_control/" + name,
                       (REPO_ROOT / "scripts/release_control" / name).read_text())
        return subprocess.run(
            ["bash", "-c", command], cwd=self.repo_root, capture_output=True, text=True,
            env={**self.env, "WORKFLOW_OUTPUT_1":
                 f"{base or self.base}...{self.git('rev-parse', 'HEAD')}"},
        )

    def test_workflow_accepts_additive_receipt_correction_but_not_unverified_edit(self) -> None:
        self.write(CHANGED_PATH, "export const a = 1;\n")
        self.write_receipt([CHANGED_PATH])
        payload = json.loads((self.repo_root / RECEIPT_PATH).read_text())
        payload["verified_at"] = "2026-10-02T13:00:00+00:00"
        self.write(RECEIPT_PATH, json.dumps(payload))
        original = self.commit("frontend with malformed timestamp")
        self.assertNotEqual(self.run_workflow_browser_step().returncode, 0)

        self.write_receipt([CHANGED_PATH])
        self.commit("correct receipt without rewriting source history")
        corrected = self.run_workflow_browser_step()
        self.assertEqual(corrected.returncode, 0, corrected.stdout + corrected.stderr)
        self.git("merge-base", "--is-ancestor", original, "HEAD")

        self.write(CHANGED_PATH, "export const a = 2;\n")
        self.commit("unverified later edit")
        self.assertNotEqual(self.run_workflow_browser_step().returncode, 0)

    def test_workflow_checks_merge_resolution_and_rejects_missing_range_base(self) -> None:
        self.merge_advanced_main_into_candidate()
        self.assertEqual(self.run_workflow_browser_step().returncode, 0)
        self.assertNotEqual(self.run_workflow_browser_step(base="b" * 40).returncode, 0)
        self.write(CHANGED_PATH, "export const a = 3;\n")
        self.commit("unverified integration edit")
        self.assertNotEqual(self.run_workflow_browser_step().returncode, 0)

    def test_blocks_frontend_edit_after_merge_without_fresh_proof(self) -> None:
        self.merge_advanced_main_into_candidate()
        self.write(CHANGED_PATH, "export const a = 2;\n")
        self.commit("coordinator correction")

        status, stderr = self.run_range()

        self.assertEqual(status, 1)
        self.assertIn(CHANGED_PATH, stderr)
        self.assertNotIn(OTHER_PATH, stderr)

    def test_merge_resolution_receipt_is_not_evidence(self) -> None:
        # A conflict resolution that changes source and rewrites the receipt
        # inside the merge commit has no browser run behind it.
        self.git("checkout", "--quiet", "-b", "candidate", self.base)
        self.verified_change(CHANGED_PATH, "export const a = 1;\n", "candidate")
        self.git("checkout", "--quiet", "main")
        self.git("merge", "--quiet", "--no-ff", "--no-commit", "candidate")
        self.write(CHANGED_PATH, "export const a = 3;\n")
        self.write_receipt([CHANGED_PATH], base=self.base)
        self.commit("integrate with resolution")
        self.assertEqual(len(self.git("log", "-1", "--format=%P").split()), 2)

        status, stderr = self.run_range()

        self.assertEqual(status, 1)
        self.assertIn(CHANGED_PATH, stderr)

    def test_receipt_bound_to_another_parent_is_rejected(self) -> None:
        self.write(CHANGED_PATH, "export const a = 1;\n")
        self.write_receipt([CHANGED_PATH], base="b" * 40)
        self.commit("stale proof")

        status, stderr = self.run_range()

        self.assertEqual(status, 1)
        self.assertIn("base_sha must match", stderr)

    def test_range_paths_can_come_from_stdin(self) -> None:
        self.merge_advanced_main_into_candidate()
        changed = self.git("diff", "--name-only", "--diff-filter=ACMRD", self.base, "HEAD")
        with patch("sys.stdin", StringIO(changed + "\n")):
            self.assertEqual(self.run_range("--files-from-stdin"), (0, ""))

    def test_base_requires_commit(self) -> None:
        with patch("sys.stderr", new=StringIO()), self.assertRaises(SystemExit):
            main(["--base", self.base])


if __name__ == "__main__":
    unittest.main()
