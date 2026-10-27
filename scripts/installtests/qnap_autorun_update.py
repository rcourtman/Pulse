#!/usr/bin/env python3
"""Run production QNAP boot-file replacement only in private ordinary-user fixtures.

No boot text, mount, service or NAS command runs. The caller test substitutes
only the fixed flash paths and models its manager's result.
"""
from pathlib import Path
import os
import shlex
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / 'scripts/install.sh'
NAMES = ('portable_path_uid', 'portable_path_mode', 'nas_agent_removal_file_safe',
         'filter_nas_agent_boot_file', 'render_qnap_autorun_block', 'append_qnap_autorun_block')


def definitions(source):
    result = []
    for name in NAMES:
        marker = name + '() {'
        if source.count(marker) != 1:
            raise ValueError('missing or ambiguous production function: ' + name)
        result.append(marker + source.split(marker, 1)[1].split('\n}\n', 1)[0] + '\n}\n')
    return '\n'.join(result)


class QNAPAutorunUpdate(unittest.TestCase):
    source = SOURCE.read_text()

    def setUp(self):
        directory = tempfile.TemporaryDirectory(prefix='pulse-qnap-boot-')
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name)
        for name in ('flash', 'state', 'tools'):
            (self.root / name).mkdir(mode=0o700)
        self.boot = self.root / 'flash/autorun.sh'
        self.wrapper = self.root / 'state/start-pulse-agent.sh'
        self.state = self.wrapper.parent
        (self.state / 'token').write_text('synthetic-secret-not-for-logs')
        self.functions = definitions(self.source)
        self.environment = dict(os.environ, PATH=str(self.root / 'tools') + ':' + os.environ['PATH'])

    def invoke(self, override=''):
        setup = 'AGENT_NAME=pulse-agent BINARY_NAME=pulse-agent INSTALL_DIR=/usr/local/bin\n'
        code = 'set -euo pipefail\n' + setup + self.functions + '\n' + override + '\nappend_qnap_autorun_block "$1" "$2" "$3"\n'
        result = subprocess.run(['bash', '-c', code, '_', str(self.boot), str(self.wrapper), str(self.state)],
                                env=self.environment, capture_output=True, text=True, timeout=10)
        self.assertNotIn('synthetic-secret-not-for-logs', result.stdout + result.stderr)
        self.assertEqual((self.state / 'token').read_text(), 'synthetic-secret-not-for-logs')
        self.assertFalse((self.root / 'executed').exists(), 'boot text must never execute')
        return result

    def write(self, content):
        self.boot.write_bytes(content if isinstance(content, bytes) else content.encode())
        self.boot.chmod(0o640)

    def stub(self, name, code):
        path = self.root / 'tools' / name
        path.write_text('#!/bin/sh\n' + code + '\n')
        path.chmod(0o700)

    def test_first_install_then_replacement_is_idempotent(self):
        self.assertEqual(self.invoke().returncode, 0)
        first = self.boot.read_bytes()
        self.assertEqual(first.count(b'# Pulse Agent bootstrap begin\n'), 1)
        self.assertEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_bytes(), first)
        self.assertTrue(os.access(self.boot, os.X_OK))
        self.assertEqual(list(self.boot.parent.glob('*.pulse-setup.*')), [])

    def test_unowned_comments_and_similarly_named_boot_jobs_survive(self):
        original = '#!/bin/sh\n# Archive start-pulse-agent.sh before running other jobs\ncp /private/start-pulse-agent.sh.bak /backup/pulse-wrapper\nprintf synthetic-secret-not-for-logs\ntouch ' + shlex.quote(str(self.root / 'executed')) + '\n'
        self.write(original)
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(self.boot.read_text().startswith(original))
        self.assertEqual(self.boot.stat().st_mode & 0o666, 0o640)
        self.assertTrue(self.boot.stat().st_mode & 0o100)
        self.assertEqual(self.invoke().returncode, 0)
        self.assertTrue(self.boot.read_text().startswith(original))

    def test_exact_legacy_and_duplicate_generated_entries_become_one(self):
        self.write('#!/bin/sh\necho before\n# Pulse Agent\n"' + str(self.wrapper) + '" &\necho after\n')
        self.assertEqual(self.invoke().returncode, 0)
        emitted = self.boot.read_text()
        block = '# Pulse Agent bootstrap begin' + emitted.split('# Pulse Agent bootstrap begin', 1)[1]
        self.write(emitted + block)
        self.assertEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_text().count('# Pulse Agent bootstrap begin\n'), 1)
        self.assertIn('echo before\necho after\n', self.boot.read_text())

    def test_malformed_unknown_and_incomplete_boot_content_is_not_rewritten(self):
        cases = ('#!/bin/sh\n# Pulse Agent bootstrap begin\necho unrelated-nightly-job\n',
                 '#!/bin/sh\n# Pulse Agent\necho unrelated-job\n',
                 '#!/bin/sh\nsh /foreign/start-pulse-agent.sh &\n',
                 '#!/bin/sh\necho start-pulse-agent.sh\n',
                 '#!/bin/sh\necho no-final-newline',
                 '#!/bin/sh\nif true; then\n', b'#!/bin/sh\necho before\0\n',
                 '#!/bin/sh\n#' + 'x' * 1048577 + '\n')
        for original in cases:
            with self.subTest(size=len(original), prefix=repr(original[:50])):
                # Retained stages deliberately bar a blind retry. Each case
                # gets an independent path, not a removed recovery directory.
                self.boot = self.boot.with_name('case-' + str(len(list(self.boot.parent.iterdir()))) + '.sh')
                self.write(original)
                before = self.boot.read_bytes()
                result = self.invoke()
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(self.boot.read_bytes(), before)

    def test_changed_generated_block_is_preserved_for_reconciliation(self):
        self.assertEqual(self.invoke().returncode, 0)
        before = self.boot.read_text().replace('_pulse_wait_max=1800', '_pulse_wait_max=999')
        self.write(before)
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_text(), before)

    def test_linked_files_directories_and_parent_links_are_not_replaced(self):
        other = self.root / 'other'; other.write_text('untouched\n')
        self.boot.symlink_to(other)
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertTrue(self.boot.is_symlink()); self.assertEqual(other.read_text(), 'untouched\n')
        self.boot.unlink(); self.boot.mkdir()
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertTrue(self.boot.is_dir())
        self.boot.rmdir(); (self.root / 'flash').rmdir()
        foreign = self.root / 'foreign'; foreign.mkdir()
        (foreign / 'autorun.sh').write_text('untouched\n')
        (self.root / 'flash').symlink_to(foreign)
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_text(), 'untouched\n')

    def test_existing_setup_or_removal_recovery_prevents_overwrite(self):
        self.write('#!/bin/sh\necho kept\n')
        for suffix in ('.pulse-setup.retained', '.pulse-removal.retained'):
            with self.subTest(suffix=suffix):
                recovery = Path(str(self.boot) + suffix); recovery.mkdir()
                before = self.boot.read_bytes()
                self.assertNotEqual(self.invoke().returncode, 0)
                self.assertEqual(self.boot.read_bytes(), before)
                recovery.rmdir()

    def test_copy_mode_rename_and_validation_failures_leave_original_intact(self):
        for command in ('cp', 'chmod', 'mv', 'sh'):
            with self.subTest(command=command):
                self.boot = self.boot.with_name(command + '.sh')
                self.write('#!/bin/sh\necho kept\n'); before = self.boot.read_bytes()
                self.stub(command, 'exit 1')
                self.assertNotEqual(self.invoke().returncode, 0)
                self.assertEqual(self.boot.read_bytes(), before)
                (self.root / 'tools' / command).unlink()

    def test_partial_renderer_cannot_truncate_active_boot_file(self):
        self.write('#!/bin/sh\necho kept\n'); before = self.boot.read_bytes()
        result = self.invoke("render_qnap_autorun_block() { if [[ -n \"${staging:-}\" && -s \"$staging/updated\" ]]; then printf partial; return 1; fi; return 0; }")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.boot.read_bytes(), before)
        self.assertTrue(list(self.boot.parent.glob('*.pulse-setup.*/original')))

    def test_source_change_while_staging_is_not_clobbered(self):
        self.write('#!/bin/sh\necho kept\n'); before = self.boot.read_bytes()
        self.stub('cp', shlex.quote(shutil.which('cp')) + ' "$@" || exit $?\nif [ "$2" = ' + shlex.quote(str(self.boot)) + ' ]; then printf "# concurrent change\\n" >> "$2"; fi')
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_bytes(), before + b'# concurrent change\n')

    def test_concurrent_first_file_or_link_is_not_overwritten(self):
        other = self.root / 'other'; other.write_text('foreign target\n')
        for kind in ('file', 'link'):
            with self.subTest(kind=kind):
                self.boot = self.boot.with_name(kind + '.sh')
                create = ('ln -s ' + shlex.quote(str(other)) + ' ' + shlex.quote(str(self.boot)) if kind == 'link'
                          else 'printf "foreign file\\n" > ' + shlex.quote(str(self.boot)))
                self.stub('cp', shlex.quote(shutil.which('cp')) + ' "$@" || exit $?\ncase "$3" in */ready) ' + create + ';; esac')
                self.assertNotEqual(self.invoke().returncode, 0)
                if kind == 'link':
                    self.assertTrue(self.boot.is_symlink())
                    self.assertEqual(other.read_text(), 'foreign target\n')
                else:
                    self.assertEqual(self.boot.read_text(), 'foreign file\n')
                self.assertTrue(list(self.boot.parent.glob(self.boot.name + '.pulse-setup.*/updated')))

    def test_unsafe_wrapper_interpolation_is_refused_without_execution(self):
        self.write('#!/bin/sh\necho kept\n'); before = self.boot.read_bytes()
        for name in ('dollar$dir', 'tick`dir', 'quote"dir', 'back\\dir', 'line\ndir'):
            with self.subTest(name=name):
                self.state = self.root / name
                self.state.mkdir(); (self.state / 'token').write_text('synthetic-secret-not-for-logs')
                self.wrapper = self.state / 'start-pulse-agent.sh'
                self.assertNotEqual(self.invoke().returncode, 0)
                self.assertEqual(self.boot.read_bytes(), before)

    def test_real_registration_caller_checks_append_and_unmount_before_success(self):
        entry = '    AUTORUN_CONFIGURED=false\n' + self.source.split('    AUTORUN_CONFIGURED=false\n', 1)[1].split('    if [[ "$AUTORUN_CONFIGURED" != true ]]; then', 1)[0]
        mount = self.root / 'tools/init-disk'
        entry = entry.replace('/etc/init.d/init_disk.sh', str(mount)).replace('/tmp/nasconfig_tmp', str(self.boot.parent))
        events = self.root / 'events'
        for fault, expected, failed in (
                ('none', ['mount_flash_config', 'append', 'umount_flash_config', 'configured'], False),
                ('append', ['mount_flash_config', 'append', 'umount_flash_config', 'failed'], True),
                ('umount', ['mount_flash_config', 'append', 'umount_flash_config', 'failed'], True)):
            with self.subTest(fault=fault):
                events.write_text('')
                mount.write_text('#!/bin/sh\nprintf "%s\\n" "$1" >> ' + shlex.quote(str(events)) + '\n' + ('[ "$1" != umount_flash_config ]\n' if fault == 'umount' else 'exit 0\n'))
                mount.chmod(0o700)
                setup = 'set -euo pipefail\nSTATE_DIR=' + shlex.quote(str(self.state)) + '\nWRAPPER_SCRIPT=' + shlex.quote(str(self.wrapper)) + '\nEXIT_GENERAL=1\n'
                setup += 'event() { printf "%s\\n" "$1" >> ' + shlex.quote(str(events)) + '; }\n'
                setup += 'append_qnap_autorun_block() { event append; ' + ('return 1' if fault == 'append' else 'return 0') + '; }\n'
                setup += 'fail() { event failed; exit 1; }\nlog_info() { event configured; }\n'
                result = subprocess.run(['bash', '-c', setup + entry], capture_output=True, text=True, timeout=5)
                self.assertEqual(result.returncode != 0, failed, result.stderr)
                self.assertEqual(events.read_text().splitlines(), expected)


if __name__ == '__main__':
    unittest.main()
