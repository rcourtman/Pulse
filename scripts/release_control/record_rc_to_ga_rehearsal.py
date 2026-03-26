#!/usr/bin/env python3
"""Public entrypoint for prerelease-to-GA rehearsal record generation."""

from __future__ import annotations

import importlib.util
from pathlib import Path
import sys
from types import ModuleType


def _load_internal_module() -> ModuleType:
    module_path = Path(__file__).with_name("internal") / "record_rc_to_ga_rehearsal.py"
    spec = importlib.util.spec_from_file_location(
        "_record_rc_to_ga_rehearsal_internal",
        module_path,
    )
    if spec is None or spec.loader is None:
        raise ImportError(f"unable to load internal module from {module_path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


_INTERNAL = _load_internal_module()

parse_args = _INTERNAL.parse_args
load_run_metadata = _INTERNAL.load_run_metadata
download_summary_artifact = _INTERNAL.download_summary_artifact
parse_summary_markdown = _INTERNAL.parse_summary_markdown
normalize_summary_command = _INTERNAL.normalize_summary_command
validate_required_summary_metadata = _INTERNAL.validate_required_summary_metadata
normalize_output_path = _INTERNAL.normalize_output_path
normalize_input_path = _INTERNAL.normalize_input_path
render_record = _INTERNAL.render_record
main = _INTERNAL.main


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (FileNotFoundError, ValueError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        raise SystemExit(1)
