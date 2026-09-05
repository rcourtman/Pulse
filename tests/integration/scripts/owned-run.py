#!/usr/bin/env python3
"""Linux qualification supervisor: reap writers before deleting owned auth/artifacts.

Never accepts output paths from the environment. SIGKILL of this supervisor or
host failure still needs operator cleanup; catchable signals are handled here.
"""
import ctypes
import os
from pathlib import Path
import secrets
import shutil
import signal
import subprocess
import sys
import time


def supervise(command, root):
    if sys.platform != 'linux':
        raise RuntimeError('isolated integration runner requires Linux child-subreaper support')
    # Adopt orphaned browser grandchildren, so waiting for the shell is not
    # mistaken for waiting for every process which can still write artifacts.
    libc = ctypes.CDLL(None, use_errno=True)
    if libc.prctl(36, 1, 0, 0, 0) != 0:  # PR_SET_CHILD_SUBREAPER
        raise OSError(ctypes.get_errno(), 'cannot become child subreaper')

    run_id = 'pulse-e2e-' + secrets.token_hex(16)
    auth = root / 'tmp' / 'playwright-auth' / run_id
    report = root / 'playwright-report' / run_id
    results = root / 'test-results' / run_id
    auth.mkdir(parents=True, mode=0o700)
    env = {**os.environ, 'PULSE_E2E_RUN_ID': run_id,
           'PULSE_E2E_COOKIE_STATE_PATH': str(auth / 'shared-cookie-session.json'),
           'PULSE_E2E_REPORT_DIR': str(report), 'PULSE_E2E_RESULTS_DIR': str(results)}
    interrupted = 0

    def on_signal(signum, _frame):
        nonlocal interrupted
        interrupted = interrupted or signum

    for sig in (signal.SIGINT, signal.SIGTERM, signal.SIGHUP):
        signal.signal(sig, on_signal)

    child = None
    drained = False
    status = 1
    try:
        child = subprocess.Popen(command, env=env, start_new_session=True)
        stopping_at = None
        main_done = False
        sent = set()
        group_sent = set()
        while True:
            try:
                pid, raw = os.waitpid(-1, os.WNOHANG)
            except ChildProcessError:
                drained = True
                break
            if pid:
                if pid == child.pid:
                    code = os.waitstatus_to_exitcode(raw)
                    child.returncode = code
                    status = code if code >= 0 else 128 - code
                    main_done = True
                continue
            if interrupted or main_done:
                if stopping_at is None:
                    stopping_at = time.monotonic()
                elapsed = time.monotonic() - stopping_at
                sig = signal.SIGKILL if elapsed >= 5 else signal.SIGTERM
                # The group catches live descendants; adopted children also
                # cover grandchildren that started their own sessions.
                if sig not in group_sent:
                    try:
                        os.killpg(child.pid, sig)
                    except ProcessLookupError:
                        pass
                    group_sent.add(sig)
                    sent.add((child.pid, sig))
                children = Path(f'/proc/self/task/{os.getpid()}/children').read_text().split()
                for adopted in children:
                    key = (int(adopted), sig)
                    if key not in sent:
                        try:
                            os.kill(key[0], sig)
                        except ProcessLookupError:
                            pass
                        sent.add(key)
                if elapsed >= 15:
                    raise RuntimeError('writers did not exit; retaining owned artifacts for cleanup')
            time.sleep(0.05)
    finally:
        if child is None or drained:
            shutil.rmtree(auth)
            if interrupted:
                for directory in (report, results):
                    if directory.exists():
                        shutil.rmtree(directory)
        else:
            print(f'Cleanup incomplete for {run_id}; artifacts retained', file=sys.stderr)
    return 128 + interrupted if interrupted else status


if __name__ == '__main__':
    root = Path(__file__).resolve().parent.parent
    sys.exit(supervise(['bash', sys.argv[1], '--owned-run', *sys.argv[2:]], root))
