#!/usr/bin/env python3
"""Retain CI container diagnostics without flooding the combined Actions log.

Full stdout/stderr belongs in this run's short-lived artifact. The console gets
only a bounded, escaped tail, byte/hash identity and an honest collection result.
This observes fixed disposable E2E containers; it never starts or changes one.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys


TAIL_BYTES = 4096
COMMAND_TIMEOUT_SECONDS = 30
COMMANDS = (
    ("containers.txt", ("docker", "ps", "-a")),
    ("pulse-test-server.log", ("docker", "logs", "pulse-test-server")),
    ("pulse-mock-github.log", ("docker", "logs", "pulse-mock-github")),
)


def checkout_identity():
    identity = {
        "source_sha": os.environ["GITHUB_SHA"],
        "run_id": os.environ["GITHUB_RUN_ID"],
        "run_attempt": os.environ["GITHUB_RUN_ATTEMPT"],
    }
    if not re.fullmatch(r"[0-9a-f]{40}", identity["source_sha"]):
        raise ValueError("invalid E2E source identity")
    if any(not re.fullmatch(r"[1-9][0-9]*", identity[key])
           for key in ("run_id", "run_attempt")):
        raise ValueError("invalid E2E run identity")
    source = subprocess.run(
        ["git", "rev-parse", "HEAD"], check=True, capture_output=True,
        text=True, timeout=COMMAND_TIMEOUT_SECONDS,
    ).stdout.strip()
    if source != identity["source_sha"]:
        raise ValueError("E2E checkout differs from this run")
    return identity


def file_observation(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(65536), b""):
            digest.update(block)
        size = stream.tell()
        stream.seek(max(0, size - TAIL_BYTES))
        tail = stream.read(TAIL_BYTES)
    return size, digest.hexdigest(), tail


def collect(directory, identity, *, registration=False, run=subprocess.run, console=None):
    console = sys.stdout if console is None else console
    # Never overwrite another attempt's evidence, including through a symlink.
    directory.mkdir(parents=True, exist_ok=False)
    records = []
    for filename, command in COMMANDS[:2] if registration else COMMANDS:
        path = directory / filename
        record = {"file": filename, "exit_code": None, "collection_complete": False}
        with path.open("xb") as stream:
            try:
                result = run(command, stdout=stream, stderr=subprocess.STDOUT,
                             timeout=COMMAND_TIMEOUT_SECONDS, check=False)
                record.update(exit_code=result.returncode,
                              collection_complete=result.returncode == 0)
            except subprocess.TimeoutExpired:
                # subprocess.run stops and waits for its Docker CLI on timeout;
                # TimeoutExpired exposes no terminal exit. Do not invent one.
                record["failure"] = "timeout"
            except OSError:
                record["failure"] = "spawn"
        size, digest, tail = file_observation(path)
        record.update(bytes=size, sha256=digest)
        records.append(record)
        # JSON escaping prevents log text becoming Actions commands. Never
        # mistake this bounded excerpt for the complete artifact bytes.
        print(json.dumps({**record, "console_tail_bytes": len(tail),
                          "console_tail_truncated": size > len(tail),
                          "console_tail": tail.decode("utf-8", errors="replace")}),
              file=console, flush=True)
    complete = all(record["collection_complete"] for record in records)
    manifest = {"schema_version": 1, **identity, "collection_complete": complete,
                "records": records}
    (directory / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    return 0 if complete else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output", type=Path)
    parser.add_argument("--registration", action="store_true")
    arguments = parser.parse_args()
    return collect(arguments.output, checkout_identity(), registration=arguments.registration)


if __name__ == "__main__":
    raise SystemExit(main())
