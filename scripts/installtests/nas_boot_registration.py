#!/usr/bin/env python3
"""Exercise the actual NAS boot-file producer and installer callers in private files.

No real boot scripts, mounts, signals, services, credentials or network are used.
Filesystem operations are real except explicitly named refusal/race controls.
"""
import os
from pathlib import Path
import shlex
import stat
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get('PULSE_NAS_BOOT_SOURCE', ROOT / 'scripts/install.sh'))
FUNCTIONS = ('portable_path_uid', 'portable_path_mode', 'nas_agent_removal_file_safe',
             'filter_nas_agent_boot_file', 'register_nas_agent_boot_file',
             'remove_qnap_autorun_block', 'render_qnap_autorun_block', 'append_qnap_autorun_block')


def definitions(source):
    result = []
    for name in FUNCTIONS:
        marker = name + '() {'
        if marker not in source:  # Exact-parent adverse control, not substituted code.
            continue
        if source.count(marker) != 1:
            raise ValueError('ambiguous production function ' + name)
        result.append(marker + source.split(marker, 1)[1].split('\n}\n', 1)[0] + '\n}\n')
    return '\n'.join(result)


class NASBootRegistration(unittest.TestCase):
    source = SOURCE.read_text()

    def exercise(self, platform='Unraid', initial='legacy', fault='', caller=False, repeat=False, state_name='state'):
        with tempfile.TemporaryDirectory(prefix='pulse-nas-boot-') as temporary:
            root = Path(temporary)
            state = root / state_name; state.mkdir()
            flash = root / 'flash'; flash.mkdir()
            boot = flash / ('autorun.sh' if platform == 'QNAP' else 'go')
            target = state / 'start-pulse-agent.sh'
            target.write_text('#!/bin/sh\nprintf "watchdog\\n" >> "$FIXTURE/events"\n')
            target.chmod(0o700)
            outside = root / 'outside'; outside.write_bytes(b'outside unchanged\n')
            untouched = ('#!/bin/bash\n# pulse-agent operator notes\necho pulse-agent-prod\n'
                         'bash /other/start-pulse-agent-prod.sh\n'
                         'printf "unrelated pulse-agent command\\n"\n'
                         'touch ' + shlex.quote(str(root / 'must-not-execute')) + '\n')
            if initial == 'legacy':
                launch = ('bash ' + str(target) if platform == 'Unraid' else 'sh "' + str(target) + '" &')
                boot.write_text(untouched + '# Pulse Agent\n' + launch + '\necho after\n')
            elif initial == 'duplicate':
                launch = ('bash "' + str(target) + '"' if platform == 'Unraid' else '"' + str(target) + '" &')
                boot.write_text(untouched + ('# Pulse Agent\n' + launch + '\n') * 2 + 'echo after\n')
            elif initial == 'fresh':
                pass
            elif initial == 'symlink': boot.symlink_to(outside)
            elif initial == 'directory': boot.mkdir()
            else:
                bad = {'ambiguous': '# Pulse Agent\necho do-not-delete\n',
                       'foreign': 'sh /other/start-pulse-agent.sh &\n',
                       'edited-block': '# Pulse Agent bootstrap begin\necho custom\n# Pulse Agent bootstrap end\n',
                       'truncated': '# Pulse Agent\n', 'no-newline': 'echo retain',
                       'control': 'echo retain\r\n', 'nul': 'echo retain\0\n',
                       'oversized': '#' + 'x' * 1048576 + '\n', 'syntax': 'if true; then\necho retain\n'}
                boot.write_text(untouched + bad[initial])
            if boot.is_file() and not boot.is_symlink(): boot.chmod(0o751)
            before = boot.read_bytes() if boot.is_file() else None
            inode_before = boot.stat().st_ino if boot.is_file() else None
            mode_before = stat.S_IMODE(boot.stat().st_mode) if boot.is_file() else None
            shim = r'''
mv() {
    command mv "$@" || return
    if [[ "$2" == */retired ]]; then
        case "$FAULT" in
            changed-retired) printf 'changed during retirement\n' >> "$2" ;;
            destination-file) printf 'concurrent owner\n' > "$BOOT" ;;
            destination-link) ln -s "$FIXTURE/outside" "$BOOT" ;;
        esac
    fi
}
cp() {
    command cp "$@" || return
    if [[ "$FAULT" == changed-source && "$2" == */original ]]; then
        printf 'concurrent source edit\n' >> "$BOOT"
    fi
}
chmod() {
    command chmod "$@" || return
    if [[ "$FAULT" == bad-readback && "$2" == "$BOOT" ]]; then
        printf 'concurrent final edit\n' >> "$BOOT"
    fi
}
cat() {
    if [[ "$FAULT" == partial-write && "${1:-}" == */replacement ]]; then
        printf 'partial write\n'; return 23
    fi
    command cat "$@"
}
'''
            setup = '\n'.join(k + '=' + shlex.quote(str(v)) for k, v in dict(
                STATE_DIR=state, INSTALL_DIR=root / 'bin', WRAPPER_SCRIPT=target,
                GO_SCRIPT=boot, UNRAID_STORAGE_DIR=state, AGENT_LOG_FILE=root / 'agent.log', QNAP_LOG_DIR=root / 'logs',
                BINARY_NAME='pulse-agent', AGENT_NAME='pulse-agent', EXIT_GENERAL=1).items())
            setup += r'''
log_info() { printf '[info] %s\n' "$1"; }
log_warn() { printf '[warn] %s\n' "$1"; }
fail() { printf '[failure] %s\n' "$1"; exit "$2"; }
complete_installation_flow() { printf 'complete\n' >> "$FIXTURE/events"; }
# The real installer disowns an infinite watchdog. Reap only our finite fixture.
disown() { wait; }
'''
            code = 'register_nas_agent_boot_file ' + platform + ' "$BOOT"\n'
            if caller and platform == 'Unraid':
                code = self.source.split('    # Preserve unrelated boot commands;', 1)[1]
                code = '    # Preserve unrelated boot commands;' + code.split('\nfi\n\n# 3b.', 1)[0]
                code = code.replace('"/var/log/${AGENT_NAME}.log"', '"$FIXTURE/agent.log"')
                code = code.replace('    exit 0', '    wait  # Reap the finite fixture watchdog before inspecting its event.\n    exit 0')
            elif caller:
                tool = root / 'flash-manager'
                tool.write_text('#!/bin/sh\nprintf "flash:%s\\n" "$1" >> "$FIXTURE/events"\n'
                                'if [ "$FAULT" = unmount-refusal ] && [ "$1" = umount_flash_config ]; then exit 23; fi\n')
                tool.chmod(0o700)
                code = '    AUTORUN_CONFIGURED=false' + self.source.split('    AUTORUN_CONFIGURED=false', 1)[1].split('\nfi\n\n# 4.', 1)[0]
                code = code.replace('/etc/init.d/init_disk.sh', shlex.quote(str(tool)))
                code = code.replace('/tmp/nasconfig_tmp', str(flash))
                code = code.replace('"/var/log/${AGENT_NAME}.log"', '"$FIXTURE/agent.log"')
                code = code.replace('    exit 0', '    wait  # Reap the finite fixture watchdog before inspecting its event.\n    exit 0')
            env = dict(os.environ, FIXTURE=str(root), BOOT=str(boot), FAULT=fault)
            results = []
            for _ in range(2 if repeat else 1):
                result = subprocess.run(['bash', '-c', 'set -euo pipefail\n' + definitions(self.source)
                                         + '\n' + shim + setup + '\n' + code],
                                        env=env, capture_output=True, text=True, timeout=15)
                results.append(result)
            after = boot.read_bytes() if boot.is_file() else None
            recovery = sorted(flash.glob(boot.name + '.pulse-install.*'))
            retained = {p.name: {f.name: f.read_bytes() for f in p.iterdir()} for p in recovery}
            events = (root / 'events').read_text().splitlines() if (root / 'events').exists() else []
            self.assertEqual(outside.read_bytes(), b'outside unchanged\n')
            self.assertFalse((root / 'must-not-execute').exists(), 'boot source was executed')
            for result in results:
                self.assertNotIn(untouched, result.stdout + result.stderr, 'boot content leaked')
            return dict(results=results, before=before, after=after, recovery=retained,
                        events=events, untouched=untouched.encode(),
                        inode_before=inode_before, inode_after=boot.stat().st_ino if boot.is_file() else None,
                        mode_before=mode_before, mode_after=stat.S_IMODE(boot.stat().st_mode) if boot.is_file() else None)

    def test_shared_boot_commands_survive_actual_registration_and_repeat(self):
        for platform in ('Unraid', 'QNAP'):
            for initial in ('fresh', 'legacy', 'duplicate'):
                with self.subTest(platform=platform, initial=initial):
                    r = self.exercise(platform, initial, repeat=True)
                    for result in r['results']: self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertEqual(r['recovery'], {})
                    if initial != 'fresh':
                        self.assertTrue(r['after'].startswith(r['untouched']))
                        self.assertIn(b'echo after\n', r['after'])
                        self.assertEqual(r['mode_after'], r['mode_before'])
                    self.assertEqual(r['after'].count(b'# Pulse Agent\n') + r['after'].count(b'# Pulse Agent bootstrap begin\n'), 1)

    def test_converged_source_is_an_inode_preserving_noop(self):
        # The repeated call in the first test checks bytes/idempotence. Here the
        # source fixture already matches the canonical Unraid producer exactly.
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); target = root / 'start-pulse-agent.sh'; boot = root / 'go'
            boot.write_text('#!/bin/bash\n# Pulse Agent\nbash "' + str(target) + '"\n')
            before = (boot.read_bytes(), boot.stat().st_ino, boot.stat().st_mode)
            setup = 'STATE_DIR=' + shlex.quote(str(root)) + '\nINSTALL_DIR=/usr/local/bin\nBINARY_NAME=pulse-agent\nAGENT_NAME=pulse-agent\n'
            result = subprocess.run(['bash', '-c', 'set -euo pipefail\n' + definitions(self.source) + setup
                                     + '\nregister_nas_agent_boot_file Unraid ' + shlex.quote(str(boot))], capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(before, (boot.read_bytes(), boot.stat().st_ino, boot.stat().st_mode))
            self.assertEqual(list(root.glob('go.pulse-install.*')), [])

    def test_unknown_or_unsafe_source_is_not_rewritten(self):
        for platform in ('Unraid', 'QNAP'):
            for initial in ('ambiguous', 'foreign', 'edited-block', 'truncated', 'no-newline', 'control', 'nul', 'oversized', 'syntax', 'symlink', 'directory'):
                with self.subTest(platform=platform, initial=initial):
                    r = self.exercise(platform, initial)
                    self.assertNotEqual(r['results'][0].returncode, 0)
                    self.assertEqual(r['before'], r['after'])
                    if initial == 'syntax':
                        self.assertEqual(next(iter(r['recovery'].values()))['original'], r['before'])
                    else: self.assertEqual(r['recovery'], {})

    def test_shell_expansion_in_boot_target_is_refused(self):
        for name in ('state spaces', 'state"quote', 'state\\slash', 'state$HOME', 'state`id`', 'state\nline'):
            with self.subTest(name=name):
                r = self.exercise(state_name=name)
                self.assertEqual(r['results'][0].returncode == 0, name == 'state spaces', r['results'][0].stderr)
                if name != 'state spaces': self.assertEqual(r['before'], r['after'])

    def test_concurrent_or_partial_publication_retains_original_and_staging(self):
        for platform in ('Unraid', 'QNAP'):
            for fault in ('changed-source', 'changed-retired', 'destination-file', 'destination-link', 'partial-write', 'bad-readback'):
                with self.subTest(platform=platform, fault=fault):
                    r = self.exercise(platform, fault=fault, repeat=True)
                    for result in r['results']: self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(len(r['recovery']), 1, 'repeat must not create another attempt')
                    recovery = next(iter(r['recovery'].values()))
                    self.assertEqual(recovery['original'], r['before'])
                    self.assertIn(r['untouched'], recovery['replacement'])
                    if fault == 'changed-source': self.assertEqual(r['after'], r['before'] + b'concurrent source edit\n')
                    if fault == 'destination-file': self.assertEqual(r['after'], b'concurrent owner\n')
                    if fault == 'partial-write': self.assertEqual(r['after'], b'partial write\n')

    def test_unraid_installer_caller_never_starts_after_refused_registration(self):
        for initial in ('legacy', 'ambiguous'):
            r = self.exercise(initial=initial, caller=True)
            self.assertEqual(r['results'][0].returncode == 0, initial == 'legacy', r['results'][0].stderr)
            self.assertEqual('complete' in r['events'], initial == 'legacy')
            self.assertEqual('watchdog' in r['events'], initial == 'legacy')
            if initial == 'ambiguous': self.assertEqual(r['before'], r['after'])

    def test_qnap_caller_unmounts_on_refusal_and_requires_unmount_success(self):
        for initial, fault in (('legacy', ''), ('ambiguous', ''), ('legacy', 'unmount-refusal')):
            with self.subTest(initial=initial, fault=fault):
                r = self.exercise('QNAP', initial, fault, caller=True)
                success = initial == 'legacy' and not fault
                self.assertEqual(r['results'][0].returncode == 0, success, r['results'][0].stderr)
                self.assertEqual(r['events'].count('flash:mount_flash_config'), 1)
                self.assertEqual(r['events'].count('flash:umount_flash_config'), 1)
                self.assertEqual('complete' in r['events'], success)
                self.assertEqual('watchdog' in r['events'], success)
                if initial == 'ambiguous': self.assertEqual(r['before'], r['after'])

    def test_installers_use_only_the_owned_boot_registration_producer(self):
        unraid = self.source.split('# 3. Unraid', 1)[1].split('# 3b. QNAP', 1)[0]
        self.assertIn('register_nas_agent_boot_file Unraid "$GO_SCRIPT"', unraid)
        self.assertNotIn("'/pulse-agent/d'", unraid)
        self.assertNotIn('cat >> "$GO_SCRIPT"', unraid)
        append = definitions(self.source).split('append_qnap_autorun_block() {', 1)[1]
        self.assertIn('register_nas_agent_boot_file QNAP "$autorun_path"', append)
        self.assertNotIn('sed', append)


def parent_control():
    source = SOURCE.read_text()
    with tempfile.TemporaryDirectory(prefix='pulse-nas-boot-parent-') as temporary:
        root = Path(temporary); boot = root / 'go'; state = root / 'state'; state.mkdir()
        original = '#!/bin/bash\necho pulse-agent-prod\nbash /other/start-pulse-agent-prod.sh\n'
        boot.write_text(original + '# Pulse Agent\nbash ' + str(state / 'start-pulse-agent.sh') + '\n')
        code = '    # Add to /boot/config/go if not already present' + source.split('    # Add to /boot/config/go if not already present', 1)[1].split('\n    log_info "Added startup entry', 1)[0]
        setup = 'GO_SCRIPT=' + shlex.quote(str(boot)) + '\nWRAPPER_SCRIPT=' + shlex.quote(str(state / 'start-pulse-agent.sh')) + '\n'
        result = subprocess.run(['bash', '-c', 'set -euo pipefail\n' + setup + code], capture_output=True, text=True)
        assert result.returncode == 0, result.stderr
        assert b'echo pulse-agent-prod\n' not in boot.read_bytes()
        assert b'bash /other/start-pulse-agent-prod.sh\n' not in boot.read_bytes()
        print('Exact parent Unraid installer returned 0 after deleting both unrelated boot commands.')
        # The original QNAP producer also removed another instance by basename.
        boot.write_text(original + 'sh ' + str(state / 'start-pulse-agent.sh') + ' &\n')
        code = definitions(source)
        if 'portable_sed_in_place() {' not in code:
            marker = 'portable_sed_in_place() {'
            code = marker + source.split(marker, 1)[1].split('\n}\n', 1)[0] + '\n}\n' + code
        setup += 'STATE_DIR=' + shlex.quote(str(state)) + '\nAGENT_NAME=pulse-agent\n'
        result = subprocess.run(['bash', '-c', 'set -euo pipefail\n' + setup + code + '\nappend_qnap_autorun_block "$GO_SCRIPT" "$WRAPPER_SCRIPT" "$STATE_DIR"'], capture_output=True, text=True)
        assert result.returncode == 0, result.stderr
        assert b'bash /other/start-pulse-agent-prod.sh\n' not in boot.read_bytes()
        print('Exact parent QNAP installer returned 0 after deleting the unrelated sibling launcher.')


if __name__ == '__main__':
    if sys.argv[1:] == ['--parent-control']: parent_control()
    else: unittest.main(verbosity=2)
