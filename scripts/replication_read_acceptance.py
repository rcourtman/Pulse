#!/usr/bin/env python3
"""Connect safe client, monitor and real HTTP-handler fixtures, without listeners.

This is focused source/consumer proof, not a native replication or restart test.
Each stage uses only the previous stage's emitted JSON, not replacement verdicts.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--race", action="store_true")
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    stages = [
        ("client", "./pkg/proxmox", "^TestReplicationRead", "REPLICATION_CLIENT_FRAMES", "PULSE_REPLICATION_ACCEPTANCE_FRAMES"),
        ("monitor", "./internal/monitoring", "^TestReplicationReadConsumerFrames$", "REPLICATION_MONITOR_FRAMES", "PULSE_REPLICATION_MONITOR_FRAMES"),
        ("http", "./internal/api", "^TestReplicationReadHTTPFrames$", "REPLICATION_HTTP_FRAMES", None),
    ]
    receipts = []
    for name, package, pattern, marker, variable in stages:
        command = ["go", "test", package, "-run", pattern, "-count=1", "-v"]
        if args.race:
            command.append("-race")
        result = subprocess.run(command, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        log = args.output / f"{name}.log"
        log.write_bytes(result.stdout)
        receipts.append({"stage": name, "command": command, "exit": result.returncode,
                         "log": log.name, "log_sha256": hashlib.sha256(result.stdout).hexdigest()})
        (args.output / "commands.json").write_text(json.dumps(receipts, indent=2) + "\n")
        if result.returncode:
            print(f"{name} failed, exit {result.returncode}; see {log}", flush=True)
            print(result.stdout.decode(errors="replace")[-5000:], flush=True)
            return 1
        candidates = [line.split(marker + "=", 1)[1] for line in result.stdout.decode().splitlines() if marker + "=" in line]
        if len(candidates) != 1:
            raise ValueError(f"Expected exactly one emitted {name} frame")
        frames = json.loads(candidates[0])
        if set(frames) != {"initial", "short-inventory", "ambiguous-status", "recovered", "empty"}:
            raise ValueError(f"Incomplete {name} phase set")
        destination = args.output / f"{name}-frames.json"
        payload = (json.dumps(frames, sort_keys=True, indent=2) + "\n").encode()
        destination.write_bytes(payload)
        receipts[-1]["frames_sha256"] = hashlib.sha256(payload).hexdigest()
        if variable:
            env[variable] = str(destination.resolve())
        print(f"{name}: exit 0, five emitted phases", flush=True)
    (args.output / "commands.json").write_text(json.dumps(receipts, indent=2) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
