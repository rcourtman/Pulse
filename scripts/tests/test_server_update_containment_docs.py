#!/usr/bin/env python3
"""Exercise the copied timer-containment commands against a fake unit manager.

No service, updater, host, credential or backup is touched. These controls prove
the documented command scope and bounded reads, not installed update recovery.
"""

import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import tempfile
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "### Pause further unattended attempts without stopping Pulse\n"
PROPERTIES = ["--property=LoadState", "--property=ActiveState",
              "--property=UnitFileState", "--property=Triggers"]

MANAGER = r'''#!/usr/bin/python3
import json, os, signal, sys, time
from pathlib import Path
args = sys.argv[1:]
name = Path(sys.argv[0]).name
with Path(os.environ['CALLS']).open('a') as output:
    output.write(json.dumps({'name': name, 'args': args}) + '\n')
if name == 'sudo':
    assert args[0] == 'systemctl', 'Unexpected administrative command'
    os.execvp(args[0], args)
assert args[0] in ('show', 'disable'), 'Unexpected manager mutation'
timer = args[1] if args[0] == 'show' else args[-1]
assert timer == os.environ['TIMER'], 'Wrong timer or a non-timer unit'
assert timer.endswith('-update.timer'), 'Not the update timer'
state_path = Path(os.environ['STATE'])
state = json.loads(state_path.read_text())
if args[0] == 'show':
    assert args[2:] == ['--property=LoadState', '--property=ActiveState',
                        '--property=UnitFileState', '--property=Triggers']
    if os.environ.get('HANG') == 'yes':
        # Verify the copied reader's kill deadline, not just its normal exit.
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
        time.sleep(30)
    if os.environ.get('READ_EXIT'):
        print('Unit state read failed', file=sys.stderr)
        sys.exit(int(os.environ['READ_EXIT']))
    for field in ('LoadState', 'ActiveState', 'UnitFileState', 'Triggers'):
        print(field + '=' + state[field])
else:
    assert args == ['disable', '--now', timer], 'Unexpected manager mutation'
    if os.environ.get('DISABLE_EXIT'):
        print('Timer pause not confirmed', file=sys.stderr)
        sys.exit(int(os.environ['DISABLE_EXIT']))
    state['ActiveState'] = 'inactive'
    if state['UnitFileState'] != 'masked':
        state['UnitFileState'] = 'disabled'
    state_path.write_text(json.dumps(state))
'''


def guide_section():
    text = (ROOT / "docs/AUTO_UPDATE.md").read_text(encoding="utf-8")
    return text.split(HEADING, 1)[1].split("\n### ", 1)[0]


