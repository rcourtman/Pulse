#!/usr/bin/env python3
"""Guard runtime code against branch-tip docs-link drift."""

from __future__ import annotations

import re
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]

FORBIDDEN_PATTERNS = (
    re.compile(r"https://github\.com/[^/\s\"')]+/[^/\s\"')]+/blob/(?:main|master)/"),
    re.compile(r"https://github\.com/[^/\s\"')]+/[^/\s\"')]+/tree/(?:main|master)/docs/"),
    re.compile(r"https://raw\.githubusercontent\.com/[^/\s\"')]+/[^/\s\"')]+/(?:main|master)/docs/"),
)

ALLOWED_BRANCH_TIP_DOC_URLS = {
    # Triage replies must always link to the current process disclosure.
    "https://github.com/rcourtman/Pulse/blob/main/docs/AI_TRANSPARENCY.md",
}

# GitHub renders this template for new contributor conversations, not from a
# shipped Pulse runtime. It must describe the current contribution policy.
# Keep the exception bound to this exact file and anchor, not runtime links or
# other branch-tip documentation in the template.
ALLOWED_REPOSITORY_POLICY_URLS = {
    ".github/PULL_REQUEST_TEMPLATE.md": {
        "https://github.com/rcourtman/Pulse/blob/main/CONTRIBUTING.md#sharing-a-tested-patch",
    },
}

SKIP_DIR_NAMES = {
    ".claude",
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


def branch_tip_docs_links(rel_path: str, content: str) -> list[str]:
    for allowed_url in ALLOWED_BRANCH_TIP_DOC_URLS | ALLOWED_REPOSITORY_POLICY_URLS.get(rel_path, set()):
        # An appended path, query or different anchor is not the reviewed URL.
        content = re.sub(re.escape(allowed_url) + r"(?=$|[\s\"'<>\)])", "", content)
    return [match.group(0) for pattern in FORBIDDEN_PATTERNS if (match := pattern.search(content))]


class RepoDocsLinkDriftTest(unittest.TestCase):
    def test_current_contribution_policy_is_allowed_only_in_the_pr_template(self) -> None:
        policy_url = next(iter(ALLOWED_REPOSITORY_POLICY_URLS[".github/PULL_REQUEST_TEMPLATE.md"]))
        link = f"[Sharing a tested patch]({policy_url})."
        self.assertEqual(branch_tip_docs_links(".github/PULL_REQUEST_TEMPLATE.md", link), [])
        for runtime_path in ("frontend-modern/src/help.ts", "internal/api/help.go", ".github/workflows/release.yml"):
            with self.subTest(path=runtime_path):
                self.assertTrue(branch_tip_docs_links(runtime_path, link))

    def test_policy_exception_does_not_allow_other_or_extended_branch_tip_links(self) -> None:
        policy_url = next(iter(ALLOWED_REPOSITORY_POLICY_URLS[".github/PULL_REQUEST_TEMPLATE.md"]))
        for url in (
            "https://github.com/rcourtman/Pulse/blob/main/docs/API.md",
            policy_url.replace("main", "master"),
            policy_url.replace("Pulse", "AnotherRepo"),
            policy_url.replace("#sharing-a-tested-patch", "#another-anchor"),
            policy_url + "?extra=1",
            policy_url + "-extra",
        ):
            with self.subTest(url=url):
                self.assertTrue(branch_tip_docs_links(".github/PULL_REQUEST_TEMPLATE.md", f"[Guide]({url})"))
        self.assertFalse(should_skip(".github/PULL_REQUEST_TEMPLATE.md"))

    def test_standing_triage_disclosure_remains_allowed(self) -> None:
        for url in ALLOWED_BRANCH_TIP_DOC_URLS:
            self.assertEqual(branch_tip_docs_links("internal/api/help.go", f"[Triage]({url})"), [])

    def test_runtime_files_do_not_reference_branch_tip_docs(self) -> None:
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

            offenders.extend(f"{rel_path}: {link}" for link in branch_tip_docs_links(rel_path, content))

        self.assertEqual(
            offenders,
            [],
            msg="runtime files still reference branch-tip docs:\n- " + "\n- ".join(offenders),
        )


if __name__ == "__main__":
    unittest.main()
