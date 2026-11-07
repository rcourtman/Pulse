#!/usr/bin/env python3
"""Execute the shell runner's selected suites and retain returned observations.

Selection belongs to scripts/tests/run.sh. Optional CI receipts contain source
identifiers and process results only, never suite output. A pending invocation
does not establish execution or liveness, and incomplete collection is not a pass.
"""

from __future__ import annotations

import errno
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time


ROOT = Path(__file__).resolve().parents[1]
RECEIPT_ENV = ("PULSE_SCRIPT_TEST_RECEIPT", "PULSE_SCRIPT_TEST_SOURCE_SHA")


def snapshot(path: Path, receipt: dict) -> None:
    """Publish one complete snapshot without truncating the previous one."""
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=path.parent,
                                         prefix=".script-suites-", delete=False) as stream:
            temporary = Path(stream.name)
            json.dump(receipt, stream, indent=2)
            stream.write("\n")
        temporary.replace(path)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def source_identity() -> str:
    expected = os.environ.get("PULSE_SCRIPT_TEST_SOURCE_SHA", "")
    if not re.fullmatch(r"[0-9a-f]{40}", expected):
        raise ValueError("receipt requires an exact expected source SHA")
    actual = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT,
                            capture_output=True, text=True, check=True).stdout.strip()
    if actual != expected:
        raise ValueError("receipt source differs from the checked-out HEAD")
    return actual


def run(tests: list[str]) -> int:
    names = [os.path.relpath(test, ROOT) for test in tests]
    path = Path(os.environ[RECEIPT_ENV[0]]) if os.environ.get(RECEIPT_ENV[0]) else None
    receipt = None
    if path is not None:
        # Never mistake a previous invocation's results for this selection.
        # lexists also refuses dangling symlinks without following them.
        if os.path.lexists(path):
            raise ValueError("receipt destination already exists")
        receipt = {"schema_version": 1, "source_sha": source_identity(),
                   "selected_suites": names, "collection_complete": False,
                   "pending_invocation": None, "observations": [],
                   "unobserved_indexes": list(range(len(tests)))}
        snapshot(path, receipt)

    # Nested harness controls must not overwrite or reuse the outer CI receipt.
    environment = {key: value for key, value in os.environ.items() if key not in RECEIPT_ENV}
    failures = []
    for index, (test, name) in enumerate(zip(tests, names)):
        print(f"==> {name}", flush=True)
        if receipt is not None:
            receipt["pending_invocation"] = index
            snapshot(path, receipt)
        start = time.monotonic()
        command = ["python3", test] if test.endswith(".py") else [test]
        try:
            result = subprocess.run(command, cwd=ROOT, env=environment)
            observation = {"started": True,
                           "exit_code": result.returncode if result.returncode >= 0 else None,
                           "signal": -result.returncode if result.returncode < 0 else None}
            # Preserve the existing shell-style console summary. A numeric
            # exit 137 remains an exit, distinct from an observed SIGKILL.
            status = result.returncode if result.returncode >= 0 else 128 - result.returncode
        except OSError as error:
            observation = {"started": False, "exit_code": None, "signal": None,
                           "start_error_errno": error.errno}
            status = 127 if error.errno == errno.ENOENT else 126
            print(f"Could not start {name} (errno {error.errno})", file=sys.stderr, flush=True)
        if receipt is not None:
            receipt["observations"].append({"index": index, "suite": name, **observation,
                                             "seconds": round(time.monotonic() - start, 6)})
            receipt["pending_invocation"] = None
            receipt["unobserved_indexes"] = list(range(index + 1, len(tests)))
            snapshot(path, receipt)
        if status:
            failures.append((name, status))
            print(f"FAIL (exit {status})\n", flush=True)
        else:
            print("PASS\n", flush=True)

    if receipt is not None:
        receipt["collection_complete"] = True
        snapshot(path, receipt)
    print(f"Summary: {len(tests) - len(failures)}/{len(tests)} passed", flush=True)
    if failures:
        print(f"Failures: {len(failures)}\nFailed suites:", flush=True)
        for name, status in failures:
            print(f"  {name} (exit {status})", flush=True)
    return int(bool(failures))


def main() -> int:
    if not sys.argv[1:]:
        print("No selected script suites; no execution or coverage verdict", file=sys.stderr)
        return 1
    try:
        return run(sys.argv[1:])
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        # A failed receipt cannot admit the next suite or a successful verdict.
        print(f"Script suite observation failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
