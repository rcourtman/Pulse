#!/usr/bin/env python3
"""Reject the existing portal manifest drift before CI installs dependencies.

Uses the same ordered path/newline/content/newline hash as the portal build
and TestPulseAccountFrontendBundleStaysInSync. It neither builds nor executes
the frontend; the Go tests and real frontend builds remain required.
"""

from __future__ import annotations

import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import sys


ROOT = Path(__file__).resolve().parents[1]
PORTAL = Path("internal/cloudcp/portal")
REBUILD = "npm --prefix internal/cloudcp/portal/frontend run build"


def check(root: Path) -> str:
    portal = root / PORTAL
    frontend = (portal / "frontend").resolve()
    manifest = json.loads((portal / "dist/build_manifest.json").read_bytes())
    if not isinstance(manifest, dict):
        raise ValueError("manifest must be an object")
    expected = manifest.get("source_hash")
    inputs = manifest.get("build_inputs")
    if not isinstance(expected, str) or not re.fullmatch(r"[0-9a-f]{64}", expected):
        raise ValueError("manifest missing a valid source_hash")
    if not isinstance(inputs, list) or not inputs:
        raise ValueError("manifest missing build_inputs")
    seen: set[str] = set()
    digest = hashlib.sha256()
    for name in inputs:
        if not isinstance(name, str) or not name or "\x00" in name or "\\" in name:
            raise ValueError("manifest contains an invalid build input")
        relative = PurePosixPath(name)
        if relative.is_absolute() or relative.as_posix() != name or ".." in relative.parts or name in seen:
            raise ValueError("manifest contains a non-canonical or repeated build input")
        seen.add(name)
        path = frontend / name
        if not path.resolve().is_relative_to(frontend):
            raise ValueError("manifest build input escapes the portal frontend")
        digest.update(name.encode("utf-8") + b"\n")
        digest.update(path.read_bytes() + b"\n")
    if not {"build_config.mjs", "src/styles.css"}.issubset(seen):
        raise ValueError("manifest missing canonical build_config.mjs or src/styles.css")
    actual = digest.hexdigest()
    if actual != expected:
        raise ValueError(f"source hash differs: manifest={expected}, actual={actual}")
    return actual


def main() -> int:
    try:
        digest = check(ROOT)
    except (OSError, ValueError) as error:
        print(f"Portal frontend sync check failed: {error}", file=sys.stderr)
        print(f"Rebuild the portal and commit its generated bundle/manifest: `{REBUILD}`.", file=sys.stderr)
        return 1
    print(f"Portal frontend inputs match the shipped manifest ({digest}); builds and Go tests remain required.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
