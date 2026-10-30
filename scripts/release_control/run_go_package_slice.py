#!/usr/bin/env python3
"""Run one exhaustive, order-preserving slice of a slow backend test package.

The two fixed packages have outgrown whole-package CI balancing. Source supplies
the test inventory; measured weights only move the contiguous boundary. Reuse
the API selector without changing source order, worker counts or test deadlines.
The existing JSON recorder checks every selected root's completion, including
explicit skips, and preserves failures and unfinished output.
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
MODULE = "github.com/rcourtman/pulse-go-rewrite/"
PACKAGES = ("internal/monitoring", "scripts/installtests")
RECORDER = ROOT / ".github/scripts/record-internal-api-test-seconds.py"
SELECTOR = ROOT / ".github/scripts/select-internal-api-shard.sh"
WEIGHTS = {
    package: ROOT / f"scripts/release_control/{package.rsplit('/', 1)[-1]}-test-seconds.txt"
    for package in PACKAGES
}


def source_tests(output: str) -> list[str]:
    names = [line for line in output.splitlines()
             if line.startswith(("Test", "Fuzz", "Example")) and line.isidentifier()]
    if not names or len(set(names)) != len(names):
        raise ValueError("source listing is empty or repeats test names")
    return names


def select(names: list[str], index: int, weights: Path | None = None) -> list[str]:
    if index not in (0, 1) or len(names) < 2:
        raise ValueError("two nonempty slices and an index of 0 or 1 are required")
    if weights is None:
        return names[len(names) * index // 2:len(names) * (index + 1) // 2]
    # The same selector already keeps API tests in contiguous source order.
    # Ignore ambient overrides: both workers must use the committed weights
    # and the same default for new, renamed or unmeasured source tests.
    env = {key: value for key, value in os.environ.items() if key != "DEFAULT_WEIGHT"}
    result = subprocess.run(["bash", str(SELECTOR), str(weights), "2", str(index)],
                            input="\n".join(names) + "\n", capture_output=True,
                            text=True, env=env, check=True)
    selected = result.stdout.splitlines()
    # Fail before test execution on a broken selector, never fall back to an
    # apparently successful partial selection. The source list is authority.
    if (not selected or len(selected) >= len(names) or
            selected != (names[:len(selected)] if index == 0 else names[-len(selected):])):
        raise ValueError("weighted selection is not a nonempty contiguous source slice")
    return selected


def test_filter(names: list[str], selected: list[str]) -> list[str]:
    chosen = set(selected)
    others = [name for name in names if name not in chosen]
    pattern = lambda items: "^(" + "|".join(re.escape(name) for name in items) + ")$"
    run, skip = pattern(selected), pattern(others)
    flag, value = ("-skip", skip) if len(skip) < len(run) else ("-run", run)
    if len(value.encode()) > 120000:
        raise ValueError("slice filter exceeds the 120000-byte argument bound")
    # Never combine -run with -skip: Go recompiles the latter per test then.
    return [flag, value]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--package", choices=PACKAGES, required=True)
    parser.add_argument("--index", type=int, choices=(0, 1), required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    package = "./" + args.package
    listing = subprocess.run(["go", "test", "-race", "-list", ".", package],
                             cwd=ROOT, capture_output=True, text=True)
    if listing.returncode:
        sys.stdout.write(listing.stdout)
        sys.stderr.write(listing.stderr)
        return listing.returncode
    try:
        names = source_tests(listing.stdout)
        weights = WEIGHTS[args.package]
        selected = select(names, args.index, weights)
        filters = test_filter(names, selected)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"Backend slice selection failed: {error}", file=sys.stderr)
        if isinstance(error, subprocess.CalledProcessError):
            sys.stdout.write(error.stdout or "")
            sys.stderr.write(error.stderr or "")
        return 2
    (args.output / "listed.tests").write_text("\n".join(names) + "\n")
    selection = args.output / "selected.tests"
    selection.write_text("\n".join(selected) + "\n")
    command = ["go", "test", "-race", "-count=1", "-timeout", "50m", "-json",
               *filters, package]
    print(f"{args.package} slice {args.index}/2: {len(selected)} of {len(names)} roots "
          f"({selected[0]} .. {selected[-1]})", flush=True)
    with subprocess.Popen(command, cwd=ROOT, stdout=subprocess.PIPE,
                          stderr=subprocess.STDOUT) as process:
        recorded = subprocess.run(
            [sys.executable, str(RECORDER), str(args.output / "timings.txt"),
             str(selection), MODULE + args.package], stdin=process.stdout, cwd=ROOT)
        process.stdout.close()
        test_exit = process.wait()
    receipt = {"package": MODULE + args.package, "index": args.index, "count": 2,
               "listed_roots": len(names), "selected_roots": len(selected),
               "weights": str(weights.relative_to(ROOT)),
               "test_exit": test_exit, "recorder_exit": recorded.returncode}
    (args.output / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
    print(json.dumps(receipt), flush=True)
    return 0 if test_exit == recorded.returncode == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
