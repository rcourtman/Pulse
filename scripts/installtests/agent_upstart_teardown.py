#!/usr/bin/env python3
"""Run DSM Upstart production admission/retirement with closed manager models.

No native DSM, host init manager, privileged installer, real credential or
network is used. Production functions and the caller execute against private
regular files, including real config moves/restoration and bounded child stop.
"""
import json
import os
from pathlib import Path
import shlex
import stat
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_AGENT_UPSTART_SOURCE", ROOT / "scripts/install.sh"))
FUNCTIONS = ("portable_path_uid", "portable_path_mode", "trusted_lifecycle_regular_file",
             "read_legacy_agent_removal_processes", "legacy_agent_processes_absent",
             "read_upstart_agent_removal_status", "upstart_agent_removal_paths_safe",
             "read_upstart_agent_removal_enablement", "quiesce_upstart_agent_for_removal",
             "teardown_upstart_agent_service")


def definitions(source):
    selected = []
    for name in FUNCTIONS:
        marker = name + "() {"
        if marker not in source:  # Exact parent adverse controls.
            continue
        if source.count(marker) != 1:
            raise ValueError("ambiguous production function")
        selected.append(marker + source.split(marker, 1)[1].split("\n}\n", 1)[0] + "\n}\n")
    return "\n".join(selected)


MODEL = r'''#!/usr/bin/python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['FIXTURE'])
path = root / 'model.json'
v = json.loads(path.read_text())
role, args = pathlib.Path(sys.argv[0]).name, sys.argv[1:]
fault = v['fault']
remote = (root / 'events').exists() and 'remote:remove' in (root / 'events').read_text()
conf = root / 'init/pulse-agent.conf'
override = root / 'init/pulse-agent.override'
retired = not conf.exists() and bool(list((root / 'init').glob('pulse-agent.conf.removal.*')))
def save():
    pending = root / 'model.pending'
    pending.write_text(json.dumps(v)); pending.replace(path)
def event(s):
    with (root / 'events').open('a') as f: f.write(s + '\n')
if role == 'ps':
    assert args == ['-ax', '-o', 'pid=', '-o', 'ppid=', '-o', 'comm=', '-o', 'args='], args
    event('process:read'); v['process_reads'] += 1; save()
    if fault == 'failed-process' or (fault == 'failed-retirement-process' and retired): sys.exit(2)
    if fault == 'empty-process': sys.exit(0)
    print('1 0 init /sbin/init')
    if fault == 'duplicate-process': print('1 0 init /sbin/init')
    if v['state'] == 'started' or fault == 'orphan-process' or (fault == 'retirement-orphan' and retired):
        print('100 1 pulse-agent /usr/local/bin/pulse-agent --token-file /synthetic/recovery-only')
    if fault == 'diagnostic-process': print('read refused', file=sys.stderr)
    sys.exit(0)
assert role == 'initctl', role
if args == ['reload-configuration']:
    event('manager:reload'); v['reloads'] += 1
    if fault == 'refused-reload' or (fault == 'refused-retirement-reload' and retired):
        save(); sys.exit(1)
    v['manual'] = override.exists() and override.read_text() == 'manual\n' and fault != 'ignored-manual'
    if fault == 'concurrent-config' and retired:
        conf.write_text('concurrent configuration retained\n'); conf.chmod(0o644)
        v['state'] = 'stopped'
    elif not conf.exists(): v['state'] = 'absent'
    save(); sys.exit(0)
assert len(args) == 2 and args[1] == 'pulse-agent', args
verb = args[0]
if verb == 'status':
    event('manager:status'); v['status_reads'] += 1; save()
    if fault == 'timeout-status':
        (root / 'timed-pid').write_text(str(os.getpid()))
        os.execl('/bin/sleep', 'sleep', '30')
    if fault == 'failed-status' or (fault == 'failed-after-remote' and remote) or (fault == 'failed-retirement-status' and retired):
        print('initctl: manager connection refused', file=sys.stderr); sys.exit(1)
    state = v['state']
    code = 1 if state == 'absent' else 0
    if fault == 'wrong-status-exit': code = 2
    if fault == 'empty-status': sys.exit(code)
    if fault == 'unknown-status': print('pulse-agent start/post-start, process 100')
    elif state == 'started': print('pulse-agent start/running, process 100')
    elif state == 'stopped': print('pulse-agent stop/waiting')
    else: print('initctl: Unknown job: pulse-agent', file=sys.stderr)
    if fault == 'duplicate-status': print('pulse-agent stop/waiting')
    if fault == 'blank-status': print()
    if fault == 'diagnostic-status': print('read refused', file=sys.stderr)
    sys.exit(code)
if verb == 'show-config':
    event('manager:config')
    if fault == 'failed-config': sys.exit(1)
    if fault == 'empty-config': sys.exit(0)
    print('other-agent' if fault == 'foreign-config' else 'pulse-agent')
    if not v['manual']: print('  start on syno.network.ready')
    print('  stop on runlevel [06]')
    if fault == 'diagnostic-config': print('read refused', file=sys.stderr)
    if fault == 'duplicate-config': print('  stop on runlevel [06]')
    if fault == 'blank-config': print()
    sys.exit(0)
assert verb == 'stop', verb
event('manager:stop')
if fault == 'refused-stop': sys.exit(1)
if fault != 'ignored-stop': v['state'] = 'stopped'
save(); sys.exit(0)
'''


