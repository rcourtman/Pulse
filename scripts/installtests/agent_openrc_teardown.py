#!/usr/bin/env python3
"""Execute production OpenRC removal with closed, ordinary-user manager models.

Only private fixture files can be deleted. The full caller's initial admission
and cleanup run too, with transport/process/state operations replaced. Paths are
redirected to the fixture; there is no privileged installer, native OpenRC,
published installation, real credential or remote contact in this proof.
"""

import hashlib
import json
import os
from pathlib import Path
import shlex
import stat
import subprocess
import tempfile
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_AGENT_OPENRC_SOURCE", ROOT / "scripts/install.sh"))
FUNCTIONS = ("truenas_scale_agent_removal_present", "read_openrc_agent_removal_status", "read_openrc_agent_removal_runlevels",
             "quiesce_openrc_agent_for_removal", "teardown_openrc_agent_service")


def definitions(source):
    # Parent adverse controls execute its existing teardown/caller as-is, not
    # a substitute implementation of the newly added admission functions.
    selected = []
    for name in FUNCTIONS:
        marker = name + "() {"
        if marker not in source:
            continue
        if source.count(marker) != 1:
            raise ValueError("ambiguous production function")
        selected.append(marker + source.split(marker, 1)[1].split("\n}\n", 1)[0] + "\n}\n")
    return "\n".join(selected)


MODEL = r'''#!/usr/bin/python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['FIXTURE'])
model = root / 'model.json'
value = json.loads(model.read_text())
role, args = pathlib.Path(sys.argv[0]).name, sys.argv[1:]
fault = value['fault']
remote = (root / 'events').exists() and 'remote:remove' in (root / 'events').read_text()
def save(): model.write_text(json.dumps(value))
def event(text):
    with (root / 'events').open('a') as out: out.write(text + '\n')
if role == 'rc-service':
    assert len(args) == 2 and args[0] == 'pulse-agent'
    verb = args[1]
    event('service:' + verb)
    if verb == 'stop':
        if fault == 'refused-stop' or (fault == 'refused-stop-after-remote' and remote): sys.exit(1)
        if fault != 'ignored-stop': value['state'] = 'stopped'
        save(); sys.exit(0)
    assert verb == 'status'
    value['status_reads'] += 1; save()
    if fault == 'timeout-status':
        (root / 'timed-pid').write_text(str(os.getpid()))
        os.execl('/bin/sleep', 'sleep', '30')
    state = value['state']
    code = 0 if state == 'started' else 3
    if fault == 'empty-status': sys.exit(code)
    if fault == 'unknown-status': state = 'crashed'
    if fault == 'failed-status': code = 2
    if fault == 'wrong-started-exit': code = 3
    if fault == 'failed-final-status' and value['status_reads'] > 1: code = 2
    if fault == 'failed-status-after-remote' and remote: code = 2
    print(' * status: ' + state)
    if fault == 'duplicate-status': print(' * status: ' + state)
    if fault == 'blank-status': print()
    if fault == 'diagnostic-status': print('manager connection refused', file=sys.stderr)
    sys.exit(code)
elif role == 'rc-update':
    if args == ['show', '-v']:
        event('runlevels:show')
        value['inventory_reads'] += 1; save()
        final = value['inventory_reads'] > 1
        if fault == 'empty-inventory': sys.exit(0)
        if fault == 'failed-inventory' or (fault == 'failed-final-inventory' and final): sys.exit(2)
        print('              unrelated | boot')
        if (root / 'init').exists() or fault == 'missing-script-listed':
            levels = value['levels']
            if fault == 'invalid-level': levels = ['default*']
            if fault == 'duplicate-level': levels = ['default', 'default']
            print('            pulse-agent | ' + ' '.join(levels))
            if fault == 'duplicate-inventory': print('            pulse-agent | default')
        if fault == 'blank-inventory': print()
        if fault == 'diagnostic-inventory': print('manager connection refused', file=sys.stderr)
        sys.exit(0)
    assert args in (['del', 'pulse-agent', '--all'], ['del', 'pulse-agent', 'default']), args
    event('runlevels:disable-all' if args[-1] == '--all' else 'runlevels:disable-default')
    if fault == 'refused-disable': sys.exit(1)
    if fault != 'ignored-disable':
        value['levels'] = [] if args[-1] == '--all' else [v for v in value['levels'] if v != 'default']
    if fault == 'reactivation': value['state'] = 'started'
    save(); sys.exit(0)
elif role == 'pgrep':
    assert args == ['-x', 'pulse-agent']
    event('process:observe')
    if fault == 'orphan-process': print('12345'); sys.exit(0)
    if fault == 'failed-process': sys.exit(2)
    if fault == 'diagnostic-process': print('read refused', file=sys.stderr)
    if fault == 'blank-process': print()
    sys.exit(1)
else: raise ValueError('operation outside fixture')
'''


