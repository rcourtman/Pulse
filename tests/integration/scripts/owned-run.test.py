import importlib.util
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest

SUPERVISOR = Path(__file__).with_name('owned-run.py')
# The child creates a separate-session writer which outlives its parent and
# writes once more during TERM handling. A shell-only wait cannot contain it.
WRITER = '''
import os, signal, time
from pathlib import Path
p = Path(os.environ['PULSE_E2E_RESULTS_DIR'])
def stop(*_):
    time.sleep(.2)
    p.mkdir(parents=True, exist_ok=True)
    (p / 'late-write').write_text('fixture')
    raise SystemExit(0)
signal.signal(signal.SIGTERM, stop)
(p / 'writer-ready').write_text(str(os.getpid()))
while True: time.sleep(.02)
'''
CHILD = '''
import json, os, subprocess, sys, time
from pathlib import Path
paths = {k: os.environ[k] for k in ('PULSE_E2E_COOKIE_STATE_PATH', 'PULSE_E2E_REPORT_DIR', 'PULSE_E2E_RESULTS_DIR')}
for key, value in paths.items():
    p = Path(value)
    if 'COOKIE' in key:
        p.write_text('fixture-only')
    else:
        p.mkdir(parents=True)
        (p / 'fixture').write_text('fixture-only')
Path(sys.argv[1]).write_text(json.dumps(paths))
if sys.argv[2] in ('writer', 'stubborn'):
    writer = sys.argv[3]
    if sys.argv[2] == 'stubborn':
        writer = writer.replace('signal.signal(signal.SIGTERM, stop)', 'signal.signal(signal.SIGTERM, signal.SIG_IGN)')
    subprocess.Popen([sys.executable, '-c', writer], start_new_session=True)
    while True: time.sleep(.02)
sys.exit(int(sys.argv[2]))
'''


class OwnedRunTest(unittest.TestCase):
    def start(self, root, mode):
        receipt = root / ('receipt-' + str(time.time_ns()))
        launch = '''import importlib.util, sys
from pathlib import Path
s = importlib.util.spec_from_file_location('owned', sys.argv[1])
m = importlib.util.module_from_spec(s); s.loader.exec_module(m)
sys.exit(m.supervise([sys.executable, '-c', sys.argv[3], sys.argv[4], sys.argv[5], sys.argv[6]], Path(sys.argv[2])))
'''
        p = subprocess.Popen([sys.executable, '-c', launch, str(SUPERVISOR), str(root), CHILD, str(receipt), mode, WRITER],
                             env={**os.environ, 'PULSE_E2E_COOKIE_STATE_PATH': str(root / 'unowned')})
        self.addCleanup(lambda: p.poll() is None and p.kill())
        deadline = time.monotonic() + 10
        while not receipt.exists():
            if time.monotonic() > deadline or p.poll() is not None:
                self.fail('fixture did not start')
            time.sleep(.02)
        paths = json.loads(receipt.read_text())
        if mode in ('writer', 'stubborn'):
            ready = Path(paths['PULSE_E2E_RESULTS_DIR']) / 'writer-ready'
            while not ready.exists():
                if time.monotonic() > deadline:
                    self.fail('writer did not start')
                time.sleep(.02)
            writer_pid = int(ready.read_text())
        else:
            writer_pid = None
        return p, paths, writer_pid

    def test_normal_success_and_failure_remove_only_owned_auth(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            sentinel = root / 'unowned'; sentinel.write_text('leave alone')
            identities = []
            for code in ('0', '23'):
                p, paths, _ = self.start(root, code)
                self.assertEqual(p.wait(timeout=10), int(code))
                auth = Path(paths['PULSE_E2E_COOKIE_STATE_PATH']).parent
                identities.append(auth)
                self.assertFalse(auth.exists())
                self.assertTrue(Path(paths['PULSE_E2E_REPORT_DIR']).exists())
            self.assertNotEqual(*identities)
            self.assertEqual(sentinel.read_text(), 'leave alone')

    def test_stubborn_writer_is_killed_and_reaped(self):
        with tempfile.TemporaryDirectory() as tmp:
            p, paths, writer_pid = self.start(Path(tmp), 'stubborn')
            p.send_signal(signal.SIGTERM)
            self.assertEqual(p.wait(timeout=12), 143)
            with self.assertRaises(ProcessLookupError):
                os.kill(writer_pid, 0)
            self.assertFalse(Path(paths['PULSE_E2E_RESULTS_DIR']).exists())

    def test_signals_reap_detached_late_writer_before_removing_artifacts(self):
        for sig in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP):
            with self.subTest(signal=sig), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                other, other_paths, _ = self.start(root, 'writer')
                p, paths, writer_pid = self.start(root, 'writer')
                self.assertNotEqual(paths, other_paths)
                p.send_signal(sig)
                self.assertEqual(p.wait(timeout=10), 128 + sig)
                with self.assertRaises(ProcessLookupError):
                    os.kill(writer_pid, 0)
                for key, value in paths.items():
                    target = Path(value).parent if 'COOKIE' in key else Path(value)
                    self.assertFalse(target.exists(), target)
                for value in other_paths.values():
                    self.assertTrue(Path(value).exists(), value)
                other.send_signal(signal.SIGTERM)
                self.assertEqual(other.wait(timeout=10), 143)


if __name__ == '__main__':
    unittest.main()
