#!/usr/bin/env python3
"""Run one exhaustive, order-preserving slice of a slow backend test package.

The two fixed packages have outgrown whole-package CI balancing. Source supplies
the test inventory; no timing file or maintained test-name allowlist defines
coverage. The existing JSON recorder checks every selected root's completion,
including explicit skips, and preserves failures and unfinished output.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
MODULE = "github.com/rcourtman/pulse-go-rewrite/"
PACKAGES = ("internal/monitoring", "scripts/installtests")
RECORDER = ROOT / ".github/scripts/record-internal-api-test-seconds.py"


def source_tests(output: str) -> list[str]:
    names = [line for line in output.splitlines()
             if line.startswith(("Test", "Fuzz", "Example")) and line.isidentifier()]
    if not names or len(set(names)) != len(names):
        raise ValueError("source listing is empty or repeats test names")
    return names


def select(names: list[str], index: int) -> list[str]:
    if index not in (0, 1) or len(names) < 2:
        raise ValueError("two nonempty slices and an index of 0 or 1 are required")
    return names[len(names) * index // 2:len(names) * (index + 1) // 2]


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
        selected = select(names, args.index)
        filters = test_filter(names, selected)
    except ValueError as error:
        print(f"Backend slice selection failed: {error}", file=sys.stderr)
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
               "test_exit": test_exit, "recorder_exit": recorded.returncode}
    (args.output / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
    print(json.dumps(receipt), flush=True)
    return 0 if test_exit == recorded.returncode == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
