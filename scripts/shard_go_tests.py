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

    return {
        "schema_version": 1,
        "test_count": len(canonical),
        "test_names_sha256": _digest(canonical),
        "shard_count": shard_count,
        "batch_size": batch_size,
        "max_regex_bytes": max_regex_bytes,
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
    parser.add_argument("--output-dir", type=Path, required=True)
    args = parser.parse_args()

    test_names = [
        line.strip()
        for line in args.tests_file.read_text().splitlines()
        if line.strip()
    ]
    plan = build_plan(
        test_names, args.shards, args.batch_size, args.max_regex_bytes
    )
    manifest_path = write_plan(plan, args.output_dir)
    print(manifest_path.read_text(), end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
