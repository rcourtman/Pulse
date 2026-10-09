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

# This GitHub submission template needs the current contribution process, not
# the process from an installed release. Do not admit that URL in runtime code
# or broaden the exception to other links in the template.
TEMPLATE_GUIDANCE_PATH = ".github/PULL_REQUEST_TEMPLATE.md"
TEMPLATE_GUIDANCE_URL = "https://github.com/rcourtman/Pulse/blob/main/CONTRIBUTING.md#sharing-a-tested-patch"

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


def link_offenders(rel_path: str, content: str) -> list[str]:
    for allowed_url in ALLOWED_BRANCH_TIP_DOC_URLS:
        content = content.replace(allowed_url, "")
    if rel_path == TEMPLATE_GUIDANCE_PATH:
        content = re.sub(
            re.escape(TEMPLATE_GUIDANCE_URL) + r"(?=$|[\s\"')>\]])", "", content
        )
    return [
        f"{rel_path}: {match.group(0)}"
        for pattern in FORBIDDEN_PATTERNS
        if (match := pattern.search(content))
    ]


class RepoDocsLinkDriftTest(unittest.TestCase):
    def test_current_contribution_link_is_limited_to_the_submission_template(self) -> None:
        link = f"[Sharing a tested patch]({TEMPLATE_GUIDANCE_URL})."
        self.assertEqual(link_offenders(TEMPLATE_GUIDANCE_PATH, link), [])
        for path in (
            "frontend-modern/src/utils/help.ts", "install.sh",
            ".github/ISSUE_TEMPLATE/bug.md", ".github/workflows/helper.js",
        ):
            with self.subTest(path=path):
                self.assertFalse(should_skip(path))
                self.assertTrue(link_offenders(path, link))

    def test_template_exception_cannot_admit_other_branches_targets_or_anchors(self) -> None:
        for link in (
            TEMPLATE_GUIDANCE_URL.replace("/main/", "/master/"),
            TEMPLATE_GUIDANCE_URL.split("#", 1)[0],
            TEMPLATE_GUIDANCE_URL.replace("sharing-a-tested-patch", "unrelated-section"),
            TEMPLATE_GUIDANCE_URL + "-unrelated",
            TEMPLATE_GUIDANCE_URL.replace("CONTRIBUTING.md#sharing-a-tested-patch", "SECURITY.md"),
        ):
            with self.subTest(link=link):
                self.assertTrue(link_offenders(TEMPLATE_GUIDANCE_PATH, f"[Guide]({link})"))

    def test_current_guidance_does_not_hide_an_additional_drifting_link(self) -> None:
        content = f"[Patch]({TEMPLATE_GUIDANCE_URL}) [Other](https://github.com/rcourtman/Pulse/blob/main/SECURITY.md)"
        self.assertTrue(link_offenders(TEMPLATE_GUIDANCE_PATH, content))

    def test_immutable_and_shipped_runtime_docs_remain_valid(self) -> None:
        for link in (
            "https://github.com/rcourtman/Pulse/blob/v6.5.0/docs/CONFIGURATION.md",
            "https://github.com/rcourtman/Pulse/blob/39043470bc9824b35e093aa51ddac11e7f30ef06/docs/CONFIGURATION.md",
            "/docs/CONFIGURATION.md",
        ):
            with self.subTest(link=link):
                self.assertEqual(link_offenders("frontend-modern/src/utils/help.ts", link), [])

    def test_existing_current_triage_disclosure_exception_is_preserved(self) -> None:
        for link in ALLOWED_BRANCH_TIP_DOC_URLS:
            self.assertEqual(link_offenders("scripts/triage.py", link), [])

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

            offenders.extend(link_offenders(rel_path, content))

        self.assertEqual(
            offenders,
            [],
            msg="runtime files still reference branch-tip docs:\n- " + "\n- ".join(offenders),
        )


if __name__ == "__main__":
    unittest.main()
