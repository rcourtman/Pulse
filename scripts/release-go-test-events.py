#!/usr/bin/env python3
"""Render streamed go test -json output and retain bounded stress-test timing.

Go's event Time is distinct from receipt time and the resource sample time.
Samples share the worker cgroup; they do not establish per-test CPU usage or
causality. Output is rendered verbatim, including package summaries and failures.
The shell's pipefail retains the Go verdict; malformed input fails this reader.
"""
import importlib.util
import json
from pathlib import Path
import sys
import time

spec = importlib.util.spec_from_file_location(
    "release_resource_snapshot", Path(__file__).with_name("release-resource-snapshot.py")
)
resources = importlib.util.module_from_spec(spec)
spec.loader.exec_module(resources)

TARGET = "TestMultiTenant_ConcurrentAPIStress"
PACKAGE = "github.com/rcourtman/pulse-go-rewrite/internal/api"
ACTIONS = {"run", "pause", "cont", "pass", "fail", "skip"}


def render(source, destination):
    invalid = False
    for line in source:
        received = time.time_ns()
        try:
            event = json.loads(line)
            if not isinstance(event, dict):
                raise ValueError("not an event object")
            output = event.get("Output", "")
            if not isinstance(output, str):
                raise ValueError("non-text output")
        except (ValueError, TypeError):
            # Drain the stream rather than masking the producer with SIGPIPE.
            destination.write(line)
            invalid = True
            continue
        destination.write(output)
        if (event.get("Package") == PACKAGE and event.get("Test") == TARGET
                and event.get("Action") in ACTIONS):
            evidence = {key: event[key] for key in
                        ("Time", "Action", "Package", "Test", "Elapsed") if key in event}
            evidence["received_unix_time_ns"] = received
            try:
                evidence["resources"] = resources.snapshot()
            except Exception:
                evidence["resources"] = {"unavailable": "snapshot collection failed"}
            destination.write("RELEASE_GO_TEST_EVENT " + json.dumps(evidence, sort_keys=True) + "\n")
        destination.flush()
    if invalid:
        destination.write("RELEASE_GO_TEST_STREAM invalid JSON event input\n")
        destination.flush()
    return int(invalid)


if __name__ == "__main__":
    raise SystemExit(render(sys.stdin, sys.stdout))