class AgentUpstartTeardown(unittest.TestCase):
    source = SOURCE.read_text()

    def exercise(self, *, fault="", flow="caller", state="started", conf="regular",
                 override="missing", reply="confirmed", repeat=False):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            tools = root / "tools"; tools.mkdir()
            for name in ("ps", "initctl"):
                p = tools / name; p.write_text(MODEL); p.chmod(0o700)
            if fault == "missing-initctl": (tools / "initctl").unlink()
            init = root / "init"; init.mkdir()
            config = init / "pulse-agent.conf"
            unrelated = root / "unrelated"; unrelated.write_text("retain unrelated recovery\n")
            if conf == "linked": config.symlink_to(unrelated)
            elif conf == "directory": config.mkdir()
            elif conf != "missing":
                text = ('description "Pulse Unified Agent"\nauthor "Pulse"\n'
                        'start on syno.network.ready\nstop on runlevel [06]\n'
                        'respawn\nrespawn limit 5 10\n\n'
                        'exec /usr/local/bin/pulse-agent --token-file /synthetic/recovery >> /private/log 2>&1\n')
                if conf == "foreign": text = text.replace("/usr/local/bin/pulse-agent", "/usr/local/bin/other-agent")
                if conf == "oversized": text += "#" * 1048577
                config.write_text(text); config.chmod(0o666 if conf == "writable" else 0o644)
            manual = init / "pulse-agent.override"
            if override == "linked": manual.symlink_to(unrelated)
            elif override != "missing":
                manual.write_text("manual\n" if override == "manual" else "start on started other\n")
                manual.chmod(0o644)
            if fault == "old-retirement":
                (init / "pulse-agent.conf.removal.prior").write_text("retained previous recovery\n")
            for name in ("state", "helper", "profile"):
                p = root / name; p.mkdir()
                for file in ("runtime.token", "agent-id", "connection.env", "install.sh"):
                    (p / file).write_text("synthetic recovery only\n")
            (root / "dsm").mkdir()
            (root / "model.json").write_text(json.dumps(dict(fault=fault, state=state, manual=False,
                                                            status_reads=0, process_reads=0, reloads=0)))
            before = self.snapshot(root)
            def redirect(source):
                source = source.replace("/etc/init/" + "$" + "{AGENT_NAME}", str(init / "pulse-agent"))
                source = source.replace("[[ -d /usr/syno ]]", "[[ -d " + shlex.quote(str(root / "dsm")) + " ]]")
                source = source.replace("[[ -d /etc/init.d ]]", "[[ -d " + shlex.quote(str(root / "no-sysv")) + " ]]")
                return source.replace("/var/log/pulse-agent.log", str(root / "absent-log"))
            values = dict(FIXTURE=root, EVENTS=root / "events", STATE_DIR=root / "state",
                          PRIVILEGED_HELPER_STATE_DIR=root / "helper", SAFE_PROFILE_STATE_DIR=root / "profile",
                          PRIVILEGE_SUDOERS_FILE=root / "absent-sudoers", PRIVILEGE_HELPER_DIR=root / "absent-helper",
                          REPLY=reply, FAULT=fault)
            setup = "\n".join(k + "=" + shlex.quote(str(v)) for k, v in values.items()) + "\n"
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
        '-v systemctl'|'-v rc-service') return 1 ;;
        '-v initctl') [[ "$FAULT" != missing-initctl ]] || return 1 ;;
    esac
    builtin command "$@"
}
if [[ "$FAULT" == missing-timeout ]]; then timeout() { return 127; }; fi
collector_credential_state_present() { return 0; }
uninstall_collector_registration() { printf 'remote:remove\n' >> "$EVENTS"; [[ "$REPLY" == confirmed ]]; }
pkill() { printf 'unchecked:process-stop\n' >> "$EVENTS"; }
sleep() { :; }
rm() {
    local arg
    for arg in "$@"; do [[ "$arg" == -* || "$arg" == "$FIXTURE/"* ]] || return 97; done
    printf 'file:remove\n' >> "$EVENTS"
    builtin command rm "$@"
}
mv() {
    local arg
    for arg in "$@"; do [[ "$arg" == -- || "$arg" == "$FIXTURE/"* ]] || return 97; done
    [[ "$FAULT" != refused-retirement-move || "$1" != -- || "$2" != "$FIXTURE/init/pulse-agent.conf" ]] || return 1
    printf 'file:move\n' >> "$EVENTS"
    builtin command mv "$@"
}
remove_agent_state_dir() { printf 'state:remove\n' >> "$EVENTS"; builtin command rm -r -- "$STATE_DIR"; }
remove_privileged_helper_state_dir() { builtin command rm -r -- "$PRIVILEGED_HELPER_STATE_DIR"; }
remove_safe_profile_state_dir() { builtin command rm -r -- "$SAFE_PROFILE_STATE_DIR"; }
'''
            if flow == "caller":
                entry = self.source.split("# --- Uninstall Logic ---\n", 1)[1].split("    # Unraid\n", 1)[0] + "fi\n"
                invoke = "main() {\n" + redirect(entry) + "}\nmain\n"
            elif flow == "status":
                invoke = "read_upstart_agent_removal_status\n"
            else:
                invoke = ("teardown_upstart_agent_service\n") * (2 if repeat else 1)
            env = dict(os.environ, PATH=str(tools) + ":" + os.environ["PATH"], FIXTURE=str(root))
            result = subprocess.run(["bash", "-c", setup + shell + redirect(definitions(self.source)) + invoke],
                                    cwd=root, env=env, text=True, capture_output=True, timeout=15)
            events = (root / "events").read_text().splitlines() if (root / "events").exists() else []
            model = json.loads((root / "model.json").read_text())
            if (root / "timed-pid").exists():
                try: os.kill(int((root / "timed-pid").read_text()), 0); model["timed_child_absent"] = False
                except ProcessLookupError: model["timed_child_absent"] = True
            return result, events, before, self.snapshot(root), model

    @staticmethod
    def snapshot(root):
        result = {}
        for path in root.rglob("*"):
            if path.name in ("model.json", "model.pending", "events", "timed-pid") or "tools" in path.parts:
                continue
            mode = path.lstat().st_mode
            payload = os.readlink(path) if stat.S_ISLNK(mode) else (path.read_bytes() if stat.S_ISREG(mode) else None)
            result[str(path.relative_to(root))] = (mode, path.lstat().st_ino, payload)
        return result

    def assert_recovery(self, before, after):
        for name, value in before.items():
            self.assertIn(name, after, name)
            self.assertEqual(after[name], value, name)
        added = set(after) - set(before)
        self.assertTrue(added <= {"init/pulse-agent.override"}, added)
        if added: self.assertEqual(after["init/pulse-agent.override"][2], b"manual\n")

    def test_refused_manager_precedes_remote_and_state_erasure(self):
        for fault in ("refused-stop", "refused-reload", "ignored-stop", "ignored-manual"):
            with self.subTest(fault=fault):
                r, events, before, after, _ = self.exercise(fault=fault)
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertNotIn("remote:remove", events)
                self.assertNotIn("state:remove", events)
                self.assert_recovery(before, after)

    def test_failed_empty_partial_diagnostic_status_cannot_authorise_removal(self):
        for fault in ("failed-status", "empty-status", "unknown-status", "duplicate-status",
                      "diagnostic-status", "blank-status", "wrong-status-exit"):
            with self.subTest(fault=fault):
                r, events, before, after, _ = self.exercise(fault=fault)
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("remote:remove", events)
                self.assertEqual(events.count("manager:status"), 1)

    def test_effective_disabled_config_is_required_after_override_reload(self):
        for fault in ("failed-config", "empty-config", "foreign-config", "diagnostic-config",
                      "duplicate-config", "blank-config"):
            with self.subTest(fault=fault):
                r, events, before, after, _ = self.exercise(fault=fault)
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertNotIn("remote:remove", events)
                self.assertNotIn("manager:stop", events)
                self.assert_recovery(before, after)

    def test_independent_process_inventory_rejects_orphan_or_unavailable_read(self):
        for fault in ("failed-process", "empty-process", "duplicate-process", "diagnostic-process", "orphan-process"):
            with self.subTest(fault=fault):
                r, events, before, after, _ = self.exercise(fault=fault, state="stopped")
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("remote:remove", events)

    def test_untrusted_config_override_and_prior_retirement_refuse_without_changes(self):
        cases = [dict(conf=c) for c in ("linked", "directory", "foreign", "writable", "oversized")]
        cases += [dict(override=o) for o in ("linked", "foreign")]
        cases += [dict(fault="old-retirement")]
        for case in cases:
            with self.subTest(case=case):
                r, events, before, after, _ = self.exercise(**case)
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertEqual(after, before)
                self.assertEqual(events, [])

    def test_missing_tools_refuse_without_remote_or_erasure(self):
        for fault in ("missing-initctl", "missing-timeout"):
            with self.subTest(fault=fault):
                r, events, before, after, _ = self.exercise(fault=fault)
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("remote:remove", events)

    def test_remote_refusal_keeps_stopped_disabled_recovery_without_restart(self):
        for reply in ("refused", "unknown"):
            with self.subTest(reply=reply):
                r, events, before, after, model = self.exercise(reply=reply)
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertEqual(model["state"], "stopped")
                self.assertTrue(model["manual"])
                self.assert_recovery(before, after)
                self.assertNotIn("state:remove", events)
                self.assertNotIn("file:move", events)

    def test_late_manager_refusal_retains_recovery_after_remote_confirmation(self):
        r, events, before, after, _ = self.exercise(fault="failed-after-remote")
        self.assertNotEqual(r.returncode, 0, r.stderr)
        self.assertIn("remote:remove", events)
        self.assertNotIn("state:remove", events)
        self.assert_recovery(before, after)

    def test_unconfirmed_job_retirement_restores_exact_config_and_retains_manual(self):
        for fault in ("refused-retirement-reload", "failed-retirement-status",
                      "failed-retirement-process", "retirement-orphan", "refused-retirement-move"):
            with self.subTest(fault=fault):
                r, events, before, after, _ = self.exercise(fault=fault)
                self.assertNotEqual(r.returncode, 0, r.stderr)
                self.assertIn("remote:remove", events)
                self.assertNotIn("state:remove", events)
                self.assert_recovery(before, after)

    def test_concurrent_config_is_not_clobbered_and_original_staged_recovery_is_retained(self):
        r, events, before, after, _ = self.exercise(fault="concurrent-config")
        self.assertNotEqual(r.returncode, 0, r.stderr)
        self.assertIn("remote:remove", events)
        self.assertNotIn("state:remove", events)
        self.assertEqual(after["init/pulse-agent.conf"][2], b"concurrent configuration retained\n")
        stages = [n for n in after if n.startswith("init/pulse-agent.conf.removal.")]
        self.assertEqual(len(stages), 1)
        self.assertEqual(after[stages[0]], before["init/pulse-agent.conf"])
        for name in before:
            if name.startswith(("state", "helper", "profile")) or name == "unrelated":
                self.assertEqual(after[name], before[name])
        self.assertEqual(after["init/pulse-agent.override"][2], b"manual\n")

    def test_confirmed_job_absence_precedes_selected_local_erasure(self):
        r, events, before, after, model = self.exercise()
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(model["state"], "absent")
        self.assertNotIn("init/pulse-agent.conf", after)
        self.assertNotIn("init/pulse-agent.override", after)
        self.assertNotIn("state", after)
        self.assertEqual(after["unrelated"], before["unrelated"])
        self.assertLess(events.index("manager:stop"), events.index("remote:remove"))
        self.assertLess(events.index("remote:remove"), events.index("file:move"))
        self.assertLess(events.index("file:move"), events.index("state:remove"))
        last_inventory = max(i for i, e in enumerate(events) if e == "process:read")
        self.assertLess(last_inventory, events.index("state:remove"))
        self.assertNotIn("syno", r.stdout + r.stderr)  # No configuration/argument dump.

    def test_absent_stopped_and_repeated_removal_require_observed_manager_absence(self):
        for case in (dict(conf="missing", state="absent"), dict(state="stopped"), dict(override="manual")):
            with self.subTest(case=case):
                r, events, before, after, _ = self.exercise(flow="teardown", repeat=True, **case)
                self.assertEqual(r.returncode, 0, r.stderr)
                self.assertNotIn("init/pulse-agent.conf", after)
                self.assertNotIn("init/pulse-agent.override", after)
                for name in before:
                    if name.startswith(("state", "helper", "profile")):
                        self.assertEqual(after[name], before[name])
        for case in (dict(conf="missing", state="started"), dict(conf="regular", state="absent")):
            with self.subTest(case=case):
                r, _, before, after, _ = self.exercise(flow="teardown", **case)
                self.assertNotEqual(r.returncode, 0)
                self.assertEqual(after, before)

    def test_source_selection_keeps_upstart_out_of_sysv_and_before_erasure(self):
        entry = self.source.split("# --- Uninstall Logic ---\n", 1)[1].split("# --- Validation ---", 1)[0]
        self.assertEqual(entry.count("quiesce_upstart_agent_for_removal"), 1)
        self.assertEqual(entry.count("teardown_upstart_agent_service"), 1)
        admission = entry.index("quiesce_upstart_agent_for_removal")
        remote = entry.index("if ! uninstall_collector_registration;")
        retirement = entry.index("teardown_upstart_agent_service")
        erase = entry.index('if ! remove_agent_state_dir "$STATE_DIR";')
        self.assertLess(admission, remote)
        self.assertLess(remote, retirement)
        self.assertLess(retirement, erase)
        self.assertIn('[[ "$uninstall_upstart" != true ]]', entry)
        self.assertFalse('initctl stop' in entry)
        self.assertNotIn('initctl start', definitions(self.source))

    def test_real_status_timeout_stops_owned_child_without_erasure(self):
        r, events, before, after, model = self.exercise(fault="timeout-status")
        self.assertNotEqual(r.returncode, 0, r.stderr)
        self.assertEqual(after, before)
        self.assertNotIn("remote:remove", events)
        self.assertTrue(model["timed_child_absent"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
