#!/usr/bin/env python3
"""Recognise receipt-only tree changes, never substitute receipts for source tests.

Canonical Governance still validates these files. Runtime CI can be omitted only
when the complete tree diff contains nothing but browser evidence JSON. Any
source, dependency, workflow, documentation or unreadable diff retains runtime CI.
"""

import argparse
import re
import subprocess
import sys


COMMIT = re.compile(r"[0-9a-f]{40}")
RECEIPT = re.compile(rb"frontend-modern/browser-verification/[A-Za-z0-9_-]+\.json")


def receipt_only_paths(paths):
    return bool(paths) and all(RECEIPT.fullmatch(path) is not None for path in paths)


def receipt_only_diff(base, head):
    if not COMMIT.fullmatch(base) or not COMMIT.fullmatch(head):
        return False
    try:
        # Null delimiters and disabled rename detection prevent quoted or
        # renamed source paths from disappearing into a receipt-only list.
        result = subprocess.run(
            ["git", "diff", "--no-ext-diff", "--no-renames", "--name-only", "-z",
             base, head, "--"],
            capture_output=True, timeout=30, check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        print("Tree diff unavailable; retaining runtime CI.", file=sys.stderr)
        return False
    if result.returncode != 0 or not result.stdout.endswith(b"\0"):
        return False
    return receipt_only_paths(result.stdout[:-1].split(b"\0"))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--head", required=True)
    args = parser.parse_args()
    print("true" if receipt_only_diff(args.base, args.head) else "false")


if __name__ == "__main__":
    main()
