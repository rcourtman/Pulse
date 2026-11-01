#!/usr/bin/env python3
"""Share four package slots between source-listed slices and ordinary packages.

The old recipe reserved two ordinary package workers even after both slow
slices finished. A single queue releases those slots without adding jobs or
increasing the four-binary envelope. Timing weights schedule, never select,
the source-derived ordinary package inventory.
"""

from __future__ import annotations

import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
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
        observation = {"started": True, "exit": result.returncode}
    except OSError as error:
        print(f"Could not start backend package {name}: {error}", file=sys.stderr)
        # No process exit was observed. In particular, do not manufacture the
        # shell's command-not-found exit for a process that never started.
        observation = {"started": False, "exit": None, "start_error_errno": error.errno}
    return {"package": name, **observation,
            "seconds": round(time.monotonic() - start, 3)}


def retain_results(output: Path, index: int, planned: list[str], results: dict,
                   start: float, *, complete: bool = False) -> None:
    # Record only returned observations, in the original source order. A
    # planned/unobserved task is not evidence of execution or current liveness.
    receipt = {"schema_version": 1, "index": index, "slots": SLOTS,
               "planned_packages": planned, "collection_complete": complete,
               "unobserved_packages": [name for name in planned if name not in results],
               "tasks": [results[name] for name in planned if name in results],
               "seconds": round(time.monotonic() - start, 3)}
    # Readers and the existing always-run artifact upload must see either the
    # previous complete JSON snapshot or this one, never a truncated overwrite.
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=output,
                                         prefix=".ordinary-packages-", delete=False) as stream:
            temporary = Path(stream.name)
            json.dump(receipt, stream, indent=2)
            stream.write("\n")
        temporary.replace(output / "ordinary-packages.json")
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


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
    admitted = tasks(packages, args.index, args.output, args.ordinary_command)
    planned = [name for name, _ in admitted]
    results = {}
    retain_results(args.output, args.index, planned, results, start)
    # Drain every admitted task even when a sibling fails. Collect in completion
    # order so a slow first task cannot hide already returned results. Retained
    # snapshots still use source order, including the final legacy tasks list.
    with ThreadPoolExecutor(max_workers=SLOTS) as pool:
        futures = {pool.submit(run_task, task): task[0] for task in admitted}
        for future in as_completed(futures):
            results[futures[future]] = future.result()
            retain_results(args.output, args.index, planned, results, start)
    retain_results(args.output, args.index, planned, results, start, complete=True)
    return 0 if all(result["exit"] == 0 for result in results.values()) else 1


if __name__ == "__main__":
    raise SystemExit(main())
