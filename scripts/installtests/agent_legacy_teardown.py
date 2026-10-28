#!/usr/bin/env python3
"""Production FreeBSD/SysV removal, confined to ordinary-user private fixtures.

Manager/process observations are closed models. The real uninstall admission,
post-remote teardown and rendered stop functions execute; no host service,
credential, signal, system file, network or native installation is exercised.
"""
import hashlib
import json
import os
from pathlib import Path
import shlex
import stat
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_AGENT_LEGACY_SOURCE", ROOT / "scripts/install.sh"))
FUNCTIONS = ("truenas_scale_agent_removal_present", "read_legacy_agent_removal_processes", "legacy_agent_processes_absent",
             "validate_legacy_agent_removal_pidfiles", "read_legacy_agent_removal_status",
             "read_freebsd_agent_removal_enablement", "quiesce_freebsd_agent_for_removal",
             "teardown_freebsd_agent_service", "read_sysv_agent_removal_links",
             "quiesce_sysv_agent_for_removal", "teardown_sysv_agent_service")


def functions(source, names=FUNCTIONS):
    result = []
    for name in names:
        marker = name + "() {"
        if marker not in source:  # Exact previous-source adverse controls.
            continue
        if source.count(marker) != 1:
            raise ValueError("ambiguous production function " + name)
        result.append(marker + source.split(marker, 1)[1].split("\n}\n", 1)[0] + "\n}\n")
    return "\n".join(result)


