#!/usr/bin/env python3
"""Exercise the actual passive helper with synthetic Proxmox commands.

No hypervisor, guest, backup or host service is touched. These controls prove
command selection, failure handling and private output, not native thaw.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts/test-vm-disk.sh"
DOC = ROOT / "docs/VM_DISK_MONITORING.md"
BASH = shutil.which("bash")
REAL_TIMEOUT = shutil.which("timeout")
PRIVATE = "synthetic-private-infrastructure-marker"
READS = [["status", "100"], ["config", "100", "--current"]]

# Record every attempted operation, including prohibited ones. The timeout
# adapter checks the helper's real deadline arguments; one test uses real
# coreutils timeout around the synthetic blocked qm child.
TOOL = '''#!/usr/bin/env python3
import json, os, pathlib, subprocess, sys, time
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
root = pathlib.Path(os.environ['VM_FIXTURE'])
with (root / 'calls.jsonl').open('a') as stream:
    stream.write(json.dumps({'name': name, 'args': args}) + '\\n')
settings = json.loads((root / 'settings.json').read_text())
if name == 'timeout':
    assert args[:4] == ['--signal=TERM', '--kill-after=2s', '10s', 'qm']
    if settings.get('real_timeout'):
        result = subprocess.run([os.environ['REAL_TIMEOUT'], *args])
    else:
        result = subprocess.run(args[3:])
    sys.exit(result.returncode)
elif name == 'qm':
    if args == ['status', '100']:
        operation = 'status'
        default = 'status: running\\n'
    elif args == ['config', '100', '--current']:
        operation = 'config'
        default = 'agent: 1\\nname: synthetic-private-infrastructure-marker\\n'
    else:
        # No fake succeeds on an unrecognised or guest-agent operation.
        sys.exit(90)
    if settings.get('sleep_operation') == operation:
        time.sleep(20)
    sys.stdout.write(settings.get(operation + '_output', default))
    sys.stderr.write('synthetic-private-infrastructure-marker\\n')
    sys.exit(settings.get(operation + '_exit', 0))
elif name == 'sudo':
    assert args == ['bash', './scripts/test-vm-disk.sh', '100']
    os.execv(os.environ['REAL_BASH'], [os.environ['REAL_BASH'], *args[1:]])
else:
    sys.exit(91)
'''


class VMDiskDiagnosticsTest(unittest.TestCase):
    def exercise(self, *, settings=None, args=None, recipe=None, source=None,
                 missing=None):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "settings.json").write_text(json.dumps(settings or {}))
            for name in ("qm", "timeout", "sudo", "pveum", "pveversion", "curl", "systemctl"):
                if name == missing:
                    continue
                tool = root / name
                tool.write_text(TOOL)
                tool.chmod(0o700)
            env = dict(os.environ, VM_FIXTURE=str(root), REAL_BASH=BASH,
                       REAL_TIMEOUT=REAL_TIMEOUT, PATH=f"{root}:{os.environ['PATH']}")
            if missing:
                # The adapters use an absolute interpreter, so an empty
                # fallback PATH can genuinely model unavailable dependencies.
                env["PATH"] = str(root)
                for tool in root.iterdir():
                    if tool.name in ("settings.json",):
                        continue
                    tool.write_text(TOOL.replace("/usr/bin/env python3", os.sys.executable))
            script = SCRIPT
            if source is not None:
                script = root / "candidate.sh"
                script.write_text(source)
            invocation = [BASH, str(script), *(args if args is not None else ["100"])]
            if recipe:
                invocation = [BASH, "-c", recipe]
            result = subprocess.run(invocation, env=env, cwd=ROOT,
                                    capture_output=True, text=True, timeout=16)
            calls_file = root / "calls.jsonl"
            calls = [json.loads(line) for line in calls_file.read_text().splitlines()] if calls_file.exists() else []
            self.assertNotIn(PRIVATE, result.stdout + result.stderr)
            return result, calls

    def assert_passive(self, calls, *, operations=READS):
        self.assertEqual([c["args"] for c in calls if c["name"] == "qm"], operations)
        self.assertTrue(all(c["name"] in ("timeout", "qm", "sudo") for c in calls))
        for call in calls:
            if call["name"] == "timeout":
                self.assertEqual(call["args"][:4], ["--signal=TERM", "--kill-after=2s", "10s", "qm"])

    def test_running_vm_preflight_is_passive_even_without_a_lock(self):
        result, calls = self.exercise()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_passive(calls)
        self.assertIn("VM status: running", result.stdout)
        self.assertIn("Guest agent configured: enabled", result.stdout)
        self.assertIn("VM lock: not reported", result.stdout)
        self.assertIn("does not establish", result.stdout)
        self.assertIn("guest thaw were not tested", result.stdout)

    def test_stopped_guest_is_not_started_or_probed(self):
        result, calls = self.exercise(settings={"status_output": "status: stopped\n"})
        self.assertEqual(result.returncode, 0)
        self.assertIn("VM status: stopped", result.stdout)
        self.assert_passive(calls)

    def test_backup_other_empty_and_future_locks_never_send_a_probe(self):
        for lock in ("backup", "snapshot", "migrate", "", PRIVATE):
            with self.subTest(lock=lock):
                result, calls = self.exercise(settings={"config_output": f"agent: 1\nlock: {lock}\nname: {PRIVATE}\n"})
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("VM lock: backup" if lock == "backup" else "VM lock: present or unknown", result.stdout)
                self.assert_passive(calls)

    def test_agent_boolean_is_not_inferred_from_other_numeric_options(self):
        cases = {
            None: "disabled", "0": "disabled", "1": "enabled",
            "0,freeze-fs-on-backup=1": "disabled", "enabled=0,fstrim_cloned_disks=1": "disabled",
            "enabled=1,freeze-fs-on-backup=0": "enabled", "freeze-fs-on-backup=1,enabled=0": "disabled",
            "fstrim_cloned_disks=1": "unknown", "enabled=2": "unknown", "": "unknown",
            "0,enabled=1": "unknown", "1,enabled=1": "unknown", "disabled": "unknown",
        }
        for value, expected in cases.items():
            with self.subTest(value=value):
                config = f"name: {PRIVATE}\n" + (f"agent: {value}\n" if value is not None else "")
                result, calls = self.exercise(settings={"config_output": config})
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn(f"Guest agent configured: {expected} (not a responsiveness test)", result.stdout)
                self.assert_passive(calls)

    def test_failed_status_never_reads_configuration_or_reports_completion(self):
        for code in (1, 13, 124, 137, 143):
            with self.subTest(code=code):
                result, calls = self.exercise(settings={"status_exit": code, "status_output": PRIVATE})
                self.assertEqual(result.returncode, 1)
                self.assertNotIn("preflight completed", result.stdout)
                self.assertNotIn("VM status:", result.stdout)
                self.assertIn("timed out" if code in (124, 137) else f"exit {code}", result.stderr)
                self.assert_passive(calls, operations=READS[:1])

    def test_config_failure_cannot_be_presented_as_an_absent_lock(self):
        for code in (1, 13, 124, 137):
            with self.subTest(code=code):
                result, calls = self.exercise(settings={"config_exit": code, "config_output": f"agent: 1\nlock: backup\n{PRIVATE}"})
                self.assertEqual(result.returncode, 1)
                self.assertNotIn("preflight completed", result.stdout)
                self.assertNotIn("VM lock:", result.stdout)
                self.assertIn("configuration read", result.stderr)
                self.assert_passive(calls)

    def test_empty_unknown_and_repeated_responses_fail_without_dumping_config(self):
        cases = [
            {"status_output": ""}, {"status_output": "status: " + PRIVATE},
            {"config_output": ""}, {"config_output": PRIVATE},
            {"config_output": f"agent: 1\nagent: 0\nname: {PRIVATE}\n"},
            {"config_output": f"lock: backup\nlock: migrate\nname: {PRIVATE}\n"},
        ]
        for settings in cases:
            with self.subTest(settings=settings):
                result, calls = self.exercise(settings=settings)
                self.assertEqual(result.returncode, 1)
                self.assertIn("preflight incomplete", result.stderr)
                self.assertNotIn("preflight completed", result.stdout)
                self.assert_passive(calls, operations=READS[:1] if "status_output" in settings else READS)

    def test_help_and_invalid_arguments_never_contact_proxmox(self):
        for args in ([], ["0"], ["-1"], ["100;touch BAD"], ["100", "--probe"],
                     ["9999999999"], ["--probe"], ["--help"]):
            with self.subTest(args=args):
                result, calls = self.exercise(args=args)
                self.assertEqual(result.returncode, 0 if args == ["--help"] else 2)
                self.assertEqual(calls, [])

    def test_missing_dependencies_do_not_run_a_partial_preflight(self):
        for missing in ("qm", "timeout"):
            with self.subTest(missing=missing):
                result, calls = self.exercise(missing=missing)
                self.assertEqual(result.returncode, 1)
                self.assertIn(f"unavailable: {missing}", result.stderr)
                self.assertEqual(calls, [])

    def test_real_deadline_stops_a_blocked_synthetic_local_read(self):
        self.assertIsNotNone(REAL_TIMEOUT)
        start = time.monotonic()
        result, calls = self.exercise(settings={"real_timeout": True, "sleep_operation": "status"})
        elapsed = time.monotonic() - start
        self.assertEqual(result.returncode, 1)
        self.assertIn("status read timed out", result.stderr)
        self.assertLess(elapsed, 14)
        self.assert_passive(calls, operations=READS[:1])

    def test_exact_documented_command_uses_the_passive_helper(self):
        recipes = re.findall(r"```bash\n(.*?)```", DOC.read_text(), re.DOTALL)
        self.assertEqual(len(recipes), 1)
        result, calls = self.exercise(recipe=recipes[0])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_passive(calls)
        self.assertEqual([c["args"] for c in calls if c["name"] == "sudo"],
                         [["bash", "./scripts/test-vm-disk.sh", "100"]])

    def test_safety_oracle_rejects_reintroduced_guest_probes_and_mutations(self):
        source = SCRIPT.read_text()
        for unsafe in ('qm agent "$vmid" ping', 'qm agent "$vmid" get-fsinfo',
                       'qm unlock "$vmid"', 'qm reset "$vmid"',
                       'pveum acl modify /', 'systemctl restart pulse'):
            with self.subTest(operation=unsafe):
                mutated = source.replace("vmid=$1", "vmid=$1\n" + unsafe)
                result, calls = self.exercise(source=mutated, settings={"config_output": "agent: 1\nlock: backup\n"})
                self.assertNotEqual(result.returncode, 0)
                with self.assertRaises(AssertionError):
                    self.assert_passive(calls)


class VMDiskHelpTest(unittest.TestCase):
    def test_guides_are_mirrored_and_preserve_the_safety_and_proof_boundaries(self):
        for name in ("VM_DISK_MONITORING", "TROUBLESHOOTING"):
            source = (ROOT / f"docs/{name}.md").read_bytes()
            self.assertEqual(source, (ROOT / f"frontend-modern/public/docs/{name}.md").read_bytes())
        guide = DOC.read_text()
        for phrase in ("monitoring and alerts", "not established as a reproduced cause",
                       "does not prove thaw", "a backup can", "older copies", "non-zero exit",
                       "host-root diagnostic does not test", "not a claim", "bounded timeouts"):
            self.assertIn(phrase, guide)
        for block in re.findall(r"```bash\n(.*?)```", guide, re.DOTALL):
            self.assertNotRegex(block, r"\b(curl|wget|qm agent|pveum|systemctl)\b")
        self.assertNotIn("GUEST_AGENT_FSINFO_TIMEOUT=", guide)


if __name__ == "__main__":
    unittest.main()
