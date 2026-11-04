#!/usr/bin/env python3
"""Select the target snapshot of the exact provider merge being validated."""

import argparse
import re
import subprocess


def git(*arguments: str) -> str:
    return subprocess.run(
        ["git", *arguments], check=True, text=True,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    ).stdout.strip()


def verified_pr_base(checkout: str, source_head: str, event_base: str) -> str:
    for label, revision in (
        ("checkout", checkout), ("source head", source_head), ("event base", event_base),
    ):
        if not re.fullmatch(r"(?:[0-9a-f]{40}|[0-9a-f]{64})", revision):
            raise ValueError(f"{label} must be a full commit SHA")
        if git("cat-file", "-t", revision) != "commit":
            raise ValueError(f"{label} must identify a commit")

    if git("rev-parse", "HEAD") != checkout:
        raise ValueError("the working checkout does not match the validation commit")
    parents = git("rev-list", "--parents", "-n", "1", checkout).split()
    if len(parents) != 3 or parents[0] != checkout:
        raise ValueError("PR validation requires the provider's two-parent merge")
    target_parent, actual_source = parents[1:]
    if actual_source != source_head:
        raise ValueError("the tested merge does not contain the expected PR source parent")

    ancestry = subprocess.run(
        ["git", "merge-base", "--is-ancestor", event_base, target_parent],
        text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )
    if ancestry.returncode == 1:
        raise ValueError("the event base is not an ancestor of the tested target parent")
    ancestry.check_returncode()
    return target_parent


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--source-head", required=True)
    parser.add_argument("--event-base", required=True)
    args = parser.parse_args()
    try:
        base = verified_pr_base(args.commit, args.source_head, args.event_base)
    except (ValueError, subprocess.CalledProcessError) as error:
        parser.error(f"cannot verify PR validation base: {error}")
    print(base)


if __name__ == "__main__":
    main()