MODEL = r'''
import json, os, pathlib, sys
root = pathlib.Path(os.environ['FIXTURE'])
path = root / 'model.json'
v = json.loads(path.read_text())
role, a = sys.argv[1], sys.argv[2:]
fault = v['fault']
remote = (root / 'events').exists() and 'remote:remove' in (root / 'events').read_text()
def save():
    # The production operation is bounded by timeout. A killed model writer
    # must not truncate the observation record used to judge retained files.
    pending = root / 'model.pending'
    pending.write_text(json.dumps(v))
    pending.replace(path)
def event(s):
    with (root / 'events').open('a') as f: f.write(s + '\n')
if role == 'ps':
    assert a == ['-ax', '-o', 'pid=', '-o', 'ppid=', '-o', 'comm=', '-o', 'args='], a
    event('process:read'); v['process_reads'] += 1; save()
    if fault == 'timeout-process':
        (root / 'timed-pid').write_text(str(os.getpid()))
        os.execl('/bin/sleep', 'sleep', '30')
    if fault == 'failed-process' or (fault == 'failed-final-process' and v['process_reads'] > 1): sys.exit(2)
    if fault == 'empty-process': sys.exit(0)
    print('1 0 init /sbin/init')
    if fault == 'duplicate-process': print('1 0 init /sbin/init')
    if v['state'] == 'started':
        if v['platform'] == 'FreeBSD':
            print('100 1 daemon daemon: /usr/local/bin/pulse-agent[101] (daemon)')
            print('101 100 pulse-agent /usr/local/bin/pulse-agent --token-file /private/synthetic')
        else: print('101 1 pulse-agent /usr/local/bin/pulse-agent --token-file /private/synthetic')
    if fault == 'orphan-supervisor' and v['state'] == 'stopped':
        print('100 1 daemon daemon: /usr/local/bin/pulse-agent[101] (daemon)')
    if fault == 'pid-reused': print('101 1 unrelated unrelated')
    if fault == 'diagnostic-process': print('read refused', file=sys.stderr)
    if fault == 'blank-process': print()
    sys.exit(0)
elif role == 'init':
    assert a in (['status'], ['onestatus'], ['stop'], ['onestop']), a
    if a[0] in ('stop', 'onestop'):
        event('service:stop')
        if fault == 'refused-stop' or (fault == 'refused-stop-after-remote' and remote): sys.exit(1)
        if fault not in ('ignored-stop', 'legacy-erasing-stop'): v['state'] = 'stopped'
        if fault == 'legacy-erasing-stop':
            for name in ('primary.pid', 'child.pid', 'sysv.pid'):
                (root / name).unlink(missing_ok=True)
        save(); sys.exit(0)
    event('status:read'); v['status_reads'] += 1; save()
    if fault == 'failed-status' or (fault == 'failed-status-after-remote' and remote): sys.exit(2)
    state = v['state']
    if fault == 'false-stopped-status': state = 'stopped'
    code = 0 if state == 'started' else (1 if v['platform'] == 'FreeBSD' else 3)
    if fault == 'empty-status': sys.exit(code)
    if v['platform'] == 'FreeBSD':
        print('pulse_agent is running as supervisor pid 100 with child pid 101.' if state == 'started' else 'pulse_agent is not running.')
    else: print('pulse-agent is running (PID 101).' if state == 'started' else 'pulse-agent is not running.')
    if fault == 'diagnostic-status': print('read refused', file=sys.stderr)
    if fault == 'blank-status': print()
    if fault == 'wrong-status-exit': code = 3 if state == 'started' else 0
    sys.exit(code)
elif role == 'sysrc':
    if fault == 'missing-sysrc': sys.exit(127)
    if a == ['-n', 'pulse_agent_enable']:
        event('enable:read'); v['enable_reads'] += 1; save()
        if fault == 'failed-enable' or (fault == 'failed-final-enable' and v['enable_reads'] > 1): sys.exit(2)
        if fault == 'empty-enable': sys.exit(0)
        print('MAYBE' if fault == 'unknown-enable' else v['enabled'])
        if fault == 'diagnostic-enable': print('read refused', file=sys.stderr)
        if fault == 'blank-enable': print()
        sys.exit(0)
    assert a in (['pulse_agent_enable=NO'], ['-x', 'pulse_agent_enable']), a
    event('enable:disable')
    if fault == 'refused-disable': sys.exit(1)
    if fault != 'ignored-disable': v['enabled'] = 'NO'
    if fault == 'reactivation': v['state'] = 'started'
    save(); sys.exit(0)
elif role in ('update-rc.d', 'chkconfig'):
    assert a in (['-f', 'pulse-agent', 'remove'], ['pulse-agent', 'off'], ['--del', 'pulse-agent']), a
    event('links:disable:' + role)
    if fault == 'refused-disable': sys.exit(1)
    if fault != 'ignored-disable':
        for p in (root / 'etc').rglob('*pulse-agent'):
            if p.is_symlink(): p.unlink()
    if fault == 'reactivation': v['state'] = 'started'
    save(); sys.exit(0)
else: raise ValueError('operation outside private fixture')
'''


def model_launcher(model, role):
    # Like the NAS controls, use the runner's selected interpreter and only
    # the standard library. Ambient site hooks must not alter manager readback
    # or spend a production observation's unchanged bounded deadline.
    return ('#!/bin/sh\nexec ' + shlex.quote(sys.executable) + ' -I -S '
            + shlex.quote(str(model)) + ' ' + shlex.quote(role) + ' "$@"\n')


