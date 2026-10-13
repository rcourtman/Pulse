#!/usr/bin/env python3
"""Run one audit with an owned deadline and process group, preserving its bytes."""

from __future__ import annotations

import os
import signal
import subprocess
import sys
import time
from pathlib import Path


def signal_group(process: subprocess.Popen, signum: int) -> None:
    # start_new_session makes this the audit's group, never the invoking
    # shell's or another job's. The leader may have exited with children left.
    try:
        os.killpg(process.pid, signum)
    except ProcessLookupError:
        pass


def run_audit(limit: float, output: Path, command: list[str]) -> int:
    process = None
    status = None
    timed_out = False
    interrupted = [None]
    previous_handlers = {}

    def interrupt(signum: int, _frame: object) -> None:
        # Do not raise between Popen creating the child and returning its
        # handle: cleanup must have the actual group identity before it acts.
        interrupted[0] = signum

    try:
        for signum in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP):
            previous_handlers[signum] = signal.signal(signum, interrupt)
        with output.open("wb") as report:
            if interrupted[0] is None:
                process = subprocess.Popen(
                    command, stdout=report, stderr=subprocess.DEVNULL,
                    start_new_session=True,
                )
                deadline = time.monotonic() + limit
                while interrupted[0] is None:
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        timed_out = True
                        break
                    try:
                        status = process.wait(timeout=min(0.1, remaining))
                        break
                    except subprocess.TimeoutExpired:
                        pass
            if timed_out:
                signal_group(process, signal.SIGTERM)
                try:
                    status = process.wait(timeout=2)
                except subprocess.TimeoutExpired:
                    pass
    except OSError as exc:
        print(f"npm audit process could not start: {exc.strerror}", file=sys.stderr)
        status = 127
    finally:
        # A second cancellation must not interrupt cleanup. KILL also owns
        # TERM-resistant descendants after a timeout or an already exited
        # leader. No sleeper/watchdog process is needed on the success path.
        for signum in previous_handlers:
            signal.signal(signum, signal.SIG_IGN)
        try:
            if process is not None:
                signal_group(process, signal.SIGKILL)
                observed_status = process.wait()
                if status is None:
                    status = observed_status
        finally:
            for signum, handler in previous_handlers.items():
                signal.signal(signum, handler)
    if interrupted[0] is not None:
        return 128 + interrupted[0]
    # A process that exits zero in its TERM handler still missed the deadline.
    # Its zero summary must not become a clean audit.
    if timed_out and status >= 0:
        status = -signal.SIGTERM
    return status if status >= 0 else 128 - status


def main() -> int:
    if len(sys.argv) < 4:
        print("Usage: npm_audit_process.py seconds report command [args...]", file=sys.stderr)
        return 2
    try:
        limit = float(sys.argv[1])
        if not 0 < limit < float("inf"):
            raise ValueError
    except ValueError:
        print("npm audit process needs a finite positive deadline", file=sys.stderr)
        return 2
    return run_audit(limit, Path(sys.argv[2]), sys.argv[3:])


if __name__ == "__main__":
    sys.exit(main())
