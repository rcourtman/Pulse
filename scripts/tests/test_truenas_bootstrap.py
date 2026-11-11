#!/usr/bin/env python3
"""Execute the shipped TrueNAS bootstrap with confined service-manager doubles.

Only fixed system paths are relocated. The production writer and its generated
script both run, without sourcing the installer main or touching a real service.
These controls do not establish native FreeBSD or original-appliance recovery.
"""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = ROOT / "scripts/install.sh"

MANAGER = r'''
import json, os, sys
from pathlib import Path
root = Path(os.environ["FIXTURE"])
args = sys.argv[1:]
with (root / "calls").open("a") as output:
    output.write(json.dumps([Path(sys.argv[0]).name, *args]) + "\n")
fault = os.environ["FAULT"]
if Path(sys.argv[0]).name == "systemctl":
    if args == ["daemon-reload"] or args == ["enable", "pulse-agent"]:
        sys.exit(0)
    if args == ["restart", "pulse-agent"]:
        sys.exit(23 if fault == "start" else 0)
    sys.exit(97)
if len(args) != 2 or args[0] != "pulse-agent":
    sys.exit(97)
action = args[1]
if action == "stop":
    sys.exit(21 if fault == "stop" else 0)
if action == "start":
    if fault == "start":
        print("fixture start refused", file=sys.stderr)
        sys.exit(23)
    sys.exit(0)
if action == "status":
    count = root / "status-count"
    observed = int(count.read_text()) + 1 if count.exists() else 1
    count.write_text(str(observed))
    if fault == "inactive" or (fault == "delayed" and observed < 3):
        sys.exit(1)
    if fault == "status-error":
        sys.exit(24)
    sys.exit(0)
sys.exit(97)
'''


