#!/usr/bin/env python3
"""Shared parsing helpers for v6 subsystem contracts."""

from __future__ import annotations

import json
from pathlib import Path
import re
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[2]
CONTRACTS_DIR = REPO_ROOT / "docs" / "release-control" / "v6" / "subsystems"
TEMPLATE_REL = "docs/release-control/v6/SUBSYSTEM_CONTRACT_TEMPLATE.md"
REQUIRED_SECTIONS = [
    "## Contract Metadata",
    "## Purpose",
    "## Canonical Files",
    "## Shared Boundaries",
    "## Extension Points",
    "## Forbidden Paths",
    "## Completion Obligations",
    "## Current State",
]
LIST_SECTIONS = {
    "## Canonical Files",
    "## Shared Boundaries",
    "## Extension Points",
    "## Forbidden Paths",
    "## Completion Obligations",
}
PATH_SUFFIXES = (
    ".go",
    ".json",
    ".md",
    ".mjs",
    ".py",
    ".sh",
    ".ts",
    ".tsx",
    ".yaml",
    ".yml",
)


def tracked_contract_files() -> dict[str, str]:
    payload: dict[str, str] = {}
    for path in sorted(CONTRACTS_DIR.glob("*.md")):
        rel = path.relative_to(REPO_ROOT).as_posix()
        payload[rel] = path.read_text(encoding="utf-8")
    return payload


def section_body(lines: list[str], heading: str) -> list[str]:
    start = next(index for index, line in enumerate(lines) if line == heading) + 1
    end = len(lines)
    for index in range(start, len(lines)):
        if lines[index].startswith("## "):
            end = index
            break
    return lines[start:end]


def section_list_items(body_lines: list[str]) -> list[tuple[int, str]]:
    items: list[tuple[int, str]] = []
    for index, line in enumerate(body_lines):
        stripped = line.strip()
        if not stripped:
            continue
        for marker in [f"{n}." for n in range(1, 100)]:
            if stripped.startswith(marker + " "):
                items.append((index, stripped[len(marker) + 1 :]))
                break
    return items


def looks_like_repo_path(token: str) -> bool:
    candidate = token.strip()
    return "/" in candidate or candidate.endswith(PATH_SUFFIXES)


def parse_contract_metadata(body_lines: list[str]) -> tuple[dict[str, Any] | None, list[str]]:
    errors: list[str] = []
    meaningful = [line for line in body_lines if line.strip()]
    if len(meaningful) < 3:
        return None, ["contract metadata section must contain a JSON fenced block"]
    if meaningful[0].strip() != "```json":
        errors.append("contract metadata section must start with ```json")
        return None, errors
    if meaningful[-1].strip() != "```":
        errors.append("contract metadata section must end with ```")
        return None, errors
    json_block = "\n".join(meaningful[1:-1]).strip()
    if not json_block:
        errors.append("contract metadata JSON block must not be empty")
        return None, errors
    try:
        payload = json.loads(json_block)
    except json.JSONDecodeError as exc:
        errors.append(f"contract metadata JSON is invalid: {exc}")
        return None, errors
    if not isinstance(payload, dict):
        errors.append("contract metadata JSON must be an object")
        return None, errors
    return payload, errors


def parse_contract_text(rel: str, content: str) -> tuple[dict[str, Any], list[str]]:
    errors: list[str] = []
    path_references: list[dict[str, str]] = []
    lines = content.splitlines()
    if not lines or not lines[0].startswith("# "):
        errors.append(f"{rel} must start with a level-1 heading")

    heading_positions: dict[str, int] = {}
    for index, line in enumerate(lines):
        if line in REQUIRED_SECTIONS:
            if line in heading_positions:
                errors.append(f"{rel} duplicates required section {line!r}")
            heading_positions[line] = index

    metadata: dict[str, Any] | None = None
    if "## Contract Metadata" in heading_positions:
        metadata, metadata_errors = parse_contract_metadata(section_body(lines, "## Contract Metadata"))
        errors.extend(f"{rel} {error}" for error in metadata_errors)

    for heading in ("## Canonical Files", "## Shared Boundaries", "## Extension Points"):
        if heading not in heading_positions:
            continue
        body = section_body(lines, heading)
        items = section_list_items(body)
        for _, item in items:
            for token in re.findall(r"`([^`]+)`", item):
                if looks_like_repo_path(token):
                    path_references.append({"heading": heading, "path": token})

    return {
        "title": lines[0].strip() if lines else "",
        "metadata": metadata,
        "path_references": path_references,
        "heading_positions": heading_positions,
        "lines": lines,
    }, errors


def load_contract_graph(contract_texts: dict[str, str] | None = None) -> dict[str, dict[str, Any]]:
    graph: dict[str, dict[str, Any]] = {}
    for rel, content in (contract_texts or tracked_contract_files()).items():
        if rel == TEMPLATE_REL or not rel.endswith(".md"):
            continue
        parsed, _ = parse_contract_text(rel, content)
        metadata = parsed.get("metadata")
        if not isinstance(metadata, dict):
            continue
        subsystem_id = str(metadata.get("subsystem_id", "")).strip()
        if not subsystem_id:
            continue
        graph[subsystem_id] = {
            "subsystem_id": subsystem_id,
            "contract": rel,
            "metadata": metadata,
            "path_references": list(parsed.get("path_references", [])),
            "title": parsed.get("title", ""),
        }
    return graph


def contract_reference_matches_path(reference_path: str, path: str) -> bool:
    if reference_path.endswith("/"):
        return path.startswith(reference_path)
    return path == reference_path


def referenced_contracts_for_path(
    path: str,
    contract_graph: dict[str, dict[str, Any]] | None = None,
) -> list[dict[str, Any]]:
    graph = contract_graph or load_contract_graph()
    matches: list[dict[str, Any]] = []
    for contract in graph.values():
        referenced_paths = [
            reference
            for reference in contract.get("path_references", [])
            if contract_reference_matches_path(str(reference.get("path", "")), path)
        ]
        if referenced_paths:
            matches.append({**contract, "matched_references": referenced_paths})
    return sorted(matches, key=lambda contract: str(contract.get("subsystem_id", "")).casefold())
