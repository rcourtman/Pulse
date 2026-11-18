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
import signal
import subprocess
import sys
import tempfile
import time


ROOT = Path(__file__).resolve().parents[1]
RECEIPT_ENV = ("PULSE_SCRIPT_TEST_RECEIPT", "PULSE_SCRIPT_TEST_SOURCE_SHA")
STOP_GRACE_SECONDS = 2


class Interruption:
    """Defer catchable cancellation until the owned child can be observed."""

    received_signal = None

    def receive(self, signum, _frame):
        # A second signal must not interrupt a receipt write or restart cleanup.
        # Signal receipt says nothing about who initiated it or why.
        if self.received_signal is None:
            self.received_signal = signum

    def __enter__(self):
        self.previous = {number: signal.signal(number, self.receive)
                         for number in (signal.SIGINT, signal.SIGTERM)}
        return self

    def __exit__(self, *_exception):
        for number, handler in self.previous.items():
            signal.signal(number, handler)


def stop_suite(process: subprocess.Popen, number: int) -> dict:
    """Bound cancellation of this suite's new process group, never the caller's.

    Only the direct child's wait result is a terminal observation. Group signal
    requests/absence are separate facts, not proof of detached-child cleanup.
    """
    stop = {"requested_signal": number, "sent_signals": [], "leader_reaped": False,
            "process_group_absent": None}

    def send(signum):
        try:
            os.killpg(process.pid, signum)
            stop["sent_signals"].append(signum)
        except ProcessLookupError:
            pass

    def group_present():
        try:
            os.killpg(process.pid, 0)
            return True
        except ProcessLookupError:
            return False

    try:
        send(number)
        deadline = time.monotonic() + STOP_GRACE_SECONDS
        while True:
            process.poll()
            present = group_present()
            if not present or time.monotonic() >= deadline:
                break
            time.sleep(0.05)
        # A leader may exit while a descendant still holds the output stream.
        # Finish the bounded group stop even when that leader already returned.
        if present:
            send(signal.SIGKILL)
        try:
            process.wait(timeout=STOP_GRACE_SECONDS)
        except subprocess.TimeoutExpired:
            pass
        stop["process_group_absent"] = not group_present()
    except OSError as error:
        # A real signalling refusal is not permission to try another target or
        # method. Preserve it and leave cleanup unverified.
        stop["stop_error_errno"] = error.errno
    stop["leader_reaped"] = process.poll() is not None
    return stop


def execute_suite(command: list[str], environment: dict, interruption: Interruption) -> dict:
    process = subprocess.Popen(command, cwd=ROOT, env=environment, start_new_session=True)
    stop = None
    while True:
        if interruption.received_signal is not None:
            stop = stop_suite(process, interruption.received_signal)
            break
        try:
            process.wait(timeout=0.1)
            # The handler can run while wait() observes a returning leader.
            # Its descendants may still hold output; do not lose that stop.
            if interruption.received_signal is not None:
                stop = stop_suite(process, interruption.received_signal)
            break
        except subprocess.TimeoutExpired:
            continue
    result = process.returncode
    observation = {"started": True, "exit_code": result if result is not None and result >= 0 else None,
                   "signal": -result if result is not None and result < 0 else None}
    if stop is not None:
        observation["interruption_stop"] = stop
    return observation


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


def run(tests: list[str], interruption: Interruption) -> int:
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
                   "interruption_signal": None,
                   "pending_invocation": None, "observations": [],
                   "unobserved_indexes": list(range(len(tests)))}
        snapshot(path, receipt)

    # Nested harness controls must not overwrite or reuse the outer CI receipt.
    environment = {key: value for key, value in os.environ.items() if key not in RECEIPT_ENV}
    failures = []

    def interrupted():
        if interruption.received_signal is None:
            return False
        if receipt is not None:
            receipt["interruption_signal"] = interruption.received_signal
            snapshot(path, receipt)
        print(f"Script suites interrupted (signal {interruption.received_signal}); "
              "no passing verdict or remaining-suite execution.", file=sys.stderr, flush=True)
        return True

    for index, (test, name) in enumerate(zip(tests, names)):
        if interrupted():
            return 128 + interruption.received_signal
        print(f"==> {name}", flush=True)
        if receipt is not None:
            receipt["pending_invocation"] = index
            snapshot(path, receipt)
        # A signal during the pending snapshot must not admit a new child.
        if interrupted():
            return 128 + interruption.received_signal
        start = time.monotonic()
        command = ["python3", test] if test.endswith(".py") else [test]
        try:
            observation = execute_suite(command, environment, interruption)
            # Preserve the existing shell-style console summary. A numeric
            # exit 137 remains an exit, distinct from an observed SIGKILL.
            status = observation["exit_code"]
            if status is None:
                status = 128 + observation["signal"] if observation["signal"] is not None else 1
        except OSError as error:
            observation = {"started": False, "exit_code": None, "signal": None,
                           "start_error_errno": error.errno}
            status = 127 if error.errno == errno.ENOENT else 126
            print(f"Could not start {name} (errno {error.errno})", file=sys.stderr, flush=True)
        if receipt is not None:
            receipt["observations"].append({"index": index, "suite": name, **observation,
                                             "seconds": round(time.monotonic() - start, 6)})
            observed = not observation["started"] or (observation["exit_code"] is not None
                                                       or observation["signal"] is not None)
            receipt["pending_invocation"] = None if observed else index
            receipt["unobserved_indexes"] = list(range(index + int(observed), len(tests)))
            snapshot(path, receipt)
        if interrupted():
            return 128 + interruption.received_signal
        if status:
            failures.append((name, status))
            print(f"FAIL (exit {status})\n", flush=True)
        else:
            print("PASS\n", flush=True)

    if receipt is not None:
        receipt["collection_complete"] = True
        snapshot(path, receipt)
    if interrupted():
        return 128 + interruption.received_signal
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
        with Interruption() as interruption:
            result = run(sys.argv[1:], interruption)
        return 128 + interruption.received_signal if interruption.received_signal is not None else result
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        # A failed receipt cannot admit the next suite or a successful verdict.
        print(f"Script suite observation failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
