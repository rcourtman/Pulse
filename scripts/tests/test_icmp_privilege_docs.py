#!/usr/bin/env python3
"""Exercise copied ICMP-help commands without systemd or privilege changes.

Synthetic command recorders check disclosure, deadlines and failure sequencing.
They do not establish native ICMP recovery, capability grants or notification
delivery. The guide deliberately requires an operator's policy/state judgment.
"""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "### ICMP probe privileges\n"
PROPERTIES = "LoadState,ActiveState,User,NoNewPrivileges,AmbientCapabilities,CapabilityBoundingSet"
PRIVATE = "synthetic-private-environment-value"


def section():
    text = (ROOT / "docs/CONFIGURATION.md").read_text(encoding="utf-8")
    return text.split(HEADING, 1)[1].split("\n---\n", 1)[0]


def recipe(needle):
    blocks = [b for b in re.findall(r"```bash\n(.*?)```", section(), re.S)
              if needle in b]
    if len(blocks) != 1:
        raise AssertionError(f"expected one ICMP-help recipe for {needle!r}")
    return blocks[0]


SYSTEMCTL = '''#!/usr/bin/env python3
import json, os, sys, time
from pathlib import Path
args = sys.argv[1:]
root = Path(os.environ['RECORD_DIR'])
with (root / 'calls.jsonl').open('a') as f:
    f.write(json.dumps(args) + '\\n')
mode = os.environ.get('READ_MODE', 'loaded')
if args[0] == 'show':
    if mode == 'denied':
        print('Read denied', file=sys.stderr)
        sys.exit(1)
    if mode == 'hang':
        time.sleep(30)
    requested = args[-1].removeprefix('--property=').split(',')
    values = {
        'LoadState': 'not-found' if mode == 'missing' else 'loaded',
        'ActiveState': 'active', 'User': 'pulse', 'NoNewPrivileges': 'yes',
        'AmbientCapabilities': '', 'CapabilityBoundingSet': 'cap_net_raw',
    }
    if args[-1] == '--all' or not args[-1].startswith('--property='):
        print('Environment=' + os.environ['PRIVATE_VALUE'])
    for key in requested:
        print(key + '=' + values.get(key, os.environ['PRIVATE_VALUE']))
elif args[0] == 'restart':
    sys.exit(int(os.environ.get('RESTART_EXIT', '0')))
elif args[0] == 'is-active':
    print('active')
else:
    sys.exit(65)
'''


def execute(command, **overrides):
    with tempfile.TemporaryDirectory(prefix="icmp docs ") as directory:
        root = Path(directory)
        tools = root / "tools"
        tools.mkdir()
        for name, body in (("systemctl", SYSTEMCTL), ("sudo", '#!/bin/sh\nexec "$@"\n')):
            path = tools / name
            path.write_text(body, encoding="utf-8")
            path.chmod(0o700)
        env = {**os.environ, "RECORD_DIR": directory, "PRIVATE_VALUE": PRIVATE,
               "PATH": str(tools) + os.pathsep + os.environ["PATH"], **overrides}
        result = subprocess.run(["bash", "-c", command], env=env, text=True,
                                capture_output=True, timeout=12)
        record = root / "calls.jsonl"
        calls = [json.loads(line) for line in record.read_text().splitlines()] if record.exists() else []
        return result, calls


class ICMPPrivilegeDocsTest(unittest.TestCase):
    def test_install_and_privilege_claims_match_existing_source(self):
        text = " ".join(section().split())
        installer = (ROOT / "install.sh").read_text()
        for setting in ("AmbientCapabilities=CAP_NET_RAW", "CapabilityBoundingSet=CAP_NET_RAW",
                        "NoNewPrivileges=true"):
            self.assertIn(setting, text)
            self.assertIn(setting, installer)
        self.assertIn("Updates keep an existing unit file untouched", installer)
        for boundary in ("Updates preserve an existing service unit", "not a reliable unit repair",
                         "An ambient line alone cannot overcome the ceiling",
                         "Preserve that restriction", "Do not reset or widen",
                         "set file capabilities on the system-wide ping binary",
                         "do not run Pulse as root"):
            self.assertIn(boundary, text)
        self.assertNotIn("Docker installs are unaffected", text)
        self.assertNotIn("it rewrites the unit", text)

    def test_location_state_and_recovery_are_not_inferred(self):
        text = " ".join(section().split())
        for boundary in ("selected observation host", "not evidence that the target is down",
                         "cannot repair an agent's permissions", "inside the Pulse LXC",
                         "not pulse-agent.service", "failed or timed-out read is **unknown**",
                         "Keep deliberately inactive services inactive", "not the running process",
                         "monitoring and alerts are unavailable", "same location",
                         "not ICMP recovery or notification delivery", "not repeated restarts",
                         "do not use `systemctl revert`", "preserve other drop-in content",
                         "preserving data and configuration", "Do not use `--privileged`",
                         "--cap-add=ALL", "not proof that ICMP recovered"):
            self.assertIn(boundary, text)
        # The installer-independent probe executes on either server or agent.
        probe = (ROOT / "internal/availabilityprobe/probe.go").read_text()
        self.assertIn('exec.CommandContext(ctx, "ping", args...)', probe)

    def test_copied_read_is_bounded_allowlisted_and_read_only(self):
        command = recipe("systemctl show")
        self.assertIn("timeout --signal=TERM --kill-after=1s 8s", command)
        self.assertIn("--property=" + PROPERTIES, command)
        self.assertNotIn("sudo", command)
        for mode in ("loaded", "missing"):
            with self.subTest(mode=mode):
                result, calls = execute(command, READ_MODE=mode)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(calls, [["show", "pulse.service", "--property=" + PROPERTIES]])
                self.assertNotIn(PRIVATE, result.stdout + result.stderr)
                self.assertEqual(set(line.split("=", 1)[0] for line in result.stdout.splitlines()),
                                 set(PROPERTIES.split(",")))
                self.assertIn("LoadState=" + ("not-found" if mode == "missing" else "loaded"),
                              result.stdout)

    def test_failed_or_hung_read_has_no_mutation_or_retry(self):
        for mode, code in (("denied", 1), ("hang", 124)):
            with self.subTest(mode=mode):
                result, calls = execute(recipe("systemctl show"), READ_MODE=mode)
                self.assertEqual(result.returncode, code, result.stderr)
                self.assertEqual(calls, [["show", "pulse.service", "--property=" + PROPERTIES]])
                self.assertNotIn(PRIVATE, result.stdout + result.stderr)

    def test_copied_restart_failure_stops_before_readback(self):
        command = recipe("systemctl restart")
        for code in (0, 5):
            with self.subTest(restart_exit=code):
                result, calls = execute(command, RESTART_EXIT=str(code))
                self.assertEqual(result.returncode, code, result.stderr)
                expected = [["restart", "pulse.service"]]
                if code == 0:
                    expected.append(["is-active", "pulse.service"])
                self.assertEqual(calls, expected)
                self.assertNotIn(PRIVATE, result.stdout + result.stderr)
                self.assertEqual(result.stdout, "active\n" if code == 0 else "")

    def test_copied_commands_parse_and_mirror_matches(self):
        blocks = re.findall(r"```bash\n(.*?)```", section(), re.S)
        self.assertEqual(len(blocks), 4)
        for command in blocks:
            subprocess.run(["bash", "-n", "-c", command], check=True, capture_output=True)
        self.assertIn("INSTALL.md#2-bare-metal--systemd", section())
        self.assertEqual((ROOT / "docs/CONFIGURATION.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/CONFIGURATION.md").read_bytes())


if __name__ == "__main__":
    unittest.main()
