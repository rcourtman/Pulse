#!/usr/bin/env python3
"""Balance whole Go packages without changing which tests run.

Usage: go list ... | select-go-package-shard.py WEIGHTS COUNT INDEX

The checked-out source supplies the package universe, never the weights file.
Assign longest measured packages first to the lightest shard, breaking ties by
package name and shard index. Print selected packages in their original order.
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


def read_weights(path: Path) -> tuple[dict[str, Decimal], Decimal]:
    weights: dict[str, Decimal] = {}
    default = Decimal("1")
    seen: set[str] = set()
    for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        fields = line.split()
        if len(fields) != 2:
            raise ValueError(f"invalid weights line {number}: expected name and seconds")
        name, raw = fields
        if name != "DEFAULT_WEIGHT" and not PACKAGE.fullmatch(name):
            raise ValueError(f"invalid package name on weights line {number}")
        if name in seen:
            raise ValueError(f"duplicate weight on line {number}: {name}")
        seen.add(name)
        try:
            seconds = Decimal(raw)
        except InvalidOperation:
            raise ValueError(f"invalid seconds on weights line {number}") from None
        if not seconds.is_finite() or not 0 < seconds <= 86400:
            raise ValueError(f"seconds must be finite and within (0, 86400] on weights line {number}")
        if name == "DEFAULT_WEIGHT":
            default = seconds
        else:
            weights[name] = seconds
    return weights, default


def assign(packages: list[str], weights: dict[str, Decimal], default: Decimal, count: int) -> dict[str, int]:
    if count < 1 or len(packages) < count:
        raise ValueError(f"{len(packages)} packages cannot fill {count} shards")
    if len(set(packages)) != len(packages):
        raise ValueError("duplicate package in source list")
    if any(not PACKAGE.fullmatch(name) for name in packages):
        raise ValueError("invalid package in source list")
    loads = [Decimal(0)] * count
    assignment: dict[str, int] = {}
    for name in sorted(packages, key=lambda name: (-weights.get(name, default), name)):
        shard = min(range(count), key=lambda index: (loads[index], index))
        assignment[name] = shard
        loads[shard] += weights.get(name, default)
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
        weights, default = read_weights(args.weights)
        packages = [line for line in sys.stdin.read().splitlines() if line]
        assignment = assign(packages, weights, default, args.count)
    except (OSError, ValueError, InvalidOperation) as error:
        print(f"package shard selection failed: {error}", file=sys.stderr)
        return 2
    for name in packages:
        if assignment[name] == args.index:
            print(name)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
