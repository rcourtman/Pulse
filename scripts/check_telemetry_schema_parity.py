#!/usr/bin/env python3
"""Verify the public telemetry payload and private receiver stay in lockstep."""

from __future__ import annotations

import argparse
from pathlib import Path
import re
import sys


FIELD_RE = re.compile(
    r'^\s*[A-Za-z0-9_]+\s+([A-Za-z0-9_]+)\s+`json:"([^",]+)(?:,[^"]*)?"`',
    re.MULTILINE,
)
TYPESCRIPT_FIELD_RE = re.compile(
    r"^\s*([a-z0-9_]+)\??:\s*(string|number|boolean);",
    re.MULTILINE,
)
TYPESCRIPT_TO_GO = {"string": "string", "number": "int", "boolean": "bool"}
LEGACY_RECEIVER_ONLY_FIELDS = {"license_tier", "api_tokens"}


def struct_body(source: str, marker: str) -> str:
    start = source.find(marker)
    if start < 0:
        raise ValueError(f"missing struct marker {marker!r}")
    opening = source.find("{", start)
    if opening < 0:
        raise ValueError(f"missing opening brace after {marker!r}")

    depth = 0
    for index in range(opening, len(source)):
        char = source[index]
        if char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return source[opening + 1 : index]
    raise ValueError(f"unterminated struct after {marker!r}")


def json_fields(source: str, marker: str) -> dict[str, str]:
    fields: dict[str, str] = {}
    for go_type, json_name in FIELD_RE.findall(struct_body(source, marker)):
        if json_name in fields:
            raise ValueError(f"duplicate JSON field {json_name!r} after {marker!r}")
        fields[json_name] = go_type
    if not fields:
        raise ValueError(f"no JSON fields found after {marker!r}")
    return fields


def typescript_fields(source: str, marker: str) -> dict[str, str]:
    fields: dict[str, str] = {}
    for json_name, typescript_type in TYPESCRIPT_FIELD_RE.findall(struct_body(source, marker)):
        if json_name in fields:
            raise ValueError(f"duplicate TypeScript field {json_name!r} after {marker!r}")
        fields[json_name] = TYPESCRIPT_TO_GO[typescript_type]
    if not fields:
        raise ValueError(f"no TypeScript fields found after {marker!r}")
    return fields


def parity_errors(
    public_source: str,
    receiver_source: str,
    frontend_source: str | None = None,
) -> list[str]:
    public = json_fields(public_source, "type Ping struct")
    receiver = json_fields(receiver_source, "var ping struct")
    errors: list[str] = []

    missing = sorted(set(public) - set(receiver))
    if missing:
        errors.append("receiver missing public fields: " + ", ".join(missing))

    unexpected = sorted(set(receiver) - set(public) - LEGACY_RECEIVER_ONLY_FIELDS)
    if unexpected:
        errors.append("receiver-only fields outside the legacy allowlist: " + ", ".join(unexpected))

    mismatched = sorted(
        name
        for name in set(public) & set(receiver)
        if public[name] != receiver[name]
    )
    if mismatched:
        errors.append(
            "field type mismatches: "
            + ", ".join(
                f"{name} (public {public[name]}, receiver {receiver[name]})"
                for name in mismatched
            )
        )

    if public.get("schema_version") != "int":
        errors.append("public payload must contain integer schema_version")

    if frontend_source is not None:
        frontend = typescript_fields(frontend_source, "export interface TelemetryPingPreview")
        frontend_missing = sorted(set(public) - set(frontend))
        if frontend_missing:
            errors.append("frontend preview missing public fields: " + ", ".join(frontend_missing))

        frontend_unexpected = sorted(set(frontend) - set(public))
        if frontend_unexpected:
            errors.append("frontend preview fields absent from public payload: " + ", ".join(frontend_unexpected))

        frontend_mismatched = sorted(
            name
            for name in set(public) & set(frontend)
            if public[name] != frontend[name]
        )
        if frontend_mismatched:
            errors.append(
                "frontend field type mismatches: "
                + ", ".join(
                    f"{name} (public {public[name]}, frontend {frontend[name]})"
                    for name in frontend_mismatched
                )
            )
    return errors


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--pulse-repo",
        type=Path,
        default=Path(__file__).resolve().parents[1],
    )
    parser.add_argument(
        "--pulse-pro-repo",
        type=Path,
        default=Path(__file__).resolve().parents[2] / "pulse-pro",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    public_path = args.pulse_repo / "internal" / "telemetry" / "telemetry.go"
    frontend_path = args.pulse_repo / "frontend-modern" / "src" / "api" / "settings.ts"
    receiver_path = args.pulse_pro_repo / "license-server" / "main.go"
    if not public_path.is_file():
        raise SystemExit(f"public telemetry source not found: {public_path}")
    if not receiver_path.is_file():
        raise SystemExit(f"private telemetry receiver source not found: {receiver_path}")
    if not frontend_path.is_file():
        raise SystemExit(f"frontend telemetry preview source not found: {frontend_path}")

    errors = parity_errors(
        public_path.read_text(),
        receiver_path.read_text(),
        frontend_path.read_text(),
    )
    if errors:
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        return 1
    print("telemetry schema parity: OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
