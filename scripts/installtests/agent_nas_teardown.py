#!/usr/bin/env python3
"""Actual NAS uninstall caller and boot transformations in private user fixtures.

Flash and kernel/manager refusal controls are models. No host signal, mount,
service, credential, boot path, network or native appliance is exercised.
"""
import json
import os
from pathlib import Path
import shlex
import shutil
import stat
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_AGENT_NAS_SOURCE", ROOT / "scripts/install.sh"))
FUNCTIONS = (
    "portable_path_uid", "portable_path_mode", "portable_sed_in_place",
    "nas_agent_removal_platform", "nas_agent_removal_file_safe", "nas_agent_removal_paths_safe",
    "filter_nas_agent_boot_file", "disable_nas_agent_boot_file", "nas_agent_boot_operation",
    "read_nas_agent_removal_processes", "nas_agent_processes_absent", "quiesce_nas_agent_for_removal",
    "confirm_nas_agent_removal_quiescent", "teardown_nas_agent_boot_owner",
    "remove_qnap_autorun_block", "render_qnap_autorun_block", "append_qnap_autorun_block",
)


def definitions(source, names=FUNCTIONS):
    result = []
    for name in names:
        marker = name + "() {"
        if marker not in source:  # Exact-parent adverse controls, not substitute code.
            continue
        if source.count(marker) != 1:
            raise ValueError("ambiguous production function " + name)
        if name == "write_qnap_wrapper_script":
            # Its heredoc contains shell functions: their braces do not close
            # the renderer. Bind the actual outer EOF/function boundary.
            start = source.index(marker)
            end = source.index("\n}\n", source.index("\nEOF\n", start)) + 3
            result.append(source[start:end] + "\n")
        else:
            result.append(marker + source.split(marker, 1)[1].split("\n}\n", 1)[0] + "\n}\n")
    return "\n".join(result)


def generated_collector_is_running(listing, runtime):
    """Match the owned executable with either Unix comm representation.

    Darwin reports an executable path where Linux reports its basename. Keep
    the argument path exact so another same-named process is not our collector.
    """
    for line in listing.splitlines():
        fields = line.split(maxsplit=1)
        if (len(fields) == 2 and Path(fields[0]).name == "pulse-agent"
                and fields[1].startswith(str(runtime) + " ")):
            return True
    return False


def write_wide_ps_adapter(tools, executable):
    """Observe real fixture processes without Darwin's display-width truncation.

    These Linux NAS wrappers run in a long private Darwin temporary path during
    CI. Both comm and args contain that path. Widen only the fixture transport;
    the production parser, exact ownership checks and deadlines are unchanged.
    """
    target = tools / "ps"
    target.write_text("#!/bin/sh\nexec " + shlex.quote(str(executable)) + ' -ww "$@"\n')
    target.chmod(0o700)
    return target


MODEL = r'''#!/usr/bin/python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['FIXTURE'])
path = root / 'model.json'
v = json.loads(path.read_text()); role, args = pathlib.Path(sys.argv[0]).name, sys.argv[1:]
fault = v['fault']
def event(s):
    with (root / 'events').open('a') as f: f.write(s + '\n')
def save():
    pending = root / 'model.pending'; pending.write_text(json.dumps(v)); pending.replace(path)
if role == 'ps':
    assert args == ['-ax', '-o', 'pid=', '-o', 'ppid=', '-o', 'comm=', '-o', 'args='], args
    event('process:read'); v['reads'] += 1
    if fault == 'reactivated-before-remote' and v['reads'] == 5: v['watchdog'] = True
    save()
    if fault == 'failed-process': sys.exit(2)
    if fault == 'empty-process': sys.exit(0)
    if fault == 'timeout-process':
        (root / 'timed-pid').write_text(str(os.getpid())); os.execl('/bin/sleep', 'sleep', '30')
    print('1 0 init /sbin/init')
    if fault == 'duplicate-process': print('1 0 init /sbin/init')
    if fault == 'diagnostic-process': print('read refused', file=sys.stderr)
    wrapper = str(root / 'state/start-pulse-agent.sh')
    if v['watchdog']:
        if fault == 'script-comm': print('100 1 start-pulse-age /bin/sh ' + wrapper)
        else: print('100 1 sh /bin/sh ' + wrapper if v['platform'] == 'QNAP' else '100 1 bash /bin/bash ' + wrapper)
    if v['collector']: print('101 100 pulse-agent ' + str(root / 'bin/pulse-agent') + ' --token-file /synthetic/private-only')
    if fault == 'foreign-process': print('102 1 pulse-agent /other/instance/pulse-agent --token-file /synthetic/private-only')
    if fault == 'relative-wrapper': print('103 1 sh sh start-pulse-agent.sh')
    if fault == 'deferred-bootstrap': print('104 1 sh /bin/sh /tmp/nasconfig_tmp/autorun.sh')
    if fault == 'live-go': print('105 1 bash /bin/bash ' + str(root / 'go'))
    print('200 1 vi vi ' + wrapper)  # Editor is not a watchdog or signal target.
    print('201 1 sh /bin/sh ' + wrapper + '.bak')
    print('202 1 pulse-agent-prod /other/pulse-agent-prod')
    sys.exit(0)
if role == 'kill':
    assert len(args) == 2 and args[0] == '-TERM', args
    pid = int(args[1]); assert pid in (100, 101), 'unrelated process signalled'
    event('signal:' + str(pid))
    boot = root / ('flash/autorun.sh' if v['platform'] == 'QNAP' else 'go')
    active = boot.exists() and '# Pulse Agent' in boot.read_text()
    event('boot:present-at-signal' if active else 'boot:absent-at-signal')
    if fault == 'refused-watchdog-stop' and pid == 100: sys.exit(1)
    if fault == 'refused-collector-stop' and pid == 101: sys.exit(1)
    if pid == 100 and fault != 'ignored-watchdog-stop': v['watchdog'] = False
    if pid == 101 and fault != 'ignored-collector-stop': v['collector'] = False
    save(); sys.exit(0)
assert role == 'flash' and args[0] in ('mount_flash_config', 'umount_flash_config'), (role, args)
verb = args[0]; event('flash:' + verb)
if fault == 'mount-refusal' and verb == 'mount_flash_config': sys.exit(1)
if fault == 'unmount-refusal' and verb == 'umount_flash_config': sys.exit(1)
sys.exit(0)
'''


