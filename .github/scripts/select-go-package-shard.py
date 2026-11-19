#!/usr/bin/env python3
"""Balance whole Go packages without changing which tests run.

Usage: go list ... | select-go-package-shard.py WEIGHTS COUNT INDEX

The checked-out source supplies the package universe, never the weights file.
Assign longest measured packages first to the lightest shard, breaking ties by
package name and shard index. The two CI rest shards can account for work
already assigned outside this package list. Start selected packages longest
first, so slow binaries do not wait behind the small-package tail.
Unlike internal/api's test slices, separate package binaries have no shared
test order to preserve. Missing/renamed weights affect balance, not coverage.
"""

from __future__ import annotations

import argparse
from decimal import Decimal, InvalidOperation
from pathlib import Path
import re
import sys


PACKAGE = re.compile(r"[A-Za-z0-9][A-Za-z0-9._~+/-]*\Z")
REST_FIXED_SECONDS = "REST_SHARD_FIXED_SECONDS"


def read_weights(
    path: Path,
) -> tuple[dict[str, Decimal], Decimal, tuple[Decimal, Decimal]]:
    weights: dict[str, Decimal] = {}
    default = Decimal("1")
    fixed = (Decimal(0), Decimal(0))
    seen: set[str] = set()
    for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        fields = line.split()
        name, *values = fields
        if name in seen:
            raise ValueError(f"duplicate weight on line {number}: {name}")
        seen.add(name)
        if name == REST_FIXED_SECONDS:
            if len(values) != 2:
                raise ValueError(f"invalid weights line {number}: expected two rest shard estimates")
            try:
                left, right = map(Decimal, values)
            except InvalidOperation:
                raise ValueError(f"invalid rest shard estimates on line {number}") from None
            if any(not v.is_finite() or not 0 <= v <= 86400 for v in (left, right)):
                raise ValueError(f"rest shard estimates must be finite and within [0, 86400] on line {number}")
            fixed = left, right
            continue
        if len(values) != 1:
            raise ValueError(f"invalid weights line {number}: expected name and seconds")
        if name != "DEFAULT_WEIGHT" and not PACKAGE.fullmatch(name):
            raise ValueError(f"invalid package name on weights line {number}")
        try:
            seconds = Decimal(values[0])
        except InvalidOperation:
            raise ValueError(f"invalid seconds on weights line {number}") from None
        if not seconds.is_finite() or not 0 < seconds <= 86400:
            raise ValueError(f"seconds must be finite and within (0, 86400] on weights line {number}")
        if name == "DEFAULT_WEIGHT":
            default = seconds
        else:
            weights[name] = seconds
    return weights, default, fixed


def assign(
    packages: list[str], weights: dict[str, Decimal], default: Decimal, count: int,
    rest_fixed: tuple[Decimal, Decimal] = (Decimal(0), Decimal(0)),
) -> dict[str, int]:
    if count < 1 or len(packages) < count:
        raise ValueError(f"{len(packages)} packages cannot fill {count} shards")
    if len(set(packages)) != len(packages):
        raise ValueError("duplicate package in source list")
    if any(not PACKAGE.fullmatch(name) for name in packages):
        raise ValueError("invalid package in source list")
    # The one-shard release queue runs the whole packages itself. Do not apply
    # a two-shard CI scheduling estimate to that (or any other) shape.
    loads = list(rest_fixed) if count == 2 else [Decimal(0)] * count
    assignment: dict[str, int] = {}
    unfilled = set(range(count))
    ordered = sorted(packages, key=lambda name: (-weights.get(name, default), name))
    for position, name in enumerate(ordered):
        # Retain at least one ordinary package per shard even if an estimate is
        # large or the source roster is small. Estimates can never omit work.
        eligible = unfilled if len(ordered) - position == len(unfilled) else range(count)
        shard = min(eligible, key=lambda index: (loads[index], index))
        assignment[name] = shard
        loads[shard] += weights.get(name, default)
        unfilled.discard(shard)
    return assignment


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("weights", type=Path)
    parser.add_argument("count", type=int)
    parser.add_argument("index", type=int)
    args = parser.parse_args()
    try:
        if args.count < 1 or not 0 <= args.index < args.count:
            raise ValueError("shard index must be within the positive shard count")
        weights, default, rest_fixed = read_weights(args.weights)
        packages = [line for line in sys.stdin.read().splitlines() if line]
        assignment = assign(packages, weights, default, args.count, rest_fixed)
    except (OSError, ValueError, InvalidOperation) as error:
        print(f"package shard selection failed: {error}", file=sys.stderr)
        return 2
    for name in sorted(packages, key=lambda name: (-weights.get(name, default), name)):
        if assignment[name] == args.index:
            print(name)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
