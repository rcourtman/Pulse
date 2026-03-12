#!/usr/bin/env python3
"""Run machine-declared readiness assertion proof commands."""

from __future__ import annotations

import argparse
from pathlib import Path
import shlex
import subprocess
import sys
from typing import Any

from repo_file_io import REPO_ROOT, load_repo_json


STATUS_REL = "docs/release-control/v6/status.json"


def load_status_payload(*, staged: bool = False) -> dict[str, Any]:
    return load_repo_json(STATUS_REL, staged=staged)


def _clean_relative_dir(path: str) -> str:
    candidate = Path(path)
    if candidate.is_absolute():
        raise ValueError(f"cwd must be relative, got {path!r}")
    normalized = candidate.as_posix()
    if normalized != path or path.startswith("../") or "/../" in path:
        raise ValueError(f"cwd must be a clean relative path, got {path!r}")
    resolved = REPO_ROOT / path
    if not resolved.exists():
        raise ValueError(f"cwd {path!r} does not exist")
    if not resolved.is_dir():
        raise ValueError(f"cwd {path!r} is not a directory")
    return path


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run proof commands declared in docs/release-control/v6/status.json."
    )
    parser.add_argument(
        "--blocking-level",
        action="append",
        choices=["repo-ready", "release-ready"],
        help="Run only assertions matching the given blocking level. Repeat to allow multiple values.",
    )
    parser.add_argument(
        "--proof-type",
        action="append",
        choices=["automated", "manual", "hybrid"],
        help="Run only assertions matching the given proof type. Repeat to allow multiple values.",
    )
    parser.add_argument(
        "--assertion",
        action="append",
        help="Run only the given readiness assertion id. Repeat to allow multiple values.",
    )
    parser.add_argument(
        "--staged",
        action="store_true",
        help="Read the readiness assertion catalog from the git index instead of the working tree.",
    )
    return parser.parse_args(argv)


def selected_proof_commands(
    payload: dict[str, Any],
    *,
    blocking_levels: set[str] | None = None,
    proof_types: set[str] | None = None,
    assertion_ids: set[str] | None = None,
) -> tuple[list[dict[str, Any]], list[str]]:
    selected: list[dict[str, Any]] = []
    errors: list[str] = []
    assertions = payload.get("readiness_assertions")
    if not isinstance(assertions, list):
        return [], ["status.json missing readiness_assertions list"]

    for raw in assertions:
        if not isinstance(raw, dict):
            continue
        assertion_id = str(raw.get("id", "")).strip()
        blocking_level = str(raw.get("blocking_level", "")).strip()
        proof_type = str(raw.get("proof_type", "")).strip()
        if assertion_ids and assertion_id not in assertion_ids:
            continue
        if blocking_levels and blocking_level not in blocking_levels:
            continue
        if proof_types and proof_type not in proof_types:
            continue

        raw_commands = raw.get("proof_commands")
        if raw_commands is None:
            if proof_type == "automated":
                errors.append(f"{assertion_id} is automated but has no proof_commands")
            continue
        if not isinstance(raw_commands, list) or not raw_commands:
            errors.append(f"{assertion_id} proof_commands must be a non-empty list")
            continue

        for raw_command in raw_commands:
            if not isinstance(raw_command, dict):
                errors.append(f"{assertion_id} proof_commands contains a non-object entry")
                continue
            command_id = str(raw_command.get("id", "")).strip()
            run = raw_command.get("run")
            cwd = raw_command.get("cwd", ".")
            if not command_id:
                errors.append(f"{assertion_id} proof command is missing an id")
                continue
            if not isinstance(run, list) or not run or any(
                not isinstance(entry, str) or not entry.strip() for entry in run
            ):
                errors.append(f"{assertion_id}:{command_id} run must be a non-empty list of strings")
                continue
            if not isinstance(cwd, str) or not cwd.strip():
                errors.append(f"{assertion_id}:{command_id} cwd must be a non-empty string when declared")
                continue
            try:
                clean_cwd = _clean_relative_dir(cwd)
            except ValueError as exc:
                errors.append(f"{assertion_id}:{command_id} {exc}")
                continue
            selected.append(
                {
                    "assertion_id": assertion_id,
                    "command_id": command_id,
                    "run": [str(entry) for entry in run],
                    "cwd": clean_cwd,
                }
            )

    return selected, errors


def deduplicated_proof_commands(commands: list[dict[str, Any]]) -> list[dict[str, Any]]:
    grouped: dict[tuple[str, tuple[str, ...]], dict[str, Any]] = {}
    ordered_keys: list[tuple[str, tuple[str, ...]]] = []

    for command in commands:
        cwd = str(command["cwd"])
        run = tuple(str(entry) for entry in command["run"])
        key = (cwd, run)
        record = grouped.get(key)
        if record is None:
            record = {
                "assertion_ids": [str(command["assertion_id"])],
                "command_ids": [str(command["command_id"])],
                "cwd": cwd,
                "run": list(run),
            }
            grouped[key] = record
            ordered_keys.append(key)
            continue
        assertion_id = str(command["assertion_id"])
        command_id = str(command["command_id"])
        if assertion_id not in record["assertion_ids"]:
            record["assertion_ids"].append(assertion_id)
        if command_id not in record["command_ids"]:
            record["command_ids"].append(command_id)

    return [grouped[key] for key in ordered_keys]


def run_selected_proof_commands(commands: list[dict[str, Any]]) -> int:
    unique_commands = deduplicated_proof_commands(commands)
    if not commands:
        print("Readiness assertion guard: no matching proof commands.")
        return 0

    print(
        "Running readiness assertion guard: "
        f"commands={len(unique_commands)} selected={len(commands)}"
    )
    for command in unique_commands:
        assertion_ids = [str(entry) for entry in command["assertion_ids"]]
        command_ids = [str(entry) for entry in command["command_ids"]]
        run = [str(entry) for entry in command["run"]]
        cwd = REPO_ROOT / str(command["cwd"])
        print(
            f"[{','.join(assertion_ids)}] {','.join(command_ids)}: {shlex.join(run)}"
        )
        result = subprocess.run(run, cwd=cwd, check=False)
        if result.returncode != 0:
            print(
                "BLOCKED: readiness assertion proof failed for "
                f"{','.join(assertion_ids)}:{','.join(command_ids)} "
                f"(exit {result.returncode})"
            )
            return result.returncode or 1

    print("Readiness assertion guard passed.")
    return 0


def main(argv: list[str] | None = None) -> int:
    args = parse_args(list(argv or []))
    commands, errors = selected_proof_commands(
        load_status_payload(staged=args.staged),
        blocking_levels=set(args.blocking_level or []),
        proof_types=set(args.proof_type or []),
        assertion_ids=set(args.assertion or []),
    )
    if errors:
        for error in errors:
            print(f"BLOCKED: {error}")
        return 1
    return run_selected_proof_commands(commands)


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
