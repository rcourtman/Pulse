#!/usr/bin/env python3
"""Run every native installer check with an independent Go process deadline.

Compile once and discover from that exact binary. Inventory growth must not
consume another check's ten-minute Go alarm. Fixture, transport, cleanup and
workflow job deadlines remain unchanged. The NAS boot-owner preflight runs
before frontend preparation; the remaining phase requires that identity too
and excludes only the check already executed by preflight. Run at most two
independent processes together, retaining complete, discovery-ordered output.
A failed wave drains both checks and starts no subsequent wave.
"""

import argparse
from concurrent.futures import ThreadPoolExecutor
from contextlib import ExitStack
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[2]
PACKAGE = "./scripts/installtests"
PREFLIGHT = ("TestInstallSHNASBootOwnersQuiesceBeforeRecoveryErasure",)
CHECKS_PER_WAVE = 2


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
    completed = False
    with subprocess.Popen(command, cwd=cwd, stdout=subprocess.PIPE,
                          stderr=subprocess.STDOUT, text=True) as process:
        for line in process.stdout:
            print(line, end="", file=output, flush=output is None)
            if re.match(r"^--- (PASS|SKIP): " + re.escape(name) + r"(?:\s|$)", line):
                completed = True
        code = process.wait()
    if code:
        raise subprocess.CalledProcessError(code, command)
    if not completed:
        raise RuntimeError("native installer check did not finish: " + name)


def run_checks(binary, names, *, cwd):
    # Separate processes retain each check's Go alarm and process-local state
    # (including cwd/environment). Do not combine roots into one Go invocation
    # or overlap workflow phases. Disk-backed logs bound parent memory and keep
    # simultaneous Go streams from corrupting one another's result identities.
    with ThreadPoolExecutor(max_workers=CHECKS_PER_WAVE) as executor:
        for start in range(0, len(names), CHECKS_PER_WAVE):
            wave = names[start:start + CHECKS_PER_WAVE]
            with ExitStack() as stack:
                logs = [stack.enter_context(tempfile.TemporaryFile(mode="w+t", encoding="utf-8"))
                        for _ in wave]
                for index, name in enumerate(wave, start + 1):
                    print(f"Native Unix installer {index}/{len(names)}: {name}", flush=True)
                futures = [executor.submit(run_check, binary, name, cwd=cwd, output=log)
                           for name, log in zip(wave, logs)]
                failures = []
                # Observe every already-started check before closing its log or
                # raising. A sibling's pass never erases an earlier failure.
                for name, future, log in zip(wave, futures, logs):
                    try:
                        future.result()
                    except Exception as error:
                        failures.append(error)
                        print(f"Native Unix installer check failed: {name}: {error}", file=sys.stderr)
                    finally:
                        log.seek(0)
                        shutil.copyfileobj(log, sys.stdout)
                        sys.stdout.flush()
                if failures:
                    raise failures[0]


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
