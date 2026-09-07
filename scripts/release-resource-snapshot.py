#!/usr/bin/env python3
"""Bounded, read-only backend phase evidence; never a release-gate verdict.

Only allowlisted kernel files and numeric runtime knobs are emitted. No full
process environment, command lines, host identity or application data is read.
Snapshots bracket the backend phase, not an individual test. A missing counter
is unavailable evidence, not zero pressure. Ancestors may include other work.
"""
import json
import os
from pathlib import Path
import platform
import re
import sys
import time

CGROUP_FILES = (
    "cpu.max", "cpu.stat", "cpu.pressure", "cpuset.cpus.effective",
    "memory.current", "memory.max", "memory.events", "memory.pressure",
    "io.pressure", "pids.current", "pids.max",
)


def read(path):
    try:
        with path.open() as stream:
            return stream.read(8192).strip()
    except OSError as error:
        return {"unavailable_errno": error.errno}


def cgroups(proc=Path("/proc"), root=Path("/sys/fs/cgroup")):
    membership = read(proc / "self/cgroup")
    if not isinstance(membership, str):
        return {"unavailable": "cgroup membership unreadable"}
    paths = [line[3:] for line in membership.splitlines() if line.startswith("0::/")]
    if len(paths) != 1 or ".." in Path(paths[0]).parts:
        return {"unavailable": "no safely mapped unified cgroup"}
    # Do not guess the mapping for a relocated or subtree cgroup mount.
    mounts = read(proc / "self/mountinfo")
    if not isinstance(mounts, str) or not any(
        len(fields := line.split()) > 6 and fields[3] == "/"
        and fields[4] == str(root) and " - cgroup2 " in line
        for line in mounts.splitlines()
    ):
        return {"unavailable": "standard root cgroup2 mount not established"}
    current = root / paths[0].lstrip("/")
    result = []
    # Label by ancestor distance rather than disclosing service/session names.
    while True:
        result.append({"ancestor_distance": len(result), "files": {
            name: read(current / name) for name in CGROUP_FILES
        }})
        if current == root:
            break
        if len(result) >= 64:
            return {"unavailable": "hierarchy exceeds collection bound", "partial": result}
        current = current.parent
    return result


def snapshot():
    knobs = {}
    for name in ("GOMAXPROCS", "GOGC", "GOMEMLIMIT"):
        value = os.environ.get(name)
        knobs[name] = value if value is None or re.fullmatch(r"(?:[0-9]+(?:[KMGTPE]i?B)?|off)", value) else "nonstandard value omitted"
    return {
        "schema_version": 1,
        "unix_time_ns": time.time_ns(),
        "monotonic_ns": time.monotonic_ns(),
        "kernel": platform.release(),
        "architecture": platform.machine(),
        "cpu_count": os.cpu_count(),
        "cpu_affinity": sorted(os.sched_getaffinity(0)),
        "runtime_knobs": knobs,
        "host": {name: read(Path("/proc") / name) for name in (
            "loadavg", "pressure/cpu", "pressure/memory", "pressure/io",
        )},
        "cgroups": cgroups(),
    }


if __name__ == "__main__":
    if len(sys.argv) != 2 or sys.argv[1] not in ("before", "after"):
        raise SystemExit("Usage: release-resource-snapshot.py before|after")
    print("RELEASE_RESOURCE_SNAPSHOT " + json.dumps({"boundary": sys.argv[1], **snapshot()}, sort_keys=True), flush=True)
