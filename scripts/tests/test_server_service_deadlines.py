#!/usr/bin/env python3
"""Real timeout/clock controls for the server installer's service boundary.

Only a private ordinary-user manager and its child run. No host service,
application binary, database, privilege change or native recovery is exercised.
"""
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh")).resolve()

MANAGER = r'''#!/usr/bin/env python3
import json, os, signal, subprocess, sys, time
from pathlib import Path
f = Path(os.environ["FIXTURE"])
args = sys.argv[1:]
mode = os.environ["MANAGER_MODE"]
with (f / "calls").open("a") as stream:
    stream.write(json.dumps(args) + "\n")
stubborn = (mode == "stubborn" or
            mode == "post-stop-stubborn" and args[0] == "show" and (f / "stopped").exists())
if stubborn:
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    child = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(60)"])
    (f / "processes").write_text(json.dumps({"group": os.getpgrp(), "pids": [os.getpid(), child.pid]}))
    child.wait()
    sys.exit(97)
if mode.startswith("slow-"):
    time.sleep(1.4)
if args[0] == "is-active":
    sys.exit(0 if mode in ("active", "slow-active") else 3)
if args[0] == "show":
    print("inactive" if (f / "stopped").exists() else "active")
elif args[0] == "stop":
    (f / "stopped").touch()
else:
    sys.exit(98)
'''


class ServiceDeadlines(unittest.TestCase):
    def exercise(self, body, mode="active", limit=8):
        with tempfile.TemporaryDirectory(prefix="pulse-service-deadline-") as directory:
            f = Path(directory)
            (f / "tools").mkdir()
            manager = f / "tools/systemctl"
            manager.write_text(MANAGER)
            manager.chmod(0o700)
            originals = {f / "saved-config": b"private synthetic state\n",
                         f / "old-executable": b"synthetic old binary\n"}
            for path, data in originals.items():
                path.write_bytes(data)
                path.chmod(0o600)
            env = {k: v for k, v in os.environ.items() if not k.startswith("PULSE_")}
            env.update(INSTALLER=str(INSTALLER), FIXTURE=str(f), MANAGER_MODE=mode,
                       PATH=str(f / "tools") + os.pathsep + env["PATH"],
                       PULSE_INSTALL_DIR=str(f / "install"), PULSE_CONFIG_DIR=str(f / "config"))
            script = 'set -euo pipefail\nsource "$INSTALLER"\n' + body
            process = subprocess.Popen(["bash", "-c", script], cwd=f, env=env,
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                       text=True, start_new_session=True)
            start = time.monotonic()
            exceeded = False
            try:
                stdout, stderr = process.communicate(timeout=limit)
            except subprocess.TimeoutExpired:
                exceeded = True
                stdout = stderr = ""
            elapsed = time.monotonic() - start
            owned = json.loads((f / "processes").read_text()) if (f / "processes").exists() else None
            # Both stubborn processes inherit these output pipes and never
            # close them. EOF before our backstop proves timeout ended that
            # fixture tree, without depending on a host /proc mount or claiming
            # that an orphaned zombie has already been reaped.
            # Always clean up this recorded, test-created group on a failure.
            if owned and exceeded:
                self.assertGreater(owned["group"], 1)
                self.assertNotEqual(owned["group"], os.getpgrp())
                try:
                    os.killpg(owned["group"], signal.SIGKILL)
                except ProcessLookupError:
                    pass
            if exceeded:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                stdout, stderr = process.communicate(timeout=3)
            self.assertEqual({p: p.read_bytes() for p in originals}, originals)
            self.assertTrue(all(p.stat().st_mode & 0o777 == 0o600 for p in originals))
            return dict(code=process.returncode, stdout=stdout, stderr=stderr,
                        exceeded=exceeded, elapsed=elapsed, owned=owned,
                        calls=[json.loads(line) for line in (f / "calls").read_text().splitlines()]
                        if (f / "calls").exists() else [])

    def assert_refused(self, result):
        self.assertFalse(result["exceeded"], result)
        self.assertEqual(result["code"], 1, result)
        self.assertNotIn("UNEXPECTED_SUCCESS", result["stdout"])

    def test_startup_budget_includes_slow_manager_reads(self):
        result = self.exercise('wait_for_service_active fixture 2\n', "slow-inactive", limit=6)
        self.assert_refused(result)
        # Bash's elapsed clock has one-second resolution. This allows that
        # granularity, not two extra five-second probes per requested second.
        self.assertLess(result["elapsed"], 3.6, result)
        self.assertLessEqual(len(result["calls"]), 2, result)

    def test_active_response_after_budget_cannot_pass(self):
        result = self.exercise('wait_for_service_active fixture 1\nprintf UNEXPECTED_SUCCESS\n',
                               "slow-active", limit=4)
        self.assert_refused(result)
        self.assertLess(result["elapsed"], 2.5, result)

    def test_startup_kills_term_ignoring_manager_and_child(self):
        result = self.exercise('wait_for_service_active fixture 1\n', "stubborn", limit=4)
        self.assert_refused(result)
        self.assertLess(result["elapsed"], 3.5, result)
        self.assertEqual(len(result["owned"]["pids"]), 2)
        self.assertEqual(result["calls"], [["is-active", "--quiet", "fixture"]])

    def test_stop_command_kills_term_ignoring_manager_and_child(self):
        result = self.exercise('safe_systemctl stop fixture\n', "stubborn")
        self.assert_refused(result)
        self.assertLess(result["elapsed"], 7.5, result)
        self.assertEqual(len(result["owned"]["pids"]), 2)
        self.assertEqual(result["calls"], [["stop", "fixture"]])

    def test_uncertain_initial_state_never_stops_or_restarts_service(self):
        result = self.exercise('stop_pulse_for_replacement fixture\nprintf UNEXPECTED_SUCCESS\n', "stubborn")
        self.assert_refused(result)
        self.assertIn("refusing binary replacement", result["stderr"])
        self.assertEqual(result["calls"], [["show", "fixture", "--property=ActiveState", "--value"]])

    def test_uncertain_post_stop_state_never_restarts_or_admits_replacement(self):
        result = self.exercise('stop_pulse_for_replacement fixture\nprintf UNEXPECTED_SUCCESS\n',
                               "post-stop-stubborn")
        self.assert_refused(result)
        self.assertIn("Could not confirm Pulse stopped", result["stderr"])
        self.assertEqual(result["calls"], [["show", "fixture", "--property=ActiveState", "--value"],
                                           ["stop", "fixture"],
                                           ["show", "fixture", "--property=ActiveState", "--value"]])

    def test_fast_active_and_confirmed_stop_still_succeed(self):
        for body in ('wait_for_service_active fixture 2\n', 'stop_pulse_for_replacement fixture\n'):
            with self.subTest(body=body):
                result = self.exercise(body)
                self.assertFalse(result["exceeded"], result)
                self.assertEqual(result["code"], 0, result)

    def test_invalid_budget_does_not_start_a_manager(self):
        for budget in ("0", "-1", "1x", "1+1"):
            with self.subTest(budget=budget):
                result = self.exercise(f'wait_for_service_active fixture "{budget}"\n')
                self.assert_refused(result)
                self.assertEqual(result["calls"], [])


if __name__ == "__main__":
    unittest.main(verbosity=2)