class AgentNASTeardown(unittest.TestCase):
    source = SOURCE.read_text()

    def exercise(self, platform, fault="", *, reply="confirmed", boot="canonical", wrapper="regular",
                 state="selected", repeat=False, flow="caller", appliance=True):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            tools = root / "tools"; tools.mkdir()
            for name in ("ps", "kill", "flash"):
                p = tools / name; p.write_text(MODEL); p.chmod(0o700)
            for name in ("state", "helper", "profile", "bin", "flash", "legacy", "foreign"):
                d = root / name; d.mkdir(mode=0o700)
                if name in ("state", "helper", "profile", "foreign"):
                    for f in ("runtime.token", "agent-id", "connection.env", "install.sh"):
                        (d / f).write_text("synthetic recovery only\n")
            (root / "legacy/telegraf").write_text("unrelated legacy tool\n")
            target = root / "state/start-pulse-agent.sh"
            other = root / "unrelated"; other.write_text("unchanged unrelated file\n")
            if wrapper == "linked": target.symlink_to(other)
            elif wrapper == "directory": target.mkdir()
            elif wrapper != "missing":
                target.write_text("#!/bin/sh\n# Pulse Agent startup script for " + ("foreign" if wrapper == "foreign" else platform) + "\n")
                target.chmod(0o700)
            vendor = root / "vendor"
            if appliance: vendor.write_text(platform + "\n")
            bootfile = root / "flash/autorun.sh" if platform == "QNAP" else root / "go"
            unrelated = "#!/bin/sh\necho before\nbash /other/start-pulse-agent-prod.sh\necho pulse-agent-prod\n/boot/config/pulse/telegraf/start_telegraf.sh\n"
            bootfile.write_text(unrelated)
            producer = definitions(self.source, ("portable_sed_in_place", "remove_qnap_autorun_block", "render_qnap_autorun_block", "append_qnap_autorun_block"))
            if platform == "QNAP":
                # Consume the real production bootstrap emitter, not a second literal.
                command = "AGENT_NAME=pulse-agent\nSTATE_DIR=" + shlex.quote(str(root / "state")) + "\n" + producer + "\nappend_qnap_autorun_block " + shlex.quote(str(bootfile)) + " " + shlex.quote(str(target)) + " " + shlex.quote(str(root / "state"))
                subprocess.run(["bash", "-c", command], check=True, capture_output=True, text=True)
            else:
                bootfile.write_text(unrelated + "# Pulse Agent\nbash " + str(target) + "\n")
            bootfile.write_text(bootfile.read_text() + "echo after\n")
            if boot == "duplicate":
                if platform == "QNAP":
                    block = bootfile.read_text().split("# Pulse Agent bootstrap begin", 1)[1].split("# Pulse Agent bootstrap end", 1)[0]
                    bootfile.write_text(bootfile.read_text() + "# Pulse Agent bootstrap begin" + block + "# Pulse Agent bootstrap end\n")
                else: bootfile.write_text(bootfile.read_text() + "# Pulse Agent\nbash " + str(target) + "\n")
            elif boot == "foreign": bootfile.write_text(bootfile.read_text().replace(str(target), "/other/start-pulse-agent.sh"))
            elif boot == "indirect": bootfile.write_text(bootfile.read_text() + "bash -c 'bash " + str(target) + "'\n")
            elif boot == "unterminated": bootfile.write_text(bootfile.read_text().replace("# Pulse Agent bootstrap end", "# unknown") if platform == "QNAP" else bootfile.read_text() + "# Pulse Agent\n")
            elif boot == "changed-block": bootfile.write_text(bootfile.read_text().replace("_pulse_wait_max=1800", "_pulse_wait_max=999") if platform == "QNAP" else bootfile.read_text().replace("bash " + str(target), "bash " + str(target) + " --unknown"))
            elif boot == "no-newline": bootfile.write_text(bootfile.read_text().rstrip("\n"))
            elif boot == "binary": bootfile.write_bytes(bootfile.read_bytes() + b'\0\n')
            elif boot == "oversized": bootfile.write_text(bootfile.read_text() + "#" * 1048577 + "\n")
            elif boot == "absent": bootfile.write_text(unrelated + "echo after\n")
            elif boot == "missing": bootfile.unlink()
            elif boot in ("linked", "directory"):
                bootfile.unlink()
                if boot == "linked": bootfile.symlink_to(other)
                else: bootfile.mkdir()
            original_boot = bootfile.read_bytes() if bootfile.is_file() and not bootfile.is_symlink() else None
            if fault == "prior-recovery": (Path(str(bootfile) + ".pulse-removal.retained")).mkdir()
            if fault == "missing-flash": (tools / "flash").unlink()
            model = dict(platform=platform, fault=fault, reads=0, watchdog=wrapper != "missing", collector=wrapper != "missing")
            if boot in ("absent", "missing") and wrapper == "missing": model.update(watchdog=False, collector=False)
            (root / "model.json").write_text(json.dumps(model))
            before = self.snapshot(root)

            replacements = {"/etc/unraid-version": str(vendor if platform == "Unraid" else root / "absent-vendor"),
                            "/boot/config/plugins/pulse-agent": str(root / "absent-plugin"),
                            "/etc/config/qpkg.conf": str(vendor if platform == "QNAP" else root / "absent-vendor"),
                            "/sbin/getcfg": str(root / "absent-getcfg"),
                            "/etc/init.d/init_disk.sh": str(tools / "flash"),
                            "/tmp/nasconfig_tmp": str(root / "flash"),
                            "/boot/config/go": str(root / "go"),
                            "[[ -d /boot/config && ! -L /boot/config ]]": "[[ -d " + str(root) + " && ! -L " + str(root) + " ]]",
                            "/boot/config/pulse": str(root / "legacy"),
                            "[[ -d /etc/init.d ]]": "[[ -d " + str(root / "no-init") + " ]]",
                            "/usr/syno": str(root / "no-dsm"),
                            "/var/log/pulse-agent.log": str(root / "absent-log"),
                            "/var/log/pulse": str(root / "absent-log-dir"),
                            "/etc/truenas-version": str(root / "no-truenas"),
                            "/etc/version": str(root / "no-version")}
            def redirect(text):
                for a, b in replacements.items(): text = text.replace(a, b)
                return text
            values = dict(FIXTURE=root, EVENTS=root / "events", PLATFORM=platform, FAULT=fault,
                          STATE_DIR=root / "state", STATE_DIR_REMOVAL_AUTHORITY=(root / "state" if state == "selected" else root / "foreign"),
                          INSTALL_DIR=root / "bin", PRIVILEGED_HELPER_STATE_DIR=root / "helper",
                          SAFE_PROFILE_STATE_DIR=root / "profile", PRIVILEGE_SUDOERS_FILE=root / "absent-sudoers",
                          PRIVILEGE_HELPER_DIR=root / "absent-helper", TRUENAS_STATE_DIR=root / "no-truenas-state",
                          TRUENAS_BOOTSTRAP_SCRIPT=root / "no-bootstrap", BOOT_PATH=bootfile, REPLY=reply)
            setup = "\n".join(k + "=" + shlex.quote(str(v)) for k, v in values.items())
            shell = r'''
set -euo pipefail
UNINSTALL=true AGENT_NAME=pulse-agent BINARY_NAME=pulse-agent
EXIT_GENERAL=1 EXIT_MISSING_ARGS=16 PULSE_URL=https://same-instance.example.invalid AGENT_ID=synthetic
ACTION_RUNNER_NAME=pulse-agent-runner PRIVILEGED_HELPER_NAME=pulse-agent-helper
log_info() { :; }; log_warn() { :; }
fail() { printf '%s\n' "$1" >&2; exit "$2"; }
uname() { printf 'Linux\n'; }
command() {
    case "$*" in
        '-v systemctl'|'-v rc-service'|'-v initctl') return 1 ;;
        '-v timeout') [[ "$FAULT" != missing-timeout ]] || return 1 ;;
        '-v ps') [[ "$FAULT" != missing-ps ]] || return 1 ;;
    esac
    builtin command "$@"
}
truenas_scale_agent_removal_present() { return 1; }
collector_credential_state_present() { [[ -e "$STATE_DIR/runtime.token" ]]; }
uninstall_collector_registration() {
    printf 'remote:remove\n' >> "$EVENTS"
    python3 -c 'import json,os,pathlib; r=pathlib.Path(os.environ["FIXTURE"]); v=json.loads((r/"model.json").read_text()); b=r/("flash/autorun.sh" if v["platform"]=="QNAP" else "go"); e=[]; e += ["remote:processes-active"] if v["watchdog"] or v["collector"] else []; e += ["remote:boot-present"] if b.exists() and "# Pulse Agent" in b.read_text() else []; f=(r/"events").open("a"); f.write("\n".join(e)+("\n" if e else "")); f.close()'
    if [[ "$FAULT" == late-boot ]]; then
        cp "$NAS_REMOVAL_BOOT_RECOVERY/original" "$BOOT_PATH"
    elif [[ "$FAULT" == late-process ]]; then
        python3 -c 'import json,os,pathlib; p=pathlib.Path(os.environ["FIXTURE"])/"model.json"; v=json.loads(p.read_text()); v["watchdog"]=True; p.write_text(json.dumps(v))'
    fi
    [[ "$REPLY" == confirmed ]]
}
find_qnap_state_dir() { [[ "$PLATFORM" != QNAP ]] || printf '%s\n' "$STATE_DIR"; }
pkill() { printf 'unchecked:process-stop\n' >> "$EVENTS"; }
kill() { builtin command "$FIXTURE/tools/kill" "$@"; }
sleep() { :; }
rm() {
    local arg
    for arg in "$@"; do [[ "$arg" == -* || "$arg" == "$FIXTURE/"* ]] || return 97; done
    printf 'file:remove\n' >> "$EVENTS"; builtin command rm "$@"
}
cp() { [[ "$FAULT" != refused-copy ]] || return 1; builtin command cp "$@"; }
chmod() { [[ "$FAULT" != refused-chmod || "${@: -1}" != "$BOOT_PATH" ]] || return 1; builtin command chmod "$@"; }
mv() {
    [[ "$FAULT" != refused-edit ]] || return 1
    if [[ "$FAULT" == source-changed ]]; then printf 'echo concurrent-owner\n' >> "$1"; fi
    builtin command mv "$@" || return 1
    case "$FAULT" in
        concurrent-file) printf 'concurrent boot owner\n' > "$BOOT_PATH" ;;
        concurrent-directory) mkdir "$BOOT_PATH" ;;
        concurrent-symlink) ln -s "$FIXTURE/unrelated" "$BOOT_PATH" ;;
    esac
}
remove_agent_state_dir() { printf 'state:remove\n' >> "$EVENTS"; rm -rf -- "$STATE_DIR"; }
remove_privileged_helper_state_dir() { rm -rf -- "$PRIVILEGED_HELPER_STATE_DIR"; }
remove_safe_profile_state_dir() { rm -rf -- "$SAFE_PROFILE_STATE_DIR"; }
'''
            entry = self.source.split("# --- Uninstall Logic ---\n", 1)[1].split("# --- Validation ---\n", 1)[0]
            if flow == "process": entry = 'read_nas_agent_removal_processes "$PLATFORM"\n'
            executable = shell + "\n" + setup + "\n" + redirect(definitions(self.source)) + "\nmain() {\n" + redirect(entry) + "}\nmain\n"
            env = dict(os.environ, FIXTURE=str(root), PATH=str(tools) + ":" + os.environ["PATH"])
            result = subprocess.run(["bash", "-c", executable], env=env, capture_output=True, text=True, timeout=14)
            second = subprocess.run(["bash", "-c", executable], env=env, capture_output=True, text=True, timeout=14) if repeat else None
            events = (root / "events").read_text().splitlines() if (root / "events").exists() else []
            after = self.snapshot(root)
            content = bootfile.read_bytes() if bootfile.is_file() and not bootfile.is_symlink() else None
            recovered = {str(p.relative_to(root)): p.read_bytes() for p in root.rglob("retired")}
            timed_stopped = not (root / "timed-pid").exists() or not Path('/proc/' + (root / "timed-pid").read_text()).exists()
            return result, second, events, before, after, json.loads((root / "model.json").read_text()), content, original_boot, recovered, timed_stopped

    @staticmethod
    def snapshot(root):
        result = {}
        for p in root.rglob("*"):
            if p.parts[len(root.parts)] not in ("state", "helper", "profile", "unrelated", "legacy", "foreign"):
                continue
            mode = p.lstat().st_mode
            data = os.readlink(p) if stat.S_ISLNK(mode) else p.read_bytes() if stat.S_ISREG(mode) else None
            result[str(p.relative_to(root))] = (mode, data, p.lstat().st_ino)
        return result

    def retained(self, result, *, remote=False):
        execution, _, events, before, after, _, _, _, _, _ = result
        self.assertEqual(execution.returncode, 1, execution.stdout + execution.stderr)
        self.assertNotIn('state:remove', events)
        self.assertEqual('remote:remove' in events, remote)
        for path in before: self.assertEqual(after.get(path), before[path], path)
        self.assertNotIn('synthetic recovery only', execution.stdout + execution.stderr)
        self.assertNotIn('/synthetic/private-only', execution.stdout + execution.stderr)
        return events

    def test_complete_process_evidence_is_required_before_boot_or_remote_mutation(self):
        for platform in ('Unraid', 'QNAP'):
            for fault in ('failed-process', 'empty-process', 'duplicate-process', 'diagnostic-process', 'missing-timeout', 'missing-ps'):
                with self.subTest(platform=platform, fault=fault):
                    result = self.exercise(platform, fault); events = self.retained(result)
                    self.assertEqual(result[6], result[7]); self.assertFalse(any(e.startswith('signal:') for e in events))

    def test_unknown_relative_foreign_or_deferred_boot_owners_are_not_signalled(self):
        for platform, fault in (('Unraid', 'foreign-process'), ('QNAP', 'foreign-process'), ('Unraid', 'relative-wrapper'), ('QNAP', 'deferred-bootstrap')):
            with self.subTest(platform=platform, fault=fault):
                result = self.exercise(platform, fault); events = self.retained(result)
                self.assertEqual(result[6], result[7]); self.assertFalse(any(e.startswith('signal:') for e in events))

    def test_non_regular_foreign_wrapper_and_unselected_state_preserve_recovery(self):
        for platform in ('Unraid', 'QNAP'):
            for wrapper in ('linked', 'directory', 'foreign'):
                with self.subTest(platform=platform, wrapper=wrapper):
                    result = self.exercise(platform, wrapper=wrapper); self.retained(result); self.assertEqual(result[6], result[7])
            self.retained(self.exercise(platform, state='foreign'))

    def test_only_complete_owned_boot_entries_are_admitted(self):
        for platform in ('Unraid', 'QNAP'):
            for boot in ('foreign', 'indirect', 'unterminated', 'changed-block', 'no-newline', 'binary', 'oversized', 'linked', 'directory'):
                with self.subTest(platform=platform, boot=boot):
                    result = self.exercise(platform, boot=boot); self.retained(result); self.assertEqual(result[6], result[7])

    def test_refused_edit_copy_or_prior_recovery_cannot_erase_credentials(self):
        for platform in ('Unraid', 'QNAP'):
            for fault in ('refused-edit', 'refused-copy', 'refused-chmod', 'prior-recovery'):
                with self.subTest(platform=platform, fault=fault): self.retained(self.exercise(platform, fault))

    def test_concurrent_boot_file_directory_and_symlink_are_not_clobbered(self):
        for platform in ('Unraid', 'QNAP'):
            for fault in ('concurrent-file', 'concurrent-directory', 'concurrent-symlink', 'source-changed'):
                with self.subTest(platform=platform, fault=fault):
                    result = self.exercise(platform, fault); self.retained(result)
                    self.assertTrue(result[8], 'actual retired boot recovery must survive')
                    if fault == 'concurrent-file': self.assertEqual(result[6], b'concurrent boot owner\n')
                    if fault == 'source-changed': self.assertTrue(any(b'echo concurrent-owner\n' in data for data in result[8].values()))

    def test_flash_mount_and_unmount_failures_are_not_local_only_permission(self):
        for fault in ('mount-refusal', 'unmount-refusal', 'missing-flash'):
            with self.subTest(fault=fault):
                events = self.retained(self.exercise('QNAP', fault)); self.assertFalse(any(e.startswith('signal:') for e in events))
                if fault != 'missing-flash': self.assertIn('flash:umount_flash_config', events)

    def test_watchdog_stops_before_collector_and_ignored_or_refused_signals_retain_state(self):
        for platform in ('Unraid', 'QNAP'):
            for fault in ('ignored-watchdog-stop', 'refused-watchdog-stop', 'ignored-collector-stop', 'refused-collector-stop'):
                with self.subTest(platform=platform, fault=fault):
                    events = self.retained(self.exercise(platform, fault))
                    self.assertNotIn('unchecked:process-stop', events)
                    if 'watchdog' in fault: self.assertNotIn('signal:101', events)

    def test_all_owned_entries_disable_before_signalling_and_remote_erasure(self):
        for platform, fault in (('Unraid', ''), ('QNAP', ''), ('QNAP', 'script-comm')):
            with self.subTest(platform=platform, fault=fault):
                r = self.exercise(platform, fault, boot='duplicate')
                self.assertEqual(r[0].returncode, 0, r[0].stderr)
                self.assertLess(r[2].index('signal:100'), r[2].index('signal:101'))
                self.assertLess(r[2].index('signal:101'), r[2].index('remote:remove'))
                self.assertLess(r[2].index('remote:remove'), r[2].index('state:remove'))
                self.assertNotIn('unchecked:process-stop', r[2]); self.assertFalse(r[5]['watchdog'] or r[5]['collector'])
                self.assertEqual(r[2].count('boot:absent-at-signal'), 2)
                for illegal in ('boot:present-at-signal', 'remote:processes-active', 'remote:boot-present'):
                    self.assertNotIn(illegal, r[2])
                for key in ('unrelated', 'legacy', 'legacy/telegraf'): self.assertEqual(r[3][key], r[4][key])
                self.assertIn(b'bash /other/start-pulse-agent-prod.sh\n', r[6]); self.assertIn(b'echo after\n', r[6])
                self.assertNotIn(b'# Pulse Agent\n', r[6]); self.assertNotIn(b'# Pulse Agent bootstrap', r[6]); self.assertEqual(r[8], {})

    def test_remote_refusal_keeps_disabled_boot_and_exact_local_recovery(self):
        for platform in ('Unraid', 'QNAP'):
            with self.subTest(platform=platform):
                r = self.exercise(platform, reply='refused'); self.retained(r, remote=True)
                self.assertFalse(r[5]['watchdog'] or r[5]['collector']); self.assertNotIn(b'# Pulse Agent', r[6]); self.assertTrue(r[8])

    def test_reactivation_before_remote_refuses_without_erasure(self):
        for platform in ('Unraid', 'QNAP'):
            with self.subTest(platform=platform): self.retained(self.exercise(platform, 'reactivated-before-remote'))

    def test_late_boot_or_process_activation_retains_recovery_after_remote(self):
        for platform in ('Unraid', 'QNAP'):
            for fault in ('late-boot', 'late-process'):
                with self.subTest(platform=platform, fault=fault): self.retained(self.exercise(platform, fault), remote=True)

    def test_absent_missing_and_repeated_removal_require_fresh_quiescence(self):
        for platform in ('Unraid', 'QNAP'):
            for boot in ('absent', 'missing'):
                with self.subTest(platform=platform, boot=boot):
                    r = self.exercise(platform, boot=boot, wrapper='missing', repeat=True)
                    self.assertEqual(r[0].returncode, 0, r[0].stderr); self.assertEqual(r[1].returncode, 0, r[1].stderr)
                    self.assertEqual(r[2].count('remote:remove'), 1); self.assertNotIn('unchecked:process-stop', r[2])
                    self.assertGreater(r[2].count('process:read'), 4)

    def test_process_observation_returns_roles_not_arguments_and_stops_timeout_child(self):
        r = self.exercise('Unraid', flow='process')
        self.assertEqual(r[0].returncode, 0, r[0].stderr)
        self.assertIn('100 1 watchdog\n', r[0].stdout); self.assertIn('101 100 collector\n', r[0].stdout)
        self.assertNotIn('/synthetic/private-only', r[0].stdout); self.assertIn('200 1 other\n', r[0].stdout)
        r = self.exercise('QNAP', 'timeout-process'); self.retained(r); self.assertTrue(r[9])

    def test_live_unraid_boot_shell_is_unknown_not_a_signal_target(self):
        r = self.exercise('Unraid', 'live-go'); events = self.retained(r)
        self.assertEqual(r[6], r[7]); self.assertFalse(any(e.startswith('signal:') for e in events))

    def test_recovered_ordinary_linux_state_is_not_qnap_platform_authority(self):
        r = self.exercise('QNAP', appliance=False)
        self.assertEqual(r[0].returncode, 0, r[0].stderr)
        self.assertFalse(any(e.startswith('flash:') or e.startswith('signal:') for e in r[2]))
        for key in ('foreign', 'foreign/runtime.token', 'legacy/telegraf'):
            self.assertEqual(r[3][key], r[4][key])

    def test_actual_generated_watchdogs_and_children_become_quiescent(self):
        # Private ordinary-user processes only. The full Go suite runs this in
        # the standard offline source VM; it is not native NAS/root acceptance.
        import signal
        import time
        for platform in ('Unraid', 'QNAP'):
            with self.subTest(platform=platform), tempfile.TemporaryDirectory() as directory:
                # Darwin's temporary-directory alias must not become a
                # different executable identity in its process observation.
                root = Path(directory).resolve()
                for name in ('state', 'bin', 'tools', 'flash', 'logs'): (root / name).mkdir(mode=0o700)
                runtime = root / 'bin/pulse-agent'; stored = root / 'state/stored-agent'
                stored.write_bytes(Path('/bin/sleep').read_bytes()); stored.chmod(0o700)
                watchdog = root / 'state/start-pulse-agent.sh'
                (root / 'tools/pkill').write_text('#!/bin/sh\nexit 0\n'); (root / 'tools/pkill').chmod(0o700)
                (root / 'tools/flash').write_text('#!/bin/sh\nexit 0\n'); (root / 'tools/flash').chmod(0o700)
                # Capture the native executable before placing the adapter on
                # PATH. The same untruncated kernel rows feed startup and the
                # unchanged production teardown parser (no model or PID claim).
                native_ps = shutil.which('ps')
                self.assertIsNotNone(native_ps, 'native process reader missing')
                fixture_ps = write_wide_ps_adapter(root / 'tools', Path(native_ps).resolve())
                env = dict(os.environ, PATH=str(root / 'tools') + ':' + os.environ['PATH'])
                setup = '\n'.join(k + '=' + shlex.quote(str(v)) for k, v in dict(
                    STATE_DIR=root / 'state', STATE_DIR_REMOVAL_AUTHORITY=root / 'state', INSTALL_DIR=root / 'bin',
                    WRAPPER_SCRIPT=watchdog, UNRAID_STORED_BINARY=stored, RUNTIME_BINARY=runtime,
                    AGENT_LOG_FILE=root / 'logs/agent.log').items())
                setup += '\nAGENT_NAME=pulse-agent BINARY_NAME=pulse-agent EXEC_ARGS=30\nEXPORT_SERVICE_ENV="" SHELL_EXPORT_LINES=""\n'
                if platform == 'QNAP':
                    renderer = definitions(self.source, ('write_qnap_wrapper_script',))
                    render = setup + renderer + '\nwrite_qnap_wrapper_script "$WRAPPER_SCRIPT" "$RUNTIME_BINARY" "$UNRAID_STORED_BINARY" ' + shlex.quote(str(root / 'logs')) + ' "$STATE_DIR"\n'
                else:
                    start = self.source.index('    cat > "$WRAPPER_SCRIPT" <<EOF', self.source.index('# 3. Unraid'))
                    end = self.source.index('\nEOF\n', start) + 5
                    render = setup + self.source[start:end].replace('/var/log/${AGENT_NAME}-watchdog.log', str(root / 'logs/watchdog.log'))
                render_file = root / 'render.sh'; render_file.write_text(render)
                rendered = subprocess.run(['bash', str(render_file)], env=env, capture_output=True, text=True)
                self.assertEqual(rendered.returncode, 0, rendered.stdout + rendered.stderr)
                bootfile = root / 'flash/autorun.sh' if platform == 'QNAP' else root / 'go'
                if platform == 'QNAP':
                    bootfile.write_text('#!/bin/sh\n')
                    emitter = definitions(self.source, ('portable_sed_in_place', 'remove_qnap_autorun_block', 'render_qnap_autorun_block', 'append_qnap_autorun_block'))
                    render_file.write_text(setup + emitter + '\nappend_qnap_autorun_block ' + shlex.quote(str(bootfile)) + ' "$WRAPPER_SCRIPT" "$STATE_DIR"\n')
                    rendered = subprocess.run(['bash', str(render_file)], env=env, capture_output=True, text=True)
                    self.assertEqual(rendered.returncode, 0, rendered.stdout + rendered.stderr)
                else: bootfile.write_text('#!/bin/bash\n# Pulse Agent\nbash ' + str(watchdog) + '\n')
                functions = definitions(self.source)
                functions = functions.replace('/boot/config/go', str(bootfile))
                functions = functions.replace('[[ -d /boot/config && ! -L /boot/config ]]', '[[ -d ' + str(root) + ' && ! -L ' + str(root) + ' ]]')
                functions = functions.replace('/etc/init.d/init_disk.sh', str(root / 'tools/flash'))
                functions = functions.replace('/tmp/nasconfig_tmp', str(root / 'flash'))
                control = root / 'control.sh'
                control.write_text('set -euo pipefail\n' + setup + '\n' + functions + '\ncommand() { [[ "$*" != "-v systemctl" ]] || return 1; builtin command "$@"; }\nquiesce_nas_agent_for_removal ' + platform + '\n')
                output = root / 'watchdog.log'
                with output.open('wb') as stream:
                    process = subprocess.Popen([str(watchdog)] if platform == 'QNAP' else ['bash', str(watchdog)], env=env,
                                               stdout=stream, stderr=stream, start_new_session=True)
                    try:
                        deadline = time.monotonic() + 8
                        while time.monotonic() < deadline:
                            listing = subprocess.run([str(fixture_ps), '-ax', '-o', 'comm=', '-o', 'args='], capture_output=True, text=True, check=True).stdout
                            if generated_collector_is_running(listing, runtime): break
                            self.assertIsNone(process.poll(), output.read_text()); time.sleep(.05)
                        else:
                            # Retain only owned fixture rows/logs, never other
                            # process arguments, if hosted startup still fails.
                            owned = '\n'.join(line for line in listing.splitlines() if str(root) in line)
                            logs = '\n'.join(p.read_text() for p in (root / 'logs').glob('*.log'))
                            self.fail('private generated collector did not start\n' + owned + '\n' + output.read_text() + '\n' + logs)
                        result = subprocess.run(['bash', str(control)], env=env, capture_output=True, text=True, timeout=12)
                        self.assertEqual(result.returncode, 0, result.stdout + result.stderr + output.read_text())
                        process.wait(timeout=3)
                        with self.assertRaises(ProcessLookupError): os.killpg(process.pid, 0)
                        self.assertNotIn('# Pulse Agent', bootfile.read_text())
                    finally:
                        try: os.killpg(process.pid, signal.SIGKILL)
                        except ProcessLookupError: pass
                        process.wait(timeout=3)

    def test_private_process_reader_requests_full_width_without_changing_fields(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            native = root / 'native ps'
            # Fixed synthetic argv recorder, not a process-inventory model.
            native.write_text('#!/bin/sh\nprintf "%s\\n" "$@"\n')
            native.chmod(0o700)
            tools = root / 'tools'; tools.mkdir()
            adapter = write_wide_ps_adapter(tools, native)
            for fields in (
                ['-ax', '-o', 'comm=', '-o', 'args='],
                ['-ax', '-o', 'pid=', '-o', 'ppid=', '-o', 'comm=', '-o', 'args='],
            ):
                with self.subTest(fields=fields):
                    result = subprocess.run([str(adapter), *fields], capture_output=True, text=True, check=True)
                    self.assertEqual(result.stdout.splitlines(), ['-ww', *fields])

    def test_generated_collector_observation_keeps_exact_identity_on_both_unix_formats(self):
        runtime = Path('/synthetic/owned/bin/pulse-agent')
        for comm in ('pulse-agent', str(runtime)):
            with self.subTest(comm=comm):
                self.assertTrue(generated_collector_is_running(f'{comm} {runtime} 30\n', runtime))
        for listing in (
            f'other {runtime} 30\n',
            'pulse-agent /synthetic/foreign/bin/pulse-agent 30\n',
            f'pulse-agent {runtime}-prod 30\n',
            f'pulse-agent /bin/sh echo {runtime} 30\n',
            f'pulse-agent {runtime}\n',
            'pulse-agent\n',
            '',
        ):
            with self.subTest(listing=listing):
                self.assertFalse(generated_collector_is_running(listing, runtime))

    def test_caller_has_no_blanket_nas_or_legacy_tree_deletion(self):
        caller = self.source.split('# --- Uninstall Logic ---\n', 1)[1].split('# --- Validation ---\n', 1)[0]
        self.assertNotIn("sed -i '/pulse-agent/d'", caller)
        self.assertNotIn('rm -rf /boot/config/pulse', caller)
        self.assertNotIn('rm -rf "$qnap_state_dir"', caller)
        self.assertIn('teardown_nas_agent_boot_owner "$uninstall_nas_platform"', caller)
        self.assertIn('[[ "$uninstall_upstart" != true && -z "$uninstall_nas_platform" ]]', caller)


if __name__ == '__main__': unittest.main(verbosity=2)
