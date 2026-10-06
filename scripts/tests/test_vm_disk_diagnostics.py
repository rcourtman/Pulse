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
import textwrap
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts/test-vm-disk.sh"
DOC = ROOT / "docs/VM_DISK_MONITORING.md"
BASH = shutil.which("bash")
REAL_TIMEOUT = shutil.which("timeout")
REAL_PYTHON = os.sys.executable
PRIVATE = "synthetic-private-infrastructure-marker"
READS = [["status", "100"], ["config", "100", "--current"]]
STATUS_LIMIT = 256
CONFIG_LIMIT = 64 * 1024


def guide_section(guide, heading):
    """Select one named H3, never another section's executable examples."""
    sections, current = [], None
    fenced = False
    for line in guide.splitlines(keepends=True):
        boundary = None if fenced else re.match(r"^(#{1,3}) (.*?)\s*$", line)
        if boundary:
            if current is not None:
                sections.append("".join(current))
            current = [] if boundary.group(1) == "###" and boundary.group(2) == heading else None
        elif current is not None:
            current.append(line)
        if line.lstrip().startswith("```"):
            fenced = not fenced
    if current is not None:
        sections.append("".join(current))
    if len(sections) != 1:
        raise AssertionError(f"Expected one guide section: {heading}")
    return sections[0]


def bash_examples(section):
    return [textwrap.dedent(block).strip()
            for block in re.findall(r"```bash\n(.*?)```", section, re.DOTALL)]

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
    # Write bytes so corrupt transport controls are not silently repaired by
    # the fixture's text encoding before they reach the actual helper.
    output = settings.get(operation + '_output', default).encode('utf-8')
    if settings.get(operation + '_invalid_utf8'):
        output += b'\\xff'
    try:
        sys.stdout.buffer.write(output)
        sys.stdout.buffer.flush()
    except BrokenPipeError:
        # A bounded reader is entitled to close the pipe before the producer
        # finishes. Do not turn this fixture's expected stop into raw output.
        os._exit(1)
    sys.stderr.write('synthetic-private-infrastructure-marker\\n')
    sys.exit(settings.get(operation + '_exit', 0))
