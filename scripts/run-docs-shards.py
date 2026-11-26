#!/usr/bin/env python3
"""Run all four isolated documentation test shards concurrently in one runner.

Each shard still executes scripts/tests/run.sh --docs-shard <0|1|2|3> in its
own process group and private output file. Every shard runs to completion
without fail-fast suppression, output is replayed in deterministic shard order,
and any missing, signalled, or non-zero shard fails the verdict.
"""

from __future__ import annotations

import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time


ROOT = Path(__file__).resolve().parents[1]
RUNNER = ROOT / "scripts/tests/run.sh"
SHARD_COUNT = 4
STOP_GRACE_SECONDS = 3


class Interruption:
    """Defer catchable cancellation so every owned shard group can be stopped."""

    received_signal: int | None = None

    def receive(self, signum: int, _frame) -> None:
        if self.received_signal is None:
            self.received_signal = signum

    def __enter__(self) -> Interruption:
        self.previous = {
            number: signal.signal(number, self.receive)
            for number in (signal.SIGINT, signal.SIGTERM)
        }
        return self

    def __exit__(self, *_exception) -> None:
        for number, handler in self.previous.items():
            signal.signal(number, handler)


def _group_present(pid: int) -> bool:
    try:
        os.killpg(pid, 0)
        return True
    except ProcessLookupError:
        return False


def _group_has_live_process(pgid: int) -> bool:
    if not _group_present(pgid):
        return False
    proc = Path("/proc")
    if not proc.is_dir():
        return True
    try:
        entries = list(proc.iterdir())
    except OSError:
        return True
    for entry in entries:
        if not entry.name.isdigit():
            continue
        try:
            stat = (entry / "stat").read_text(encoding="utf-8", errors="replace")
            fields = stat.rsplit(") ", 1)[1].split()
            if len(fields) >= 3 and int(fields[2]) == pgid and fields[0] != "Z":
                return True
        except (FileNotFoundError, ProcessLookupError, PermissionError, IndexError, ValueError):
            continue
    return False


def _send_group(pid: int, signum: int) -> None:
    try:
        os.killpg(pid, signum)
    except ProcessLookupError:
        pass


def stop_shards(processes: dict[int, subprocess.Popen], signum: int) -> None:
    """Stop every started shard's owned process group with bounded grace."""
    for process in processes.values():
        if process.poll() is None or _group_has_live_process(process.pid):
            _send_group(process.pid, signum)

    deadline = time.monotonic() + STOP_GRACE_SECONDS
    while True:
        still_present = False
        for process in processes.values():
            if process.poll() is None or _group_has_live_process(process.pid):
                still_present = True
        if not still_present or time.monotonic() >= deadline:
            break
        time.sleep(0.05)

    killed = False
    for process in processes.values():
        if process.poll() is None or _group_has_live_process(process.pid):
            _send_group(process.pid, signal.SIGKILL)
            killed = True

    kill_deadline = time.monotonic() + STOP_GRACE_SECONDS
    for process in processes.values():
        remaining = max(0.0, kill_deadline - time.monotonic())
        try:
            process.wait(timeout=remaining)
        except subprocess.TimeoutExpired:
            pass

    if killed:
        while time.monotonic() < kill_deadline:
            if not any(_group_has_live_process(process.pid) for process in processes.values()):
                break
            time.sleep(0.02)


def format_status(returncode: int | None, start_error: int | None) -> str:
    if start_error is not None:
        return f"start error errno {start_error}"
    if returncode is None:
        return "unobserved"
    if returncode < 0:
        return f"signal {-returncode}"
    return f"exit {returncode}"


def run_all_shards(interruption: Interruption) -> int:
    if not RUNNER.is_file() or RUNNER.is_symlink():
        print(f"Documentation shard runner is missing or not a regular file: {RUNNER}",
              file=sys.stderr, flush=True)
        return 1

    with tempfile.TemporaryDirectory(prefix="pulse-docs-shards-") as temporary:
        log_dir = Path(temporary)
        processes: dict[int, subprocess.Popen] = {}
        log_files: dict[int, Path] = {}
        streams: dict[int, object] = {}
        start_times: dict[int, float] = {}
        elapsed: dict[int, float] = {}
        results: dict[int, int | None] = {shard: None for shard in range(SHARD_COUNT)}
        start_errors: dict[int, int | None] = {shard: None for shard in range(SHARD_COUNT)}

        try:
            for shard in range(SHARD_COUNT):
                if interruption.received_signal is not None:
                    break
                log_path = log_dir / f"shard-{shard}.log"
                log_files[shard] = log_path
                stream = log_path.open("wb")
                streams[shard] = stream
                start_times[shard] = time.monotonic()
                try:
                    processes[shard] = subprocess.Popen(
                        ["bash", str(RUNNER), "--docs-shard", str(shard)],
                        cwd=ROOT,
                        stdout=stream,
                        stderr=subprocess.STDOUT,
                        start_new_session=True,
                    )
                except OSError as error:
                    start_errors[shard] = error.errno
                    elapsed[shard] = time.monotonic() - start_times[shard]
                    stream.close()
                    del streams[shard]

            while True:
                if interruption.received_signal is not None:
                    stop_shards(processes, interruption.received_signal)
                    for shard, process in processes.items():
                        if results[shard] is None:
                            results[shard] = process.poll()
                            elapsed[shard] = time.monotonic() - start_times[shard]
                    break

                pending = False
                for shard, process in processes.items():
                    if results[shard] is not None:
                        continue
                    code = process.poll()
                    if code is None:
                        pending = True
                    else:
                        results[shard] = code
                        elapsed[shard] = time.monotonic() - start_times[shard]
                if not pending:
                    break
                time.sleep(0.05)
        finally:
            for stream in streams.values():
                stream.close()

        for shard in range(SHARD_COUNT):
            status = format_status(results[shard], start_errors[shard])
            seconds = elapsed.get(shard, 0.0)
            print(f"=== Documentation shard {shard}/{SHARD_COUNT} ({status}, {seconds:.2f}s) ===",
                  flush=True)
            log_path = log_files.get(shard)
            if log_path is not None and log_path.is_file():
                text = log_path.read_text(encoding="utf-8", errors="replace")
                if text:
                    sys.stdout.write(text)
                    if not text.endswith("\n"):
                        sys.stdout.write("\n")
                    sys.stdout.flush()

    if interruption.received_signal is not None:
        print(
            f"Documentation shards interrupted (signal {interruption.received_signal}); "
            "no passing verdict.",
            file=sys.stderr,
            flush=True,
        )
        return 128 + interruption.received_signal

    failed = [
        (shard, format_status(results[shard], start_errors[shard]))
        for shard in range(SHARD_COUNT)
        if start_errors[shard] is not None or results[shard] != 0
    ]
    passed = SHARD_COUNT - len(failed)
    print(f"Documentation shards summary: {passed}/{SHARD_COUNT} passed", flush=True)
    if failed:
        print(f"Failed shards: {len(failed)}", flush=True)
        for shard, status in failed:
            print(f"  shard {shard}/{SHARD_COUNT} ({status})", flush=True)
        return 1
    return 0


def main() -> int:
    if len(sys.argv) != 1:
        print("Usage: python3 scripts/run-docs-shards.py", file=sys.stderr)
        return 2
    with Interruption() as interruption:
        code = run_all_shards(interruption)
    if interruption.received_signal is not None:
        return 128 + interruption.received_signal
    return code


if __name__ == "__main__":
    raise SystemExit(main())
