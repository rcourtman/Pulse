#!/usr/bin/env python3
"""Narrow inert precompilation only for an exact, exclusively test-file diff.

Keep early rejection of broken fixtures before backend fan-out. Test files
cannot be imported by another package, so compile their packages and imports
instead of every unrelated test binary. All runtime/vet checks remain required.
Any incomplete or ambiguous source evidence retains the full compilation.
"""

import argparse
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys


COMMIT = re.compile(r"[0-9a-f]{40}")
HEADER = re.compile(rb":(000000|100644) 100644 ([0-9a-f]{40}) ([0-9a-f]{40}) ([AM])")
TEST_PATH = re.compile(rb"(?:[A-Za-z0-9][A-Za-z0-9_.-]*/)*[A-Za-z0-9][A-Za-z0-9_]*_test\.go")
FULL = ["./..."]


def changed_test_packages(raw, tree):
    if not raw or not raw.endswith(b"\0") or not tree.endswith(b"\0"):
        return FULL
    records = raw[:-1].split(b"\0")
    files = tree[:-1].split(b"\0")
    if (len(records) % 2 or len(files) != len(set(files))
            or b"go.mod" not in files or b"go.work" in files):
        return FULL
    files = set(files)
    seen = set()
    packages = set()
    for header, path in zip(records[::2], records[1::2]):
        match = HEADER.fullmatch(header)
        if not match or not TEST_PATH.fullmatch(path) or path in seen or path not in files:
            return FULL
        old_mode, old_blob, new_blob, status = match.groups()
        if (new_blob == b"0" * 40 or (status == b"A" and (old_mode != b"000000" or old_blob != b"0" * 40))
                or (status == b"M" and (old_mode != b"100644" or old_blob == b"0" * 40))):
            return FULL
        seen.add(path)
        parent = PurePosixPath(path.decode("ascii")).parent
        if any(part in ("vendor", "testdata") for part in parent.parts):
            return FULL
        # A nested module is not part of the root ./... contract. Do not infer
        # a new compiler/module invocation from a similarly named test file.
        if any(str(ancestor / "go.mod").encode() in files
               for ancestor in (parent, *parent.parents) if str(ancestor) != "."):
            return FULL
        packages.add("./" + str(parent) if str(parent) != "." else ".")
    return sorted(packages) or FULL


def read_git(*args):
    return subprocess.run(["git", *args], capture_output=True, timeout=30, check=True).stdout


def root_package_directories():
    result = subprocess.run(["go", "list", "-f", "{{.Dir}}", "./..."],
                            capture_output=True, text=True, timeout=30, check=True)
    return set(result.stdout.splitlines())


def compile_packages(base, head):
    if not COMMIT.fullmatch(base) or not COMMIT.fullmatch(head):
        return FULL
    try:
        if read_git("rev-parse", "--show-prefix").strip() or read_git("rev-parse", "HEAD").strip() != head.encode():
            return FULL
        read_git("diff", "--quiet", "--no-ext-diff", head, "--")
        raw = read_git("diff", "--no-ext-diff", "--no-renames", "--raw", "--abbrev=40", "-z", base, head, "--")
        tree = read_git("ls-tree", "-r", "--full-tree", "--name-only", "-z", head)
        packages = changed_test_packages(raw, tree)
        if packages != FULL:
            # Match the root module's actual platform/build-tag inventory.
            # An ignored or non-buildable directory is not a new invocation.
            directories = root_package_directories()
            if any(str(Path(package).resolve()) not in directories for package in packages):
                return FULL
        return packages
    except (OSError, subprocess.CalledProcessError, subprocess.TimeoutExpired):
        print("Exact test-only compilation scope unavailable; retaining full Go precompilation.", file=sys.stderr)
        return FULL


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--head", required=True)
    args = parser.parse_args()
    print("\n".join(compile_packages(args.base, args.head)))


if __name__ == "__main__":
    main()
