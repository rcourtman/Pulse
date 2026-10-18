#!/usr/bin/env python3
"""TrueNAS SCALE uninstall, using production functions and private manager models.

Only fixed vendor/service paths are redirected into owned temporary files.
No appliance, root installer, network, credential or native acceptance is used.
The complete uninstall caller runs; middleware/process/systemd are closed models.
"""
import json
import os
from pathlib import Path
import shlex
import stat
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_AGENT_TRUENAS_SOURCE", ROOT / "scripts/install.sh"))
FUNCTIONS = (
    "portable_path_uid", "portable_path_mode", "trusted_lifecycle_regular_file",
    "read_agent_removal_unit_state", "confirm_agent_unit_quiescent",
    "quiesce_agent_units_for_removal", "teardown_systemd_agent_service",
    "read_legacy_agent_removal_processes", "legacy_agent_processes_absent",
    "truenas_scale_agent_removal_present", "read_truenas_agent_removal_tasks",
    "quiesce_truenas_agent_boot_tasks_for_removal", "confirm_truenas_agent_removal_quiescent",
    "teardown_truenas_agent_boot_tasks",
)


def definitions(source):
    selected = []
    for name in FUNCTIONS:
        marker = name + "() {"
        if marker not in source:  # The exact-parent adverse comparison.
            continue
        if source.count(marker) != 1:
            raise ValueError("ambiguous production function " + name)
        selected.append(marker + source.split(marker, 1)[1].split("\n}\n", 1)[0] + "\n}\n")
    return "\n".join(selected)


