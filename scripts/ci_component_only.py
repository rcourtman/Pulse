"""Select component PRs using the entire base-to-tested-tree diff."""

import argparse
import re
import subprocess
import sys

from ci_browser_proof_only import COMMIT, RECEIPT


SOURCE_DIR = rb"frontend-modern/src/(?:components|features)/(?:[A-Za-z0-9_-]+/)*"
COMPONENT = re.compile(SOURCE_DIR + rb"[A-Z][A-Za-z0-9]*\.tsx")
TEST = re.compile(SOURCE_DIR + rb"[A-Za-z0-9_.-]+\.(?:test|spec)\.tsx?")
# Include both sides of every change. A symlink, submodule, executable or
# type change at a component-looking path is outside this contract.
REGULAR_CHANGE = re.compile(
    rb":(?:100644 100644 [a-f0-9]+ [a-f0-9]+ M|"
    rb"000000 100644 [a-f0-9]+ [a-f0-9]+ A|"
    rb"100644 000000 [a-f0-9]+ [a-f0-9]+ D)"
)


def component_only_diff(base, head):
    if not COMMIT.fullmatch(base) or not COMMIT.fullmatch(head):
        return False
    try:
        # A stale event base can hide a backend revert in the tested merge.
        # The raw first parent is available even in Actions' depth-one clone.
        commit = subprocess.check_output(["git", "cat-file", "commit", head], timeout=30)
        if commit.split(b"\n", 2)[1:2] != [b"parent " + base.encode("ascii")]:
            return False
        raw = subprocess.check_output(
            ["git", "diff", "--raw", "--no-ext-diff", "--no-renames", "-z", base, head, "--"],
            timeout=30,
        )
    except (OSError, subprocess.SubprocessError) as error:
        print(f"Component diff unavailable: {error}", file=sys.stderr)
        return False
    records = raw.split(b"\0")
    if records.pop() != b"" or len(records) % 2:
        return False
    has_component = False
    for header, path in zip(records[::2], records[1::2]):
        if not REGULAR_CHANGE.fullmatch(header):
            return False
        if COMPONENT.fullmatch(path) and b"/__tests__/" not in path:
            has_component = True
        elif not (TEST.fullmatch(path) or RECEIPT.fullmatch(path)):
            return False
    return has_component


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True)
    parser.add_argument("--head", required=True)
    args = parser.parse_args()
    print("true" if component_only_diff(args.base, args.head) else "false")
