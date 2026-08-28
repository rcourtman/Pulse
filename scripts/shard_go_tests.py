#!/usr/bin/env python3
"""Create deterministic, coverage-complete batches for a Go test package."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
from pathlib import Path
from typing import Sequence


TEST_NAME_PATTERN = re.compile(r"^Test[A-Za-z0-9_]+$")
MAX_SAFE_REGEX_BYTES = 120_000
DEFAULT_MAX_REGEX_BYTES = 64 * 1024


class _RegexTrie(dict[str, "_RegexTrie"]):
    terminal: bool

    def __init__(self) -> None:
        super().__init__()
        self.terminal = False


def _digest(names: Sequence[str]) -> str:
    payload = "".join(f"{name}\n" for name in names).encode()
    return hashlib.sha256(payload).hexdigest()


def allocate_cpu_plan(
    test_counts: Sequence[int], vcpus: int
) -> tuple[list[int], int]:
    """Allocate a non-oversubscribed CPU plan for API and non-API tests.

    API shards each receive one CPU before remaining API capacity is assigned
    proportionally by planned test count. Two CPUs are reserved for the
    concurrent non-API package graph when the worker is wide enough, otherwise
    one is reserved. A worker with no spare CPU must run the non-API graph
    before its API shards.
    """
    counts = list(test_counts)
    if not counts:
        raise ValueError("test_counts must contain at least one API shard")
    if vcpus < 1:
        raise ValueError("vcpus must be at least 1")
    if any(
        isinstance(count, bool) or not isinstance(count, int) or count < 1
        for count in counts
    ):
        raise ValueError("test_counts must contain positive integers")
    if len(counts) > vcpus:
        raise ValueError("API shard count cannot exceed available vCPUs")

    reserved_other = min(2, vcpus - len(counts))
    extra_budget = vcpus - reserved_other - len(counts)
    total = sum(counts)
    scaled = [extra_budget * count for count in counts]
    extras = [value // total for value in scaled]
    remaining = extra_budget - sum(extras)
    by_remainder = sorted(
        range(len(counts)),
        key=lambda index: (scaled[index] % total, counts[index], -index),
        reverse=True,
    )
    for index in by_remainder[:remaining]:
        extras[index] += 1
    return [1 + extra for extra in extras], reserved_other


def _test_regex(names: Sequence[str]) -> str:
    root = _RegexTrie()
    for name in names:
        node = root
        for character in name:
            node = node.setdefault(character, _RegexTrie())
        node.terminal = True

    def render(node: _RegexTrie) -> str:
        suffix_groups: dict[str, list[str]] = {}
        for character, child in sorted(node.items()):
            suffix_groups.setdefault(render(child), []).append(character)

        alternatives: list[str] = []
        for suffix, characters in sorted(suffix_groups.items()):
            if len(characters) == 1:
                prefix = re.escape(characters[0])
            else:
                prefix = (
                    "["
                    + "".join(re.escape(value) for value in characters)
                    + "]"
                )
            alternatives.append(prefix + suffix)

        if node.terminal:
            if not alternatives:
                return ""
            body = (
                alternatives[0]
                if len(alternatives) == 1
                else "(?:" + "|".join(alternatives) + ")"
            )
            return "(?:" + body + ")?"
        if len(alternatives) == 1:
            return alternatives[0]
        return "(?:" + "|".join(alternatives) + ")"

    return "^(?:" + render(root) + ")$"


def _batch_names(
    names: Sequence[str], batch_size: int, max_regex_bytes: int
) -> list[list[str]]:
    batches: list[list[str]] = []
    offset = 0
    while offset < len(names):
        if len(_test_regex([names[offset]]).encode()) > max_regex_bytes:
            raise ValueError(
                f"top-level Go test name exceeds max regex bytes: {names[offset]}"
            )

        low = offset + 1
        high = min(len(names), offset + batch_size)
        best = low
        while low <= high:
            midpoint = (low + high) // 2
            encoded_bytes = len(_test_regex(names[offset:midpoint]).encode())
            if encoded_bytes <= max_regex_bytes:
                best = midpoint
                low = midpoint + 1
            else:
                high = midpoint - 1
        batches.append(list(names[offset:best]))
        offset = best
    return batches


def build_plan(
    test_names: Sequence[str],
    shard_count: int,
    batch_size: int,
    max_regex_bytes: int = DEFAULT_MAX_REGEX_BYTES,
    shard_weights: Sequence[int] | None = None,
    shard_boundaries: Sequence[str] | None = None,
) -> dict[str, object]:
    if shard_count < 1:
        raise ValueError("shard_count must be at least 1")
    if batch_size < 1:
        raise ValueError("batch_size must be at least 1")
    if max_regex_bytes < 1:
        raise ValueError("max_regex_bytes must be at least 1")
    if max_regex_bytes > MAX_SAFE_REGEX_BYTES:
        raise ValueError(
            f"max_regex_bytes cannot exceed the safe per-argument ceiling "
            f"of {MAX_SAFE_REGEX_BYTES}"
        )
    if not test_names:
        raise ValueError("the Go package did not expose any top-level tests")
    if shard_count > len(test_names):
        raise ValueError("shard_count cannot exceed the number of top-level tests")

    weights = list(shard_weights) if shard_weights is not None else None
    boundaries_by_name = (
        list(shard_boundaries) if shard_boundaries is not None else None
    )
    if weights is not None and boundaries_by_name is not None:
        raise ValueError("shard weights and shard boundaries are mutually exclusive")
    if weights is not None:
        if len(weights) != shard_count:
            raise ValueError("shard weights must contain one value per shard")
        if any(
            isinstance(weight, bool) or not isinstance(weight, int) or weight < 1
            for weight in weights
        ):
            raise ValueError("shard weights must be positive integers")
    if boundaries_by_name is not None and len(boundaries_by_name) != shard_count - 1:
        raise ValueError("shard boundaries must contain one fewer value than shards")

    invalid = sorted({name for name in test_names if not TEST_NAME_PATTERN.fullmatch(name)})
    if invalid:
        raise ValueError(f"invalid top-level Go test name(s): {', '.join(invalid)}")
    if len(set(test_names)) != len(test_names):
        duplicates = sorted(
            name for name in set(test_names) if test_names.count(name) > 1
        )
        raise ValueError(f"duplicate top-level Go test name(s): {', '.join(duplicates)}")

    canonical = list(test_names)
    # Preserve the exact order emitted by the compiled test binary. Some
    # legacy API tests still exercise package-global state, so sorting, hash
    # distribution, or extra process boundaries can change their historical
    # neighbours.
    shards: list[list[str]] = []
    if boundaries_by_name is not None:
        unknown = [name for name in boundaries_by_name if name not in canonical]
        if unknown:
            raise ValueError(f"unknown shard boundary test(s): {', '.join(unknown)}")
        boundaries = [canonical.index(name) + 1 for name in boundaries_by_name]
        if boundaries != sorted(set(boundaries)):
            raise ValueError("shard boundaries must follow compiled test order")
        points = [0, *boundaries, len(canonical)]
        sizes = [end - start for start, end in zip(points, points[1:])]
    elif weights is None:
        base_size, remainder = divmod(len(canonical), shard_count)
        sizes = [
            base_size + (1 if shard_index < remainder else 0)
            for shard_index in range(shard_count)
        ]
    else:
        total_weight = sum(weights)
        boundaries: list[int] = []
        cumulative_weight = 0
        for shard_index, weight in enumerate(weights[:-1], start=1):
            cumulative_weight += weight
            boundary = (
                len(canonical) * cumulative_weight + total_weight // 2
            ) // total_weight
            minimum = shard_index
            maximum = len(canonical) - (shard_count - shard_index)
            boundaries.append(max(minimum, min(boundary, maximum)))
        points = [0, *boundaries, len(canonical)]
        sizes = [end - start for start, end in zip(points, points[1:])]

    offset = 0
    for size in sizes:
        shards.append(canonical[offset : offset + size])
        offset += size

    shard_records: list[dict[str, object]] = []
    assigned: list[str] = []
    for shard_index, names in enumerate(shards, start=1):
        ordered_names = list(names)
        assigned.extend(ordered_names)
        batches = _batch_names(ordered_names, batch_size, max_regex_bytes)
        for batch in batches:
            pattern = re.compile(_test_regex(batch))
            matched_names = [
                name for name in canonical if pattern.fullmatch(name) is not None
            ]
            if matched_names != batch:
                raise RuntimeError(
                    "generated test regex is not an exact ordered partition"
                )
        shard_records.append(
            {
                "index": shard_index,
                "test_count": len(ordered_names),
                "test_names_sha256": _digest(ordered_names),
                "batches": batches,
            }
        )

    if assigned != canonical or len(assigned) != len(set(assigned)):
        raise RuntimeError("generated shard plan is not a complete, disjoint partition")

    plan: dict[str, object] = {
        "schema_version": 1,
        "test_count": len(canonical),
        "test_names_sha256": _digest(canonical),
        "shard_count": shard_count,
        "batch_size": batch_size,
        "max_regex_bytes": max_regex_bytes,
        "shards": shard_records,
    }
    if weights is not None:
        plan["shard_weights"] = weights
    if boundaries_by_name is not None:
        plan["shard_boundaries"] = boundaries_by_name
    return plan


def write_plan(plan: dict[str, object], output_dir: Path) -> Path:
    output_dir.mkdir(parents=True, exist_ok=True)
    manifest_shards: list[dict[str, object]] = []
    for shard in plan["shards"]:  # type: ignore[index]
        shard_record = dict(shard)
        batches = shard_record.pop("batches")
        batch_records: list[dict[str, object]] = []
        for batch_index, names in enumerate(batches, start=1):
            filename = f"shard-{shard_record['index']:02d}-batch-{batch_index:02d}.regex"
            regex = _test_regex(names)
            (output_dir / filename).write_text(f"{regex}\n")
            batch_records.append(
                {
                    "index": batch_index,
                    "test_count": len(names),
                    "regex_file": filename,
                    "test_names_sha256": _digest(names),
                }
            )
        shard_record["batches"] = batch_records
        manifest_shards.append(shard_record)

    manifest = dict(plan)
    manifest["shards"] = manifest_shards
    manifest_path = output_dir / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")
    return manifest_path


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--tests-file", type=Path, required=True)
    parser.add_argument("--shards", type=int, required=True)
    parser.add_argument("--batch-size", type=int, default=10000)
    parser.add_argument(
        "--max-regex-bytes", type=int, default=DEFAULT_MAX_REGEX_BYTES
    )
    parser.add_argument(
        "--shard-weights",
        help="comma-separated positive integer weights for contiguous shard sizes",
    )
    parser.add_argument(
        "--shard-boundaries",
        help="comma-separated test names that end each contiguous shard except the last",
    )
    parser.add_argument("--output-dir", type=Path, required=True)
    args = parser.parse_args()

    test_names = [
        line.strip()
        for line in args.tests_file.read_text().splitlines()
        if line.strip()
    ]
    shard_weights = None
    if args.shard_weights:
        try:
            shard_weights = [
                int(value.strip()) for value in args.shard_weights.split(",")
            ]
        except ValueError as error:
            parser.error(f"invalid --shard-weights: {error}")
    shard_boundaries = None
    if args.shard_boundaries:
        shard_boundaries = [
            value.strip() for value in args.shard_boundaries.split(",")
        ]
    plan = build_plan(
        test_names,
        args.shards,
        args.batch_size,
        args.max_regex_bytes,
        shard_weights,
        shard_boundaries,
    )
    manifest_path = write_plan(plan, args.output_dir)
    print(manifest_path.read_text(), end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
