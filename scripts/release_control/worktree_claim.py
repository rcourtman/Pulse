#!/usr/bin/env python3
"""Public entrypoint for governed isolated-worktree claims."""

from __future__ import annotations

from pathlib import Path
import runpy
import sys


def main(argv: list[str] | None = None) -> int:
    internal_dir = Path(__file__).resolve().parent / "internal"
    sys.path.insert(0, str(internal_dir))
    try:
        namespace = runpy.run_path(str(internal_dir / "worktree_claim.py"))
        return namespace["main"](list(sys.argv[1:] if argv is None else argv))
    finally:
        sys.path.pop(0)


if __name__ == "__main__":
    raise SystemExit(main())
