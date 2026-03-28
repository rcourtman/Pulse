#!/usr/bin/env python3
"""Guard runtime code against GitHub main docs-link drift."""

from __future__ import annotations

import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]

FORBIDDEN_SNIPPETS = (
    "https://github.com/rcourtman/Pulse/blob/main/",
    "https://raw.githubusercontent.com/rcourtman/Pulse/main/docs/",
)

SKIP_DIR_NAMES = {
    ".git",
    ".next",
    ".pytest_cache",
    "dist",
    "node_modules",
    "tmp",
}

SKIP_PATH_PARTS = {
    "docs/release-control",
    "frontend-modern/public/docs",
}

SKIP_FILE_SUFFIXES = (
    ".png",
    ".jpg",
    ".jpeg",
    ".gif",
    ".ico",
    ".pdf",
    ".svg",
    ".woff",
    ".woff2",
    ".ttf",
)


def should_skip(rel_path: str) -> bool:
    if any(part in SKIP_DIR_NAMES for part in Path(rel_path).parts):
        return True
    if any(fragment in rel_path for fragment in SKIP_PATH_PARTS):
        return True

    name = Path(rel_path).name
    if name.endswith((".test.ts", ".test.tsx", ".test.js", ".test.jsx", ".spec.ts", ".spec.tsx", ".spec.js", ".spec.jsx")):
        return True
    if name.endswith("_test.py") or name.startswith("test_"):
        return True
    if name.endswith(".log"):
        return True
    if rel_path.endswith(SKIP_FILE_SUFFIXES):
        return True
    return False


class RepoDocsLinkDriftTest(unittest.TestCase):
    def test_runtime_files_do_not_reference_github_main_docs(self) -> None:
        offenders: list[str] = []

        for path in REPO_ROOT.rglob("*"):
            if not path.is_file():
                continue

            rel_path = path.relative_to(REPO_ROOT).as_posix()
            if should_skip(rel_path):
                continue

            try:
                content = path.read_text(encoding="utf-8")
            except UnicodeDecodeError:
                continue

            for snippet in FORBIDDEN_SNIPPETS:
                if snippet in content:
                    offenders.append(f"{rel_path}: {snippet}")

        self.assertEqual(
            offenders,
            [],
            msg="runtime files still reference GitHub main docs:\n- " + "\n- ".join(offenders),
        )


if __name__ == "__main__":
    unittest.main()
