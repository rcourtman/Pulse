#!/usr/bin/env python3
"""Keep shipped doc copies under frontend-modern/public/docs in sync.

Every Markdown file under frontend-modern/public/docs/ is a byte-for-byte
copy of a repo doc: docs/<same relative path>, except a small set shipped
from the repository root. CI enforces this in the docsLinks vitest
("keeps shipped docs content synced with repo docs"), but the git hooks do
not run vitest, so a commit that edits a mirrored doc without its copy
passes the hooks and breaks main's Frontend job.

Modes:
  --staged  Compare index blobs; fail only when this commit stages either
            side of an out-of-sync pair (pre-existing drift warns). Run
            from the pre-commit hook.
  (none)    Compare working-tree bytes for every shipped doc.
"""

from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]

SHIPPED_ROOT = "frontend-modern/public/docs"

# Shipped from the repository root rather than docs/. Mirrors the
# rootSourcedDocs set in frontend-modern/src/utils/__tests__/docsLinks.test.ts.
ROOT_SOURCED_DOCS = frozenset(
    {"SECURITY.md", "TERMS.md", "ARCHITECTURE.md", "CONTRIBUTING.md"}
)

# Shipped docs must not deep-link into the GitHub tree; the docsLinks vitest
# rejects this in CI.
FORBIDDEN_LINK = "https://github.com/rcourtman/Pulse/blob/main/"


def source_for(mirror: str) -> str:
    relative = mirror[len(SHIPPED_ROOT) + 1 :]
    if relative in ROOT_SOURCED_DOCS:
        return relative
    return f"docs/{relative}"


def git_output(root: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(root), *args],
        check=True,
        capture_output=True,
        text=True,
    ).stdout


def index_blobs(root: Path) -> dict[str, str]:
    """Map index path -> blob OID for docs surfaces (equal OID = equal bytes)."""

    records = git_output(
        root,
        "ls-files",
        "-s",
        "-z",
        "--",
        SHIPPED_ROOT,
        "docs",
        *sorted(ROOT_SOURCED_DOCS),
    )
    blobs: dict[str, str] = {}
    for record in records.split("\0"):
        if not record:
            continue
        meta, path = record.split("\t", 1)
        blobs[path] = meta.split()[1]
    return blobs


def staged_paths(root: Path) -> set[str]:
    return {
        path
        for path in git_output(
            root, "diff", "--cached", "--name-only", "-z"
        ).split("\0")
        if path
    }


def sync_command(source: str, mirror: str) -> str:
    return f"git show :{source} > {mirror} && git add {mirror}"


def check_staged(root: Path) -> tuple[list[str], list[str]]:
    blobs = index_blobs(root)
    staged = staged_paths(root)
    errors: list[str] = []
    warnings: list[str] = []

    for mirror in sorted(blobs):
        if not mirror.startswith(f"{SHIPPED_ROOT}/") or not mirror.endswith(".md"):
            continue
        source = source_for(mirror)
        touched = mirror in staged or source in staged
        source_blob = blobs.get(source)

        if source_blob is None:
            message = (
                f"{mirror}: shipped copy has no repo source {source}; "
                f"remove the copy too (git rm {mirror}) or restore the source"
            )
        elif source_blob != blobs[mirror]:
            message = (
                f"{source} and {mirror} differ in the staged tree; sync the "
                f"shipped copy from the staged source:\n"
                f"    {sync_command(source, mirror)}\n"
                f"  (if the edit was made to the shipped copy, apply it to "
                f"{source} instead)"
            )
        else:
            if touched and FORBIDDEN_LINK in git_output(
                root, "cat-file", "blob", blobs[mirror]
            ):
                errors.append(
                    f"{mirror}: shipped docs must not link to "
                    f"{FORBIDDEN_LINK} (link the shipped path instead)"
                )
            continue

        (errors if touched else warnings).append(message)

    return errors, warnings


def check_worktree(root: Path) -> tuple[list[str], int]:
    errors: list[str] = []
    shipped_root = root / SHIPPED_ROOT
    mirrors = sorted(
        path.relative_to(root).as_posix() for path in shipped_root.rglob("*.md")
    )

    for mirror in mirrors:
        source = source_for(mirror)
        source_path = root / source
        if not source_path.exists():
            errors.append(f"{mirror}: shipped copy has no repo source {source}")
            continue
        mirror_bytes = (root / mirror).read_bytes()
        if mirror_bytes != source_path.read_bytes():
            errors.append(
                f"{source} and {mirror} differ; sync the shipped copy:\n"
                f"    cp {source} {mirror}"
            )
        elif FORBIDDEN_LINK.encode() in mirror_bytes:
            errors.append(
                f"{mirror}: shipped docs must not link to {FORBIDDEN_LINK}"
            )

    return errors, len(mirrors)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--staged",
        action="store_true",
        help="compare index blobs and fail only on pairs this commit touches",
    )
    args = parser.parse_args()

    if args.staged:
        errors, warnings = check_staged(ROOT)
        if warnings:
            print(
                "Warning: shipped docs already out of sync in HEAD "
                "(not touched by this commit; CI is failing on main):",
                file=sys.stderr,
            )
            for warning in warnings:
                print(f"- {warning}", file=sys.stderr)
        if errors:
            print("Shipped docs mirror check failed:", file=sys.stderr)
            for error in errors:
                print(f"- {error}", file=sys.stderr)
            return 1
        print("Shipped docs mirror check passed.")
        return 0

    errors, checked = check_worktree(ROOT)
    if errors:
        print("Shipped docs mirror check failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1
    print(f"Shipped docs mirror check passed ({checked} shipped docs).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