class AgentOpenRCTeardown(unittest.TestCase):
    source = SOURCE.read_text()

    def exercise(self, *, fault="", flow="teardown", state="started", levels=("default", "custom"),
                 path="regular", repeat=False, reply="confirmed"):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            tools = root / "tools"
            tools.mkdir()
            for name in ("rc-service", "rc-update", "pgrep"):
                tool = tools / name
                tool.write_text(MODEL)
                tool.chmod(0o700)
            init = root / "init"
            unrelated = root / "unrelated"
            unrelated.write_text("unrelated recovery file\n")
            if path == "linked":
                init.symlink_to(unrelated)
            elif path == "directory":
                init.mkdir()
            elif path != "missing":
                init.write_text("owned service script\n")
                init.chmod(0o600 if path == "not-executable" else 0o700)
            for name in ("state", "helper", "profile"):
                directory = root / name
                directory.mkdir()
                (directory / "token").write_text("synthetic recovery material\n")
            # A glob must never turn an invalid runlevel into this valid name.
            (root / "default-other").write_text("not a runlevel\n")
            model = dict(state=state, levels=list(levels), fault=fault, inventory_reads=0, status_reads=0)
            (root / "model.json").write_text(json.dumps(model))
            before = self.snapshot(root)
            values = dict(INIT_PATH=init, STATE_DIR=root / "state", PRIVILEGED_HELPER_STATE_DIR=root / "helper",
                          SAFE_PROFILE_STATE_DIR=root / "profile", EVENTS=root / "events", REPLY=reply)
            prefix = "\n".join(k + "=" + shlex.quote(str(v)) for k, v in values.items()) + "\n"
            shell = r'''
set -euo pipefail
AGENT_NAME=pulse-agent UNINSTALL=true EXIT_GENERAL=1 EXIT_MISSING_ARGS=16
PULSE_URL=https://same-instance.example.invalid AGENT_ID=synthetic
ACTION_RUNNER_NAME=pulse-agent-runner PRIVILEGED_HELPER_NAME=pulse-agent-helper
log_info() { :; }
log_warn() { :; }
fail() { printf '%s\n' "$1" >&2; exit "${2:-1}"; }
command() {
    if [[ "$*" == '-v systemctl' ]]; then return 1; fi
    builtin command "$@"
}
# No operation below can mutate a service, process or non-fixture path.
collector_credential_state_present() { return 0; }
uninstall_collector_registration() {
    printf 'remote:remove\n' >> "$EVENTS"
    python3 - <<'END'
import json, os, pathlib
path = pathlib.Path(os.environ['FIXTURE']) / 'model.json'
value = json.loads(path.read_text())
if value['fault'] == 'refused-stop-after-remote':
    value['state'], value['levels'] = 'started', ['custom']
    path.write_text(json.dumps(value))
END
    [[ "$REPLY" == confirmed ]]
}
pkill() { printf 'process:stop\n' >> "$EVENTS"; }
sleep() { :; }
rm() {
    [[ $# == 2 && "$1" == -f && "$2" == "$INIT_PATH" ]] || return 97
    printf 'init:remove\n' >> "$EVENTS"
    builtin command rm "$@"
}
remove_agent_state_dir() {
    [[ "$1" == "$STATE_DIR" ]] || return 96
    printf 'state:remove\n' >> "$EVENTS"
    builtin command rm -r -- "$STATE_DIR"
}
remove_privileged_helper_state_dir() { builtin command rm -r -- "$PRIVILEGED_HELPER_STATE_DIR"; }
remove_safe_profile_state_dir() { builtin command rm -r -- "$SAFE_PROFILE_STATE_DIR"; }
'''
            if flow == "caller":
                entry = self.source.split("# --- Uninstall Logic ---\n", 1)[1].split(
                    "    # Remove least-privilege helper artifacts.", 1)[0] + "fi\n"
                # Redirect only the fixed init path, as the fixture does with
                # STATE_DIR. The actual caller/production function bodies run.
                entry = entry.replace("quiesce_openrc_agent_for_removal ||",
                                      'quiesce_openrc_agent_for_removal "$INIT_PATH" ||')
                entry = entry.replace("        teardown_openrc_agent_service\n",
                                      '        teardown_openrc_agent_service "$INIT_PATH"\n')
                invocation = "main() {\n" + entry + "}\nmain\n"
            elif flow == "status":
                invocation = "read_openrc_agent_removal_status\n"
            elif flow == "inventory":
                invocation = "read_openrc_agent_removal_runlevels\n"
            else:
                invocation = 'teardown_openrc_agent_service "$INIT_PATH"\n' * (2 if repeat else 1)
            env = dict(os.environ, PATH=str(tools) + ":" + os.environ["PATH"], FIXTURE=str(root))
            result = subprocess.run(["bash", "-c", prefix + shell + definitions(self.source) + invocation],
                                    cwd=root, env=env, capture_output=True, text=True, timeout=10)
            events = (root / "events").read_text().splitlines() if (root / "events").exists() else []
            model = json.loads((root / "model.json").read_text())
            if (root / "timed-pid").exists():
                pid = int((root / "timed-pid").read_text())
                try:
                    os.kill(pid, 0)
                    model["timeout_child_absent"] = False
                except ProcessLookupError:
                    model["timeout_child_absent"] = True
            return result, events, before, self.snapshot(root), model

    @staticmethod
    def snapshot(root):
        result = {}
        for name in ("init", "state", "helper", "profile", "unrelated", "default-other"):
            path = root / name
            paths = [path] + (list(path.rglob("*")) if path.is_dir() else [])
            for item in paths:
                if not item.exists() and not item.is_symlink():
                    continue
                info = item.lstat()
                payload = (os.readlink(item) if stat.S_ISLNK(info.st_mode) else
                           item.read_bytes() if stat.S_ISREG(info.st_mode) else None)
                result[str(item.relative_to(root))] = (info.st_mode, info.st_ino, payload)
        return result

    def test_stop_and_disable_refusals_retain_every_local_file(self):
        for fault in ("refused-stop", "refused-disable", "ignored-stop", "ignored-disable", "reactivation"):
            with self.subTest(fault=fault):
                result, events, before, after, _ = self.exercise(fault=fault)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("init:remove", events)

    def test_failed_partial_or_diagnostic_observations_are_not_replaced_by_another_read(self):
        for fault in ("failed-inventory", "empty-inventory", "duplicate-inventory", "blank-inventory",
                      "diagnostic-inventory", "invalid-level", "duplicate-level", "failed-status", "empty-status",
                      "unknown-status", "duplicate-status", "blank-status", "diagnostic-status", "wrong-started-exit"):
            with self.subTest(fault=fault):
                result, events, before, after, model = self.exercise(fault=fault)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("init:remove", events)
                self.assertLessEqual(model["inventory_reads"], 1)
                self.assertLessEqual(model["status_reads"], 1)

    def test_independent_post_stop_and_runlevel_reads_are_mandatory(self):
        for fault in ("failed-final-status", "failed-final-inventory", "ignored-disable", "reactivation"):
            with self.subTest(fault=fault):
                result, events, before, after, _ = self.exercise(fault=fault)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("init:remove", events)

    def test_refused_manager_cannot_start_remote_or_local_removal(self):
        for fault in ("refused-stop", "refused-disable", "failed-status", "failed-inventory", "reactivation"):
            with self.subTest(fault=fault):
                result, events, before, after, _ = self.exercise(fault=fault, flow="caller")
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(after, before)
                for forbidden in ("remote:remove", "process:stop", "state:remove"):
                    self.assertNotIn(forbidden, events)

    def test_quiescence_precedes_server_process_and_state_mutation(self):
        result, events, _, after, model = self.exercise(flow="caller")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertLess(events.index("runlevels:disable-all"), events.index("remote:remove"))
        observations = [i for i, e in enumerate(events) if e == "service:status"]
        self.assertEqual(len(observations), 4)
        self.assertLess(observations[1], events.index("remote:remove"))
        self.assertLess(events.index("remote:remove"), observations[2])
        self.assertLess(observations[3], events.index("init:remove"))
        self.assertLess(events.index("init:remove"), events.index("state:remove"))
        self.assertLess(events.index("remote:remove"), events.index("state:remove"))
        self.assertNotIn("state/token", after)
        self.assertEqual(model["state"], "stopped")
        self.assertEqual(model["levels"], [])

    def test_server_refusal_keeps_stopped_disabled_recovery_without_restart(self):
        result, events, before, after, model = self.exercise(flow="caller", reply="refused")
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(after, before)
        self.assertEqual(model["state"], "stopped")
        self.assertEqual(model["levels"], [])
        self.assertNotIn("state:remove", events)
        self.assertNotIn("process:stop", events)

    def test_late_manager_refusal_keeps_recovery_after_confirmed_remote_removal(self):
        for fault in ("failed-status-after-remote", "refused-stop-after-remote"):
            with self.subTest(fault=fault):
                result, events, before, after, _ = self.exercise(flow="caller", fault=fault)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(after, before)
                self.assertEqual(events.count("remote:remove"), 1)
                for forbidden in ("state:remove", "process:stop", "init:remove"):
                    self.assertNotIn(forbidden, events)

    def test_known_stopped_state_and_every_runlevel_allow_only_selected_teardown(self):
        for state, levels in (("started", ("default",)), ("started", ("custom", "boot", "default")),
                              ("stopped", ("custom",)), ("stopped", ())):
            with self.subTest(state=state, levels=levels):
                result, events, before, after, model = self.exercise(state=state, levels=levels)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(after, {k: v for k, v in before.items() if k != "init"})
                self.assertEqual(model["state"], "stopped")
                self.assertEqual(model["levels"], [])
                self.assertEqual(events.count("service:stop"), int(state == "started"))
                self.assertEqual(events.count("runlevels:disable-all"), int(bool(levels)))

    def test_absent_and_repeated_removal_need_manager_and_process_absence(self):
        result, events, before, after, _ = self.exercise(path="missing")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(after, before)
        self.assertIn("process:observe", events)
        result, events, _, _, _ = self.exercise(repeat=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(events.count("service:stop"), 1)
        self.assertEqual(events.count("runlevels:disable-all"), 1)
        for fault in ("missing-script-listed", "orphan-process", "failed-process", "diagnostic-process", "blank-process"):
            with self.subTest(fault=fault):
                result, _, before, after, _ = self.exercise(path="missing", fault=fault)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(after, before)

    def test_unexpected_service_paths_refuse_before_stop_or_deletion(self):
        for path in ("linked", "directory", "not-executable"):
            with self.subTest(path=path):
                result, events, before, after, _ = self.exercise(path=path)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("service:stop", events)
                self.assertNotIn("init:remove", events)

    def test_source_selection_and_caller_order_have_one_bounded_observation_per_operation(self):
        selected = definitions(self.source)
        for name in FUNCTIONS:
            self.assertEqual(selected.count(name + "() {"), 1)
        for command in ("rc-service", "rc-update", "pgrep"):
            self.assertNotRegex(selected, r"(?m)^\s*" + command + r"\s")
        self.assertIn('timeout -k 1 5 rc-service', selected)
        self.assertIn('timeout -k 1 5 rc-update', selected)
        self.assertIn('timeout -k 1 5 pgrep', selected)
        entry = self.source.split("# --- Uninstall Logic ---\n", 1)[1]
        admission = entry.index("quiesce_openrc_agent_for_removal ||")
        for later in ("if ! uninstall_collector_registration;", "pkill -f", 'remove_agent_state_dir "$STATE_DIR"'):
            self.assertLess(admission, entry.index(later))
        teardown = entry.index("        teardown_openrc_agent_service\n")
        self.assertLess(entry.index("if ! uninstall_collector_registration;"), teardown)
        self.assertLess(teardown, entry.index('remove_agent_state_dir "$STATE_DIR"'))
        print("OPENRC_SOURCE_SHA256=" + hashlib.sha256(self.source.encode()).hexdigest())
        print("OPENRC_DEFINITIONS_SHA256=" + hashlib.sha256(selected.encode()).hexdigest())

    def test_real_status_timeout_stops_its_owned_child_without_erasure(self):
        started = time.monotonic()
        result, events, before, after, model = self.exercise(fault="timeout-status")
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(before, after)
        self.assertNotIn("init:remove", events)
        self.assertTrue(model["timeout_child_absent"])
        self.assertGreaterEqual(time.monotonic() - started, 4.8)
        self.assertLess(time.monotonic() - started, 9)


if __name__ == "__main__":
    unittest.main(verbosity=2)
