#!/usr/bin/env python3

from __future__ import annotations

from pathlib import Path
import runpy


def main(argv: list[str] | None = None) -> int:
    namespace = runpy.run_path(
        str(
            Path(__file__).resolve().parent
            / "internal"
            / "mobile_relay_auth_approvals_proof.py"
        )
    )
    return namespace["main"](argv)


if __name__ == "__main__":
    raise SystemExit(main())
