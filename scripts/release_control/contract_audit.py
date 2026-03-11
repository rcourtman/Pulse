#!/usr/bin/env python3
"""Machine audit for v6 subsystem contracts."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import re
import sys
from typing import Any

from canonical_completion_guard import REPO_ROOT


CONTRACTS_DIR = REPO_ROOT / "docs" / "release-control" / "v6" / "subsystems"
REGISTRY_PATH = CONTRACTS_DIR / "registry.json"
STATUS_PATH = REPO_ROOT / "docs" / "release-control" / "v6" / "status.json"
REGISTRY_REL = "docs/release-control/v6/subsystems/registry.json"
STATUS_REL = "docs/release-control/v6/status.json"
TEMPLATE_REL = "docs/release-control/v6/SUBSYSTEM_CONTRACT_TEMPLATE.md"
REQUIRED_SECTIONS = [
    "## Contract Metadata",
    "## Purpose",
    "## Canonical Files",
    "## Extension Points",
    "## Forbidden Paths",
    "## Completion Obligations",
    "## Current State",
]
LIST_SECTIONS = {
    "## Canonical Files",
    "## Extension Points",
    "## Forbidden Paths",
    "## Completion Obligations",
}
METADATA_REQUIRED_FIELDS = {
    "subsystem_id",
    "lane",
    "contract_file",
    "status_file",
    "registry_file",
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


def load_registry_payload() -> dict[str, Any]:
    return json.loads(REGISTRY_PATH.read_text(encoding="utf-8"))


def load_status_payload() -> dict[str, Any]:
    return json.loads(STATUS_PATH.read_text(encoding="utf-8"))


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


def validate_repo_path_token(token: str, *, rel: str, heading: str, errors: list[str]) -> None:
    raw = token.rstrip("/") if token.endswith("/") else token
    if not raw:
        errors.append(f"{rel} {heading} contains non-clean repo-relative path {token!r}")
        return
    candidate = Path(raw)
    normalized = candidate.as_posix()
    if candidate.is_absolute() or raw.startswith("../") or "/../" in raw or normalized != raw:
        errors.append(f"{rel} {heading} contains non-clean repo-relative path {token!r}")
        return
    resolved = REPO_ROOT / raw
    if not resolved.exists():
        errors.append(f"{rel} {heading} references missing path {token!r}")
        return
    if token.endswith("/") and not resolved.is_dir():
        errors.append(f"{rel} {heading} expects directory path {token!r}")


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


def audit_contract_text(rel: str, content: str) -> tuple[dict[str, Any], list[str]]:
    errors: list[str] = []
    lines = content.splitlines()
    if not lines or not lines[0].startswith("# "):
        errors.append(f"{rel} must start with a level-1 heading")

    heading_positions: dict[str, int] = {}
    for index, line in enumerate(lines):
        if line in REQUIRED_SECTIONS:
            if line in heading_positions:
                errors.append(f"{rel} duplicates required section {line!r}")
            heading_positions[line] = index

    positions: list[int] = []
    for heading in REQUIRED_SECTIONS:
        if heading not in heading_positions:
            errors.append(f"{rel} missing required section {heading!r}")
            continue
        positions.append(heading_positions[heading])
    if positions and positions != sorted(positions):
        errors.append(f"{rel} required sections must appear in canonical order")

    metadata: dict[str, Any] | None = None
    if "## Contract Metadata" in heading_positions:
        metadata, metadata_errors = parse_contract_metadata(section_body(lines, "## Contract Metadata"))
        errors.extend(f"{rel} {error}" for error in metadata_errors)

    for heading in REQUIRED_SECTIONS:
        if heading not in heading_positions or heading == "## Contract Metadata":
            continue
        body = section_body(lines, heading)
        if not any(line.strip() for line in body):
            errors.append(f"{rel} section {heading!r} must not be empty")
            continue
        if heading in LIST_SECTIONS:
            items = section_list_items(body)
            if not items:
                errors.append(f"{rel} section {heading!r} must contain a numbered list")
                continue
            if heading == "## Canonical Files":
                for _, item in items:
                    path_tokens = [token for token in re.findall(r"`([^`]+)`", item) if looks_like_repo_path(token)]
                    if not path_tokens:
                        errors.append(f"{rel} section {heading!r} entries must include at least one repo path")
                        continue
                    for token in path_tokens:
                        validate_repo_path_token(token, rel=rel, heading=heading, errors=errors)
            if heading == "## Extension Points":
                for _, item in items:
                    for token in re.findall(r"`([^`]+)`", item):
                        if looks_like_repo_path(token):
                            validate_repo_path_token(token, rel=rel, heading=heading, errors=errors)

    return {
        "title": lines[0].strip() if lines else "",
        "metadata": metadata,
    }, errors


def audit_contract_payload(
    *,
    registry_payload: dict[str, Any],
    status_payload: dict[str, Any],
    contract_texts: dict[str, str],
) -> dict[str, Any]:
    errors: list[str] = []
    warnings: list[str] = []

    registry_subsystems = registry_payload.get("subsystems")
    if not isinstance(registry_subsystems, list):
        return {
            "errors": ["registry.json missing subsystems list"],
            "warnings": warnings,
            "summary": {},
            "contracts": [],
        }

    status_lanes = {
        lane.get("id")
        for lane in status_payload.get("lanes", [])
        if isinstance(lane, dict) and isinstance(lane.get("id"), str)
    }
    expected_contracts = {
        str(subsystem.get("contract")): subsystem
        for subsystem in registry_subsystems
        if isinstance(subsystem, dict) and isinstance(subsystem.get("contract"), str)
    }

    actual_contracts = {rel for rel in contract_texts if rel.endswith(".md") and rel != TEMPLATE_REL}
    missing_contracts = sorted(set(expected_contracts) - actual_contracts)
    extra_contracts = sorted(actual_contracts - set(expected_contracts))
    for rel in missing_contracts:
        errors.append(f"missing registered subsystem contract {rel}")
    for rel in extra_contracts:
        errors.append(f"unregistered subsystem contract present: {rel}")

    contract_summaries: list[dict[str, Any]] = []
    seen_subsystem_ids: set[str] = set()

    for rel in sorted(actual_contracts):
        parsed, parse_errors = audit_contract_text(rel, contract_texts[rel])
        errors.extend(parse_errors)
        metadata = parsed.get("metadata")
        subsystem = expected_contracts.get(rel)
        if metadata is None:
            contract_summaries.append({"contract": rel})
            continue

        metadata_keys = set(metadata)
        if metadata_keys != METADATA_REQUIRED_FIELDS:
            errors.append(
                f"{rel} contract metadata keys = {sorted(metadata_keys)!r}, want {sorted(METADATA_REQUIRED_FIELDS)!r}"
            )

        for field in sorted(METADATA_REQUIRED_FIELDS):
            value = metadata.get(field)
            if not isinstance(value, str) or not value.strip():
                errors.append(f"{rel} contract metadata field {field!r} must be a non-empty string")

        subsystem_id = str(metadata.get("subsystem_id", "")).strip()
        lane = str(metadata.get("lane", "")).strip()
        contract_file = str(metadata.get("contract_file", "")).strip()
        status_file = str(metadata.get("status_file", "")).strip()
        registry_file = str(metadata.get("registry_file", "")).strip()

        if subsystem_id:
            if subsystem_id in seen_subsystem_ids:
                errors.append(f"{rel} duplicates contract metadata subsystem_id {subsystem_id!r}")
            seen_subsystem_ids.add(subsystem_id)
        if contract_file and contract_file != rel:
            errors.append(f"{rel} contract metadata contract_file = {contract_file!r}, want {rel!r}")
        if status_file and status_file != STATUS_REL:
            errors.append(f"{rel} contract metadata status_file must be {STATUS_REL!r}")
        if registry_file and registry_file != REGISTRY_REL:
            errors.append(f"{rel} contract metadata registry_file must be {REGISTRY_REL!r}")
        if lane and lane not in status_lanes:
            errors.append(f"{rel} contract metadata references unknown lane {lane!r}")

        if subsystem is None:
            errors.append(f"{rel} is not registered in registry.json")
        else:
            expected_id = str(subsystem.get("id", "")).strip()
            expected_lane = str(subsystem.get("lane", "")).strip()
            if subsystem_id and subsystem_id != expected_id:
                errors.append(f"{rel} contract metadata subsystem_id = {subsystem_id!r}, want {expected_id!r}")
            if lane and lane != expected_lane:
                errors.append(f"{rel} contract metadata lane = {lane!r}, want {expected_lane!r}")

        contract_summaries.append(
            {
                "contract": rel,
                "subsystem_id": subsystem_id,
                "lane": lane,
                "title": parsed.get("title", ""),
            }
        )

    expected_subsystem_ids = {
        str(subsystem.get("id", "")).strip()
        for subsystem in registry_subsystems
        if isinstance(subsystem, dict) and isinstance(subsystem.get("id"), str)
    }
    if seen_subsystem_ids != expected_subsystem_ids:
        missing_ids = sorted(expected_subsystem_ids - seen_subsystem_ids)
        extra_ids = sorted(seen_subsystem_ids - expected_subsystem_ids)
        for subsystem_id in missing_ids:
            errors.append(f"missing contract metadata for registered subsystem_id {subsystem_id!r}")
        for subsystem_id in extra_ids:
            errors.append(f"unregistered subsystem_id present in contract metadata: {subsystem_id!r}")

    return {
        "errors": errors,
        "warnings": warnings,
        "summary": {
            "contract_count": len(contract_summaries),
        },
        "contracts": contract_summaries,
    }


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Audit docs/release-control/v6/subsystems/*.md contracts.")
    parser.add_argument(
        "--check",
        action="store_true",
        help="Exit non-zero if the contract audit finds any errors.",
    )
    parser.add_argument(
        "--pretty",
        action="store_true",
        help="Print a concise human-readable summary instead of JSON.",
    )
    return parser.parse_args(argv)


def render_pretty(report: dict[str, Any]) -> str:
    lines: list[str] = []
    summary = report.get("summary", {})
    if summary:
        lines.append(f"summary: contracts={summary.get('contract_count', 0)}")
    for contract in report.get("contracts", []):
        lines.append(
            f"{contract.get('subsystem_id', '?')}: lane={contract.get('lane', '?')} contract={contract.get('contract', '?')}"
        )
    if report.get("warnings"):
        lines.append("warnings:")
        for warning in report["warnings"]:
            lines.append(f"  - {warning}")
    if report.get("errors"):
        lines.append("errors:")
        for err in report["errors"]:
            lines.append(f"  - {err}")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(list(argv or []))
    report = audit_contract_payload(
        registry_payload=load_registry_payload(),
        status_payload=load_status_payload(),
        contract_texts=tracked_contract_files(),
    )
    output = render_pretty(report) if args.pretty else json.dumps(report, indent=2, sort_keys=True)
    print(output)
    if args.check and report["errors"]:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
