#!/usr/bin/env python3
"""Execute the shell runner's selected suites and retain returned observations.

Selection belongs to scripts/tests/run.sh. Optional CI receipts contain source
identifiers and process results only, never suite output. A pending invocation
does not establish execution or liveness, and incomplete collection is not a pass.
"""

from __future__ import annotations

from decimal import Decimal, InvalidOperation
import errno
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time


ROOT = Path(__file__).resolve().parents[1]
RECEIPT_ENV = ("PULSE_SCRIPT_TEST_RECEIPT", "PULSE_SCRIPT_TEST_SOURCE_SHA")
STOP_GRACE_SECONDS = 2
MAX_WORKERS = 16


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


def _suite_name_valid(name: str) -> bool:
    return Path(name).name == name and (
        name.startswith("test_") and name.endswith(".py")
        or name.startswith("test-") and name.endswith(".sh")
    )


def read_weights(path: Path) -> tuple[Decimal, dict[str, Decimal]]:
    default = None
    weights: dict[str, Decimal] = {}
    for number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw.split("#", 1)[0].strip()
        if not line:
            continue
        fields = line.split()
        if len(fields) != 2:
            raise ValueError(f"invalid timing hint at line {number}")
        name, value = fields
        try:
            seconds = Decimal(value)
        except InvalidOperation as error:
            raise ValueError(f"invalid timing value at line {number}") from error
        if not seconds.is_finite() or seconds <= 0:
            raise ValueError(f"timing hint must be finite and positive at line {number}")
        if name == "DEFAULT_WEIGHT":
            if default is not None:
                raise ValueError("duplicate DEFAULT_WEIGHT")
            default = seconds
        else:
            if not _suite_name_valid(name) or name in weights:
                raise ValueError(f"invalid or duplicate suite timing at line {number}")
            weights[name] = seconds
    if default is None:
        raise ValueError("missing DEFAULT_WEIGHT")
    return default, weights


def parse_args(argv: list[str]) -> tuple[int, Path | None, list[str]]:
    workers = 1
    weights_path: Path | None = None
    index = 0
    while index < len(argv):
        token = argv[index]
        if token == "--":
            index += 1
            break
        if token == "--workers":
            if index + 1 >= len(argv):
                raise ValueError("--workers requires a positive integer")
            raw_workers = argv[index + 1]
            if not re.fullmatch(r"[1-9][0-9]*", raw_workers):
                raise ValueError(f"invalid --workers value: {raw_workers}")
            workers = int(raw_workers)
            if not 1 <= workers <= MAX_WORKERS:
                raise ValueError(f"--workers must be between 1 and {MAX_WORKERS}")
            index += 2
            continue
        if token == "--weights":
            if index + 1 >= len(argv) or not argv[index + 1]:
                raise ValueError("--weights requires a path")
            weights_path = Path(argv[index + 1])
            index += 2
            continue
        if token.startswith("--"):
            raise ValueError(f"unknown option: {token}")
        break
    return workers, weights_path, argv[index:]


def _is_observed(observation: dict) -> bool:
    return not observation["started"] or (
        observation["exit_code"] is not None or observation["signal"] is not None
    )


def _status_from_observation(observation: dict) -> int:
    if not observation["started"]:
        return 127 if observation.get("start_error_errno") == errno.ENOENT else 126
    status = observation["exit_code"]
    if status is None:
        return 128 + observation["signal"] if observation["signal"] is not None else 1
    return status


def _replay_stream(path: Path, destination) -> None:
    if not path.is_file():
        return
    with path.open("r", encoding="utf-8", errors="replace") as stream:
        shutil.copyfileobj(stream, destination)
    destination.flush()


