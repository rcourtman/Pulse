#!/usr/bin/env python3
"""Rebuild .github/scripts/internal-api-test-seconds.txt from CI timings.

Every `Backend tests (api-N)` job of Build and Test uploads an
`internal-api-test-seconds-N` artifact written by
record-internal-api-test-seconds.py, holding the elapsed seconds of each
top-level internal/api test it ran under -race on the GitHub runner. This
script downloads those artifacts for one or more runs (or reads files already
on disk), takes the median seconds of every test across the runs, and writes
the weights file that select-internal-api-shard.sh uses to cut the shards.

Tests at or above --min-seconds are listed by name. Every other test, and any
test added later, weighs the file's DEFAULT_WEIGHT, which is set to the mean
of the unlisted tests so the per-shard totals stay close to what CI measured.

Usage, from the repository root:
  python3 .github/scripts/refresh-internal-api-test-seconds.py --run <run-id> [--run <run-id> ...]
  python3 .github/scripts/refresh-internal-api-test-seconds.py --dir <downloaded-artifacts>

Use runs whose api shards all passed, ideally two or three recent ones, since
a single runner can be 30% faster or slower than the next. Pass --tests with
the output of `go test -list . ./internal/api` to also print the shard cut
the new weights produce.
"""

from __future__ import annotations

import argparse
import datetime
import statistics
import subprocess
import sys
import tempfile
from collections import defaultdict
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
WEIGHTS = REPO / ".github" / "scripts" / "internal-api-test-seconds.txt"
SELECTOR = REPO / ".github" / "scripts" / "select-internal-api-shard.sh"
ARTIFACT_PATTERN = "internal-api-test-seconds-*"


def download(run_id: str, into: Path, repo: str) -> None:
    subprocess.run(
        ["gh", "run", "download", run_id, "--repo", repo, "--pattern", ARTIFACT_PATTERN, "--dir", str(into)],
        check=True,
    )


def read_timings(files: list[Path]) -> tuple[dict[str, list[float]], list[tuple[Path, float | None, float, int]]]:
    samples: dict[str, list[float]] = defaultdict(list)
    shards = []
    for path in files:
        package_seconds = None
        total = 0.0
        count = 0
        for line in path.read_text(encoding="utf-8").splitlines():
            fields = line.split()
            if len(fields) == 3 and fields[:2] == ["#", "package-seconds"]:
                package_seconds = float(fields[2])
                continue
            if len(fields) != 2 or fields[0].startswith("#"):
                continue
            seconds = float(fields[1])
            samples[fields[0]].append(seconds)
            total += seconds
            count += 1
        shards.append((path, package_seconds, total, count))
    return samples, shards


def render(weights: dict[str, float], default: float, sources: list[str], min_seconds: float) -> str:
    today = datetime.date.today().isoformat()
    header = f"""\
# Run time in seconds of the slowest internal/api top-level tests under -race
# on a GitHub ubuntu-24.04 runner, the median of the Build and Test runs named
# below. Used only by select-internal-api-shard.sh to choose where the
# Backend tests (api-N) shards cut go test's list. Tests not named here weigh
# DEFAULT_WEIGHT, the mean of the tests under {min_seconds:g}s in the same runs. A
# missing, stale or renamed entry can make the shards uneven but can never
# drop or repeat a test.
#
# Generated {today} by .github/scripts/refresh-internal-api-test-seconds.py
# from {', '.join(sources)}. Every api shard uploads an
# internal-api-test-seconds-N artifact, so rerun that script with recent
# passing runs when one api shard runs well over the others.
DEFAULT_WEIGHT {default:.2f}
"""
    body = "".join(f"{name} {seconds:.2f}\n" for name, seconds in weights.items())
    return header + body


def report_cut(weights_path: Path, tests_path: Path, shards: int, samples: dict[str, float], default: float) -> None:
    tests = [line.strip() for line in tests_path.read_text(encoding="utf-8").splitlines()]
    tests = [t for t in tests if t.startswith(("Test", "Fuzz", "Example"))]
    stdin = "\n".join(tests) + "\n"
    print(f"Predicted cut with {shards} shards (measured seconds, default {default:.2f}s when unmeasured):")
    for index in range(shards):
        selected = subprocess.run(
            ["bash", str(SELECTOR), str(weights_path), str(shards), str(index)],
            input=stdin, capture_output=True, text=True, check=True,
        ).stdout.split()
        seconds = sum(samples.get(name, default) for name in selected)
        print(f"  api-{index}: {len(selected):5d} tests  {seconds:7.1f}s  {selected[0]} .. {selected[-1]}")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--run", action="append", default=[], help="Build and Test run id to download artifacts from")
    parser.add_argument("--dir", action="append", default=[], type=Path, help="directory of downloaded artifacts")
    parser.add_argument("--repo", default="rcourtman/Pulse")
    parser.add_argument("--min-seconds", type=float, default=0.5)
    parser.add_argument("--output", type=Path, default=WEIGHTS)
    parser.add_argument("--tests", type=Path, help="go test -list output, to print the resulting shard cut")
    parser.add_argument("--shards", type=int, help="shard count for --tests (default: API_SHARD_COUNT from the workflow)")
    args = parser.parse_args()
    if not args.run and not args.dir:
        parser.error("pass at least one --run or --dir")

    with tempfile.TemporaryDirectory() as scratch:
        dirs = list(args.dir)
        for run_id in args.run:
            target = Path(scratch) / run_id
            download(run_id, target, args.repo)
            dirs.append(target)
        files = sorted(path for d in dirs for path in d.rglob("*.txt"))
        if not files:
            print("no timing files found", file=sys.stderr)
            return 1
        by_test, shards = read_timings(files)

    for path, package_seconds, total, count in shards:
        wall = f"{package_seconds:7.1f}s" if package_seconds is not None else "      ?"
        print(f"{path.parent.name}/{path.name}: {count} tests, {total:.1f}s in tests, package {wall}")

    median = {name: statistics.median(values) for name, values in by_test.items()}
    listed = {name: seconds for name, seconds in median.items() if seconds >= args.min_seconds}
    rest = [seconds for name, seconds in median.items() if name not in listed]
    default = max(statistics.fmean(rest) if rest else 0.05, 0.01)

    sources = [f"run {run_id}" for run_id in args.run] + [str(d) for d in args.dir]
    args.output.write_text(render(listed, default, sources, args.min_seconds), encoding="utf-8")
    print(f"Wrote {len(listed)} tests at {args.min_seconds:g}s or more and DEFAULT_WEIGHT {default:.2f} to {args.output}")

    if args.tests:
        shards_count = args.shards
        if shards_count is None:
            workflow = (REPO / ".github" / "workflows" / "build-and-test.yml").read_text(encoding="utf-8")
            shards_count = int(workflow.split("API_SHARD_COUNT: ", 1)[1].split()[0])
        report_cut(args.output, args.tests, shards_count, median, default)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
