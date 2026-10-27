#!/usr/bin/env python3
"""Exercise the real Unraid boot writer in private ordinary-user directories.

Boot text is never executed. Filesystem faults/concurrency and registration
results are models, not a native NAS, flash, watchdog or installed-agent proof.
"""
from pathlib import Path
import os
import shlex
import shutil
import subprocess
import tempfile
import unittest

SOURCE = Path(__file__).resolve().parents[2] / 'scripts/install.sh'
BASH = shutil.which('bash')
NAMES = ('portable_path_uid', 'nas_agent_removal_file_safe',
         'filter_nas_agent_boot_file', 'render_unraid_go_launch', 'write_unraid_go_entry')


def definitions(source):
    result = []
    for name in NAMES:
        marker = name + '() {'
        if source.count(marker) != 1:
            raise ValueError('missing or ambiguous production function: ' + name)
        result.append(marker + source.split(marker, 1)[1].split('\n}\n', 1)[0] + '\n}\n')
    return '\n'.join(result)


class UnraidGoUpdate(unittest.TestCase):
    source = SOURCE.read_text()

    def setUp(self):
        directory = tempfile.TemporaryDirectory(prefix='pulse-unraid-boot-')
        self.addCleanup(directory.cleanup)
        self.root = Path(directory.name)
        for name in ('flash', 'state', 'tools'):
            (self.root / name).mkdir(mode=0o700)
        self.boot = self.root / 'flash/go'
        self.wrapper = self.root / 'state/start-pulse-agent.sh'
        self.runtime = self.root / 'bin/pulse-agent'
        self.secret = self.root / 'state/token'
        self.secret.write_text('synthetic-secret-not-for-logs')
        self.environment = dict(os.environ, PATH=str(self.root / 'tools') + ':' + os.environ['PATH'])

    def invoke(self, override=''):
        code = ('set -euo pipefail\nAGENT_NAME=pulse-agent\n' + definitions(self.source) +
                '\n' + override + '\nwrite_unraid_go_entry "$1" "$2" "$3"\n')
        result = subprocess.run([BASH, '-c', code, '_', str(self.boot), str(self.wrapper), str(self.runtime)],
                                env=self.environment, capture_output=True, text=True, timeout=10)
        self.assertNotIn('synthetic-secret-not-for-logs', result.stdout + result.stderr)
        self.assertEqual(self.secret.read_text(), 'synthetic-secret-not-for-logs')
        self.assertFalse((self.root / 'executed').exists(), 'boot text must not execute')
        return result

    def write(self, content):
        self.boot.write_bytes(content if isinstance(content, bytes) else content.encode())
        self.boot.chmod(0o640)

    def stub(self, name, code):
        path = self.root / 'tools' / name
        path.write_text('#!/bin/sh\n' + code + '\n')
        path.chmod(0o700)

    def assert_refused_unchanged(self, original):
        self.write(original)
        before = self.boot.read_bytes()
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.boot.read_bytes(), before)

    def test_first_install_and_repeat_create_one_detached_entry(self):
        self.assertEqual(self.invoke().returncode, 0)
        first = self.boot.read_bytes()
        self.assertTrue(first.startswith(b'#!/bin/bash\n'))
        self.assertEqual(first.count(b'# Pulse Agent\n'), 1)
        self.assertTrue(first.endswith(b' >> /var/log/pulse-agent.log 2>&1 &\n'))
        self.assertEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_bytes(), first)
        self.assertEqual(list(self.boot.parent.glob('*.pulse-setup.*')), [])

    def test_unrelated_jobs_comments_and_multiline_constructs_survive(self):
        original = ('#!/bin/bash\n# Archive start-pulse-agent.sh first\n'
                    'cp /private/start-pulse-agent.sh.bak /backup/wrapper\n'
                    'bash /other/start-pulse-agent-prod.sh &\n'
                    'if true; then\n  echo unrelated\nfi\n'
                    'cat <<EOF\nordinary data\nEOF\n'
                    'printf synthetic-secret-not-for-logs\n'
                    'touch ' + shlex.quote(str(self.root / 'executed')) + '\n')
        self.write(original)
        self.assertEqual(self.invoke().returncode, 0)
        self.assertTrue(self.boot.read_text().startswith(original))
        self.assertEqual(self.boot.stat().st_mode & 0o777, 0o640)
        first = self.boot.read_bytes()
        self.assertEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_bytes(), first)

    def test_exact_legacy_current_and_crlf_entries_become_one(self):
        self.write('#!/bin/bash\r\necho before\r\n# Pulse Agent\r\nbash "' + str(self.wrapper) + '"\r\necho after\r\n')
        self.assertEqual(self.invoke().returncode, 0)
        first = self.boot.read_text()
        self.assertTrue(first.startswith('#!/bin/bash\necho before\necho after\n'))
        entry = first[first.index('# Pulse Agent\n'):]
        self.write(first + entry)
        self.assertEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_text(), first)

    def test_compound_foreign_and_indirect_launches_are_not_deleted(self):
        commands = ('printf unrelated; bash ' + str(self.wrapper),
                    'bash ' + str(self.wrapper) + ' && echo unrelated',
                    'bash /other/start-pulse-agent.sh &',
                    'setsid bash ' + str(self.wrapper) + ' &',
                    str(self.runtime) + ' --url https://pulse.example.invalid &',
                    'w=start-pulse-agent.sh; bash "$w"',
                    'echo literal-start-pulse-agent.sh',
                    'swapon /cache/swapfile # start-pulse-agent.sh needs this')
        for i, command in enumerate(commands):
            with self.subTest(command=command):
                self.boot = self.boot.with_name('unknown-' + str(i))
                self.assert_refused_unchanged('#!/bin/bash\n' + command + '\necho later\n')

    def test_exact_text_inside_shell_context_is_not_an_owned_launch(self):
        launch = 'bash ' + str(self.wrapper) + '\n'
        contexts = ('cat <<EOF\n' + launch + 'EOF\n',
                    'printf \'multiline\n' + launch + '\'\n',
                    'if true; then\n' + launch + 'fi\n',
                    'echo unrelated \\\n' + launch,
                    'echo unrelated |\n' + launch,
                    'cat <<EOF\n# Pulse Agent\n' + launch + 'EOF\n')
        for i, context in enumerate(contexts):
            with self.subTest(context=context):
                self.boot = self.boot.with_name('context-' + str(i))
                self.assert_refused_unchanged('#!/bin/bash\n' + context)

    def test_malformed_incomplete_binary_and_oversized_files_stay_unchanged(self):
        contents = ('#!/bin/bash\n# Pulse Agent\necho unrelated\n',
                    '#!/bin/bash\necho incomplete', '#!/bin/bash\nif true; then\n',
                    '#!/bin/bash\ncat <<EOF\nincomplete heredoc\n',
                    b'#!/bin/bash\necho before\0\n', '#!/bin/bash\n#' + 'x' * 1048577 + '\n')
        for i, content in enumerate(contents):
            with self.subTest(case=i):
                self.boot = self.boot.with_name('invalid-' + str(i))
                self.assert_refused_unchanged(content)

    def test_linked_files_directories_and_linked_parent_are_not_rewritten(self):
        other = self.root / 'foreign'; other.write_text('untouched\n')
        self.boot.symlink_to(other)
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertTrue(self.boot.is_symlink()); self.assertEqual(other.read_text(), 'untouched\n')
        self.boot.unlink(); self.boot.mkdir()
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertTrue(self.boot.is_dir())
        self.boot.rmdir(); self.boot.parent.rmdir()
        foreign = self.root / 'foreign-directory'; foreign.mkdir()
        (foreign / 'go').write_text('untouched\n')
        (self.root / 'flash').symlink_to(foreign)
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_text(), 'untouched\n')

    def test_unreadable_or_foreign_owned_file_is_not_rewritten(self):
        self.write('#!/bin/bash\necho kept\n')
        self.boot.chmod(0)
        try:
            self.assertNotEqual(self.invoke().returncode, 0)
        finally:
            self.boot.chmod(0o640)
        before = self.boot.read_bytes()
        self.assertNotEqual(self.invoke('portable_path_uid() { printf "999999\\n"; }').returncode, 0)
        self.assertEqual(self.boot.read_bytes(), before)

    def test_pending_setup_and_removal_recovery_bar_another_write(self):
        self.write('#!/bin/bash\necho kept\n')
        for suffix in ('.pulse-setup.retained', '.pulse-removal.retained'):
            with self.subTest(suffix=suffix):
                recovery = Path(str(self.boot) + suffix); recovery.mkdir()
                before = self.boot.read_bytes()
                self.assertNotEqual(self.invoke().returncode, 0)
                self.assertEqual(self.boot.read_bytes(), before)
                recovery.rmdir()

    def test_copy_rename_and_parser_failures_preserve_original(self):
        for command in ('cp', 'mv', 'bash'):
            with self.subTest(command=command):
                self.boot = self.boot.with_name(command + '-failure')
                self.write('#!/bin/bash\necho kept\n'); before = self.boot.read_bytes()
                self.stub(command, 'exit 1')
                self.assertNotEqual(self.invoke().returncode, 0)
                self.assertEqual(self.boot.read_bytes(), before)
                (self.root / 'tools' / command).unlink()

    def test_fat_boot_does_not_require_chmod(self):
        # Model the unsupported operation, not an actual FAT mount. go is
        # consumed by bash, so no successful permission change is required.
        self.stub('chmod', 'exit 1')
        self.assertEqual(self.invoke().returncode, 0)
        first = self.boot.read_bytes()
        self.assertEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_bytes(), first)

    def test_partial_stage_write_cannot_truncate_original(self):
        self.write('#!/bin/bash\necho kept\n'); before = self.boot.read_bytes()
        self.stub('cat', 'printf partial; exit 28')
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_bytes(), before)
        originals = list(self.boot.parent.glob('*.pulse-setup.*/original'))
        self.assertEqual(len(originals), 1); self.assertEqual(originals[0].read_bytes(), before)

    def test_partial_renderer_preserves_original_and_recovery(self):
        self.write('#!/bin/bash\necho kept\n'); before = self.boot.read_bytes()
        result = self.invoke('render_unraid_go_launch() { printf partial; return 1; }')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.boot.read_bytes(), before)
        self.assertTrue(list(self.boot.parent.glob('*.pulse-setup.*/original')))

    def test_source_change_during_staging_is_not_clobbered(self):
        self.write('#!/bin/bash\necho kept\n'); before = self.boot.read_bytes()
        self.stub('cp', shlex.quote(shutil.which('cp')) + ' "$@" || exit $?\nif [ "$2" = ' + shlex.quote(str(self.boot)) + ' ]; then printf "# concurrent change\\n" >> "$2"; fi')
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_bytes(), before + b'# concurrent change\n')

    def test_first_write_refuses_concurrent_file_directory_or_link(self):
        other = self.root / 'foreign'; other.write_text('foreign target\n')
        for kind in ('file', 'directory', 'link'):
            with self.subTest(kind=kind):
                self.boot = self.boot.with_name('new-' + kind)
                create = ('ln -s ' + shlex.quote(str(other)) + ' ' + shlex.quote(str(self.boot)) if kind == 'link'
                          else 'mkdir ' + shlex.quote(str(self.boot)) if kind == 'directory'
                          else 'printf "foreign file\\n" > ' + shlex.quote(str(self.boot)))
                self.stub('cat', shlex.quote(shutil.which('cat')) + ' "$@" || exit $?\ncase "$1" in */updated) ' + create + ';; esac')
                self.assertNotEqual(self.invoke().returncode, 0)
                self.assertEqual(other.read_text(), 'foreign target\n')
                if kind == 'link': self.assertTrue(self.boot.is_symlink())
                elif kind == 'directory': self.assertTrue(self.boot.is_dir())
                else: self.assertEqual(self.boot.read_text(), 'foreign file\n')
                self.assertTrue(list(self.boot.parent.glob(self.boot.name + '.pulse-setup.*/updated')))

    def test_partial_first_write_is_not_success_and_blocks_blind_retry(self):
        self.stub('cat', 'case "$1" in */ready) printf partial; exit 28;; esac\n' + shlex.quote(shutil.which('cat')) + ' "$@"')
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_bytes(), b'partial')
        staged = list(self.boot.parent.glob('*.pulse-setup.*/updated'))
        self.assertEqual(len(staged), 1); self.assertIn(b'# Pulse Agent\n', staged[0].read_bytes())
        (self.root / 'tools/cat').unlink()
        self.assertNotEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_bytes(), b'partial')

    def test_final_readback_and_cleanup_failures_are_not_success(self):
        for command in ('mv', 'rmdir'):
            with self.subTest(command=command):
                self.boot = self.boot.with_name('terminal-' + command)
                self.write('#!/bin/bash\necho kept\n')
                code = ('exit 1' if command == 'rmdir' else shlex.quote(shutil.which('mv')) + ' "$@" || exit $?\nprintf "# concurrent change\\n" >> ' + shlex.quote(str(self.boot)))
                self.stub(command, code)
                self.assertNotEqual(self.invoke().returncode, 0)
                self.assertTrue(self.boot.read_text().startswith('#!/bin/bash\necho kept\n'))
                self.assertTrue(list(self.boot.parent.glob(self.boot.name + '.pulse-setup.*')))
                (self.root / 'tools' / command).unlink()
                self.assertNotEqual(self.invoke().returncode, 0)

    def test_too_many_entries_need_reconciliation_not_unbounded_parser_work(self):
        self.assert_refused_unchanged('#!/bin/bash\n' + ('# Pulse Agent\nbash ' + str(self.wrapper) + '\n') * 17)

    def test_wrapper_paths_with_spaces_remain_idempotent(self):
        self.wrapper = self.root / 'state with spaces/start-pulse-agent.sh'
        self.assertEqual(self.invoke().returncode, 0)
        first = self.boot.read_bytes()
        self.assertIn(b'state\\ with\\ spaces/', first)
        self.assertEqual(self.invoke().returncode, 0)
        self.assertEqual(self.boot.read_bytes(), first)

    def test_registration_failure_exits_before_start_or_persistence_success(self):
        start = self.source.index('    if write_unraid_go_entry "$GO_SCRIPT"')
        entry = self.source[start:self.source.index('    # Start the agent now using the wrapper', start)]
        events = self.root / 'events'
        for fault, expected in ((False, ['replace', 'configured', 'would-start']), (True, ['replace', 'failed'])):
            with self.subTest(fault=fault):
                events.write_text('')
                setup = ('set -euo pipefail\nGO_SCRIPT=fixture WRAPPER_SCRIPT=fixture RUNTIME_BINARY=fixture EXIT_GENERAL=1\n'
                         'event() { printf "%s\\n" "$1" >> ' + shlex.quote(str(events)) + '; }\n'
                         'write_unraid_go_entry() { event replace; return ' + ('1' if fault else '0') + '; }\n'
                         'log_info() { event configured; }\nfail() { event failed; exit 1; }\n')
                result = subprocess.run([BASH, '-c', setup + entry + '\nevent would-start\n'], capture_output=True, text=True, timeout=5)
                self.assertEqual(result.returncode != 0, fault, result.stderr)
                self.assertEqual(events.read_text().splitlines(), expected)


if __name__ == '__main__':
    unittest.main()