MODEL = r'''#!/usr/bin/python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['FIXTURE'])
path = root / 'model.json'
v = json.loads(path.read_text())
role, args = pathlib.Path(sys.argv[0]).name, sys.argv[1:]
fault = v['fault']
def save():
    pending = root / 'model.pending'
    pending.write_text(json.dumps(v)); pending.replace(path)
def event(s):
    with (root / 'events').open('a') as f: f.write(s + '\n')
remote = (root / 'events').exists() and 'remote:remove' in (root / 'events').read_text()
if role == 'ps':
    assert args == ['-ax', '-o', 'pid=', '-o', 'ppid=', '-o', 'comm=', '-o', 'args='], args
    event('process:read')
    if fault == 'failed-process': sys.exit(1)
    if fault == 'empty-process': sys.exit(0)
    print('1 0 init /sbin/init')
    if fault == 'duplicate-process': print('1 0 init /sbin/init')
    if v['active'] == 'active' or fault == 'orphan-agent':
        print('100 1 pulse-agent /usr/local/bin/pulse-agent --token-file /synthetic/recovery-only')
    if fault == 'live-bootstrap': print('101 1 bash /bin/bash ' + str(root / 'state/bootstrap-pulse-agent.sh'))
    if fault == 'diagnostic-process': print('read unavailable', file=sys.stderr)
    sys.exit(0)
if role == 'systemctl':
    verb = args[0]
    if verb == 'daemon-reload': event('unit:reload'); sys.exit(0)
    assert len(args) >= 2, args
    unit = args[1]
    assert unit in ('pulse-agent.service', 'pulse-agent-helper.service', 'pulse-agent-helper.socket', 'pulse-agent-runner.service'), args
    if verb == 'show':
        assert args[2:] == ['--property=LoadState,ActiveState,UnitFileState'], args
        event('unit:read:' + unit)
        if fault == 'failed-unit-read': sys.exit(1)
        present = unit == 'pulse-agent.service' and (root / 'unit/pulse-agent.service').exists()
        if present:
            print('LoadState=loaded\nActiveState=' + v['active'] + '\nUnitFileState=' + v['enabled'])
        else: print('LoadState=not-found\nActiveState=inactive\nUnitFileState=')
        sys.exit(0)
    assert verb in ('stop', 'disable'), args
    event('unit:' + verb)
    if fault == 'refused-stop' and verb == 'stop': sys.exit(1)
    if verb == 'stop' and fault != 'ignored-stop': v['active'] = 'inactive'
    if verb == 'stop' and fault == 'reactivated-task-on-stop': v['tasks'][0]['enabled'] = True
    if verb == 'disable': v['enabled'] = 'disabled'
    save(); sys.exit(0)
assert role == 'midclt' and args[0] == 'call', (role, args)
verb = args[1]
if verb == 'initshutdownscript.query':
    assert len(args) in (2, 3), args
    event('task:query'); v['queries'] += 1
    if fault == 'timeout-query':
        save(); (root / 'timed-pid').write_text(str(os.getpid()))
        os.execl('/bin/sleep', 'sleep', '30')
    if fault == 'failed-query' or (fault == 'failed-after-remote-query' and remote): save(); sys.exit(1)
    if fault == 'new-task-after-remote' and remote and not any(t['id'] == 3 for t in v['tasks']):
        v['tasks'].append(dict(v['tasks'][0], id=3, enabled=True))
    if fault == 'reactivated-task' and v['updates'] and v['tasks']: v['tasks'][0]['enabled'] = True
    if fault == 'recreated-task' and v['deletes'] and not any(t['script'] == str(root / 'state/bootstrap-pulse-agent.sh') for t in v['tasks']):
        v['tasks'].append(dict(id=3, type='SCRIPT', script=str(root / 'state/bootstrap-pulse-agent.sh'), command='', when='POSTINIT', enabled=True, timeout=30, comment='Pulse Agent Bootstrap'))
    save()
    tasks = v['tasks']
    if len(args) == 3:  # Preserve the parent's exact filter, not a new graph.
        filters = json.loads(args[2]); assert filters[0][:2] == ['script', '='], filters
        tasks = [t for t in tasks if t['script'] == filters[0][2]]
    if fault == 'empty-query': sys.exit(0)
    if fault == 'malformed-query': print('not-json'); sys.exit(0)
    if fault == 'object-query': print('{}'); sys.exit(0)
    if fault == 'duplicate-field': print('[{"id":1,"id":2}]'); sys.exit(0)
    if fault == 'oversized-query': print(' ' * 1048577 + '[]'); sys.exit(0)
    if fault == 'oversized-inventory': tasks = [dict(v['tasks'][0], id=i + 1) for i in range(513)]
    if fault == 'duplicate-id': tasks = tasks + [tasks[0]]
    if fault == 'bool-id': tasks[0]['id'] = True
    if fault == 'string-enabled': tasks[0]['enabled'] = 'false'
    if fault == 'foreign-owner': tasks[0]['comment'] = 'not owned by Pulse'
    if fault == 'wrong-kind': tasks[0]['type'] = 'UNKNOWN'
    if fault == 'missing-script': tasks[0].pop('script')
    if fault == 'null-script': tasks[0]['script'] = None
    if fault == 'empty-command': tasks[0].update(type='COMMAND', script='', command='')
    if fault == 'command-bootstrap': tasks[0].update(type='COMMAND', script='', command='/bin/bash ' + str(root / 'state/bootstrap-pulse-agent.sh'))
    if fault == 'other-bootstrap': tasks[0]['script'] = '/other-install/bootstrap-pulse-agent.sh'
    if fault == 'diagnostic-query': print('read unavailable', file=sys.stderr)
    print(json.dumps(tasks)); sys.exit(0)
if verb == 'initshutdownscript.update':
    assert len(args) == 4 and json.loads(args[3]) == {'enabled': False}, args
    ident = int(args[2]); event('task:disable:' + str(ident)); v['updates'] += 1
    if fault == 'refused-disable' or (fault == 'partial-disable' and ident == 2): save(); sys.exit(1)
    task = next(t for t in v['tasks'] if t['id'] == ident)
    if fault != 'ignored-disable': task['enabled'] = False
    save(); print(json.dumps(task)); sys.exit(0)
assert verb == 'initshutdownscript.delete' and len(args) == 3, args
ident = int(args[2]); event('task:delete:' + str(ident)); v['deletes'] += 1
if fault == 'refused-delete' or (fault == 'partial-delete' and ident == 2): save(); sys.exit(1)
if fault != 'ignored-delete': v['tasks'] = [t for t in v['tasks'] if t['id'] != ident]
if fault == 'unit-reactivation-on-delete': v['active'] = 'active'
save(); print('true')
'''


