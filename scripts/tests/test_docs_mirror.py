#!/usr/bin/env python3
"""Guard the shipped-docs mirror check that backs the pre-commit hook."""

from __future__ import annotations

import importlib.util
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "check_docs_mirror.py"
SPEC = importlib.util.spec_from_file_location("check_docs_mirror", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
docs_mirror = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = docs_mirror
SPEC.loader.exec_module(docs_mirror)


# Background git processes (auto-gc, fsmonitor, maintenance) can still be
# writing under .git when TemporaryDirectory cleanup runs, which makes rmtree
# fail with "Directory not empty". Disable them for the throwaway repos and
# ignore host config so it cannot re-enable them.
GIT_ENV = {
    **os.environ,
    "GIT_CONFIG_GLOBAL": os.devnull,
    "GIT_CONFIG_SYSTEM": os.devnull,
}


def run_git(root: Path, *args: str) -> None:
    subprocess.run(
        [
            "git",
            "-C",
            str(root),
            "-c",
            "user.email=test@example.invalid",
            "-c",
            "user.name=test",
            "-c",
            "gc.auto=0",
            "-c",
            "core.fsmonitor=false",
            "-c",
            "maintenance.auto=false",
            *args,
        ],
        check=True,
        capture_output=True,
        env=GIT_ENV,
    )


def write(root: Path, relative: str, content: str) -> None:
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


class DocsMirrorMappingTest(unittest.TestCase):
    def test_docs_sourced_mapping(self) -> None:
        self.assertEqual(
            docs_mirror.source_for("frontend-modern/public/docs/i18n/de/README.md"),
            "docs/i18n/de/README.md",
        )

    def test_root_sourced_mapping(self) -> None:
        self.assertEqual(
            docs_mirror.source_for("frontend-modern/public/docs/SECURITY.md"),
            "SECURITY.md",
        )


class DocsMirrorStagedTest(unittest.TestCase):
    def setUp(self) -> None:
        self._temporary = tempfile.TemporaryDirectory(ignore_cleanup_errors=True)
        self.addCleanup(self._temporary.cleanup)
        self.root = Path(self._temporary.name)
        run_git(self.root, "init", "-q")

    def test_synced_staged_pair_passes(self) -> None:
        write(self.root, "docs/GUIDE.md", "# Guide\n")
        write(self.root, "frontend-modern/public/docs/GUIDE.md", "# Guide\n")
        run_git(self.root, "add", "docs/GUIDE.md", "frontend-modern/public/docs/GUIDE.md")

        errors, warnings = docs_mirror.check_staged(self.root)

        self.assertEqual(errors, [])
        self.assertEqual(warnings, [])

    def test_staged_source_with_stale_mirror_fails(self) -> None:
        write(self.root, "docs/GUIDE.md", "# Guide v1\n")
        write(self.root, "frontend-modern/public/docs/GUIDE.md", "# Guide v1\n")
        run_git(self.root, "add", "-A")
        run_git(self.root, "commit", "-q", "-m", "seed")
        write(self.root, "docs/GUIDE.md", "# Guide v2\n")
        run_git(self.root, "add", "docs/GUIDE.md")

        errors, warnings = docs_mirror.check_staged(self.root)

        self.assertEqual(len(errors), 1)
        self.assertIn("docs/GUIDE.md and frontend-modern/public/docs/GUIDE.md", errors[0])
        self.assertIn("git show :docs/GUIDE.md", errors[0])
        self.assertEqual(warnings, [])

    def test_staged_root_sourced_doc_with_stale_mirror_fails(self) -> None:
        write(self.root, "SECURITY.md", "# Security v1\n")
        write(self.root, "frontend-modern/public/docs/SECURITY.md", "# Security v1\n")
        run_git(self.root, "add", "-A")
        run_git(self.root, "commit", "-q", "-m", "seed")
        write(self.root, "SECURITY.md", "# Security v2\n")
        run_git(self.root, "add", "SECURITY.md")

        errors, _warnings = docs_mirror.check_staged(self.root)

        self.assertEqual(len(errors), 1)
        self.assertIn("SECURITY.md and frontend-modern/public/docs/SECURITY.md", errors[0])

    def test_staged_orphan_mirror_fails(self) -> None:
        write(self.root, "frontend-modern/public/docs/NEW.md", "# New\n")
        run_git(self.root, "add", "frontend-modern/public/docs/NEW.md")

        errors, _warnings = docs_mirror.check_staged(self.root)

        self.assertEqual(len(errors), 1)
        self.assertIn("no repo source docs/NEW.md", errors[0])

    def test_preexisting_drift_only_warns_on_unrelated_commit(self) -> None:
        write(self.root, "docs/GUIDE.md", "# Guide v2\n")
        write(self.root, "frontend-modern/public/docs/GUIDE.md", "# Guide v1\n")
        run_git(self.root, "add", "-A")
        run_git(self.root, "commit", "-q", "-m", "seed drift")
        write(self.root, "unrelated.txt", "x\n")
        run_git(self.root, "add", "unrelated.txt")

        errors, warnings = docs_mirror.check_staged(self.root)

        self.assertEqual(errors, [])
        self.assertEqual(len(warnings), 1)
        self.assertIn("docs/GUIDE.md and frontend-modern/public/docs/GUIDE.md", warnings[0])

    def test_staged_pair_with_github_tree_link_fails(self) -> None:
        content = "See https://github.com/rcourtman/Pulse/blob/main/docs/OTHER.md\n"
        write(self.root, "docs/GUIDE.md", content)
        write(self.root, "frontend-modern/public/docs/GUIDE.md", content)
        run_git(self.root, "add", "-A")

        errors, _warnings = docs_mirror.check_staged(self.root)

        self.assertEqual(len(errors), 1)
        self.assertIn("must not link to", errors[0])


    def test_index_gallery_exception_is_exact_in_staged_and_worktree_checks(self) -> None:
        gallery = docs_mirror.GALLERY_LINK
        for target in ("README.md", "GUIDE.md", "nested/README.md"):
            for url in (
                gallery, gallery + "?view=1", gallery + "#other", gallery + ".extra",
                gallery.replace("/main/", "/master/"),
                gallery.replace("SCREENSHOTS.md", "CONFIGURATION.md"),
            ):
                with self.subTest(target=target, url=url):
                    content = f"[Product screenshots on GitHub]({url})\n"
                    write(self.root, f"docs/{target}", content)
                    write(self.root, f"frontend-modern/public/docs/{target}", content)
                    run_git(self.root, "add", f"docs/{target}", f"frontend-modern/public/docs/{target}")
                    expected = target == "README.md" and url == gallery
                    # master is outside this check's existing main-only rule;
                    # the repository-wide drift guard still rejects it.
                    expected = expected or "/master/" in url
                    self.assertEqual(not docs_mirror.check_staged(self.root)[0], expected)
                    self.assertEqual(not docs_mirror.check_worktree(self.root)[0], expected)
                    write(self.root, f"docs/{target}", "# Restored\n")
                    write(self.root, f"frontend-modern/public/docs/{target}", "# Restored\n")
                    run_git(self.root, "add", f"docs/{target}", f"frontend-modern/public/docs/{target}")

    def test_gallery_cannot_hide_another_branch_tip_help_link(self) -> None:
        content = (
            f"[Gallery]({docs_mirror.GALLERY_LINK})\n"
            f"[Setup]({docs_mirror.FORBIDDEN_LINK}docs/CONFIGURATION.md)\n"
        )
        write(self.root, "docs/README.md", content)
        write(self.root, "frontend-modern/public/docs/README.md", content)
        run_git(self.root, "add", "docs/README.md", "frontend-modern/public/docs/README.md")
        self.assertTrue(docs_mirror.check_staged(self.root)[0])
        self.assertTrue(docs_mirror.check_worktree(self.root)[0])


class DocsMirrorWorktreeTest(unittest.TestCase):
    def test_worktree_drift_and_sync(self) -> None:
        with tempfile.TemporaryDirectory(ignore_cleanup_errors=True) as temporary:
            root = Path(temporary)
            write(root, "docs/GUIDE.md", "# Guide v2\n")
            write(root, "frontend-modern/public/docs/GUIDE.md", "# Guide v1\n")
            write(root, "SECURITY.md", "# Security\n")
            write(root, "frontend-modern/public/docs/SECURITY.md", "# Security\n")

            errors, checked = docs_mirror.check_worktree(root)

            self.assertEqual(checked, 2)
            self.assertEqual(len(errors), 1)
            self.assertIn("cp docs/GUIDE.md frontend-modern/public/docs/GUIDE.md", errors[0])

    def test_repo_shipped_docs_are_synced(self) -> None:
        errors, checked = docs_mirror.check_worktree(ROOT)

        self.assertEqual(errors, [])
        self.assertGreater(checked, 0)


if __name__ == "__main__":
    unittest.main()
