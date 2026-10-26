#!/usr/bin/env python3
"""Balance whole smoke suites; discovery, never timing hints, defines coverage."""
from __future__ import annotations

import argparse
from decimal import Decimal, InvalidOperation
from pathlib import Path
import sys


def suite_name(name: str) -> bool:
    return Path(name).name == name and (
        name.startswith("test_") and name.endswith(".py")
        or name.startswith("test-") and name.endswith(".sh")
    )


def read_weights(path: Path) -> tuple[Decimal, dict[str, Decimal]]:
    default = None
    weights: dict[str, Decimal] = {}
    for number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw.split("#", 1)[0].strip()
        if not line:
            continue
        fields = line.split()
        if len(fields) != 2:
            raise ValueError(f"invalid timing hint at line {number}")
        name, value = fields
        try:
            seconds = Decimal(value)
        except InvalidOperation as error:
            raise ValueError(f"invalid timing value at line {number}") from error
        if not seconds.is_finite() or seconds <= 0:
            raise ValueError(f"timing hint must be finite and positive at line {number}")
        if name == "DEFAULT_WEIGHT":
            if default is not None:
                raise ValueError("duplicate DEFAULT_WEIGHT")
            default = seconds
        else:
            if not suite_name(name) or name in weights:
                raise ValueError(f"invalid or duplicate suite timing at line {number}")
            weights[name] = seconds
    if default is None:
        raise ValueError("missing DEFAULT_WEIGHT")
    return default, weights


def select(paths: list[str], default: Decimal, weights: dict[str, Decimal],
           count: int, index: int) -> list[str]:
    if not 1 <= count <= 64 or not 0 <= index < count:
        raise ValueError("invalid smoke shard count or index")
    if len(paths) != len(set(paths)) or any(
        not path or "\0" in path or not suite_name(Path(path).name) for path in paths
    ):
        raise ValueError("invalid or duplicate discovered smoke suite")
    loads = [Decimal(0) for _ in range(count)]
    shards: list[list[str]] = [[] for _ in range(count)]
    # Stable ties keep unmeasured suites balanced too. We never iterate hints
    # to construct the inventory, so new, renamed and removed suites are safe.
    for path in sorted(paths, key=lambda item: (-weights.get(Path(item).name, default), item)):
        shard = min(range(count), key=lambda candidate: (loads[candidate], candidate))
        shards[shard].append(path)
        loads[shard] += weights.get(Path(path).name, default)
    # Retain the runner's sorted execution within each isolated shard. No
    # suite's command, deadline, process cleanup or failure handling changes.
    return sorted(shards[index])


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("weights", type=Path)
    parser.add_argument("count", type=int)
    parser.add_argument("index", type=int)
    args = parser.parse_args()
    try:
        default, weights = read_weights(args.weights)
        selected = select(sys.stdin.read().splitlines(), default, weights, args.count, args.index)
    except (OSError, UnicodeError, ValueError) as error:
        print(f"Cannot partition complete smoke inventory: {error}", file=sys.stderr)
        return 1
    if selected:
        print("\n".join(selected))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