def stop_suites(active: dict[int, dict], number: int) -> dict[int, dict]:
    """Stop every active suite's owned process group within one shared grace window."""
    stops: dict[int, dict] = {}
    signallable: list[int] = []

    def send(pid: int, signum: int, stop: dict) -> None:
        try:
            os.killpg(pid, signum)
            stop["sent_signals"].append(signum)
        except ProcessLookupError:
            pass

    def group_present(pid: int) -> bool:
        try:
            os.killpg(pid, 0)
            return True
        except ProcessLookupError:
            return False

    for idx, entry in active.items():
        stop = {"requested_signal": number, "sent_signals": [], "leader_reaped": False,
                "process_group_absent": None}
        stops[idx] = stop
        try:
            send(entry["process"].pid, number, stop)
            signallable.append(idx)
        except OSError as error:
            stop["stop_error_errno"] = error.errno

    deadline = time.monotonic() + STOP_GRACE_SECONDS
    while signallable:
        any_present = False
        for idx in list(signallable):
            process = active[idx]["process"]
            process.poll()
            try:
                if group_present(process.pid):
                    any_present = True
            except OSError as error:
                stops[idx]["stop_error_errno"] = error.errno
                signallable.remove(idx)
        if not any_present or time.monotonic() >= deadline:
            break
        time.sleep(0.05)

    for idx in list(signallable):
        process = active[idx]["process"]
        try:
            if group_present(process.pid):
                send(process.pid, signal.SIGKILL, stops[idx])
        except OSError as error:
            stops[idx]["stop_error_errno"] = error.errno
            signallable.remove(idx)

    wait_deadline = time.monotonic() + STOP_GRACE_SECONDS
    for idx in signallable:
        process = active[idx]["process"]
        remaining = max(0.0, wait_deadline - time.monotonic())
        try:
            process.wait(timeout=remaining)
        except subprocess.TimeoutExpired:
            pass
        try:
            stops[idx]["process_group_absent"] = not group_present(process.pid)
        except OSError as error:
            stops[idx]["stop_error_errno"] = error.errno

    for idx, entry in active.items():
        stops[idx]["leader_reaped"] = entry["process"].poll() is not None
    return stops


def _run_concurrent(
    tests: list[str],
    names: list[str],
    interruption: Interruption,
    *,
    workers: int,
    schedule: list[int],
    path: Path | None,
    receipt: dict | None,
    environment: dict[str, str],
) -> int:
    observations: dict[int, dict] = {}
    active: dict[int, dict] = {}
    pending_indexes: set[int] = set()
    failures: dict[int, tuple[str, int]] = {}
    queue = list(schedule)

    def publish(complete: bool = False) -> None:
        if receipt is None or path is None:
            return
        pending_list = sorted(pending_indexes)
        receipt["pending_invocation"] = pending_list[0] if pending_list else None
        receipt["pending_invocations"] = pending_list
        receipt["observations"] = [observations[i] for i in sorted(observations)]
        receipt["unobserved_indexes"] = [
            i for i in range(len(tests))
            if i not in observations or not _is_observed(observations[i])
        ]
        receipt["collection_complete"] = complete
        snapshot(path, receipt)

    def finish_interruption() -> int:
        if active:
            stops = stop_suites(active, interruption.received_signal)
            for idx in sorted(active):
                entry = active[idx]
                entry["stdout_stream"].close()
                entry["stderr_stream"].close()
                result = entry["process"].returncode
                observation = {
                    "started": True,
                    "exit_code": result if result is not None and result >= 0 else None,
                    "signal": -result if result is not None and result < 0 else None,
                    "interruption_stop": stops[idx],
                }
                observations[idx] = {
                    "index": idx,
                    "suite": names[idx],
                    **observation,
                    "seconds": round(time.monotonic() - entry["start"], 6),
                }
                if _is_observed(observation):
                    pending_indexes.discard(idx)
            active.clear()
        if receipt is not None and path is not None:
            receipt["interruption_signal"] = interruption.received_signal
            publish(complete=False)
        print(f"Script suites interrupted (signal {interruption.received_signal}); "
              "no passing verdict or remaining-suite execution.", file=sys.stderr, flush=True)
        return 128 + interruption.received_signal

    with tempfile.TemporaryDirectory(prefix="pulse-script-suites-") as temporary:
        log_dir = Path(temporary)
        try:
            while queue or active:
                if interruption.received_signal is not None:
                    return finish_interruption()

                while len(active) < workers and queue and interruption.received_signal is None:
                    idx = queue.pop(0)
                    test = tests[idx]
                    name = names[idx]
                    pending_indexes.add(idx)
                    publish(complete=False)
                    if interruption.received_signal is not None:
                        return finish_interruption()
                    start = time.monotonic()
                    command = ["python3", test] if test.endswith(".py") else [test]
                    stdout_path = log_dir / f"suite-{idx}.out"
                    stderr_path = log_dir / f"suite-{idx}.err"
                    stdout_stream = stdout_path.open("wb")
                    stderr_stream = stderr_path.open("wb")
                    try:
                        process = subprocess.Popen(
                            command,
                            cwd=ROOT,
                            env=environment,
                            stdout=stdout_stream,
                            stderr=stderr_stream,
                            start_new_session=True,
                        )
                    except OSError as error:
                        stdout_stream.close()
                        stderr_stream.close()
                        stdout_path.unlink(missing_ok=True)
                        stderr_path.unlink(missing_ok=True)
                        observation = {"started": False, "exit_code": None, "signal": None,
                                       "start_error_errno": error.errno}
                        status = _status_from_observation(observation)
                        observations[idx] = {
                            "index": idx, "suite": name, **observation,
                            "seconds": round(time.monotonic() - start, 6),
                        }
                        pending_indexes.discard(idx)
                        publish(complete=False)
                        print(f"==> {name}", flush=True)
                        print(f"Could not start {name} (errno {error.errno})",
                              file=sys.stderr, flush=True)
                        failures[idx] = (name, status)
                        print(f"FAIL (exit {status})\n", flush=True)
                        continue
                    active[idx] = {
                        "process": process,
                        "start": start,
                        "stdout_path": stdout_path,
                        "stderr_path": stderr_path,
                        "stdout_stream": stdout_stream,
                        "stderr_stream": stderr_stream,
                    }

                if interruption.received_signal is not None:
                    return finish_interruption()

                completed = [idx for idx in sorted(active) if active[idx]["process"].poll() is not None]
                if interruption.received_signal is not None:
                    return finish_interruption()

                if not completed:
                    time.sleep(0.05)
                    continue

                for idx in completed:
                    entry = active.pop(idx)
                    entry["stdout_stream"].close()
                    entry["stderr_stream"].close()
                    result = entry["process"].returncode
                    observation = {
                        "started": True,
                        "exit_code": result if result is not None and result >= 0 else None,
                        "signal": -result if result is not None and result < 0 else None,
                    }
                    observations[idx] = {
                        "index": idx,
                        "suite": names[idx],
                        **observation,
                        "seconds": round(time.monotonic() - entry["start"], 6),
                    }
                    if _is_observed(observation):
                        pending_indexes.discard(idx)
                    publish(complete=False)

                    name = names[idx]
                    status = _status_from_observation(observation)
                    print(f"==> {name}", flush=True)
                    _replay_stream(entry["stdout_path"], sys.stdout)
                    _replay_stream(entry["stderr_path"], sys.stderr)
                    entry["stdout_path"].unlink(missing_ok=True)
                    entry["stderr_path"].unlink(missing_ok=True)
                    if status:
                        failures[idx] = (name, status)
                        print(f"FAIL (exit {status})\n", flush=True)
                    else:
                        print("PASS\n", flush=True)
        finally:
            if active:
                stop_suites(active, signal.SIGTERM)
            for entry in active.values():
                entry["stdout_stream"].close()
                entry["stderr_stream"].close()

    publish(complete=True)
    if interruption.received_signal is not None:
        return finish_interruption()
    print(f"Summary: {len(tests) - len(failures)}/{len(tests)} passed", flush=True)
    if failures:
        print(f"Failures: {len(failures)}\nFailed suites:", flush=True)
        for idx in sorted(failures):
            name, status = failures[idx]
            print(f"  {name} (exit {status})", flush=True)
    return int(bool(failures))