class AgentLegacyTeardown(unittest.TestCase):
    source = SOURCE.read_text()

    def exercise(self, platform, *, fault="", flow="teardown", state="started", enabled="YES",
                 manager="update-rc.d", path="regular", pid="normal", links="normal", reply="confirmed", repeat=False):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            tools = root / "tools"; tools.mkdir()
            model_source = root / "model.py"; model_source.write_text(MODEL)
            for name in ("ps", "sysrc", "update-rc.d", "chkconfig"):
                p = tools / name; p.write_text(model_launcher(model_source, name)); p.chmod(0o700)
            init = root / "init"
            unrelated = root / "unrelated"; unrelated.write_text("retain unrelated file\n")
            if path == "linked": init.symlink_to(unrelated)
            elif path == "directory": init.mkdir()
            elif path != "missing":
                init.write_text(model_launcher(model_source, "init")); init.chmod(0o600 if path == "not-executable" else 0o700)
            for name in ("state", "helper", "profile"):
                p = root / name; p.mkdir(); (p / "token").write_text("synthetic recovery only\n")
            (root / "wrapper").write_text("owned boot wrapper\n")
            for name in ("primary.pid", "child.pid", "sysv.pid"):
                (root / name).write_text(("100" if name == "primary.pid" else "101") + "\n")
            if pid == "malformed": (root / "child.pid" if platform == "FreeBSD" else root / "sysv.pid").write_text("101\n42\n")
            if pid == "overlong": (root / "child.pid" if platform == "FreeBSD" else root / "sysv.pid").write_text("1" * 40 + "\n")
            if pid == "linked":
                p = root / "child.pid" if platform == "FreeBSD" else root / "sysv.pid"
                p.unlink(); p.symlink_to(unrelated)
            if path == "bootstrap": (root / "bootstrap").write_text("appliance re-enables service\n")
            for name in ("rc3.d", "rcCustom.d", "rc.d/rc5.d"):
                d = root / "etc" / name; d.mkdir(parents=True)
                if platform == "SysV" and links != "none":
                    (d / "S20pulse-agent").symlink_to(init if links != "foreign" else unrelated)
                    (d / "K01pulse-agent").symlink_to(init)
                (d / "S99pulse-agent-other").symlink_to(unrelated)
            if links == "malformed": (root / "etc/rc3.d/Sbadpulse-agent").symlink_to(init)
            if links == "linked-directory":
                d = root / "etc/rc3.d"; d.rename(root / "hidden-runlevel"); d.symlink_to(root / "hidden-runlevel")
            (root / "model.json").write_text(json.dumps(dict(platform=platform, fault=fault, state=state,
                                                           enabled=enabled, process_reads=0, status_reads=0, enable_reads=0)))
            before = self.snapshot(root)
            replacements = {'/usr/local/etc/rc.d/${AGENT_NAME}': str(init), '/usr/local/etc/rc.d/pulse_agent.sh': str(root / "wrapper"),
                            '/var/run/pulse_agent.child.pid': str(root / "child.pid"), '/var/run/pulse_agent.pid': str(root / "primary.pid"),
                            '/var/run/${AGENT_NAME}.pid': str(root / "sysv.pid"), '/etc/init.d/${AGENT_NAME}': str(init),
                            '/etc/rc.d/rc*.d': str(root / "etc/rc.d") + '/rc*.d', '/etc/rc*.d': str(root / "etc") + '/rc*.d',
                            '[[ -d /etc/init.d ]]': '[[ -d ' + shlex.quote(str(root / "etc")) + ' ]]'}
            def redirect(s):
                for key, value in replacements.items(): s = s.replace(key, value)
                return s
            values = dict(INIT_PATH=init, STATE_DIR=root / "state", PRIVILEGED_HELPER_STATE_DIR=root / "helper",
                          SAFE_PROFILE_STATE_DIR=root / "profile", EVENTS=root / "events", REPLY=reply,
                          TRUENAS_BOOTSTRAP_SCRIPT=root / "bootstrap", FIXTURE=root, PLATFORM=platform, MANAGER=manager, TEST_FAULT=fault)
            prefix = "\n".join(k + "=" + shlex.quote(str(v)) for k, v in values.items()) + "\n"
            shell = r'''
set -euo pipefail
AGENT_NAME=pulse-agent BINARY_NAME=pulse-agent UNINSTALL=true EXIT_GENERAL=1 EXIT_MISSING_ARGS=16
PULSE_URL=https://same-instance.example.invalid AGENT_ID=synthetic
ACTION_RUNNER_NAME=pulse-agent-runner PRIVILEGED_HELPER_NAME=pulse-agent-helper
log_info() { :; }; log_warn() { :; }
fail() { printf '%s\n' "$1" >&2; exit "${2:-1}"; }
timeout() {
    local result=0 role="${4##*/}"
    builtin command timeout "$@" || result=$?
    # Only a fixed model role and numeric exit, never arbitrary argv or output.
    case "$role" in ps|sysrc|init|update-rc.d|chkconfig) ;;
        *) return 96 ;;
    esac
    printf 'bounded:%s:%s\n' "$role" "$result" >> "$EVENTS"
    return "$result"
}
uname() { [[ "$PLATFORM" == FreeBSD ]] && printf 'FreeBSD\n' || printf 'Linux\n'; }
command() {
    case "$*" in
        '-v systemctl'|'-v rc-service') return 1 ;;
        '-v update-rc.d') [[ "$MANAGER" == update-rc.d ]] || return 1 ;;
        '-v chkconfig') [[ "$MANAGER" == chkconfig ]] || return 1 ;;
    esac
    builtin command "$@"
}
if [[ "$TEST_FAULT" == missing-timeout ]]; then timeout() { return 127; }; fi
if [[ "$TEST_FAULT" == missing-sysrc ]]; then sysrc() { return 127; }; fi
collector_credential_state_present() { return 0; }
uninstall_collector_registration() {
    printf 'remote:remove\n' >> "$EVENTS"
    python3 - <<'END'
import json, os, pathlib
p = pathlib.Path(os.environ['FIXTURE']) / 'model.json'; v = json.loads(p.read_text())
if v['fault'] == 'refused-stop-after-remote':
    v['state'] = 'started'
    pending = p.parent / 'model.pending'; pending.write_text(json.dumps(v)); pending.replace(p)
END
    [[ "$REPLY" == confirmed ]]
}
pkill() { printf 'process:stop\n' >> "$EVENTS"; }
sleep() { :; }
rm() {
    local arg
    for arg in "$@"; do
        [[ "$arg" == -* || "$arg" == "$FIXTURE/"* ]] || return 97
    done
    [[ "$*" != *"$INIT_PATH"* ]] || printf 'init:remove\n' >> "$EVENTS"
    builtin command rm "$@"
}
remove_agent_state_dir() { printf 'state:remove\n' >> "$EVENTS"; builtin command rm -r -- "$STATE_DIR"; }
remove_privileged_helper_state_dir() { builtin command rm -r -- "$PRIVILEGED_HELPER_STATE_DIR"; }
remove_safe_profile_state_dir() { builtin command rm -r -- "$SAFE_PROFILE_STATE_DIR"; }
'''
            if flow == "caller":
                entry = self.source.split("# --- Uninstall Logic ---\n", 1)[1].split("    # Remove least-privilege helper artifacts.", 1)[0] + "fi\n"
                invoke = "main() {\n" + redirect(entry) + "}\nmain\n"
            elif flow == "process": invoke = "read_legacy_agent_removal_processes\n"
            else:
                helper = "teardown_freebsd_agent_service" if platform == "FreeBSD" else "teardown_sysv_agent_service"
                invoke = (helper + ' "$INIT_PATH"\n') * (2 if repeat else 1)
            env = dict(os.environ, PATH=str(tools) + ":" + os.environ["PATH"], FIXTURE=str(root))
            result = subprocess.run(["bash", "-c", prefix + shell + redirect(functions(self.source)) + invoke],
                                    cwd=root, env=env, text=True, capture_output=True, timeout=12)
            events = (root / "events").read_text().splitlines() if (root / "events").exists() else []
            model = json.loads((root / "model.json").read_text())
            if (root / "timed-pid").exists():
                try: os.kill(int((root / "timed-pid").read_text()), 0); model["timed_child_absent"] = False
                except ProcessLookupError: model["timed_child_absent"] = True
            return result, events, before, self.snapshot(root), model

    @staticmethod
    def snapshot(root):
        result = {}
        for p in root.rglob("*"):
            if p.name in ("model.json", "model.pending", "events", "timed-pid") or "tools" in p.parts: continue
            info = p.lstat()
            data = os.readlink(p) if stat.S_ISLNK(info.st_mode) else (p.read_bytes() if stat.S_ISREG(info.st_mode) else None)
            result[str(p.relative_to(root))] = (info.st_mode, info.st_ino, data)
        return result

    def assert_recovery_retained(self, before, after):
        # Successful disabling may remove owned activation links before a later
        # read fails. Never recreate those links (or automatically restart).
        # Every non-activation file, and every surviving link, remains exact.
        for name, value in before.items():
            owned_activation = name.startswith("etc/") and name.rsplit("/", 1)[-1] in ("S20pulse-agent", "K01pulse-agent")
            if owned_activation and name not in after:
                continue
            self.assertIn(name, after)
            self.assertEqual(after[name], value)
        self.assertFalse(set(after) - set(before))

    def test_stop_refusal_retains_all_local_recovery(self):
        for platform in ("FreeBSD", "SysV"):
            with self.subTest(platform=platform):
                r, events, before, after, _ = self.exercise(platform, fault="refused-stop")
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("init:remove", events)

    def test_manager_refusal_precedes_remote_and_state_erasure(self):
        for platform in ("FreeBSD", "SysV"):
            with self.subTest(platform=platform):
                r, events, before, after, _ = self.exercise(platform, fault="refused-stop", flow="caller")
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("remote:remove", events)
                self.assertNotIn("state:remove", events)

    def test_ignored_stop_disable_and_reactivation_are_not_success(self):
        for platform in ("FreeBSD", "SysV"):
            for fault in ("ignored-stop", "refused-disable", "ignored-disable", "reactivation"):
                with self.subTest(platform=platform, fault=fault):
                    r, events, before, after, _ = self.exercise(platform, fault=fault)
                    self.assertNotEqual(r.returncode, 0, r.stderr)
                    self.assert_recovery_retained(before, after)
                    self.assertNotIn("init:remove", events)

    def test_failed_empty_diagnostic_or_duplicate_reads_are_not_absence(self):
        for platform in ("FreeBSD", "SysV"):
            for fault in ("failed-process", "empty-process", "duplicate-process", "diagnostic-process", "blank-process",
                          "failed-status", "empty-status", "diagnostic-status", "blank-status", "wrong-status-exit", "false-stopped-status"):
                with self.subTest(platform=platform, fault=fault):
                    r, events, before, after, model = self.exercise(platform, fault=fault)
                    self.assertNotEqual(r.returncode, 0, r.stderr)
                    self.assertEqual(after, before)
                    self.assertNotIn("service:stop", events)
                    self.assertLessEqual(model["process_reads"], 1)

    def test_enablement_refusals_and_unknown_values_retain_recovery(self):
        for fault in ("failed-enable", "empty-enable", "unknown-enable", "diagnostic-enable", "blank-enable", "failed-final-enable"):
            with self.subTest(fault=fault):
                r, events, before, after, _ = self.exercise("FreeBSD", fault=fault)
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("init:remove", events)

    def test_missing_required_tools_refuse_without_losing_recovery(self):
        for platform, fault in (("FreeBSD", "missing-timeout"), ("SysV", "missing-timeout"), ("FreeBSD", "missing-sysrc")):
            with self.subTest(platform=platform, fault=fault):
                r, events, before, after, _ = self.exercise(platform, fault=fault, flow="caller")
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("remote:remove", events)
                self.assertNotIn("service:stop", events)

    def test_final_process_read_and_orphan_supervisor_are_required(self):
        for platform, fault in (("SysV", "failed-final-process"), ("FreeBSD", "failed-final-process"), ("FreeBSD", "orphan-supervisor")):
            with self.subTest(platform=platform, fault=fault):
                r, events, before, after, _ = self.exercise(platform, fault=fault)
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assert_recovery_retained(before, after)
                self.assertIn("service:stop", events)

    def test_pid_ownership_and_unexpected_paths_refuse_before_signals(self):
        for platform in ("FreeBSD", "SysV"):
            for args in (dict(fault="pid-reused"), dict(pid="malformed"), dict(pid="linked"), dict(pid="overlong"), dict(path="linked"),
                         dict(path="directory"), dict(path="not-executable")):
                with self.subTest(platform=platform, args=args):
                    r, events, before, after, _ = self.exercise(platform, **args)
                    self.assertNotEqual(r.returncode, 0, r.stderr)
                    self.assertEqual(after, before)
                    self.assertNotIn("service:stop", events)
        r, events, before, after, _ = self.exercise("FreeBSD", path="bootstrap", flow="caller")
        self.assertNotEqual(r.returncode, 0)
        self.assertEqual(after, before)
        self.assertNotIn("remote:remove", events)

    def test_every_owned_runlevel_link_and_both_layouts_are_removed(self):
        for manager in ("update-rc.d", "chkconfig", "manual"):
            with self.subTest(manager=manager):
                r, events, before, after, _ = self.exercise("SysV", manager=manager)
                self.assertEqual(r.returncode, 0, r.stderr)
                self.assertFalse(any(k.endswith("S20pulse-agent") or k.endswith("K01pulse-agent") for k in after))
                for k, value in before.items():
                    if k.endswith("pulse-agent-other") or k.startswith(("state", "helper", "profile", "unrelated")):
                        self.assertEqual(after[k], value)

    def test_foreign_malformed_or_unobservable_runlevels_are_not_deleted(self):
        for links in ("foreign", "malformed", "linked-directory"):
            with self.subTest(links=links):
                r, events, before, after, _ = self.exercise("SysV", links=links)
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("service:stop", events)

    def test_confirmed_manager_then_server_then_recheck_then_erasure(self):
        for platform in ("FreeBSD", "SysV"):
            with self.subTest(platform=platform):
                r, events, before, after, model = self.exercise(platform, flow="caller")
                self.assertEqual(r.returncode, 0, r.stderr)
                self.assertLess(events.index("service:stop"), events.index("remote:remove"))
                self.assertLess(events.index("remote:remove"), events.index("init:remove"))
                self.assertLess(events.index("init:remove"), events.index("process:stop"))
                self.assertLess(events.index("init:remove"), events.index("state:remove"))
                self.assertGreaterEqual(model["process_reads"], 4)
                self.assertNotIn("state/token", after)
                self.assertEqual(after["unrelated"], before["unrelated"])

    def test_remote_refusal_keeps_stopped_install_and_all_recovery_files(self):
        for platform in ("FreeBSD", "SysV"):
            with self.subTest(platform=platform):
                r, events, before, after, model = self.exercise(platform, flow="caller", reply="refused")
                self.assertNotEqual(r.returncode, 0, r.stderr)
                for k in ("init", "state/token", "helper/token", "profile/token", "primary.pid", "child.pid", "sysv.pid"):
                    self.assertEqual(after[k], before[k])
                self.assertEqual(model["state"], "stopped")
                self.assertNotIn("init:remove", events)
                self.assertNotIn("service:start", events)

    def test_late_refusal_keeps_local_recovery_after_remote_confirmation(self):
        for platform in ("FreeBSD", "SysV"):
            for fault in ("refused-stop-after-remote", "failed-status-after-remote"):
                with self.subTest(platform=platform, fault=fault):
                    r, events, before, after, _ = self.exercise(platform, flow="caller", fault=fault)
                    self.assertNotEqual(r.returncode, 0, r.stderr)
                    for k in ("init", "state/token", "helper/token", "profile/token"):
                        self.assertEqual(after[k], before[k])
                    self.assertIn("remote:remove", events)
                    self.assertNotIn("state:remove", events)
                    self.assertNotIn("service:start", events)

    def test_legacy_script_pid_erasure_cannot_authorise_credential_erasure(self):
        # Old deployed renderers can remove PID files even when signals fail.
        # The independent process read must still prevent remote/credential
        # removal. Do not pretend the installer recreated those ephemeral PIDs.
        for platform in ("FreeBSD", "SysV"):
            with self.subTest(platform=platform):
                r, events, before, after, model = self.exercise(platform, fault="legacy-erasing-stop", flow="caller")
                self.assertNotEqual(r.returncode, 0, r.stderr)
                for k in ("init", "state/token", "helper/token", "profile/token", "unrelated"):
                    self.assertEqual(after[k], before[k])
                self.assertEqual(model["state"], "started")
                self.assertNotIn("remote:remove", events)
                self.assertNotIn("state:remove", events)
                self.assertNotIn("service:start", events)
                self.assertNotIn("primary.pid", after)

    def test_known_stopped_missing_and_repeated_removal_are_safe(self):
        for platform in ("FreeBSD", "SysV"):
            for path, repeat in (("regular", False), ("missing", False), ("regular", True)):
                with self.subTest(platform=platform, path=path, repeat=repeat):
                    r, events, before, after, model = self.exercise(platform, state="stopped", enabled="NO", links="none", path=path, repeat=repeat)
                    self.assertEqual(r.returncode, 0, r.stderr + "\nPrivate model observations: " + json.dumps(dict(
                        events=events, state=model["state"], enabled=model["enabled"],
                        process_reads=model["process_reads"], status_reads=model["status_reads"], enable_reads=model["enable_reads"])))
                    self.assertNotIn("service:stop", events)
                    for k in ("state/token", "helper/token", "profile/token", "unrelated"):
                        self.assertEqual(after[k], before[k])

    def test_model_uses_selected_isolated_interpreter(self):
        with tempfile.TemporaryDirectory(prefix="legacy model ") as directory:
            root = Path(directory)
            model = root / "model.py"; model.write_text(MODEL)
            invocations = root / "interpreter-arguments"
            interpreter = root / "selected python"
            interpreter.write_text('#!/bin/sh\nprintf "%s\\n" "$@" >> '
                                   + shlex.quote(str(invocations)) + '\nexec '
                                   + shlex.quote(sys.executable) + ' "$@"\n')
            interpreter.chmod(0o700)
            with mock.patch.object(sys, "executable", str(interpreter)):
                launcher = model_launcher(model, "sysrc")
            tool = root / "sysrc"; tool.write_text(launcher); tool.chmod(0o700)
            (root / "model.json").write_text(json.dumps(dict(
                platform="FreeBSD", fault="", state="stopped", enabled="NO", enable_reads=0)))
            marker = root / "site-ran"
            (root / "sitecustomize.py").write_text('from pathlib import Path\nPath('
                + repr(str(marker)) + ').touch()\nprint("unexpected site hook")\n')
            env = dict(os.environ, FIXTURE=str(root), PYTHONPATH=str(root))
            # Deliberate adverse control: the old unisolated model protocol
            # accepts unrelated startup output. Do not turn that into a pass.
            adverse = subprocess.run([sys.executable, str(model), "sysrc", "-n", "pulse_agent_enable"],
                                     env=env, capture_output=True, text=True, timeout=5)
            self.assertEqual(adverse.returncode, 0, adverse.stderr)
            self.assertIn("unexpected site hook", adverse.stdout)
            self.assertTrue(marker.exists())
            marker.unlink()
            result = subprocess.run([str(tool), "-n", "pulse_agent_enable"], env=env,
                                    capture_output=True, text=True, timeout=5)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout, "NO\n")
            self.assertEqual(result.stderr, "")
            self.assertFalse(marker.exists())
            self.assertEqual(invocations.read_text().splitlines(),
                             ["-I", "-S", str(model), "sysrc", "-n", "pulse_agent_enable"])

    def test_process_inventory_does_not_return_arguments_or_secrets(self):
        r, _, _, _, _ = self.exercise("FreeBSD", flow="process")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("100 1 supervisor\n", r.stdout)
        self.assertIn("101 100 agent\n", r.stdout)
        self.assertNotIn("synthetic", r.stdout)
        self.assertNotIn("token", r.stdout)

    def test_real_process_timeout_stops_owned_child_without_erasure(self):
        r, events, before, after, model = self.exercise("FreeBSD", fault="timeout-process")
        self.assertNotEqual(r.returncode, 0, r.stderr)
        self.assertIn("bounded:ps:124", events)
        self.assertEqual(after, before)
        self.assertTrue(model["timed_child_absent"])
        self.assertNotIn("service:stop", events)

    def test_rendered_stop_retains_pidfiles_on_refused_or_ignored_signals(self):
        for platform in ("FreeBSD", "SysV"):
            for fault in ("refused", "ignored", ""):
                with self.subTest(platform=platform, fault=fault):
                    self.rendered_stop(platform, fault)

    def rendered_stop(self, platform, fault):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            primary, child = root / "primary.pid", root / "child.pid"
            primary.write_text("100\n"); child.write_text("101\n")
            if platform == "FreeBSD":
                # This production function contains a heredoc with nested
                # rc.d functions: the next top-level comment, not the first
                # closing brace inside that heredoc, binds its complete body.
                marker = "render_freebsd_rc_agent_script() {"
                self.assertEqual(self.source.count(marker), 1)
                start = self.source.index(marker)
                end = self.source.index("\n# report_proxmox_registration_outcome", start)
                render = self.source[start:end]
                rendered = root / "rendered"
                r = subprocess.run(["bash", "-c", 'SHELL_EXPORT_LINES=""\n' + render + '\nrender_freebsd_rc_agent_script "$1" /synthetic/pulse-agent ""', "proof", str(rendered)], text=True, capture_output=True)
                self.assertEqual(r.returncode, 0, r.stderr)
                source = rendered.read_text()
                names = ("pulse_agent_pid_command", "pulse_agent_supervisor_pid", "pulse_agent_wait_stopped", "pulse_agent_stop")
                bodies = []
                for name in names:
                    marker = name + "()\n{"
                    if marker in source: bodies.append(marker + source.split(marker, 1)[1].split("\n}\n", 1)[0] + "\n}\n")
                code = "\n".join(bodies); invocation = "pulse_agent_stop"
            else:
                source = self.source.split("# Pulse Unified Agent SysV init script", 1)[1].split("\nINITEOF", 1)[0]
                marker = "do_stop() {"
                code = marker + source.split(marker, 1)[1].split("\n}\n", 1)[0] + "\n}\n"
                invocation = "do_stop"
            values = dict(pidfile=primary, child_pidfile=child, PIDFILE=primary, EVENTS=root / "events", FAULT=fault)
            prefix = "\n".join(k + "=" + shlex.quote(str(v)) for k, v in values.items()) + "\n"
            shell = r'''
name=pulse_agent NAME=pulse-agent alive100=true alive101=true
sleep() { :; }
ps() { [[ "$*" == *ppid* ]] && printf '1\n' || printf 'daemon: /synthetic/pulse-agent[101]\n'; }
kill() {
    local pid="${@: -1}" flag="alive${@: -1}"
    [[ "$pid" == 100 || "$pid" == 101 ]] || return 90
    if [[ "$1" == -0 ]]; then [[ "${!flag}" == true ]]; return; fi
    printf 'signal:%s\n' "$pid" >> "$EVENTS"
    [[ "$FAULT" != refused ]] || return 1
    [[ "$FAULT" == ignored ]] || printf -v "$flag" false
}
'''
            before = (primary.read_bytes(), child.read_bytes())
            r = subprocess.run(["bash", "-c", prefix + shell + code + "\n" + invocation], text=True, capture_output=True, timeout=5)
            if fault:
                self.assertNotEqual(r.returncode, 0, r.stdout + r.stderr)
                self.assertEqual((primary.read_bytes(), child.read_bytes()), before)
            else:
                self.assertEqual(r.returncode, 0, r.stderr)
                self.assertFalse(primary.exists())
                if platform == "FreeBSD":
                    self.assertFalse(child.exists())
                    self.assertEqual((root / "events").read_text().splitlines(), ["signal:100", "signal:101"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