class AgentTrueNASTeardown(unittest.TestCase):
    source = SOURCE.read_text()

    def exercise(self, fault="", reply="confirmed", bootstrap="regular", tasks="duplicate",
                 appliance=True, repeat=False, flow="caller"):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            tools = root / "tools"; tools.mkdir()
            models = root / "models"; models.mkdir()
            for name in ("midclt", "ps", "systemctl"):
                model = models / name; model.write_text(MODEL)
                # These closed stdlib models share the interpreter selected by
                # the containing suite. The host's /usr/bin/python3 may differ
                # and pays its site startup cost on every observed operation.
                file = tools / name
                file.write_text("#!/bin/sh\nexec " + shlex.quote(sys.executable)
                                + " -I -S " + shlex.quote(str(model)) + ' "$@"\n')
                file.chmod(0o700)
            if fault == "missing-midclt": (tools / "midclt").unlink()
            for name in ("state", "helper", "profile", "unit"):
                directory = root / name; directory.mkdir(mode=0o700)
                if name != "unit":
                    for file in ("runtime.token", "agent-id", "connection.env", "install.sh"):
                        (directory / file).write_text("synthetic recovery only\n")
            script = root / "state/bootstrap-pulse-agent.sh"
            unrelated = root / "unrelated"; unrelated.write_text("do not alter\n")
            if bootstrap == "linked": script.symlink_to(unrelated)
            elif bootstrap == "directory": script.mkdir()
            elif bootstrap != "missing":
                script.write_text("#!/bin/bash\n# Pulse Agent Bootstrap for TrueNAS\n")
                script.chmod(0o777 if bootstrap == "writable" else 0o755)
            vendor = root / "truenas-version"
            if appliance: vendor.write_text("TrueNAS-SCALE\n")
            (root / "version").write_text("unrelated platform\n")
            (root / "unit/pulse-agent.service").write_text("owned service\n")
            owned = dict(id=1, type="SCRIPT", script=str(script), command="", when="POSTINIT",
                         enabled=True, timeout=30, comment="Pulse Agent Bootstrap")
            foreign = dict(owned, id=99, script="/unrelated/boot.sh", comment="unrelated task")
            selected = [] if tasks == "absent" else [owned]
            if tasks == "duplicate": selected.append(dict(owned, id=2))
            if tasks == "disabled": selected[0]["enabled"] = False
            selected.append(foreign)
            model = dict(fault=fault, tasks=selected, updates=0, deletes=0, queries=0,
                         active="active", enabled="enabled")
            (root / "model.json").write_text(json.dumps(model))
            before = self.snapshot(root)

            def redirect(source):
                for path, target in (
                    ("/etc/truenas-version", vendor), ("/etc/version", root / "version"),
                    ("/data/ix-applications", root / "no-ix"), ("/etc/ix-apps.d", root / "no-apps"),
                    ("/etc/ix.rc.d", root / "no-rc"), ("/usr/syno", root / "no-dsm"),
                    ("/etc/unraid-version", root / "no-unraid"),
                    ("/boot/config/plugins/pulse-agent", root / "no-plugin"),
                    ("/etc/config/qpkg.conf", root / "no-qnap"),
                    ("/sbin/getcfg", root / "no-getcfg"),
                    ("/var/log/pulse-agent.log", root / "absent-log"),
                    ("/etc/systemd/system/${AGENT_NAME}.service", root / "unit/pulse-agent.service"),
                ): source = source.replace(path, str(target))
                return source

            values = dict(FIXTURE=root, EVENTS=root / "events", STATE_DIR=root / "state",
                          INSTALL_DIR=root / "bin",
                          STATE_DIR_REMOVAL_AUTHORITY=root / "state",
                          TRUENAS_STATE_DIR=root / "state", TRUENAS_BOOTSTRAP_SCRIPT=script,
                          PRIVILEGED_HELPER_STATE_DIR=root / "helper", SAFE_PROFILE_STATE_DIR=root / "profile",
                          PRIVILEGE_SUDOERS_FILE=root / "absent-sudoers", PRIVILEGE_HELPER_DIR=root / "absent-helper",
                          REPLY=reply, FAULT=fault)
            setup = "\n".join(k + "=" + shlex.quote(str(v)) for k, v in values.items())
            shell = r'''
set -euo pipefail
UNINSTALL=true AGENT_NAME=pulse-agent BINARY_NAME=pulse-agent INSTALL_DIR=/usr/local/bin
EXIT_GENERAL=1 EXIT_MISSING_ARGS=16 PULSE_URL=https://same-instance.example.invalid AGENT_ID=synthetic
ACTION_RUNNER_NAME=pulse-agent-runner PRIVILEGED_HELPER_NAME=pulse-agent-helper
log_info() { :; }; log_warn() { :; }
fail() { printf '%s\n' "$1" >&2; exit "$2"; }
uname() { printf 'Linux\n'; }
command() {
    case "$*" in
        '-v rc-service') return 1 ;;
        '-v midclt') [[ "$FAULT" != missing-midclt ]] || return 1 ;;
        '-v python3') [[ "$FAULT" != missing-python ]] || return 1 ;;
        '-v timeout') [[ "$FAULT" != missing-timeout ]] || return 1 ;;
    esac
    builtin command "$@"
}
collector_credential_state_present() { [[ -e "$STATE_DIR/runtime.token" ]]; }
uninstall_collector_registration() { printf 'remote:remove\n' >> "$EVENTS"; [[ "$REPLY" == confirmed ]]; }
teardown_action_runner_service() { :; }
teardown_privileged_helper_service() { :; }
find_qnap_state_dir() { :; }
pkill() { printf 'unchecked:process-stop\n' >> "$EVENTS"; }
sleep() { :; }
rm() {
    local arg
    for arg in "$@"; do [[ "$arg" == -* || "$arg" == "$FIXTURE/"* ]] || return 97; done
    printf 'file:remove\n' >> "$EVENTS"
    builtin command rm "$@"
}
remove_agent_state_dir() { printf 'state:remove\n' >> "$EVENTS"; rm -rf -- "$STATE_DIR"; }
remove_privileged_helper_state_dir() { rm -rf -- "$PRIVILEGED_HELPER_STATE_DIR"; }
remove_safe_profile_state_dir() { rm -rf -- "$SAFE_PROFILE_STATE_DIR"; }
'''
            entry = self.source.split("# --- Uninstall Logic ---\n", 1)[1].split("# --- Validation ---\n", 1)[0]
            selected = definitions(self.source)
            if flow == "process": entry = 'read_legacy_agent_removal_processes "$TRUENAS_BOOTSTRAP_SCRIPT"\n'
            executable = shell + "\n" + setup + "\n" + redirect(selected) + "\nmain() {\n" + redirect(entry) + "}\nmain\n"
            env = dict(os.environ, FIXTURE=str(root), PATH=str(tools) + ":" + os.environ["PATH"])
            result = subprocess.run(["bash", "-c", executable], env=env, capture_output=True, text=True, timeout=12)
            second = subprocess.run(["bash", "-c", executable], env=env, capture_output=True, text=True, timeout=12) if repeat else None
            events = (root / "events").read_text() if (root / "events").exists() else ""
            after = self.snapshot(root)
            state = json.loads((root / "model.json").read_text())
            timed_stopped = not (root / "timed-pid").exists() or not Path('/proc/' + (root / "timed-pid").read_text()).exists()
            return result, second, events.splitlines(), before, after, state, timed_stopped

    @staticmethod
    def snapshot(root):
        result = {}
        for path in root.rglob("*"):
            if path.parts[len(root.parts)] not in ("state", "helper", "profile", "unit", "unrelated"):
                continue
            mode = path.lstat().st_mode
            contents = os.readlink(path) if stat.S_ISLNK(mode) else path.read_bytes() if stat.S_ISREG(mode) else None
            result[str(path.relative_to(root))] = (mode, contents, path.lstat().st_ino)
        return result

    def retained(self, result):
        execution, _, events, before, after, _, _ = result
        self.assertEqual(execution.returncode, 1, execution.stdout + execution.stderr)
        self.assertNotIn('state:remove', events)
        for path in before:
            if path.startswith(('state', 'helper', 'profile')) or path == 'unrelated': self.assertEqual(after.get(path), before[path], path)
        self.assertNotIn('synthetic recovery only', execution.stdout + execution.stderr)
        return events

    def test_unavailable_or_untrusted_task_inventory_cannot_remove_recovery(self):
        for fault in ('failed-query', 'empty-query', 'malformed-query', 'object-query', 'duplicate-field',
                      'diagnostic-query', 'duplicate-id', 'bool-id', 'string-enabled', 'wrong-kind',
                      'oversized-query', 'oversized-inventory'):
            with self.subTest(fault=fault):
                events = self.retained(self.exercise(fault))
                self.assertNotIn('remote:remove', events)
                self.assertFalse(any(e.startswith('task:disable:') or e.startswith('task:delete:') for e in events))

    def test_foreign_or_indirect_bootstrap_is_not_mutated(self):
        for fault in ('foreign-owner', 'command-bootstrap', 'other-bootstrap'):
            with self.subTest(fault=fault):
                events = self.retained(self.exercise(fault))
                self.assertNotIn('remote:remove', events)
                self.assertFalse(any(e.startswith('task:disable:') or e.startswith('task:delete:') for e in events))

    def test_missing_activation_target_cannot_hide_an_owned_task(self):
        for fault in ('missing-script', 'null-script', 'empty-command'):
            with self.subTest(fault=fault):
                events = self.retained(self.exercise(fault))
                self.assertNotIn('remote:remove', events)
                self.assertFalse(any(e.startswith('task:disable:') or e.startswith('task:delete:') for e in events))

    def test_bootstrap_symlink_directory_or_writable_file_refuses_before_mutation(self):
        for bootstrap in ('linked', 'directory', 'writable'):
            with self.subTest(bootstrap=bootstrap):
                events = self.retained(self.exercise(bootstrap=bootstrap))
                self.assertEqual(events, [])

    def test_missing_tools_are_not_local_only_removal_permission(self):
        for fault in ('missing-midclt', 'missing-python', 'missing-timeout'):
            with self.subTest(fault=fault):
                events = self.retained(self.exercise(fault))
                self.assertEqual(events, [])

    def test_every_matching_task_is_disabled_and_independently_observed_before_stop(self):
        result = self.exercise()
        self.assertEqual(result[0].returncode, 0, result[0].stderr)
        events = result[2]
        self.assertLess(events.index('task:disable:2'), events.index('unit:stop'))
        self.assertEqual(events[:4], ['task:query', 'task:disable:1', 'task:disable:2', 'task:query'])
        self.assertLess(events.index('process:read'), events.index('remote:remove'))

    def test_partial_refused_ignored_or_reactivated_disable_retains_recovery(self):
        for fault in ('refused-disable', 'partial-disable', 'ignored-disable', 'reactivated-task'):
            with self.subTest(fault=fault):
                events = self.retained(self.exercise(fault))
                self.assertNotIn('remote:remove', events)
                self.assertNotIn('unit:stop', events)

    def test_unit_or_process_uncertainty_keeps_stopped_task_recovery(self):
        for fault in ('failed-unit-read', 'refused-stop', 'ignored-stop', 'failed-process', 'empty-process',
                      'duplicate-process', 'diagnostic-process', 'orphan-agent', 'live-bootstrap'):
            with self.subTest(fault=fault):
                events = self.retained(self.exercise(fault))
                self.assertNotIn('remote:remove', events)

    def test_task_activation_during_systemd_stop_refuses_before_remote(self):
        result = self.exercise('reactivated-task-on-stop')
        events = self.retained(result)
        self.assertIn('unit:stop', events)
        self.assertNotIn('remote:remove', events)
        self.assertFalse(any(e.startswith('task:delete:') for e in events))

    def test_server_refusal_retains_disabled_tasks_and_all_recovery_without_restart(self):
        result = self.exercise(reply='refused')
        events = self.retained(result)
        self.assertIn('remote:remove', events)
        self.assertFalse(any(e.startswith('task:delete:') for e in events))
        self.assertTrue(all(not t['enabled'] for t in result[5]['tasks'] if t['id'] != 99))

    def test_changed_or_refused_post_remote_inventory_precedes_first_deletion(self):
        for fault in ('failed-after-remote-query', 'new-task-after-remote'):
            with self.subTest(fault=fault):
                events = self.retained(self.exercise(fault))
                self.assertIn('remote:remove', events)
                self.assertFalse(any(e.startswith('task:delete:') for e in events))

    def test_partial_refused_ignored_or_recreated_task_deletion_keeps_recovery(self):
        for fault in ('refused-delete', 'partial-delete', 'ignored-delete', 'recreated-task', 'unit-reactivation-on-delete'):
            with self.subTest(fault=fault): self.retained(self.exercise(fault))

    def test_all_owned_tasks_retire_before_state_erasure_and_unrelated_task_survives(self):
        result = self.exercise()
        self.assertEqual(result[0].returncode, 0, result[0].stderr)
        events = result[2]
        self.assertLess(events.index('task:delete:2'), events.index('state:remove'))
        self.assertIn('task:query', events[events.index('task:delete:2') + 1:events.index('state:remove')])
        self.assertEqual([t['id'] for t in result[5]['tasks']], [99])
        self.assertTrue(result[5]['tasks'][0]['enabled'])
        self.assertNotIn('state/runtime.token', result[4])
        self.assertEqual(result[4]['unrelated'], result[3]['unrelated'])

    def test_disabled_absent_missing_bootstrap_and_repeated_removal_are_observed(self):
        for tasks, bootstrap in (('disabled', 'regular'), ('absent', 'missing'), ('single', 'missing')):
            with self.subTest(tasks=tasks, bootstrap=bootstrap):
                result = self.exercise(tasks=tasks, bootstrap=bootstrap, repeat=True)
                self.assertEqual(result[0].returncode, 0, result[0].stderr)
                self.assertEqual(result[1].returncode, 0, result[1].stderr)
                self.assertEqual([t['id'] for t in result[5]['tasks']], [99])

    def test_ordinary_linux_state_is_not_appliance_evidence(self):
        result = self.exercise(bootstrap='missing', tasks='absent', appliance=False)
        self.assertEqual(result[0].returncode, 0, result[0].stderr)
        self.assertFalse(any(e.startswith('task:') for e in result[2]))

    def test_process_read_exposes_only_roles_and_real_timeout_child_is_stopped(self):
        result = self.exercise(fault='live-bootstrap', flow='process')
        self.assertEqual(result[0].returncode, 0, result[0].stderr)
        self.assertIn('101 1 bootstrap', result[0].stdout)
        self.assertNotIn('recovery-only', result[0].stdout + result[0].stderr)
        result = self.exercise(fault='timeout-query')
        self.retained(result)
        self.assertTrue(result[6], 'timed middleware child still exists')


if __name__ == '__main__':
    unittest.main(verbosity=2)
