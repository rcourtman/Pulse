#!/usr/bin/env python3
"""Keep isolated completion-helper changes on their required governance checks.

Canonical Governance always exercises these exact helpers, their full test
suites and consuming audits. They are not application inputs. Nothing else,
including this selector, its tests, workflows or the completion-pair registry,
is admitted here. Empty, mixed and unavailable diffs do not select this exception.
"""

import argparse
import re
import subprocess
import sys


COMMIT = re.compile(r"[0-9a-f]{40}")
COMPLETION_INPUTS = frozenset({
    b"scripts/release_control/canonical_completion_guard.py",
    b"scripts/release_control/canonical_completion_guard_test.py",
    b"scripts/release_control/canonical_completion_history.py",
    b"scripts/release_control/canonical_completion_history_test.py",
})


def completion_only_paths(paths):
    return bool(paths) and all(path in COMPLETION_INPUTS for path in paths)


def completion_only_diff(base, head):
    if not COMMIT.fullmatch(base) or not COMMIT.fullmatch(head):
        return False
    try:
        # All changed names, including both sides of renames and deletions.
        # NUL framing prevents Git's display quoting from hiding source paths.
        result = subprocess.run(
            ["git", "diff", "--no-ext-diff", "--no-renames", "--name-only", "-z",
             base, head, "--"],
            capture_output=True, timeout=30, check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        print("Completion-helper diff unavailable; retaining application CI.", file=sys.stderr)
        return False
    if result.returncode != 0 or not result.stdout.endswith(b"\0"):
        return False
    return completion_only_paths(result.stdout[:-1].split(b"\0"))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--head", required=True)
    args = parser.parse_args()
    print("true" if completion_only_diff(args.base, args.head) else "false")


if __name__ == "__main__":
    main()
