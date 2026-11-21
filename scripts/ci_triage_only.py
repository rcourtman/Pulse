#!/usr/bin/env python3
"""Select the dedicated issue-triage check for an isolated triage tree change.

These inputs are not part of the Pulse application. Their locked parser and
silent-mutation tests still run on the proposed source. The two intake forms
also need the independent governance policy suite, so admit them only on main.
Mixed, empty or unavailable diffs retain the ordinary application checks.
Changes to this selector or its workflow/tests cannot opt themselves out.
"""

import argparse
import os
import re
import subprocess
import sys


COMMIT = re.compile(r"[0-9a-f]{40}")
TRIAGE_INPUTS = frozenset({
    b".github/scripts/issue-version-triage.cjs",
    b".github/scripts/issue-version-triage.test.cjs",
    b".github/scripts/triage-comment-policy.test.cjs",
    b".github/scripts/package.json",
    b".github/scripts/package-lock.json",
    b"docs/ISSUE_TRIAGE.md",
})
MAIN_INTAKE_FORMS = frozenset({
    b".github/ISSUE_TEMPLATE/bug_report.yml",
    b".github/ISSUE_TEMPLATE/v6_rc_feedback.yml",
})


def triage_only_paths(paths, *, target_branch=""):
    allowed = TRIAGE_INPUTS | MAIN_INTAKE_FORMS if target_branch == "main" else TRIAGE_INPUTS
    return bool(paths) and all(path in allowed for path in paths)


def triage_only_diff(base, head, *, target_branch=""):
    if not COMMIT.fullmatch(base) or not COMMIT.fullmatch(head):
        return False
    try:
        # Inspect the entire range, including deleted names. Neither a rename
        # into this allowlist nor Git's quoted display of a filename may hide
        # a product change. A missing/truncated diff always keeps runtime CI.
        result = subprocess.run(
            ["git", "diff", "--no-ext-diff", "--no-renames", "--name-only", "-z",
             base, head, "--"],
            capture_output=True, timeout=30, check=False,
        )
    except (OSError, subprocess.TimeoutExpired):
        print("Triage tree diff unavailable; retaining application CI.", file=sys.stderr)
        return False
    if result.returncode != 0 or not result.stdout.endswith(b"\0"):
        return False
    return triage_only_paths(result.stdout[:-1].split(b"\0"), target_branch=target_branch)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--head", required=True)
    args = parser.parse_args()
    # Stable patches can copy the workflow without updating this helper.
    # Use the existing workflow environment; keep the original CLI intact.
    target_branch = os.environ.get("GOVERNANCE_TARGET_BRANCH", "")
    result = triage_only_diff(args.base, args.head, target_branch=target_branch)
    print("true" if result else "false")


if __name__ == "__main__":
    main()
