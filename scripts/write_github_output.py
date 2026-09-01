#!/usr/bin/env python3
"""Append one value to GITHUB_OUTPUT without permitting command injection."""

from __future__ import annotations

import os
import re
import secrets
import sys
from collections.abc import Callable
from pathlib import Path


OUTPUT_NAME_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")


def append_output(
    path: Path,
    name: str,
    value: str,
    token_factory: Callable[[], str] | None = None,
) -> None:
    """Append *value* under *name* using a collision-free multiline record."""
    if not OUTPUT_NAME_RE.fullmatch(name):
        raise ValueError(f"invalid GitHub output name: {name!r}")
    if "\x00" in value:
        raise ValueError("GitHub output values must not contain NUL bytes")

    make_token = token_factory or (lambda: secrets.token_hex(16))
    value_lines = set(value.splitlines())
    while True:
        delimiter = f"pulse_output_{make_token()}"
        if delimiter not in value_lines:
            break

    with path.open("a", encoding="utf-8", newline="\n") as output:
        output.write(f"{name}<<{delimiter}\n")
        output.write(value)
        if not value.endswith("\n"):
            output.write("\n")
        output.write(f"{delimiter}\n")


def main() -> int:
    if len(sys.argv) != 3:
        print(f"usage: {Path(sys.argv[0]).name} NAME VALUE", file=sys.stderr)
        return 2
    output_path = os.environ.get("GITHUB_OUTPUT")
    if not output_path:
        print("GITHUB_OUTPUT is required", file=sys.stderr)
        return 2
    try:
        append_output(Path(output_path), sys.argv[1], sys.argv[2])
    except (OSError, ValueError) as error:
        print(f"could not write GitHub output: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
