#!/usr/bin/env python3
"""Exercise the real lab importer only against synthetic temporary files.

The one fixed installed path is mapped in a script copy, never read on the
worker. Native copies/renames preserve bytes; closed peers inject read and
publication failures. No config is decrypted or sourced, no service is run,
and no real credentials, production directory or monitored system is used.
"""

import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = Path(os.environ.get("PULSE_DEV_CONFIG_TEST_SCRIPT", ROOT / "scripts/sync-production-config.sh"))
BASH = shutil.which("bash")
PRIVATE = "SYNTHETIC-PRIVATE-CONTENT-NOT-A-KEY"

PEER = r'''import os
from pathlib import Path
import sys
name = Path(sys.argv[0]).name
args = sys.argv[1:]
root = Path(os.environ['FIXTURE'])
with (root / 'calls').open('a') as output:
    output.write(name + '\n')
if name == 'cp':
    assert args[0] == '--' and len(args) == 3, args
    if Path(args[1]) == root / 'installed/email.enc':
        sys.exit(23)
if name == 'mv':
    assert args[:2] == ['-T', '--'] and len(args) == 4, args
    if Path(args[-1]).name == 'nodes.enc':
        sys.exit(24)
if name == 'cmp' and os.environ.get('CHANGE_SOURCE'):
    if '/.config-import.' in args[-1]:
        (root / 'installed/nodes.enc').write_bytes(b'observed synthetic source change')
os.execv(os.environ['REAL_' + name.upper()], [os.environ['REAL_' + name.upper()], *args])
'''


class DevConfigImportTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="pulse-dev-import-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.installed = self.root / "installed"
        self.installed.mkdir()
        self.dev = self.root / "development"
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.script = self.root / "sync-production-config.sh"
        self.script.write_text(SCRIPT.read_text().replace("/etc/pulse", str(self.installed)))
        self.script.chmod(0o700)
        self.env = {"PATH": f"{self.bin}:/usr/bin:/bin", "HOME": str(self.root), "LC_ALL": "C",
                    "DEV_DIR": str(self.dev), "FIXTURE": str(self.root)}
        self.seed = {".encryption.key": PRIVATE + "-source-key\n", "nodes.enc": PRIVATE + "-nodes\n",
                     "email.enc": PRIVATE + "-email\n", "webhooks.enc": PRIVATE + "-webhooks\n",
                     "ai.enc": PRIVATE + "-AI\n", "api_tokens.json": PRIVATE + "-tokens\n",
                     "system.json": '{"fixture": "installed"}\n', "guest_metadata.json": "{}\n"}
        for name, content in self.seed.items():
            self.write(self.installed / name, content)
        self.installed.chmod(0o750)

    def write(self, path, content, mode=0o600):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(content.encode() if isinstance(content, str) else content)
        path.chmod(mode)
        return path

    def snapshot(self, directory):
        if not directory.exists():
            return None
        result = {}
        for path in [directory, *sorted(directory.rglob("*"))]:
            info = path.lstat()
            value = os.readlink(path) if path.is_symlink() else path.read_bytes() if path.is_file() else None
            result[str(path.relative_to(directory))] = (stat.S_IFMT(info.st_mode), stat.S_IMODE(info.st_mode), value)
        return result

    def adapter(self, name):
        peer = self.bin / name
        peer.write_text(f"#!{sys.executable}\n" + PEER)
        peer.chmod(0o700)
        self.env["REAL_" + name.upper()] = shutil.which(name)

    def run_import(self, **settings):
        result = subprocess.run([BASH, str(self.script)], cwd=self.root, env=self.env | settings,
                                capture_output=True, text=True, timeout=10)
        self.output = result.stdout + result.stderr
        self.assertNotIn(PRIVATE, self.output)
        return result

    def assert_ok(self, result):
        self.assertEqual(result.returncode, 0, self.output)

    def assert_refused(self, result):
        self.assertNotEqual(result.returncode, 0, self.output)
        self.assertNotIn('Selected configuration imported.', self.output)

    def seed_dev(self):
        self.dev.mkdir()
        for name, value in {".encryption.key": self.seed[".encryption.key"],
                            "nodes.enc": PRIVATE + "-original-dev-nodes\n",
                            "email.enc": PRIVATE + "-original-dev-email\n",
                            "sessions.json": PRIVATE + "-sessions\n", "csrf_tokens.json": PRIVATE + "-csrf\n",
                            "oidc.enc": PRIVATE + "-dev-only-oidc\n", ".env": "synthetic environment text, never source me\n",
                            "alerts/retained": PRIVATE + "-alerts\n", "history/retained": PRIVATE + "-history\n"}.items():
            self.write(self.dev / name, value)

    def test_missing_source_key_never_restores_backup_or_changes_destination(self):
        (self.installed / '.encryption.key').unlink()
        for name in ('.encryption.key.bak-first', '.encryption.key.bak-second'):
            self.write(self.installed / name, PRIVATE + name)
        self.seed_dev()
        before_source, before_dev = self.snapshot(self.installed), self.snapshot(self.dev)
        result = self.run_import()
        with self.subTest(observation='installed state'):
            self.assertEqual(self.snapshot(self.installed), before_source)
        with self.subTest(observation='development state'):
            self.assertEqual(self.snapshot(self.dev), before_dev)
        self.assert_refused(result)
        self.assertIn('matching key independently', self.output)

    def test_different_destination_key_preserves_all_existing_state(self):
        self.seed_dev()
        self.write(self.dev / '.encryption.key', PRIVATE + '-different-key\n')
        before_source, before_dev = self.snapshot(self.installed), self.snapshot(self.dev)
        result = self.run_import()
        self.assertEqual(self.snapshot(self.installed), before_source)
        with self.subTest(observation='development state'):
            self.assertEqual(self.snapshot(self.dev), before_dev)
        self.assert_refused(result)
        self.assertIn('keys differ', self.output)

    def test_success_preserves_runtime_files_and_excluded_configuration(self):
        self.seed_dev()
        excluded = ('.env', 'oidc.enc', 'sessions.json', 'csrf_tokens.json', 'alerts/retained', 'history/retained')
        before = {name: (self.dev / name).read_bytes() for name in excluded}
        for name in ('.env', 'oidc.enc', 'sessions.json', 'csrf_tokens.json'):
            self.write(self.installed / name, PRIVATE + '-not-an-import\n')
        source = self.snapshot(self.installed)
        self.assert_ok(self.run_import())
        for name, content in before.items():
            self.assertEqual((self.dev / name).read_bytes(), content, name)
        self.assertEqual(self.snapshot(self.installed), source)
        for name, content in self.seed.items():
            self.assertEqual((self.dev / name).read_text(), content)
        snapshots = list(self.dev.glob('.config-import.*/previous'))
        self.assertEqual(len(snapshots), 1)
        self.assertEqual((snapshots[0] / 'nodes.enc').read_text(), PRIVATE + '-original-dev-nodes\n')
        self.assertEqual((snapshots[0] / 'email.enc').read_text(), PRIVATE + '-original-dev-email\n')
        for path in snapshots[0].iterdir():
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
        self.assertEqual(stat.S_IMODE(snapshots[0].parent.stat().st_mode), 0o700)

    def test_fresh_import_uses_private_modes_and_does_not_create_runtime_files(self):
        source = self.snapshot(self.installed)
        self.assert_ok(self.run_import())
        self.assertEqual(self.snapshot(self.installed), source)
        self.assertEqual(stat.S_IMODE(self.dev.stat().st_mode), 0o700)
        self.assertEqual({p.name for p in self.dev.iterdir()}, set(self.seed))
        for name, content in self.seed.items():
            self.assertEqual((self.dev / name).read_text(), content)
            self.assertEqual(stat.S_IMODE((self.dev / name).stat().st_mode), 0o600)

    def test_repeated_matching_import_is_noop_without_resetting_sessions(self):
        self.assert_ok(self.run_import())
        self.write(self.dev / 'sessions.json', PRIVATE + '-active-session\n')
        before = self.snapshot(self.dev)
        self.assert_ok(self.run_import())
        self.assertEqual(self.snapshot(self.dev), before)
        self.assertIn('already matches', self.output)

    def test_orphaned_development_ciphertext_or_backups_cannot_receive_a_guessed_key(self):
        for name in ('nodes.enc', 'nodes.enc.bak-fixture', 'oidc.enc', 'email.enc.previous'):
            with self.subTest(name=name), tempfile.TemporaryDirectory(dir=self.root) as temp:
                dev = Path(temp)
                self.write(dev / name, PRIVATE)
                self.write(dev / '.encryption.key.bak-fixture', PRIVATE + '-unproven-backup')
                before, source = self.snapshot(dev), self.snapshot(self.installed)
                self.assert_refused(self.run_import(DEV_DIR=str(dev)))
                self.assertEqual(self.snapshot(dev), before)
                self.assertEqual(self.snapshot(self.installed), source)

    def test_empty_or_special_keys_refuse_before_changes(self):
        for directory in (self.installed, self.dev):
            for kind in ('empty', 'directory', 'symlink', 'fifo'):
                with self.subTest(directory=directory.name, kind=kind):
                    path = directory / '.encryption.key'
                    directory.mkdir(exist_ok=True)
                    if path.exists(): path.unlink()
                    if kind == 'empty': path.touch()
                    elif kind == 'directory': path.mkdir()
                    elif kind == 'symlink': path.symlink_to(self.root / 'absent-key')
                    else: os.mkfifo(path)
                    before_source, before_dev = self.snapshot(self.installed), self.snapshot(self.dev)
                    self.assert_refused(self.run_import())
                    self.assertEqual(self.snapshot(self.installed), before_source)
                    self.assertEqual(self.snapshot(self.dev), before_dev)
                    if kind == 'directory': path.rmdir()
                    else: path.unlink()
                    self.write(path, self.seed['.encryption.key'])

    def test_overlapping_directories_and_aliases_refuse_before_creation_or_chmod(self):
        alias = self.root / 'installed-alias'
        alias.symlink_to(self.installed, target_is_directory=True)
        paths = (self.installed, self.installed / 'new/subdirectory', alias,
                 alias / 'new/../subdirectory', self.root)
        before = self.snapshot(self.installed)
        for path in paths:
            with self.subTest(path=str(path.relative_to(self.root))):
                self.assert_refused(self.run_import(DEV_DIR=str(path)))
                self.assertEqual(self.snapshot(self.installed), before)
                self.assertFalse((self.installed / 'new').exists())
                self.assertFalse((self.installed / 'subdirectory').exists())

    def test_linked_or_special_source_and_destination_files_are_not_followed(self):
        self.seed_dev()
        outside = self.write(self.root / 'external', PRIVATE + '-external')
        for directory in (self.installed, self.dev):
            for kind in ('symlink', 'dangling', 'directory', 'fifo'):
                with self.subTest(directory=directory.name, kind=kind):
                    path = directory / 'api_tokens.json'
                    path.unlink(missing_ok=True)
                    if kind == 'symlink': path.symlink_to(outside)
                    elif kind == 'dangling': path.symlink_to(self.root / 'absent')
                    elif kind == 'directory': path.mkdir()
                    else: os.mkfifo(path)
                    source, dev = self.snapshot(self.installed), self.snapshot(self.dev)
                    self.assert_refused(self.run_import())
                    self.assertEqual(self.snapshot(self.installed), source)
                    self.assertEqual(self.snapshot(self.dev), dev)
                    self.assertEqual(outside.read_text(), PRIVATE + '-external')
                    if kind == 'directory': path.rmdir()
                    else: path.unlink()
                    self.write(path, self.seed['api_tokens.json'])

    def test_copy_failure_before_publication_keeps_originals_and_cleans_only_owned_stage(self):
        self.seed_dev()
        self.write(self.dev / '.config-import.retained/previous/nodes.enc', PRIVATE + '-old-evidence')
        self.adapter('cp')
        source, dev = self.snapshot(self.installed), self.snapshot(self.dev)
        self.assert_refused(self.run_import())
        self.assertEqual(self.snapshot(self.installed), source)
        self.assertEqual(self.snapshot(self.dev), dev)
        self.assertNotIn('mv', (self.root / 'calls').read_text())

    def test_observed_source_change_refuses_publication_and_preserves_destination(self):
        self.seed_dev()
        dev = self.snapshot(self.dev)
        self.adapter('cmp')
        self.assert_refused(self.run_import(CHANGE_SOURCE='true'))
        self.assertEqual(self.snapshot(self.dev), dev)
        self.assertIn('changed or could not be rechecked', self.output)

    def test_publication_failure_retains_previous_and_unpublished_files_without_success(self):
        self.seed_dev()
        self.adapter('mv')
        source, key = self.snapshot(self.installed), (self.dev / '.encryption.key').read_bytes()
        self.assert_refused(self.run_import())
        self.assertEqual(self.snapshot(self.installed), source)
        self.assertEqual((self.dev / '.encryption.key').read_bytes(), key)
        snapshots = list(self.dev.glob('.config-import.*'))
        self.assertEqual(len(snapshots), 1)
        self.assertEqual((snapshots[0] / 'previous/nodes.enc').read_text(), PRIVATE + '-original-dev-nodes\n')
        self.assertEqual((snapshots[0] / 'new/nodes.enc').read_text(), self.seed['nodes.enc'])
        self.assertIn('Import incomplete', self.output)

    def test_destination_hardlink_is_replaced_without_modifying_installed_file_or_external_peer(self):
        self.seed_dev()
        external = self.write(self.root / 'external-settings', PRIVATE + '-external-settings', 0o640)
        (self.dev / 'system.json').unlink(missing_ok=True)
        os.link(external, self.dev / 'system.json')
        source = self.snapshot(self.installed)
        self.assert_ok(self.run_import())
        self.assertEqual(external.read_text(), PRIVATE + '-external-settings')
        self.assertEqual(stat.S_IMODE(external.stat().st_mode), 0o640)
        self.assertEqual(self.snapshot(self.installed), source)
        self.assertNotEqual(external.stat().st_ino, (self.dev / 'system.json').stat().st_ino)

    def test_legacy_plaintext_nodes_import_without_making_a_key_or_removing_data(self):
        for name in ('.encryption.key', 'nodes.enc', 'email.enc', 'webhooks.enc', 'ai.enc'):
            (self.installed / name).unlink()
        self.write(self.installed / 'nodes.json', PRIVATE + '-legacy-nodes')
        source = self.snapshot(self.installed)
        self.assert_ok(self.run_import())
        self.assertEqual((self.dev / 'nodes.json').read_text(), PRIVATE + '-legacy-nodes')
        self.assertFalse((self.dev / '.encryption.key').exists())
        self.assertEqual(self.snapshot(self.installed), source)

    def test_plaintext_source_cannot_be_hidden_by_retained_encrypted_development_nodes(self):
        self.seed_dev()
        (self.installed / 'nodes.enc').unlink()
        self.write(self.installed / 'nodes.json', PRIVATE + '-legacy')
        source, dev = self.snapshot(self.installed), self.snapshot(self.dev)
        self.assert_refused(self.run_import())
        self.assertEqual(self.snapshot(self.installed), source)
        self.assertEqual(self.snapshot(self.dev), dev)
        self.assertIn('plaintext installed nodes conflict', self.output)

    def test_missing_installation_leaves_new_or_existing_development_unchanged(self):
        shutil.rmtree(self.installed)
        for existing in (False, True):
            if existing: self.seed_dev()
            before = self.snapshot(self.dev)
            self.assert_ok(self.run_import())
            self.assertEqual(self.snapshot(self.dev), before)
            self.assertFalse(self.installed.exists())

    def test_toggle_import_failure_does_not_stop_or_restart_runtime(self):
        (self.installed / '.encryption.key').unlink()
        self.write(self.installed / '.encryption.key.bak-fixture', PRIVATE + '-unproven')
        scripts = self.root / 'repo/scripts'
        scripts.mkdir(parents=True)
        shutil.copyfile(self.script, scripts / self.script.name)
        (scripts / self.script.name).chmod(0o700)
        program = '''set -euo pipefail
source "$TOGGLE_SCRIPT"
ROOT_DIR="$FIXTURE/repo"
DEV_DATA_DIR="$FIXTURE/development"
detect_runtime_mode() { echo managed-hot-dev; }
set_mock_mode() { echo mode-change >> "$FIXTURE/operations"; }
stop_hot_dev_runtime() { echo stop >> "$FIXTURE/operations"; }
start_hot_dev_runtime() { echo start >> "$FIXTURE/operations"; }
apply_mode false
'''
        source = self.snapshot(self.installed)
        result = subprocess.run([BASH, '-c', program], cwd=self.root,
                                env=self.env | {'TOGGLE_SCRIPT': str(ROOT / 'scripts/toggle-mock.sh')},
                                capture_output=True, text=True, timeout=10)
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertFalse((self.root / 'operations').exists())
        self.assertEqual(self.snapshot(self.installed), source)


if __name__ == '__main__':
    unittest.main(verbosity=2)