class TrueNASBootstrapTest(unittest.TestCase):
    def exercise(self, platform="FreeBSD", fault="", *, separate_runtime=False,
                 missing="", enabled=False):
        with tempfile.TemporaryDirectory(prefix="pulse-truenas-bootstrap-") as directory:
            root = Path(directory)
            for name in ("tools", "state", "runtime", "units", "rc"):
                (root / name).mkdir()
            binary = root / "state/pulse-agent"
            binary.write_bytes(b"synthetic saved agent\n")
            binary.chmod(0o700)
            service = root / "state/pulse-agent.service"
            service.write_text("synthetic saved service\n")
            saved = root / "state/token"
            saved.write_text("synthetic saved credential\n")
            saved.chmod(0o600)
            rc = root / "rc.conf"
            original_rc = ('pulse_agent_enable="YES"\n' if enabled
                           else 'unrelated_enable="YES"\npulse_agent_enable="NO"\n')
            if fault == "enable":
                rc.mkdir()  # Neither append nor either sed variant can publish here.
            else:
                rc.write_text(original_rc)
            if missing:
                {"binary": binary, "service": service}[missing].unlink()
            before = {path.name: (path.read_bytes(), path.stat().st_mode)
                      for path in (binary, service, saved) if path.exists()}
            for name in ("service", "systemctl"):
                path = root / "tools" / name
                path.write_text(f"#!{sys.executable}\n" + MANAGER)
                path.chmod(0o700)
            # Retain the exact requested delay without making this inert manager
            # fixture wait. No production deadline or startup count is changed.
            sleep = root / "tools/sleep"
            sleep.write_text('#!/bin/sh\nprintf \'%s\\n\' "$*" >> "$FIXTURE/sleeps"\n')
            sleep.chmod(0o700)

            text = INSTALLER.read_text()
            start = text.index("write_truenas_bootstrap_script() {")
            end = text.index("render_privileged_helper_socket_unit() {", start)
            writer = text[start:end]
            for original, relocated in (
                ("/etc/systemd/system/${AGENT_NAME}.service", str(root / "units/${AGENT_NAME}.service")),
                ("/usr/local/etc/rc.d/${AGENT_NAME}", str(root / "rc/${AGENT_NAME}")),
                ("/etc/rc.conf", str(rc)),
            ):
                self.assertIn(original, writer)
                writer = writer.replace(original, relocated)
            script = root / "state/bootstrap-pulse-agent.sh"
            runtime = root / "runtime/pulse-agent" if separate_runtime else binary
            harness = root / "writer.sh"
            harness.write_text("set -euo pipefail\n" + writer + r'''
AGENT_NAME=pulse-agent
TRUENAS_STATE_DIR="$FIXTURE/state"
TRUENAS_BOOTSTRAP_SCRIPT="$TRUENAS_STATE_DIR/bootstrap-pulse-agent.sh"
TRUENAS_RUNTIME_BINARY="$RUNTIME_BINARY"
write_truenas_bootstrap_script "$PLATFORM"
''')
            env = {key: value for key, value in os.environ.items()
                   if not key.startswith(("PULSE_", "BASH_FUNC_"))
                   and key not in ("BASH_ENV", "ENV")}
            env.update(FIXTURE=str(root), PLATFORM=platform, FAULT=fault,
                       RUNTIME_BINARY=str(runtime),
                       PATH=str(root / "tools") + os.pathsep + os.environ["PATH"])
            rendered = subprocess.run(["bash", str(harness)], env=env, cwd=root,
                                      capture_output=True, text=True, timeout=10)
            self.assertEqual(rendered.returncode, 0, rendered.stdout + rendered.stderr)
            result = subprocess.run(["bash", str(script)], env=env, cwd=root,
                                    capture_output=True, text=True, timeout=10)
            calls = ([json.loads(line) for line in (root / "calls").read_text().splitlines()]
                     if (root / "calls").exists() else [])
            after = {path.name: (path.read_bytes(), path.stat().st_mode)
                     for path in (binary, service, saved) if path.exists()}
            self.assertEqual(before, after, "bootstrap changed persistent agent/configuration")
            link = root / ("units/pulse-agent.service" if platform == "Linux" else "rc/pulse-agent")
            if not missing:
                self.assertTrue(link.is_symlink())
                self.assertEqual(link.readlink(), service)
            return dict(result=result, calls=calls,
                        rc=rc.read_text() if rc.is_file() else None,
                        runtime=runtime.read_bytes() if runtime.is_file() else None,
                        sleeps=(root / "sleeps").read_text().splitlines()
                        if (root / "sleeps").exists() else [])

    def test_core_bootstrap_is_self_contained_and_confirms_running(self):
        for separate_runtime in (False, True):
            for enabled in (False, True):
                with self.subTest(separate_runtime=separate_runtime, enabled=enabled):
                    evidence = self.exercise(separate_runtime=separate_runtime, enabled=enabled)
                    result = evidence["result"]
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertIn("Pulse agent started successfully", result.stdout)
                    self.assertEqual(evidence["calls"], [
                        ["service", "pulse-agent", action] for action in ("stop", "start", "status")])
                    self.assertIn('pulse_agent_enable="YES"', evidence["rc"])
                    if not enabled:
                        self.assertIn('unrelated_enable="YES"', evidence["rc"])
                    self.assertEqual(evidence["runtime"], b"synthetic saved agent\n")

    def test_core_failed_phases_cannot_claim_startup_or_continue(self):
        expected = {"enable": [], "stop": ["stop"], "start": ["stop", "start"],
                    "inactive": ["stop", "start", *(["status"] * 5)],
                    "status-error": ["stop", "start", *(["status"] * 5)]}
        for fault, actions in expected.items():
            with self.subTest(fault=fault):
                evidence = self.exercise(fault=fault)
                result = evidence["result"]
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertNotIn("Pulse agent started successfully", result.stdout)
                self.assertIn("ERROR:", result.stderr)
                self.assertEqual(evidence["calls"], [["service", "pulse-agent", action]
                                                     for action in actions])
                if fault == "start":
                    self.assertIn("fixture start refused", result.stderr)

    def test_core_delayed_status_has_a_finite_grace_period(self):
        evidence = self.exercise(fault="delayed")
        result = evidence["result"]
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual([call[-1] for call in evidence["calls"]],
                         ["stop", "start", "status", "status", "status"])
        self.assertEqual(evidence["sleeps"], ["1", "1", "1"])

    def test_missing_persistent_inputs_stop_before_manager_mutation(self):
        for platform in ("FreeBSD", "Linux"):
            for missing in ("binary", "service"):
                with self.subTest(platform=platform, missing=missing):
                    evidence = self.exercise(platform=platform, missing=missing)
                    self.assertNotEqual(evidence["result"].returncode, 0)
                    self.assertIn("not found", evidence["result"].stdout)
                    self.assertEqual(evidence["calls"], [])

    def test_scale_restart_success_and_failure_still_propagate(self):
        for fault in ("", "start"):
            with self.subTest(fault=fault):
                evidence = self.exercise(platform="Linux", fault=fault, separate_runtime=True)
                result = evidence["result"]
                self.assertEqual(result.returncode, 23 if fault else 0, result.stdout + result.stderr)
                self.assertEqual("Pulse agent started successfully" in result.stdout, not bool(fault))
                self.assertEqual(evidence["calls"], [["systemctl", "daemon-reload"],
                                 ["systemctl", "enable", "pulse-agent"],
                                 ["systemctl", "restart", "pulse-agent"]])


if __name__ == "__main__":
    unittest.main()