class ServerUpdateContainmentDocsTest(unittest.TestCase):
    def recipes(self, timer="pulse-update.timer"):
        blocks = re.findall(r"```bash\n(.*?)```", guide_section(), re.S)
        self.assertEqual(len(blocks), 2, "Only a bounded state read and timer pause")
        return [block.replace("pulse-update.timer", timer) for block in blocks]

    def exercise(self, steps, *, timer="pulse-update.timer", **settings):
        with tempfile.TemporaryDirectory() as temporary:
            fixture = Path(temporary)
            tools = fixture / "tools"
            tools.mkdir()
            for name in ("systemctl", "sudo"):
                path = tools / name
                path.write_text(MANAGER, encoding="utf-8")
                path.chmod(0o700)
            # Use real coreutils timeout; no command can reach a real manager.
            timeout = shutil.which("timeout")
            self.assertIsNotNone(timeout, "The documented bounded reader needs timeout")
            (tools / "timeout").symlink_to(timeout)
            state = {
                "LoadState": "loaded", "ActiveState": "active",
                "UnitFileState": "enabled", "Triggers": timer.replace(".timer", ".service"),
                "PulseService": "active", "UpdaterService": "active", "Host": "running",
                "Backups": "preserved", "PrivateEnvironment": "fixture-secret-never-print",
            }
            state.update(settings.pop("state", {}))
            state_path = fixture / "state.json"
            state_path.write_text(json.dumps(state))
            calls = fixture / "calls.jsonl"
            env = dict(os.environ, PATH=str(tools), TIMER=timer,
                       STATE=str(state_path), CALLS=str(calls), **settings)
            results = []
            for step in steps:
                result = subprocess.run(["/bin/bash", "-c", step], env=env,
                                        capture_output=True, text=True, timeout=9)
                self.assertNotIn(state["PrivateEnvironment"], result.stdout + result.stderr)
                results.append(result)
                if result.returncode:
                    break  # A failed/uncertain action is not silently replayed.
            commands = [json.loads(line) for line in calls.read_text().splitlines()]
            return results, commands, state, json.loads(state_path.read_text())

    def test_pause_and_readback_touch_only_the_known_timer(self):
        for timer in ("pulse-update.timer", "pulse-backend-update.timer", "custom-pulse-update.timer"):
            with self.subTest(timer=timer):
                read, pause = self.recipes(timer)
                results, calls, before, after = self.exercise([read, pause, read], timer=timer)
                self.assertEqual([r.returncode for r in results], [0, 0, 0])
                self.assertIn("ActiveState=active", results[0].stdout)
                self.assertIn("UnitFileState=enabled", results[0].stdout)
                self.assertIn("ActiveState=inactive", results[-1].stdout)
                self.assertIn("UnitFileState=disabled", results[-1].stdout)
                self.assertEqual(calls, [
                    {"name": "systemctl", "args": ["show", timer] + PROPERTIES},
                    {"name": "sudo", "args": ["systemctl", "disable", "--now", timer]},
                    {"name": "systemctl", "args": ["disable", "--now", timer]},
                    {"name": "systemctl", "args": ["show", timer] + PROPERTIES},
                ])
                for field in ("PulseService", "UpdaterService", "Host", "Backups", "PrivateEnvironment"):
                    self.assertEqual(after[field], before[field], field)

    def test_masked_and_disabled_timers_are_not_enabled_or_unmasked(self):
        read, pause = self.recipes()
        for unit_state in ("masked", "disabled"):
            with self.subTest(unit_state=unit_state):
                results, calls, before, after = self.exercise(
                    [read, pause, read], state={"UnitFileState": unit_state, "ActiveState": "inactive"})
                self.assertEqual([r.returncode for r in results], [0, 0, 0])
                self.assertEqual(after, before)
                self.assertFalse(any("enable" in c["args"] or "unmask" in c["args"] for c in calls))

    def test_failed_read_does_not_continue_to_a_mutation(self):
        read, pause = self.recipes()
        results, calls, before, after = self.exercise([read, pause], READ_EXIT="1")
        self.assertEqual([r.returncode for r in results], [1])
        self.assertEqual(len(calls), 1)
        self.assertEqual(after, before)

    def test_failed_pause_is_not_hidden_or_replayed(self):
        _, pause = self.recipes()
        results, calls, before, after = self.exercise([pause], DISABLE_EXIT="5")
        self.assertEqual([r.returncode for r in results], [5])
        self.assertEqual(len(calls), 2)
        self.assertEqual(after, before)
        self.assertIn("not confirmed", results[0].stderr)

    def test_stalled_reader_has_a_kill_deadline(self):
        read, pause = self.recipes()
        start = time.monotonic()
        results, calls, before, after = self.exercise([read, pause], HANG="yes")
        # subprocess preserves SIGKILL as a negative return code when bash
        # execs its final timeout command; an intervening shell reports 137.
        self.assertIn(results[0].returncode, (124, 137, -signal.SIGKILL))
        self.assertLess(time.monotonic() - start, 8)
        self.assertEqual(len(calls), 1)
        self.assertEqual(after, before)

    def test_absent_timer_and_unknown_identity_require_a_stop_not_a_fallback(self):
        read, _ = self.recipes()
        results, calls, before, after = self.exercise(
            [read], state={"LoadState": "not-found", "ActiveState": "inactive", "UnitFileState": ""})
        self.assertEqual(results[0].returncode, 0)
        self.assertIn("LoadState=not-found", results[0].stdout)
        self.assertEqual(len(calls), 1)
        self.assertEqual(after, before)
        text = " ".join(guide_section().split())
        for phrase in ("identity is unknown, stop here", "not an inactive timer",
                       "do not try every matching unit", "runtime-enabled",
                       "do not substitute an unbounded reader", "A name alone does not establish ownership"):
            self.assertIn(phrase, text)

    def test_guidance_preserves_evidence_and_separately_recorded_restore_state(self):
        text = " ".join(guide_section().split())
        for phrase in (
            "containment, not rollback or a fix", "does not cancel an updater already running",
            "prevent a manual or in-app update", "inside its LXC", "not on the Proxmox host",
            "legacy installations", "pulse-backend-update.timer", "actual known timer name",
            "starting and intended versions", "time and timezone", "any recovery already performed",
            "version at failure", "A verified signature is not a completed install or rollback",
            "`Broken pipe` does not explain a host stop", "later boot does not supply its time or cause",
            "already available", "keep unknowns unknown", "Do not repeat the update",
            "delete rollback backups", "do not assume the schedule was paused",
            "enable only if previously enabled", "start only if previously active",
            "Do not unmask a unit", "create a second schedule", "does not undo the saved",
            "next stable update", "independent monitoring and notifications",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)

    def test_shipped_doc_is_the_same_and_pause_never_edits_update_consent(self):
        source = (ROOT / "docs/AUTO_UPDATE.md").read_bytes()
        self.assertEqual(source, (ROOT / "frontend-modern/public/docs/AUTO_UPDATE.md").read_bytes())
        for recipe in self.recipes():
            self.assertNotRegex(recipe, r"\b(?:sed|tee|rm|cp|mv|restart|reset-failed|enable|unmask)\b")
            self.assertNotIn("system.json", recipe)


if __name__ == "__main__":
    unittest.main()
