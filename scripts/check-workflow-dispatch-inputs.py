#!/usr/bin/env python3
"""Validate that the default-branch copy of a workflow accepts required inputs."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR / "release_control"))

from release_promotion_policy_support import missing_workflow_dispatch_inputs


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--workflow-path", required=True)
    parser.add_argument("--branch", default="")
    parser.add_argument("--require", action="append", default=[])
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    branch, missing = missing_workflow_dispatch_inputs(
        workflow_path=args.workflow_path,
        required_inputs=tuple(args.require),
        branch=args.branch or None,
    )
    if missing:
        missing_display = ", ".join(missing)
        raise SystemExit(
            f"default-branch workflow contract drift: origin/{branch}:{args.workflow_path} "
            f"is missing workflow_dispatch inputs: {missing_display}"
        )
    print(
        f"[OK] origin/{branch}:{args.workflow_path} accepts required workflow_dispatch inputs"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
