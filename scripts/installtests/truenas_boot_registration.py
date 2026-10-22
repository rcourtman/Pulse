#!/usr/bin/env python3
"""Exercise the shipped TrueNAS registration/completion caller with closed middleware.

No appliance, privileged installer, native service, network or real credential.
The original shell caller and production functions run against private files;
only middleware, runtime readiness and platform service operations are models.
"""
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_TRUENAS_BOOT_SOURCE", ROOT / "scripts/install.sh"))
FUNCTIONS = ("read_truenas_agent_removal_tasks", "register_truenas_agent_boot_task", "complete_installation_flow")


def definitions(source):
    selected = []
    for name in FUNCTIONS:
        marker = name + "() {"
        if marker not in source:  # Exact-parent adverse control.
            continue
        if source.count(marker) != 1:
            raise ValueError("ambiguous production function " + name)
        selected.append(marker + source.split(marker, 1)[1].split("\n}\n", 1)[0] + "\n}\n")
    return "\n".join(selected)


MODEL = r'''
import json, os, pathlib, sys, time
root = pathlib.Path(os.environ['FIXTURE'])
path = root / 'model.json'
v = json.loads(path.read_text())
args, fault = sys.argv[1:], v['fault']
def save(): path.write_text(json.dumps(v))
def event(s):
    with (root / 'events').open('a') as f: f.write(s + '\n')
assert args[0] == 'call', args
verb = args[1]
if verb == 'initshutdownscript.query':
    assert len(args) in (2, 3), args
    v['queries'] += 1; event('task:query'); save()
    if fault == 'timeout-query':
        (root / 'timed-pid').write_text(str(os.getpid())); time.sleep(30)
    if fault == 'failed-query' or (fault == 'failed-readback' and v['queries'] > 1):
        print('synthetic-private-middleware-error', file=sys.stderr); sys.exit(23)
    if fault == 'empty-query': sys.exit(0)
    if fault == 'malformed-query': print('not-json'); sys.exit(0)
    if fault == 'object-query': print('{}'); sys.exit(0)
    if fault == 'duplicate-field': print('[{"id":17,"id":18}]'); sys.exit(0)
    if fault == 'oversized-query': print(' ' * 1048577 + '[]'); sys.exit(0)
    tasks = v['tasks']
    if len(args) == 3:
        filters = json.loads(args[2]); assert filters[0][:2] == ['script', '='], filters
        tasks = [t for t in tasks if t['script'] == filters[0][2]]
    if fault == 'diagnostic-query': print('synthetic-private-middleware-error', file=sys.stderr)
    print(json.dumps(tasks)); sys.exit(0)
assert verb in ('initshutdownscript.create', 'initshutdownscript.update'), args
event('task:' + verb.rsplit('.', 1)[1]); v['mutations'] += 1
if fault == 'timeout-mutation':
    save(); (root / 'timed-pid').write_text(str(os.getpid())); time.sleep(30)
if fault == 'refused-mutation': save(); sys.exit(23)
payload = json.loads(args[-1])
assert payload == dict(type='SCRIPT', script=str(root / 'state/bootstrap-pulse-agent.sh'),
                       when='POSTINIT', enabled=True, timeout=30, comment='Pulse Agent Bootstrap'), payload
if verb == 'initshutdownscript.create':
    assert len(args) == 3, args
    task = dict(payload, id=17)
    if fault != 'ignored-mutation': v['tasks'].append(task)
else:
    assert len(args) == 4 and args[2] == '17', args
    task = next(t for t in v['tasks'] if t['id'] == 17)
    if fault != 'ignored-mutation': task.update(payload)
if fault == 'wrong-readback-path': task['script'] = '/unrelated/unchanged.sh'
if fault == 'disabled-readback': task['enabled'] = False
if fault == 'changed-readback-id': task['id'] = 18
if fault == 'missing-readback': v['tasks'] = [t for t in v['tasks'] if t['id'] == 99]
if fault == 'duplicate-readback': v['tasks'].append(dict(task, id=18))
save()
if fault == 'uncertain-mutation': sys.exit(23)  # State may already have changed; never replay.
print(json.dumps(task))
'''