def run(
    tests: list[str],
    interruption: Interruption,
    *,
    workers: int = 1,
    weights_path: Path | None = None,
) -> int:
    if not 1 <= workers <= MAX_WORKERS:
        raise ValueError(f"workers must be between 1 and {MAX_WORKERS}")
    schedule = list(range(len(tests)))
    if weights_path is not None:
        default_weight, weights = read_weights(weights_path)
        if workers > 1:
            schedule = sorted(
                schedule,
                key=lambda idx: (-weights.get(Path(tests[idx]).name, default_weight), idx),
            )

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
        if workers > 1:
            receipt["workers"] = workers
            receipt["pending_invocations"] = []
        snapshot(path, receipt)

    # Nested harness controls must not overwrite or reuse the outer CI receipt.
    environment = {key: value for key, value in os.environ.items() if key not in RECEIPT_ENV}
    if workers > 1:
        return _run_concurrent(
            tests,
            names,
            interruption,
            workers=workers,
            schedule=schedule,
            path=path,
            receipt=receipt,
            environment=environment,
        )

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
    try:
        workers, weights_path, tests = parse_args(sys.argv[1:])
    except ValueError as error:
        print(f"Script suite observation failed: {error}", file=sys.stderr)
        return 1
    if not tests:
        print("No selected script suites; no execution or coverage verdict", file=sys.stderr)
        return 1
    try:
        with Interruption() as interruption:
            result = run(tests, interruption, workers=workers, weights_path=weights_path)
        return 128 + interruption.received_signal if interruption.received_signal is not None else result
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        # A failed receipt cannot admit the next suite or a successful verdict.
        print(f"Script suite observation failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
