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


def _digest(names: Sequence[str]) -> str:
    payload = "".join(f"{name}\n" for name in names).encode()
    return hashlib.sha256(payload).hexdigest()


def build_plan(
    test_names: Sequence[str], shard_count: int, batch_size: int
) -> dict[str, object]:
    if shard_count < 1:
        raise ValueError("shard_count must be at least 1")
    if batch_size < 1:
        raise ValueError("batch_size must be at least 1")
    if not test_names:
        raise ValueError("the Go package did not expose any top-level tests")
    if shard_count > len(test_names):
        raise ValueError("shard_count cannot exceed the number of top-level tests")

    invalid = sorted({name for name in test_names if not TEST_NAME_PATTERN.fullmatch(name)})
    if invalid:
        raise ValueError(f"invalid top-level Go test name(s): {', '.join(invalid)}")
    if len(set(test_names)) != len(test_names):
        duplicates = sorted(
            name for name in set(test_names) if test_names.count(name) > 1
        )
        raise ValueError(f"duplicate top-level Go test name(s): {', '.join(duplicates)}")

    canonical = sorted(test_names)
    # Preserve the suite's canonical ordering inside each shard. Some legacy
    # API tests still exercise package-global state, so hash distribution or
    # extra process boundaries can change their historical neighbours.
    base_size, remainder = divmod(len(canonical), shard_count)
    shards: list[list[str]] = []
    offset = 0
    for shard_index in range(shard_count):
        size = base_size + (1 if shard_index < remainder else 0)
        shards.append(canonical[offset : offset + size])
        offset += size

    shard_records: list[dict[str, object]] = []
    assigned: list[str] = []
    for shard_index, names in enumerate(shards, start=1):
        ordered_names = sorted(names)
        assigned.extend(ordered_names)
        batches = [
            ordered_names[offset : offset + batch_size]
            for offset in range(0, len(ordered_names), batch_size)
        ]
        shard_records.append(
            {
                "index": shard_index,
                "test_count": len(ordered_names),
                "test_names_sha256": _digest(ordered_names),
                "batches": batches,
            }
        )

    if sorted(assigned) != canonical or len(assigned) != len(set(assigned)):
        raise RuntimeError("generated shard plan is not a complete, disjoint partition")

    return {
        "schema_version": 1,
        "test_count": len(canonical),
        "test_names_sha256": _digest(canonical),
        "shard_count": shard_count,
        "batch_size": batch_size,
        "shards": shard_records,
    }


def write_plan(plan: dict[str, object], output_dir: Path) -> Path:
    output_dir.mkdir(parents=True, exist_ok=True)
    manifest_shards: list[dict[str, object]] = []
    for shard in plan["shards"]:  # type: ignore[index]
        shard_record = dict(shard)
        batches = shard_record.pop("batches")
        batch_records: list[dict[str, object]] = []
        for batch_index, names in enumerate(batches, start=1):
            filename = f"shard-{shard_record['index']:02d}-batch-{batch_index:02d}.regex"
            regex = "^(?:" + "|".join(re.escape(name) for name in names) + ")$"
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
    parser.add_argument("--output-dir", type=Path, required=True)
    args = parser.parse_args()

    test_names = [
        line.strip()
        for line in args.tests_file.read_text().splitlines()
        if line.strip()
    ]
    plan = build_plan(test_names, args.shards, args.batch_size)
    manifest_path = write_plan(plan, args.output_dir)
    print(manifest_path.read_text(), end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