class TrueNASBootRegistration(unittest.TestCase):
    source = SOURCE.read_text()

    def exercise(self, fault='', existing=False, enabled=True, platform='Linux',
                 health=0, output='text', repeat=False, quoted=False):
        with tempfile.TemporaryDirectory(prefix=('pulse boot \"quoted\" \\ ' if quoted else 'pulse-boot-')) as temporary:
            root = Path(temporary)
            tools = root / 'tools'; tools.mkdir()
            state = root / 'state'; state.mkdir()
            for leaf in ('runtime.token', 'agent-id', 'connection.env', 'pulse-agent', 'pulse-agent.service', 'bootstrap-pulse-agent.sh'):
                (state / leaf).write_text('synthetic retained recovery bytes\n')
            before = {p.name: p.read_bytes() for p in state.iterdir()}
            task = dict(id=17, type='SCRIPT', script=str(state / 'bootstrap-pulse-agent.sh'), command='',
                        when='POSTINIT', enabled=enabled, timeout=30, comment='Pulse Agent Bootstrap')
            tasks = [task] if existing else []
            if fault == 'duplicate-existing': tasks.append(dict(task, id=18))
            if fault == 'foreign-owner': task['comment'] = 'not Pulse owned'
            if fault == 'wrong-kind': task.update(type='COMMAND', command='/other/command')
            if fault == 'bool-id': task['id'] = True
            if fault == 'string-enabled': task['enabled'] = 'true'
            if fault == 'wrong-timing': task['when'] = 'PREINIT'
            if fault == 'wrong-timeout': task['timeout'] = 31
            foreign = dict(id=99, type='SCRIPT', script='/unrelated/unchanged.sh', command='',
                           when='SHUTDOWN', enabled=True, timeout=60, comment='unrelated task')
            tasks.append(foreign)
            model = dict(fault=fault, tasks=tasks, queries=0, mutations=0)
            (root / 'model.json').write_text(json.dumps(model))
            model_script = root / 'manager.py'; model_script.write_text(MODEL)
            midclt = tools / 'midclt'
            midclt.write_text('#!/bin/sh\nexec ' + shlex.quote(sys.executable) + ' -I -S ' + shlex.quote(str(model_script)) + ' "$@"\n')
            midclt.chmod(0o700)
            caller = self.source.split('    # Register Init/Shutdown task using midclt\n', 1)[1]
            caller = caller.split('\nfi\n\n# 5. OpenRC', 1)[0]
            # The actual completion function and entire caller remain unchanged.
            harness = definitions(self.source) + r'''
set -euo pipefail
log_info() { printf '[INFO] %s\n' "$1"; }
log_warn() { printf '[WARN] %s\n' "$1"; }
log_error() { printf '[ERROR] %s\n' "$1"; }
json_event() { if [[ "$OUTPUT_FORMAT" == json ]]; then printf '{"phase":"%s","code":"%s","message":"%s","exitCode":%d}\n' "$1" "$2" "$3" "${4:-0}"; fi; }
save_connection_info() { printf 'connection:saved\n' >> "$FIXTURE/events"; }
verify_agent_started() { printf 'runtime:verified\n' >> "$FIXTURE/events"; return "$FIXTURE_HEALTH"; }
report_proxmox_registration_outcome() { return 0; }
restart_systemd_agent_service() { printf 'runtime:start:Linux\n' >> "$FIXTURE/events"; }
ensure_freebsd_agent_enabled() { printf 'runtime:enable:FreeBSD\n' >> "$FIXTURE/events"; }
restart_service_command_agent() { printf 'runtime:start:FreeBSD\n' >> "$FIXTURE/events"; }
uname() { printf '%s\n' "$FIXTURE_PLATFORM"; }
command() {
    if [[ "$*" == "-v ${FIXTURE_MISSING_TOOL:-never}" ]]; then return 1; fi
    builtin command "$@"
}
TRUENAS_STATE_DIR="$FIXTURE/state"
TRUENAS_BOOTSTRAP_SCRIPT="$TRUENAS_STATE_DIR/bootstrap-pulse-agent.sh"
TRUENAS_STORED_BINARY="$TRUENAS_STATE_DIR/pulse-agent"
TRUENAS_RUNTIME_BINARY="$TRUENAS_STORED_BINARY"
TRUENAS_SERVICE_STORAGE="$TRUENAS_STATE_DIR/pulse-agent.service"
TRUENAS_LOG_FILE="$TRUENAS_STATE_DIR/agent.log"
AGENT_LOG_FILE="$TRUENAS_LOG_FILE"
CONNECTION_INFO_PERSISTED=false
UPGRADE_MODE=false
SAVED_INSTALL_SCRIPT=''
EXIT_GENERAL=1
EXIT_AUTH_REJECTED=18
EXIT_SERVICE_START_FAILED=13
''' + '\n' + caller
            missing = fault.removeprefix('missing-') if fault.startswith('missing-') else ''
            env = dict(os.environ, FIXTURE=str(root), FIXTURE_HEALTH=str(health), FIXTURE_PLATFORM=platform,
                       FIXTURE_MISSING_TOOL=missing, OUTPUT_FORMAT=output, PATH=str(tools) + os.pathsep + os.environ['PATH'])
            results = []
            for _ in range(2 if repeat else 1):
                result = subprocess.run(['bash', '-c', harness], env=env, capture_output=True, text=True, timeout=25)
                results.append(result)
            final = json.loads((root / 'model.json').read_text())
            events = (root / 'events').read_text().splitlines() if (root / 'events').exists() else []
            self.assertEqual(before, {p.name: p.read_bytes() for p in state.iterdir()}, 'recovery/state bytes changed')
            self.assertIn(foreign, final['tasks'], 'unrelated task changed')
            for result in results:
                self.assertNotIn('synthetic-private-middleware-error', result.stdout + result.stderr)
            if (root / 'timed-pid').exists():
                pid = int((root / 'timed-pid').read_text())
                with self.assertRaises(ProcessLookupError, msg='middleware child not stopped'):
                    os.kill(pid, 0)
            return results, final, events

    def assert_incomplete(self, results, events, platform='Linux', health=0):
        result = results[-1]
        self.assertEqual(result.returncode, {0: 1, 1: 13, 2: 18}[health], result.stdout + result.stderr)
        self.assertIn('runtime:start:' + platform, events)
        self.assertIn('connection:saved', events)
        self.assertIn('runtime:verified', events)
        self.assertNotIn('Installation complete!', result.stdout)
        self.assertNotIn('ensures the agent survives', result.stdout)
        if health == 0:
            self.assertIn('boot task was not confirmed', result.stdout)
            self.assertIn('connection state were retained', result.stdout)

    def test_failed_query_is_not_absence_and_does_not_create(self):
        results, final, events = self.exercise('failed-query')
        self.assert_incomplete(results, events)
        self.assertEqual(final['mutations'], 0)

    def test_create_update_and_noop_are_read_back_on_both_platforms(self):
        for platform in ('Linux', 'FreeBSD'):
            for existing, enabled in ((False, True), (True, False), (True, True)):
                with self.subTest(platform=platform, existing=existing, enabled=enabled):
                    results, final, events = self.exercise(existing=existing, enabled=enabled, platform=platform, quoted=True)
                    self.assertEqual(results[0].returncode, 0, results[0].stdout + results[0].stderr)
                    self.assertIn('Installation complete!', results[0].stdout)
                    self.assertEqual(final['mutations'], 0 if existing and enabled else 1)
                    self.assertEqual(final['queries'], 2)
                    self.assertIn('runtime:start:' + platform, events)
                    self.assertEqual(sum(t['script'].endswith('bootstrap-pulse-agent.sh') for t in final['tasks']), 1)

    def test_invalid_inventory_cannot_mutate_or_claim_complete(self):
        for fault in ('empty-query', 'malformed-query', 'object-query', 'duplicate-field', 'oversized-query',
                      'diagnostic-query', 'duplicate-existing', 'foreign-owner', 'wrong-kind', 'bool-id',
                      'string-enabled', 'wrong-timing', 'wrong-timeout', 'missing-midclt', 'missing-python3', 'missing-timeout'):
            with self.subTest(fault=fault):
                results, final, events = self.exercise(fault, existing=True)
                self.assert_incomplete(results, events)
                self.assertEqual(final['mutations'], 0)

    def test_mutation_failure_or_changed_readback_never_replays(self):
        for existing in (False, True):
            for fault in ('refused-mutation', 'ignored-mutation', 'uncertain-mutation', 'failed-readback',
                          'wrong-readback-path', 'disabled-readback', 'missing-readback', 'duplicate-readback'):
                with self.subTest(existing=existing, fault=fault):
                    results, final, events = self.exercise(fault, existing=existing, enabled=False)
                    self.assert_incomplete(results, events)
                    self.assertEqual(final['mutations'], 1)
        results, final, events = self.exercise('changed-readback-id', existing=True, enabled=False)
        self.assert_incomplete(results, events)
        self.assertEqual(final['mutations'], 1)

    def test_timeout_does_not_replay_and_child_is_stopped(self):
        for fault in ('timeout-query', 'timeout-mutation'):
            with self.subTest(fault=fault):
                results, final, events = self.exercise(fault)
                self.assert_incomplete(results, events)
                self.assertEqual(final['mutations'], int(fault == 'timeout-mutation'))

    def test_repetition_keeps_one_task_and_existing_state(self):
        results, final, events = self.exercise(repeat=True)
        self.assertEqual([r.returncode for r in results], [0, 0])
        self.assertEqual(final['mutations'], 1)
        self.assertEqual(final['queries'], 4)

    def test_partial_persistence_is_machine_readable_and_health_stays_distinct(self):
        for health in (0, 1, 2):
            with self.subTest(health=health):
                results, final, events = self.exercise('failed-query', health=health, output='json')
                self.assert_incomplete(results, events, health=health)
                records = [json.loads(line) for line in results[0].stdout.splitlines() if line.startswith('{')]
                self.assertEqual(len(records), 1)
                self.assertEqual(records[0]['code'], {0: 'persistence_unconfirmed', 1: 'installed_unhealthy', 2: 'auth_rejected'}[health])


if __name__ == '__main__':
    unittest.main()
