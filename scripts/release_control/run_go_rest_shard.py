#!/usr/bin/env python3
"""Share four package slots between source-listed slices and ordinary packages.

The old recipe reserved two ordinary package workers even after both slow
slices finished. A single queue releases those slots without adding jobs or
increasing the four-binary envelope. Timing weights schedule, never select,
the source-derived ordinary package inventory.
"""

from __future__ import annotations

import argparse
from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import subprocess
import sys
import time


ROOT = Path(__file__).resolve().parents[2]
MODULE = "github.com/rcourtman/pulse-go-rewrite/"
SLICED = ("internal/monitoring", "scripts/installtests")
SLOTS = 4
ORDINARY_COMMAND = "go test -race -timeout 50m $pkgs"


def ordinary_packages(index: int) -> list[str]:
    listing = subprocess.run(["go", "list", "./..."], cwd=ROOT,
                             capture_output=True, text=True, check=True)
    packages = listing.stdout.splitlines()
    required = {MODULE + name for name in (*SLICED, "internal/api")}
    if (not required.issubset(packages) or len(packages) != len(set(packages))
            or any(not name.startswith(MODULE) for name in packages)):
        raise ValueError("incomplete, duplicate or foreign source package inventory")
    ordinary = [name for name in packages if name not in required]
    selection = subprocess.run(
        [sys.executable, str(ROOT / ".github/scripts/select-go-package-shard.py"),
         str(ROOT / ".github/scripts/go-package-test-seconds.txt"), "2", str(index)],
        cwd=ROOT, input="\n".join(ordinary) + "\n",
        capture_output=True, text=True, check=True)
    selected = selection.stdout.splitlines()
    if (not selected or len(selected) != len(set(selected))
            or not set(selected).issubset(ordinary)):
        raise ValueError("empty or invalid ordinary package shard")
    return selected


def tasks(packages: list[str], index: int, output: Path,
          ordinary_command: str) -> list[tuple[str, list[str]]]:
    sliced = [(name, [sys.executable, str(ROOT / "scripts/release_control/run_go_package_slice.py"),
                      "--package", name, "--index", str(index),
                      "--output", str(output / name.rsplit("/", 1)[-1])])
              for name in SLICED]
    # One package per lease. -p=1 bounds its compiler as well as its binary;
    # GOMAXPROCS=2 still bounds each binary, exactly as in the previous recipe.
    # A completed slice can immediately admit another ordinary package.
    return sliced + [(name, ["bash", "-euo", "pipefail", "-c", ordinary_command])
                     for name in packages]


def run_task(task: tuple[str, list[str]]) -> dict:
    name, command = task
    start = time.monotonic()
    try:
        env = dict(os.environ)
        if name not in SLICED:
            env["pkgs"] = name
            env["GOFLAGS"] = env.get("GOFLAGS", "") + " -p=1"
        result = subprocess.run(command, cwd=ROOT, env=env)
        code = result.returncode
    except OSError as error:
        print(f"Could not start backend package {name}: {error}", file=sys.stderr)
        code = 127
    return {"package": name, "exit": code,
            "seconds": round(time.monotonic() - start, 3)}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--index", type=int, choices=(0, 1), required=True)
    parser.add_argument("--output", type=Path, required=True)
    # Only the existing fixed Go recipe is supported, not arbitrary shell
    # input. Keeping it in the workflow also preserves stable-shipped source
    # contract tests when main's controls are copied to a mechanical patch.
    parser.add_argument("--ordinary-command", choices=(ORDINARY_COMMAND,), required=True)
    args = parser.parse_args()
    try:
        packages = ordinary_packages(args.index)
        if sys.stdin.read().splitlines() != packages:
            raise ValueError("supplied package order differs from the complete source selection")
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"Backend package admission failed: {error}", file=sys.stderr)
        if isinstance(error, subprocess.CalledProcessError):
            sys.stdout.write(error.stdout or "")
            sys.stderr.write(error.stderr or "")
        return 1
    args.output.mkdir(parents=True, exist_ok=True)
    print("Ordinary packages in this shard:\n" + "\n".join(packages), flush=True)
    start = time.monotonic()
    # Drain every admitted task even when a sibling fails. Executor.map retains
    # source scheduling order in the receipt, not concurrent completion order.
    with ThreadPoolExecutor(max_workers=SLOTS) as pool:
        results = list(pool.map(run_task, tasks(packages, args.index, args.output, args.ordinary_command)))
    receipt = {"index": args.index, "slots": SLOTS, "tasks": results,
               "seconds": round(time.monotonic() - start, 3)}
    (args.output / "ordinary-packages.json").write_text(json.dumps(receipt, indent=2) + "\n")
    return 0 if all(result["exit"] == 0 for result in results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
