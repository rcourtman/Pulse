#!/usr/bin/env python3
"""Run every native installer check with an independent Go process deadline.

Compile once and discover from that exact binary. Inventory growth must not
consume another check's ten-minute Go alarm. Fixture, transport, cleanup and
workflow job deadlines remain unchanged. The NAS boot-owner preflight runs
before frontend preparation; the remaining phase requires that identity too
and excludes only the check already executed by preflight. Run at most two
independent processes together, retaining complete, discovery-ordered output.
Refill a free slot without waiting for its sibling. An observed failure stops
admission and drains every already-started check before returning failure.
Observe process exit independently of inherited output descriptors; a timed-out
test's child must not keep a proof slot waiting for pipe EOF.
"""

import argparse
from concurrent.futures import FIRST_COMPLETED, ThreadPoolExecutor, wait
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[2]
PACKAGE = "./scripts/installtests"
PREFLIGHT = ("TestInstallSHNASBootOwnersQuiesceBeforeRecoveryErasure",)
MAX_CHECKS = 2
PROCESS_EXIT_GRACE_SECONDS = 5


def process_deadline(timeout):
    # This internal interface uses Go duration strings. Keep Go's own alarm
    # unchanged, with a separate small allowance to observe its terminal output
    # and exit. Never admit an unbounded (zero) or malformed duration.
    parts = re.findall(r"(\d+(?:\.\d+)?)(ms|s|m|h)", timeout)
    if not parts or "".join(value + unit for value, unit in parts) != timeout:
        raise ValueError("invalid native installer check timeout")
    seconds = sum(float(value) * {"ms": .001, "s": 1, "m": 60, "h": 3600}[unit]
                  for value, unit in parts)
    if not 0 < seconds < float("inf"):
        raise ValueError("native installer check timeout must be finite and positive")
    return seconds + PROCESS_EXIT_GRACE_SECONDS


def partition(output):
    names = []
    for name in output.splitlines():
        if not name.startswith(("Test", "Example", "Fuzz")):
            continue
        if not name.isidentifier() or name in names:
            raise ValueError("invalid or repeated Go test identity")
        names.append(name)
    if (not set(PREFLIGHT).issubset(names)
            or sum(name.startswith("TestRootInstallReset") for name in names) < 1
            or sum(name not in PREFLIGHT and not name.startswith("TestRootInstallReset")
                   for name in names) < 2):
        raise ValueError("missing preflight, general or reset installer inventory")
    # Preserve the compiled binary's order within each workflow phase. Every
    # discovered test, example and fuzz seed belongs to exactly one phase.
    return ([name for name in names if name in PREFLIGHT],
            [name for name in names if name not in PREFLIGHT])


def run_check(binary, name, *, cwd, timeout="10m", output=None):
    command = [str(binary), "-test.count=1", "-test.timeout=" + timeout,
               "-test.paniconexit0", "-test.v", "-test.run=^" + name + "$"]
    deadline = process_deadline(timeout)
    completed = False
    descendants = False
    # A pipe can stay open after the Go process exits, even after its alarm has
    # fired. Spool to disk instead: wait observes that exact process, not EOF
    # from an inherited descriptor. No output-sized in-memory capture.
    with tempfile.TemporaryFile(mode="w+", encoding="utf-8") as log:
        # Do not use Popen's context manager: its implicit unbounded wait would
        # defeat the explicit cleanup bound if observing stop itself failed.
        process = subprocess.Popen(command, cwd=cwd, stdout=log,
                                   stderr=subprocess.STDOUT, start_new_session=True)
        try:
            try:
                code = process.wait(timeout=deadline)
            finally:
                # The new session owns only this check's process group, not
                # its concurrent sibling or the caller. Also retire children
                # left after a normal Go exit before reading the shared log.
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                    descendants = True
                except ProcessLookupError:
                    pass
                process.wait(timeout=PROCESS_EXIT_GRACE_SECONDS)
        finally:
            log.seek(0)
            for line in log:
                print(line, end="", file=output, flush=output is None)
                if re.match(r"^--- (PASS|SKIP): " + re.escape(name) + r"(?:\s|$)", line):
                    completed = True
    if code:
        raise subprocess.CalledProcessError(code, command)
    if descendants:
        raise RuntimeError("native installer check left an owned process group: " + name)
    if not completed:
        raise RuntimeError("native installer check did not finish: " + name)


def run_checks(binary, names, *, cwd):
    # Separate processes retain each check's Go alarm and process-local state
    # (including cwd/environment). Do not combine roots into one Go invocation
    # or overlap workflow phases. Disk-backed logs keep simultaneous Go streams
    # separate. Open them only while running or emitting, so a slow first check
    # cannot retain one descriptor per later completed check.
    with tempfile.TemporaryDirectory(prefix="pulse-native-check-logs-") as directory, \
            ThreadPoolExecutor(max_workers=MAX_CHECKS) as executor:
        def logged_check(name, path):
            with path.open("w", encoding="utf-8") as output:
                run_check(binary, name, cwd=cwd, output=output)

        running, completed, failures = {}, {}, {}
        admitted, emitted = 0, 0
        while running or (admitted < len(names) and not failures):
            # Never enqueue the whole inventory: at most MAX_CHECKS roots are
            # admitted, and no new root starts after a failure is observed.
            while not failures and admitted < len(names) and len(running) < MAX_CHECKS:
                name = names[admitted]
                path = Path(directory) / f"{admitted}.log"
                print(f"Native Unix installer {admitted + 1}/{len(names)}: {name}", flush=True)
                future = executor.submit(logged_check, name, path)
                running[future] = (admitted, name, path)
                admitted += 1
            wait(running, return_when=FIRST_COMPLETED)
            # Inspect every completed result, not just the first returned one.
            # A fast later failure must stop admission even while an earlier
            # check still owns the next discovery-ordered output position.
            for future in list(running):
                if not future.done():
                    continue
                index, name, path = running.pop(future)
                try:
                    future.result()
                except Exception as error:
                    failures[index] = error
                    print(f"Native Unix installer check failed: {name}: {error}", file=sys.stderr)
                completed[index] = path
            while emitted in completed:
                path = completed.pop(emitted)
                with path.open(encoding="utf-8") as log:
                    shutil.copyfileobj(log, sys.stdout)
                path.unlink()
                sys.stdout.flush()
                emitted += 1
        if failures:
            raise failures[min(failures)]


def main(phase="all"):
    if phase not in ("all", "preflight", "remaining"):
        raise ValueError("unknown native installer phase")
    with tempfile.TemporaryDirectory(prefix="pulse-native-contracts-") as directory:
        binary = Path(directory) / "installtests.test"
        subprocess.run(["go", "test", "-c", "-o", str(binary), PACKAGE],
                       cwd=ROOT, check=True)
        inventory = subprocess.run([str(binary), "-test.list=."], cwd=ROOT / "scripts/installtests",
                                   check=True, capture_output=True, text=True)
        early, remaining = partition(inventory.stdout)
        selected = early if phase == "preflight" else remaining if phase == "remaining" else early + remaining
        print(f"Native Unix installer {phase}: {len(selected)} discovered checks", flush=True)
        if phase == "all":
            run_checks(binary, early, cwd=ROOT / "scripts/installtests")
            run_checks(binary, remaining, cwd=ROOT / "scripts/installtests")
        else:
            run_checks(binary, selected, cwd=ROOT / "scripts/installtests")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--phase", choices=("all", "preflight", "remaining"), default="all")
    main(parser.parse_args().phase)