elif name == 'python3':
    # The non-isolated form exists only for the startup-code negative control.
    assert (len(args) == 4 and args[:2] == ['-I', '-c'] or
            len(args) == 3 and args[0] == '-c') and args[-1] in ('256', '65536')
    if settings.get('reader_exit') is not None:
        sys.exit(settings['reader_exit'])
    os.execv(os.environ['REAL_PYTHON'], [os.environ['REAL_PYTHON'], *args])
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
            for name in ("qm", "timeout", "python3", "sudo", "pveum", "pveversion", "curl", "systemctl"):
                if name == missing:
                    continue
                tool = root / name
                # The collector adapter is itself named python3. Pin the
                # fixture interpreter so /usr/bin/env cannot recurse through
                # that adapter instead of ever executing the fake qm.
                tool.write_text(TOOL.replace("/usr/bin/env python3", f"{REAL_PYTHON} -I"))
                tool.chmod(0o700)
            env = dict(os.environ, VM_FIXTURE=str(root), REAL_BASH=BASH,
                       REAL_TIMEOUT=REAL_TIMEOUT, REAL_PYTHON=REAL_PYTHON,
                       PATH=f"{root}:{os.environ['PATH']}")
            if missing:
                # The adapters use an absolute interpreter, so an empty
                # fallback PATH can genuinely model unavailable dependencies.
                env["PATH"] = str(root)
            startup_marker = root / "startup-executed"
            if (settings or {}).get("poison_pythonpath"):
                startup = root / "python-startup"
                startup.mkdir()
                (startup / "sitecustomize.py").write_text(
                    "from pathlib import Path\n"
                    f"Path({str(startup_marker)!r}).write_text('synthetic startup')\n"
                    "raise SystemExit(29)\n"
                )
                env["PYTHONPATH"] = str(startup)
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
            self.assertFalse(startup_marker.exists(), "untrusted Python startup code executed")
            self.assertNotIn(PRIVATE, result.stdout + result.stderr)
            return result, calls

    def assert_passive(self, calls, *, operations=READS):
        self.assertEqual([c["args"] for c in calls if c["name"] == "qm"], operations)
        self.assertTrue(all(c["name"] in ("timeout", "qm", "python3", "sudo") for c in calls))
        self.assertEqual([c["args"][-1] for c in calls if c["name"] == "python3"],
                         [str(STATUS_LIMIT if operation[0] == "status" else CONFIG_LIMIT)
                          for operation in operations])
        for call in calls:
            if call["name"] == "timeout":
                self.assertEqual(call["args"][:4], ["--signal=TERM", "--kill-after=2s", "10s", "qm"])
            if call["name"] == "python3":
                self.assertEqual(call["args"][:2], ["-I", "-c"])
                self.assertIn(call["args"][3], (str(STATUS_LIMIT), str(CONFIG_LIMIT)))

    def test_oversized_reads_fail_before_interpreting_partial_configuration(self):
        cases = [
            {"status_output": "status: running\n" + PRIVATE + "x" * (1024 * 1024)},
            {"config_output": "agent: 1\nname: " + PRIVATE + "\ndescription: "
                              + "x" * (1024 * 1024) + "\nlock: backup\n"},
        ]
        for settings in cases:
            with self.subTest(operation=next(iter(settings))):
                result, calls = self.exercise(settings=settings)
                self.assertEqual(result.returncode, 1, result.stdout)
                self.assertIn("exceeded the", result.stderr)
                self.assertIn("byte limit; preflight incomplete", result.stderr)
                self.assertNotIn("Guest agent configured:", result.stdout)
                self.assertNotIn("VM lock:", result.stdout)
                self.assertNotIn("preflight completed", result.stdout)
                self.assertLess(len(result.stdout + result.stderr), 600)
                self.assert_passive(calls, operations=READS[:1] if "status_output" in settings else READS)

    def test_config_limit_is_measured_in_bytes_and_includes_trailing_newlines(self):
        prefix = "agent: 1\nname: " + PRIVATE + "\ndescription: "
        padding = CONFIG_LIMIT - len(prefix.encode()) - 1
        exact = prefix + "x" * padding + "\n"
        self.assertEqual(len(exact.encode()), CONFIG_LIMIT)
        result, calls = self.exercise(settings={"config_output": exact})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_passive(calls)
        # Shell command substitution removes trailing newlines. Neither that
        # nor a multi-byte name may make an oversized response look complete.
        for oversized in (exact + "\n", prefix + "é" * padding + "\n"):
            with self.subTest(bytes=len(oversized.encode())):
                result, calls = self.exercise(settings={"config_output": oversized})
                self.assertEqual(result.returncode, 1, result.stdout)
                self.assertIn("byte limit; preflight incomplete", result.stderr)
                self.assertNotIn("VM lock:", result.stdout)
                self.assertNotIn("preflight completed", result.stdout)
                self.assert_passive(calls)

    def test_status_limit_includes_trailing_newlines_before_interpretation(self):
        exact = "status: running" + "\n" * (STATUS_LIMIT - len("status: running"))
        result, calls = self.exercise(settings={"status_output": exact})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("VM status: running", result.stdout)
        self.assert_passive(calls)
        result, calls = self.exercise(settings={"status_output": exact + "\n"})
        self.assertEqual(result.returncode, 1)
        self.assertIn("256-byte limit; preflight incomplete", result.stderr)
        self.assertNotIn("VM status:", result.stdout)
        self.assert_passive(calls, operations=READS[:1])

    def test_corrupt_reads_cannot_be_silently_normalised_into_a_success(self):
        for settings in (
            {"status_output": "status: run\u0000ning\n"},
            {"config_output": "agent: \u00001\nname: " + PRIVATE + "\n"},
            {"config_output": "agent: 1\nname: " + PRIVATE + "\n", "config_invalid_utf8": True},
        ):
            with self.subTest(settings=settings):
                result, calls = self.exercise(settings=settings)
                self.assertEqual(result.returncode, 1, result.stdout)
                self.assertIn("non-text bytes; preflight incomplete", result.stderr)
                self.assertNotIn("Guest agent configured:", result.stdout)
                self.assertNotIn("VM lock:", result.stdout)
                self.assertNotIn("preflight completed", result.stdout)
                self.assert_passive(calls, operations=READS[:1] if "status_output" in settings else READS)

    def test_valid_unicode_configuration_remains_passive(self):
        result, calls = self.exercise(settings={"config_output": "agent: 1\nname: café\nlock: backup\n"})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Guest agent configured: enabled", result.stdout)
        self.assertIn("VM lock: backup", result.stdout)
        self.assertNotIn("café", result.stdout + result.stderr)
        self.assert_passive(calls)

    def test_producer_failures_are_independent_of_reader_result_codes(self):
        # Producer exit codes are independent of bounded-reader result codes.
        for code in (80, 81):
            with self.subTest(code=code):
                result, calls = self.exercise(settings={"config_exit": code})
                self.assertEqual(result.returncode, 1)
                self.assertIn(f"configuration read failed (exit {code})", result.stderr)
                self.assertNotIn("preflight completed", result.stdout)
                self.assert_passive(calls)

    def test_reader_failure_is_incomplete_not_an_empty_or_successful_response(self):
        result, calls = self.exercise(settings={"reader_exit": 23})
        self.assertEqual(result.returncode, 1)
        self.assertIn("status read failed (exit 23)", result.stderr)
        self.assertNotIn("VM status:", result.stdout)
        self.assertNotIn("preflight completed", result.stdout)
        self.assert_passive(calls, operations=READS[:1])

    def test_reader_does_not_execute_python_environment_startup_code(self):
        result, calls = self.exercise(settings={"poison_pythonpath": True})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_passive(calls)
        unsafe = SCRIPT.read_text().replace("python3 -I -c", "python3 -c")
        self.assertNotEqual(unsafe, SCRIPT.read_text())
        with self.assertRaisesRegex(AssertionError, "untrusted Python startup"):
            self.exercise(settings={"poison_pythonpath": True}, source=unsafe)

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

    def test_malformed_lines_cannot_hide_agent_or_lock_configuration(self):
        lines = (
            " lock: backup", "\tlock: backup", "lock=backup", "lock : backup",
            "Lock: backup", "LOCK: backup", "lock", "agent 1", "Agent: 1",
            "warning " + PRIVATE,
        )
        for line in lines:
            for suffix in ("", "\n"):
                with self.subTest(line=line, suffix=suffix):
                    config = f"agent: 1\nname: {PRIVATE}\n{line}{suffix}"
                    result, calls = self.exercise(settings={"config_output": config})
                    self.assertEqual(result.returncode, 1, result.stdout)
                    self.assertIn("unrecognised line; preflight incomplete", result.stderr)
                    self.assertNotIn("Guest agent configured:", result.stdout)
                    self.assertNotIn("VM lock:", result.stdout)
                    self.assertNotIn("preflight completed", result.stdout)
                    self.assert_passive(calls)

    def test_blank_comments_and_unknown_well_formed_fields_remain_passive(self):
        for agent in (None, "0", "1"):
            with self.subTest(agent=agent):
                config = (f"\n# {PRIVATE}\n \t\nfuture_read_option: {PRIVATE}\n"
                          f"\t# lock: backup {PRIVATE}\nname: {PRIVATE}\n")
                if agent is not None:
                    config += f"agent: {agent}\n"
                result, calls = self.exercise(settings={"config_output": config})
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("Guest agent configured: enabled" if agent == "1"
                              else "Guest agent configured: disabled", result.stdout)
                self.assertIn("VM lock: not reported", result.stdout)
                self.assert_passive(calls)

        result, calls = self.exercise(settings={"config_output": f"\n# {PRIVATE}\n \t\n"})
        self.assertEqual(result.returncode, 1)
        self.assertIn("preflight incomplete", result.stderr)
        self.assertNotIn("VM lock:", result.stdout)
        self.assert_passive(calls)

    def test_help_and_invalid_arguments_never_contact_proxmox(self):
        for args in ([], ["0"], ["-1"], ["100;touch BAD"], ["100", "--probe"],
                     ["9999999999"], ["--probe"], ["--help"]):
            with self.subTest(args=args):
                result, calls = self.exercise(args=args)
                self.assertEqual(result.returncode, 0 if args == ["--help"] else 2)
                self.assertEqual(calls, [])

    def test_missing_dependencies_do_not_run_a_partial_preflight(self):
        for missing in ("qm", "timeout", "python3"):
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
        recipes = bash_examples(guide_section(DOC.read_text(), "Passive host preflight"))
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
    def test_documented_read_limits_and_failure_meaning_match_the_helper(self):
        section = guide_section(DOC.read_text(), "Passive host preflight")
        for phrase in ("256 bytes", "64 KiB", "Python 3", "truncated prefix",
                       "printing raw", "saving it to disk", "Inspect the configuration privately"):
            self.assertIn(phrase, section)
        self.assertIn(f"read_bounded status status_output {STATUS_LIMIT}", SCRIPT.read_text())
        self.assertIn(f"read_bounded configuration config {CONFIG_LIMIT}", SCRIPT.read_text())

    def test_section_selection_preserves_shell_comments_and_rejects_missing_or_duplicate_headings(self):
        content = "```bash\n# From the reviewed checkout\n### Shell comment, not a heading\ntrue\n```\n"
        guide = "### Passive host preflight\n" + content + "## Next\nOther text\n"
        self.assertEqual(guide_section(guide, "Passive host preflight"), content)
        for invalid in ("## No passive section\n", guide + "### Passive host preflight\n"):
            with self.subTest(guide=invalid), self.assertRaises(AssertionError):
                guide_section(invalid, "Passive host preflight")

    def assert_guide_commands(self, guide):
        # The planned server pause is intentionally active, unlike the passive
        # hypervisor diagnostic. Admit only the six reviewed server/timer blocks
        # here; do not permit arbitrary systemctl or guest commands elsewhere.
        precaution = guide_section(guide, "Pause Pulse for a planned freeze-enabled backup")
        self.assertEqual(bash_examples(precaution), [
            "systemctl show pulse.service --property=LoadState,ActiveState,MainPID",
            "systemctl show pulse-update.timer --property=LoadState,ActiveState",
            "sudo systemctl stop pulse-update.timer\n"
            "systemctl show pulse-update.timer pulse-update.service \\\n"
            "  --property=Id,LoadState,ActiveState,MainPID",
            "sudo systemctl stop pulse.service\n"
            "systemctl show pulse.service --property=LoadState,ActiveState,MainPID",
            "sudo systemctl start pulse.service\nsystemctl is-active pulse.service",
            "sudo systemctl start pulse-update.timer\nsystemctl is-active pulse-update.timer",
        ])
        for block in bash_examples(guide.replace(precaution, "", 1)):
            self.assertNotRegex(block, r"\b(curl|wget|qm agent|pveum|systemctl)\b")

    def test_command_boundaries_reject_active_diagnostics_and_unreviewed_pause_operations(self):
        guide = DOC.read_text()
        mutations = [
            guide.replace("sudo bash ./scripts/test-vm-disk.sh 100",
                          "systemctl restart pulse.service"),
            guide.replace("sudo systemctl stop pulse.service", "sudo systemctl restart pulse.service"),
            guide.replace("sudo systemctl start pulse-update.timer", "qm agent 100 ping"),
            guide + "\n```bash\nsystemctl stop pulse.service\n```\n",
            guide.replace("sudo bash ./scripts/test-vm-disk.sh 100", "curl https://example.invalid"),
        ]
        for index, mutated in enumerate(mutations):
            with self.subTest(mutation=index), self.assertRaises(AssertionError):
                self.assert_guide_commands(mutated)

    def test_guides_are_mirrored_and_preserve_the_safety_and_proof_boundaries(self):
        for name in ("VM_DISK_MONITORING", "TROUBLESHOOTING"):
            source = (ROOT / f"docs/{name}.md").read_bytes()
            self.assertEqual(source, (ROOT / f"frontend-modern/public/docs/{name}.md").read_bytes())
        guide = DOC.read_text()
        for phrase in ("monitoring and alerts", "not established as a reproduced cause",
                       "does not prove thaw", "a backup can", "older copies", "non-zero exit",
                       "host-root diagnostic does not test", "not a claim", "bounded timeouts"):
            self.assertIn(phrase, guide)
        self.assert_guide_commands(guide)
        self.assertNotIn("GUEST_AGENT_FSINFO_TIMEOUT=", guide)


if __name__ == "__main__":
    unittest.main()
