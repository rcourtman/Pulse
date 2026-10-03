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
    LEGACY_RECEIPT_PATH,
    RECEIPT_DIR,
    formatting_only_paths,
    frontend_runtime_paths,
    is_receipt_path,
    main,
    receipt_path_for,
    validate_receipt,
)
from format_staged_frontend_test import REAL_PRETTIER
from repo_file_io import strip_local_git_env


BASE_SHA = "a" * 40
CHANGED_PATH = "frontend-modern/src/components/Example.tsx"
OTHER_PATH = "frontend-modern/src/components/Other.tsx"
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

    def test_shared_receipt_is_retired_from_the_tree(self) -> None:
        self.assertFalse((REPO_ROOT / LEGACY_RECEIPT_PATH).exists())

    def test_frontend_runtime_paths_exclude_tests_and_receipt(self) -> None:
        self.assertEqual(
            frontend_runtime_paths(
                [
                    CHANGED_PATH,
                    "frontend-modern/src/components/Example.test.tsx",
                    "frontend-modern/src/components/__tests__/Example.tsx",
                    LEGACY_RECEIPT_PATH,
                    receipt_path_for({CHANGED_PATH: CONTENT_SHA}),
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

    def test_receipt_name_is_the_fingerprint_of_the_content_it_verified(self) -> None:
        digests = {CHANGED_PATH: CONTENT_SHA, OTHER_PATH: "d" * 64}
        path = receipt_path_for(digests)

        self.assertTrue(is_receipt_path(path))
        self.assertEqual(path, receipt_path_for(dict(reversed(digests.items()))))
        self.assertNotEqual(path, receipt_path_for({**digests, OTHER_PATH: "e" * 64}))
        self.assertFalse(is_receipt_path(LEGACY_RECEIPT_PATH))
        self.assertFalse(is_receipt_path(f"{RECEIPT_DIR}/README.md"))
        self.assertFalse(is_receipt_path(f"{RECEIPT_DIR}/nested/receipt.json"))

    def test_rejects_receipt_stored_under_a_hand_picked_name(self) -> None:
        errors = validate_receipt(
            valid_receipt(),
            changed_paths=[CHANGED_PATH],
            expected_base=BASE_SHA,
            expected_content_sha256={CHANGED_PATH: CONTENT_SHA},
            receipt_path=f"{RECEIPT_DIR}/storage-fix.json",
        )

        self.assertEqual(len(errors), 1)
        self.assertIn(receipt_path_for({CHANGED_PATH: CONTENT_SHA}), errors[0])

    def test_proof_only_receipt_still_needs_a_real_parent_id(self) -> None:
        receipt = valid_receipt()
        receipt["base_sha"] = "HEAD"

        errors = validate_receipt(
            receipt,
            changed_paths=[CHANGED_PATH],
            expected_base=None,
            expected_content_sha256={CHANGED_PATH: CONTENT_SHA},
        )

        self.assertTrue(any("base_sha" in error for error in errors))
        receipt["base_sha"] = BASE_SHA
        self.assertEqual(
            validate_receipt(
                receipt,
                changed_paths=[CHANGED_PATH],
                expected_base=None,
                expected_content_sha256={CHANGED_PATH: CONTENT_SHA},
            ),
            [],
        )


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


class GuardRepoTestCase(unittest.TestCase):
    """A throwaway repository the guard runs against instead of this one."""

    def setUp(self) -> None:
        self.tmpdir = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmpdir.cleanup)
        self.repo_root = Path(self.tmpdir.name)
        # Under the pre-commit hook, GIT_DIR and GIT_INDEX_FILE are exported
        # and would point this temp repo's plumbing at the real repository.
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

    def build_receipt(self, paths: list[str], *, base: str | None = None) -> dict:
        receipt = valid_receipt()
        receipt["base_sha"] = base or self.git("rev-parse", "HEAD")
        receipt["changed_paths"] = paths
        receipt["content_sha256"] = {
            path: hashlib.sha256((self.repo_root / path).read_bytes()).hexdigest()
            for path in paths
        }
        return receipt

    def write_receipt(
        self, paths: list[str], *, base: str | None = None, at: str | None = None
    ) -> str:
        receipt = self.build_receipt(paths, base=base)
        receipt_path = at or receipt_path_for(receipt["content_sha256"])
        self.write(receipt_path, json.dumps(receipt, indent=2) + "\n")
        return receipt_path

    def verified_change(self, path: str, text: str, message: str) -> str:
        self.write(path, text)
        self.write_receipt([path])
        return self.commit(message)

    def run_guard(self, *argv: str, stdin: str = "") -> tuple[int, str, str]:
        stdout, stderr = StringIO(), StringIO()
        with (
            patch.dict("os.environ", self.env, clear=True),
            patch("browser_verification_guard.REPO_ROOT", self.repo_root),
            patch("sys.stdin", StringIO(stdin)),
            patch("sys.stdout", new=stdout),
            patch("sys.stderr", new=stderr),
        ):
            status = main(list(argv))
        return status, stdout.getvalue(), stderr.getvalue()

    def run_range(self, *extra: str, base: str | None = None) -> tuple[int, str]:
        status, _, stderr = self.run_guard(
            "--base", base or self.base, "--commit", self.git("rev-parse", "HEAD"), *extra
        )
        return status, stderr

    def receipts_in_tree(self, revision: str = "HEAD") -> list[str]:
        listing = self.git("ls-tree", "-r", "--name-only", revision, "--", RECEIPT_DIR)
        return [line for line in listing.splitlines() if line]


class StagedIndexTest(GuardRepoTestCase):
    """The pre-commit form: one commit owes exactly one receipt of its own."""

    def stage_change(self) -> str:
        self.write(CHANGED_PATH, "export const a = 1;\n")
        self.git("add", CHANGED_PATH)
        return receipt_path_for(self.build_receipt([CHANGED_PATH])["content_sha256"])

    def test_blocks_frontend_change_until_its_receipt_is_staged(self) -> None:
        expected = self.stage_change()

        status, _, stderr = self.run_guard()

        self.assertEqual(status, 1)
        self.assertIn(f"{expected} must be added and staged", stderr)
        self.assertIn("--write-template", stderr)

    def test_stdin_path_list_is_held_to_the_same_receipt(self) -> None:
        self.assertEqual(self.run_guard("--files-from-stdin", stdin=CHANGED_PATH + "\n")[0], 1)

    def test_accepts_receipt_staged_at_its_fingerprint_path(self) -> None:
        self.stage_change()
        self.git("add", self.write_receipt([CHANGED_PATH]))

        status, stdout, stderr = self.run_guard()

        self.assertEqual((status, stderr), (0, ""))
        self.assertIn("1 frontend source file(s)", stdout)

    def test_write_template_creates_the_owed_receipt_but_never_a_passing_one(self) -> None:
        expected = self.stage_change()

        status, stdout, _ = self.run_guard("--write-template")
        self.assertEqual((status, stdout.strip()), (0, expected))
        self.git("add", expected)
        status, _, stderr = self.run_guard()
        self.assertEqual(status, 1)
        self.assertIn('result must be "passed"', stderr)

        # A completed receipt is never overwritten by a second template.
        self.assertEqual(self.run_guard("--write-template")[0], 1)
        self.git("add", self.write_receipt([CHANGED_PATH]))
        self.assertEqual(self.run_guard()[0], 0)

    def test_print_template_names_the_file_to_save(self) -> None:
        expected = self.stage_change()

        status, stdout, stderr = self.run_guard("--print-template")

        self.assertEqual(status, 0)
        self.assertEqual(json.loads(stdout)["changed_paths"], [CHANGED_PATH])
        self.assertIn(expected, stderr)
        self.assertFalse((self.repo_root / expected).exists())

    def test_write_template_refuses_when_nothing_needs_verifying(self) -> None:
        self.assertEqual(self.run_guard("--write-template")[0], 1)
        self.assertEqual(self.receipts_in_tree(), [])

    def test_blocks_receipt_named_by_hand(self) -> None:
        expected = self.stage_change()
        self.git("add", self.write_receipt([CHANGED_PATH], at=f"{RECEIPT_DIR}/storage-fix.json"))

        status, _, stderr = self.run_guard()

        self.assertEqual(status, 1)
        self.assertIn(f"{expected} must be added and staged", stderr)
        self.assertIn("storage-fix.json verifies different content", stderr)

    def test_blocks_source_edited_after_its_receipt_was_written(self) -> None:
        self.stage_change()
        stale = self.write_receipt([CHANGED_PATH])
        self.write(CHANGED_PATH, "export const a = 2;\n")
        self.git("add", CHANGED_PATH, stale)

        status, _, stderr = self.run_guard()

        self.assertEqual(status, 1)
        self.assertIn("the staged source changed after it was written", stderr)

    def test_blocks_a_second_receipt_in_one_commit(self) -> None:
        self.stage_change()
        self.git("add", self.write_receipt([CHANGED_PATH]))
        self.git("add", self.write_receipt([OTHER_PATH]))

        status, _, stderr = self.run_guard()

        self.assertEqual(status, 1)
        self.assertIn("exactly one receipt", stderr)

    def test_blocks_writing_the_retired_shared_receipt(self) -> None:
        self.stage_change()
        self.git("add", self.write_receipt([CHANGED_PATH], at=LEGACY_RECEIPT_PATH))

        status, _, stderr = self.run_guard()

        self.assertEqual(status, 1)
        self.assertIn(f"{LEGACY_RECEIPT_PATH} is retired", stderr)

    def test_blocks_recording_and_pruning_in_one_commit(self) -> None:
        landed = self.verified_change(OTHER_PATH, "export const b = 1;\n", "landed")
        self.assertTrue(landed)
        (old,) = self.receipts_in_tree()
        self.stage_change()
        self.git("add", self.write_receipt([CHANGED_PATH]))
        self.git("rm", "--quiet", old)

        status, _, stderr = self.run_guard()

        self.assertEqual(status, 1)
        self.assertIn("must not delete another", stderr)
        self.assertIn("--prune", stderr)

    def test_proof_only_commit_is_checked_against_the_content_it_names(self) -> None:
        self.write(CHANGED_PATH, "export const a = 1;\n")
        self.commit("frontend change without proof")
        receipt_path = self.write_receipt([CHANGED_PATH])
        self.git("add", receipt_path)
        self.assertEqual(self.run_guard()[0], 0)

        payload = json.loads((self.repo_root / receipt_path).read_text())
        payload["viewports"] = [{"width": 1280, "height": 800}]
        self.write(receipt_path, json.dumps(payload))
        self.git("add", receipt_path)
        status, _, stderr = self.run_guard()
        self.assertEqual(status, 1)
        self.assertIn("narrow width", stderr)

    def test_pruning_alone_needs_no_receipt(self) -> None:
        self.verified_change(OTHER_PATH, "export const b = 1;\n", "landed")
        self.git("rm", "--quiet", *self.receipts_in_tree())

        self.assertEqual(self.run_guard()[0], 0)


class ConcludingMergeTest(GuardRepoTestCase):
    """A merge is not asked to vouch for what its other parent already verified.

    With one shared receipt, concluding `git merge main` staged main's
    frontend files as if the author had changed them, so the guard demanded a
    receipt for files the author never ran and a branch behind main could only
    be rebuilt and re-verified.
    """

    def start_merge(self, *, ours: tuple[str, str], theirs: tuple[str, str]) -> None:
        self.git("checkout", "--quiet", "-b", "topic", self.base)
        self.verified_change(*ours, "topic change")
        self.git("checkout", "--quiet", "main")
        self.verified_change(*theirs, "main moves")
        self.git("checkout", "--quiet", "topic")
        subprocess.run(
            ["git", "-c", "user.email=t@example.com", "-c", "user.name=t",
             "merge", "--quiet", "--no-ff", "--no-commit", "main"],
            cwd=self.repo_root, capture_output=True, env=self.env,
        )

    def test_passes_frontend_work_the_merged_parent_already_carries(self) -> None:
        self.start_merge(
            ours=(CHANGED_PATH, "export const a = 1;\n"),
            theirs=(OTHER_PATH, "export const b = 1;\n"),
        )
        self.assertIn(OTHER_PATH, self.git("diff", "--cached", "--name-only"))

        status, stdout, stderr = self.run_guard()

        self.assertEqual((status, stderr), (0, ""))
        self.assertIn("beyond what its parents already carry", stdout)
        self.commit("merge main")
        self.assertEqual(self.run_range(base=self.git("rev-parse", "main"))[0], 0)

    def test_flags_merged_content_and_range_holds_it_to_a_follow_up_receipt(self) -> None:
        self.start_merge(
            ours=(CHANGED_PATH, "export const a = 1;\n"),
            theirs=(CHANGED_PATH, "export const a = 2;\n"),
        )
        self.write(CHANGED_PATH, "export const a = 3;\n")
        self.git("add", CHANGED_PATH)

        status, _, stderr = self.run_guard()
        self.assertEqual(status, 0)
        self.assertIn(CHANGED_PATH, stderr)
        self.assertIn("no browser pass", stderr)

        self.commit("merge main")
        main_tip = self.git("rev-parse", "main")
        status, stderr = self.run_range(base=main_tip)
        self.assertEqual(status, 1)
        self.assertIn(CHANGED_PATH, stderr)

        self.git("add", self.write_receipt([CHANGED_PATH]))
        self.assertEqual(self.run_guard()[0], 0)
        self.commit("record proof for the merged content")
        self.assertEqual(self.run_range(base=main_tip)[0], 0)

    def test_blocks_a_receipt_written_inside_the_merge(self) -> None:
        self.start_merge(
            ours=(CHANGED_PATH, "export const a = 1;\n"),
            theirs=(CHANGED_PATH, "export const a = 2;\n"),
        )
        self.write(CHANGED_PATH, "export const a = 3;\n")
        self.git("add", CHANGED_PATH, self.write_receipt([CHANGED_PATH]))

        status, _, stderr = self.run_guard()

        self.assertEqual(status, 1)
        self.assertIn("a merge commit cannot record browser evidence", stderr)


class IntegrationRangeTest(GuardRepoTestCase):
    """--base validates a merged integration range, not one tip receipt.

    The maintainer coordinator merges reviewed candidates onto a main that
    keeps moving, so the range tip is a merge whose parent no lane receipt can
    name. Each frontend file must still ship with content a browser pass
    verified, recorded by a non-merge commit bound to its own parent.
    """

    def merge_advanced_main_into_candidate(self) -> None:
        # main advances with its own verified frontend change while the
        # candidate is verified against the older base, then the coordinator
        # merges the candidate onto the newer main.
        self.git("checkout", "--quiet", "-b", "candidate", self.base)
        self.verified_change(CHANGED_PATH, "export const a = 1;\n", "candidate")
        self.git("checkout", "--quiet", "main")
        self.verified_change(OTHER_PATH, "export const b = 1;\n", "upstream")
        # Each side added a receipt file of its own, so git merges them
        # unaided; self.git raises if the merge stops on a conflict.
        self.git("merge", "--quiet", "--no-ff", "candidate", "-m", "integrate candidate")
        self.assertEqual(len(self.git("log", "-1", "--format=%P").split()), 2)
        self.assertEqual(len(self.receipts_in_tree()), 2)

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
        receipt_path = self.write_receipt([CHANGED_PATH])
        payload = json.loads((self.repo_root / receipt_path).read_text())
        payload["verified_at"] = "2026-10-02T13:00:00+00:00"
        self.write(receipt_path, json.dumps(payload))
        original = self.commit("frontend with malformed timestamp")
        self.assertNotEqual(self.run_workflow_browser_step().returncode, 0)

        # Same verified content, so the correction edits the same file.
        self.assertEqual(self.write_receipt([CHANGED_PATH]), receipt_path)
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

    def test_second_branch_merges_cleanly_after_the_first_and_its_range_passes(self) -> None:
        # Two pull requests cut from the same main, touching different
        # frontend files. With one shared receipt the second always conflicted
        # once the first merged.
        for branch, path in (("first", CHANGED_PATH), ("second", OTHER_PATH)):
            self.git("checkout", "--quiet", "-b", branch, self.base)
            self.verified_change(path, "export const changed = 1;\n", branch)
        self.git("checkout", "--quiet", "main")
        self.git("merge", "--quiet", "--no-ff", "first", "-m", "merge first")
        main_after_first = self.git("rev-parse", "HEAD")
        self.assertEqual(self.run_range()[0], 0)

        self.git("merge", "--quiet", "--no-ff", "second", "-m", "merge second")

        self.assertEqual(self.run_range(base=main_after_first), (0, ""))
        self.assertEqual(len(self.receipts_in_tree()), 2)

    def test_edits_to_one_file_that_git_combines_still_need_their_own_proof(self) -> None:
        # Receipts no longer conflict, so two pull requests editing different
        # parts of one file both merge. Neither browser pass saw the combined
        # content, and range validation of the landed merge must say so.
        lines = ["export const a = 0;", "", "", "", "export const z = 0;", ""]
        self.write(CHANGED_PATH, "\n".join(lines))
        seeded = self.commit("multi-line source")
        for branch, index in (("first", 0), ("second", 4)):
            self.git("checkout", "--quiet", "-b", branch, seeded)
            edited = list(lines)
            edited[index] = edited[index].replace("0", "1")
            self.verified_change(CHANGED_PATH, "\n".join(edited), branch)
        self.git("checkout", "--quiet", "main")
        self.git("merge", "--quiet", "--no-ff", "first", "-m", "merge first")
        main_after_first = self.git("rev-parse", "HEAD")
        self.git("merge", "--quiet", "--no-ff", "second", "-m", "merge second")

        status, stderr = self.run_range(base=main_after_first)

        self.assertEqual(status, 1)
        self.assertIn(f"{CHANGED_PATH} final content", stderr)

        self.write_receipt([CHANGED_PATH])
        self.commit("record proof for the combined content")
        self.assertEqual(self.run_range(base=main_after_first)[0], 0)

    def test_receipt_named_by_hand_is_not_evidence(self) -> None:
        self.write(CHANGED_PATH, "export const a = 1;\n")
        self.write_receipt([CHANGED_PATH], at=f"{RECEIPT_DIR}/storage-fix.json")
        self.commit("hand-named proof")

        status, stderr = self.run_range()

        self.assertEqual(status, 1)
        self.assertIn("receipt must be stored at", stderr)

    def test_commit_that_records_and_prunes_blocks_a_fully_covered_range(self) -> None:
        self.verified_change(OTHER_PATH, "export const b = 1;\n", "landed")
        landed = self.git("rev-parse", "HEAD")
        (old,) = self.receipts_in_tree()
        self.write(CHANGED_PATH, "export const a = 1;\n")
        self.write_receipt([CHANGED_PATH])
        self.git("rm", "--quiet", old)
        self.commit("verified change that also prunes")

        status, stderr = self.run_range(base=landed)

        self.assertEqual(status, 1)
        self.assertIn("records a receipt and deletes another", stderr)
        self.assertNotIn("is not verified", stderr)


class RetiredSharedReceiptTest(GuardRepoTestCase):
    """Work verified against the shared receipt keeps its evidence; nothing new may use it."""

    def setUp(self) -> None:
        super().setUp()
        self.write_receipt([CHANGED_PATH], at=LEGACY_RECEIPT_PATH)
        self.base = self.commit("main while the shared receipt was current")

    def test_in_flight_commit_survives_a_merge_of_the_retirement(self) -> None:
        self.git("checkout", "--quiet", "-b", "in-flight", self.base)
        self.write(CHANGED_PATH, "export const a = 1;\n")
        self.write_receipt([CHANGED_PATH], at=LEGACY_RECEIPT_PATH)
        self.commit("verified before the retirement")
        self.git("checkout", "--quiet", "main")
        self.git("rm", "--quiet", LEGACY_RECEIPT_PATH)
        retired = self.commit("retire the shared receipt")

        self.git("checkout", "--quiet", "in-flight")
        merge = subprocess.run(
            ["git", "-c", "user.email=t@example.com", "-c", "user.name=t",
             "merge", "--quiet", "--no-ff", "main"],
            cwd=self.repo_root, capture_output=True, text=True, env=self.env,
        )
        self.assertNotEqual(merge.returncode, 0, "modify/delete on the shared receipt")

        # Keeping the file changes nothing against HEAD, but would restore it
        # on main once this branch merges.
        self.git("add", LEGACY_RECEIPT_PATH)
        status, _, stderr = self.run_guard()
        self.assertEqual(status, 1)
        self.assertIn(f"{LEGACY_RECEIPT_PATH} is retired", stderr)

        self.git("rm", "--quiet", "--force", LEGACY_RECEIPT_PATH)
        self.assertEqual(self.run_guard()[0], 0)
        self.commit("merge main")
        self.assertEqual(self.run_range(base=retired), (0, ""))

    def test_recreating_the_shared_receipt_is_rejected_even_with_a_valid_one(self) -> None:
        self.git("rm", "--quiet", LEGACY_RECEIPT_PATH)
        retired = self.commit("retire the shared receipt")
        self.write(CHANGED_PATH, "export const a = 1;\n")
        self.write_receipt([CHANGED_PATH])
        self.write_receipt([CHANGED_PATH], at=LEGACY_RECEIPT_PATH)
        self.commit("verified, but also recreates the shared receipt")

        status, stderr = self.run_range(base=retired)

        self.assertEqual(status, 1)
        self.assertIn(f"{LEGACY_RECEIPT_PATH} is retired", stderr)
        self.assertNotIn("is not verified", stderr)


class PruneTest(GuardRepoTestCase):
    """Landed receipts are deleted only by changes that record none."""

    def setUp(self) -> None:
        super().setUp()
        self.verified_change(CHANGED_PATH, "export const a = 1;\n", "landed change")
        self.landed_receipts = self.receipts_in_tree()

    def test_prunes_receipts_already_on_the_landed_ref(self) -> None:
        self.git("checkout", "--quiet", "-b", "chore/prune")

        status, stdout, _ = self.run_guard("--prune", "--landed-on", "main")

        self.assertEqual(status, 0)
        self.assertIn("Pruned 1 receipt(s)", stdout)
        self.assertEqual(
            self.git("diff", "--cached", "--name-only", "--diff-filter=D").splitlines(),
            self.landed_receipts,
        )
        self.assertEqual(self.run_guard()[0], 0)
        self.assertEqual(self.run_guard("--prune", "--landed-on", "main")[0], 0)

    def test_refuses_on_a_branch_that_records_a_receipt(self) -> None:
        self.git("checkout", "--quiet", "-b", "fix/topic")
        self.verified_change(OTHER_PATH, "export const b = 1;\n", "topic change")

        status, _, stderr = self.run_guard("--prune", "--landed-on", "main")

        self.assertEqual(status, 1)
        self.assertIn("records 1 receipt(s) of its own", stderr)
        self.assertEqual(self.git("diff", "--cached", "--name-only"), "")

    def test_pruning_never_conflicts_with_an_open_frontend_change(self) -> None:
        self.git("checkout", "--quiet", "-b", "fix/topic")
        self.verified_change(OTHER_PATH, "export const b = 1;\n", "topic change")
        self.git("checkout", "--quiet", "-b", "chore/prune", "main")
        self.assertEqual(self.run_guard("--prune", "--landed-on", "main")[0], 0)
        self.commit("prune landed receipts")

        for first, second in (("chore/prune", "fix/topic"), ("fix/topic", "chore/prune")):
            self.git("checkout", "--quiet", "-B", "trial", "main")
            self.git("merge", "--quiet", "--no-ff", first, "-m", f"merge {first}")
            before_second = self.git("rev-parse", "HEAD")
            self.git("merge", "--quiet", "--no-ff", second, "-m", f"merge {second}")
            self.assertEqual(self.run_range(base=before_second)[0], 0, (first, second))
            self.assertEqual(len(self.receipts_in_tree()), 1)


if __name__ == "__main__":
    unittest.main()
