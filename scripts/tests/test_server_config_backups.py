#!/usr/bin/env python3
"""Run real configuration snapshot copy/publication against private fixtures.

Only the clock, disk-capacity observation and injected command failures are
controlled. No service, database, production config or network is touched.
These checks do not establish application-consistent backup or native recovery.
"""

import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh")).resolve()
STAMP = "20261023-154500"

PRELUDE = r'''
source "$INSTALLER"
print_info() { :; }
print_warn() { echo "$*" >&2; }
print_error() { echo "$*" >&2; }
ensure_config_backup_headroom() { return "${HEADROOM_EXIT:-0}"; }
date() { printf '%s\n' "$STAMP"; }
'''


class ConfigBackupPublication(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="pulse-config-backup-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.config = self.root / "source parent" / "config with spaces"
        self.config.mkdir(parents=True, mode=0o700)
        (self.config / "settings.json").write_text("current state\n")
        (self.config / ".env").write_text("private synthetic setting\n")
        (self.config / ".env").chmod(0o600)
        (self.config / "empty").mkdir()
        (self.config / "setting-link").symlink_to("settings.json")
        self.install = self.root / "install"
        self.install.mkdir()

    def old_snapshot(self, stamp=STAMP, owned=True):
        path = self.config.parent / (self.config.name + ".backup." + stamp)
        path.mkdir()
        (path / "settings.json").write_text("prior recovery state\n")
        if owned:
            self.write_receipt(path)
        return path

    def write_receipt(self, path, source=None, destination=None, identity=None):
        observed = path.stat()
        fields = (b"pulse-config-snapshot-v1", os.fsencode(source or self.config),
                  os.fsencode(destination or path),
                  (identity or f"{observed.st_dev}:{observed.st_ino}").encode())
        receipt = Path(str(path) + ".pulse-snapshot")
        receipt.write_bytes(b"\0".join(fields) + b"\0")
        receipt.chmod(0o600)
        return receipt

    def shell(self, body, **extra):
        env = {k: v for k, v in os.environ.items() if not k.startswith("PULSE_")}
        env.update(INSTALLER=str(INSTALLER), FIXTURE=str(self.root), STAMP=STAMP,
                   PULSE_INSTALL_DIR=str(self.install), PULSE_CONFIG_DIR=str(self.config))
        env.update(extra)
        result = subprocess.run(["bash", "-c", PRELUDE + body], cwd=self.root,
                                env=env, capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def completed(self):
        # The configured namespace is data, not a glob in the oracle either.
        return sorted(p for p in self.config.parent.iterdir()
                      if p.is_dir() and not p.is_symlink()
                      and p.name.startswith(self.config.name + ".backup.")
                      and not p.name.endswith(".pulse-snapshot"))

    def assert_no_stages(self):
        self.assertEqual([p for p in self.config.parent.iterdir()
                          if p.name.startswith("." + self.config.name + ".backup.")], [])

    def assert_prior(self, old):
        self.assertEqual((old / "settings.json").read_text(), "prior recovery state\n")
        self.assertEqual(sorted(p.name for p in old.iterdir()), ["settings.json"])

    def test_two_successful_same_second_copies_are_separate_complete_snapshots(self):
        old = self.old_snapshot()
        self.shell(r'''
backup_existing
printf '%s\n' "$LAST_CONFIG_BACKUP_DIR" > "$FIXTURE/first"
printf 'second state\n' > "$CONFIG_DIR/settings.json"
backup_existing
printf '%s\n' "$LAST_CONFIG_BACKUP_DIR" > "$FIXTURE/second"
''')
        first = Path((self.root / "first").read_text().strip())
        second = Path((self.root / "second").read_text().strip())
        self.assertNotEqual(first, second)
        self.assertEqual((first / "settings.json").read_text(), "current state\n")
        self.assertEqual((second / "settings.json").read_text(), "second state\n")
        self.assertEqual(len(self.completed()), 3)
        for snapshot in (first, second):
            self.assertEqual((snapshot / ".env").read_bytes(), (self.config / ".env").read_bytes())
            self.assertEqual(stat.S_IMODE((snapshot / ".env").stat().st_mode), 0o600)
            self.assertEqual(stat.S_IMODE(snapshot.stat().st_mode), 0o700)
            self.assertTrue((snapshot / "empty").is_dir())
            self.assertTrue((snapshot / "setting-link").is_symlink())
            self.assertEqual(os.readlink(snapshot / "setting-link"), "settings.json")
        self.assert_prior(old)
        self.assert_no_stages()

    def test_failed_partial_copy_does_not_mutate_or_delete_a_previous_snapshot(self):
        old = self.old_snapshot()
        result = self.shell(r'''
LAST_CONFIG_BACKUP_DIR="$CONFIG_DIR.backup.$STAMP"
cp() { command cp "$@"; return 23; }
if backup_existing; then exit 90; fi
test -z "$LAST_CONFIG_BACKUP_DIR"
''')
        self.assertIn("existing snapshots are unchanged", result.stderr)
        self.assert_prior(old)
        self.assertEqual(self.completed(), [old])
        self.assert_no_stages()

    def test_copy_and_published_snapshot_remain_private_with_broad_source_permissions(self):
        # Older or manually managed configs can be traversable by other users.
        # cp -a copies the source directory's owner/mode too, not just its files.
        # Inspect the completed copy before publication, not just a later chmod.
        self.config.chmod(0o755)
        self.config.parent.chmod(0o755)
        self.shell(r'''
cp() {
    command cp "$@" || return
    python3 - "${@: -1}" "$(dirname "$CONFIG_DIR")" <<'PY'
import os
from pathlib import Path
import stat
import sys

path, parent = (Path(arg).resolve() for arg in sys.argv[1:])
while path != parent:
    observed = path.stat()
    if observed.st_uid == os.geteuid() and stat.S_IMODE(observed.st_mode) & 0o077 == 0:
        break
    path = path.parent
else:
    sys.exit("configuration copy exposed source permissions before publication")
PY
}
backup_existing
printf '%s\n' "$LAST_CONFIG_BACKUP_DIR" > "$FIXTURE/latest"
''')
        snapshot = Path((self.root / "latest").read_text().strip())
        self.assertEqual(stat.S_IMODE(snapshot.stat().st_mode), 0o700)
        self.assertEqual(snapshot.stat().st_uid, self.config.stat().st_uid)
        self.assertEqual(snapshot.stat().st_gid, self.config.stat().st_gid)
        self.assertEqual((snapshot / ".env").read_bytes(), (self.config / ".env").read_bytes())
        self.assertEqual(stat.S_IMODE((snapshot / ".env").stat().st_mode), 0o600)
        self.assertEqual(stat.S_IMODE((snapshot / "settings.json").stat().st_mode),
                         stat.S_IMODE((self.config / "settings.json").stat().st_mode))
        self.assertTrue((snapshot / "empty").is_dir())
        self.assertEqual(os.readlink(snapshot / "setting-link"), "settings.json")
        self.assertEqual(stat.S_IMODE(self.config.stat().st_mode), 0o755)
        self.assert_no_stages()

    def test_snapshot_privacy_failure_preserves_prior_snapshots_without_rotation(self):
        old = self.old_snapshot("privacy-mode")
        result = self.shell(r'''
chmod() { return 23; }
prune_config_backups() { echo unexpected-prune > "$FIXTURE/unexpected"; }
if backup_existing; then exit 90; fi
test -z "$LAST_CONFIG_BACKUP_DIR"
''')
        self.assertIn("Could not protect configuration backup", result.stderr)
        self.assertFalse((self.root / "unexpected").exists())
        self.assert_prior(old)
        self.assert_no_stages()
        self.assertEqual(self.completed(), [old])

    def test_allocation_failure_stops_before_copy_or_rotation(self):
        old = self.old_snapshot()
        self.shell(r'''
mktemp() { return 23; }
cp() { echo unexpected-copy > "$FIXTURE/unexpected"; return 97; }
prune_config_backups() { echo unexpected-prune > "$FIXTURE/unexpected"; }
if backup_existing; then exit 90; fi
test -z "$LAST_CONFIG_BACKUP_DIR"
''')
        self.assertFalse((self.root / "unexpected").exists())
        self.assert_prior(old)
        self.assert_no_stages()

    def test_headroom_failure_creates_no_stage_and_preserves_prior_snapshot(self):
        old = self.old_snapshot()
        self.shell('if backup_existing; then exit 90; fi\ntest -z "$LAST_CONFIG_BACKUP_DIR"\n', HEADROOM_EXIT="23")
        self.assert_prior(old)
        self.assert_no_stages()

    def test_failed_publication_does_not_rotate_or_publish_a_snapshot(self):
        old = self.old_snapshot()
        self.shell(r'''
mv() { return 23; }
prune_config_backups() { echo unexpected-prune > "$FIXTURE/unexpected"; }
if backup_existing; then exit 90; fi
test -z "$LAST_CONFIG_BACKUP_DIR"
''')
        self.assertFalse((self.root / "unexpected").exists())
        self.assertEqual(self.completed(), [old])
        self.assert_prior(old)
        self.assert_no_stages()

    def test_exit_zero_skipped_publication_preserves_file_directory_and_link(self):
        for kind in ("file", "directory", "link"):
            with self.subTest(kind=kind):
                old = self.old_snapshot("collision-" + kind)
                result = self.shell(r'''
mv() {
    local destination="${@: -1}"
    printf '%s\n' "$destination" > "$FIXTURE/collision"
    case "$KIND" in
      file) printf 'do not replace\n' > "$destination" ;;
      directory) mkdir "$destination"; printf 'do not replace\n' > "$destination/marker" ;;
      link) ln -s "$FIXTURE/absent-target" "$destination" ;;
    esac
    command mv "$@"
}
if backup_existing; then exit 90; fi
test -z "$LAST_CONFIG_BACKUP_DIR"
''', KIND=kind)
                collision = Path((self.root / "collision").read_text().strip())
                self.assertIn("Could not publish configuration backup", result.stderr)
                if kind == "file":
                    self.assertEqual(collision.read_text(), "do not replace\n")
                    collision.unlink()
                elif kind == "directory":
                    self.assertEqual([p.name for p in collision.iterdir()], ["marker"])
                    self.assertEqual((collision / "marker").read_text(), "do not replace\n")
                    (collision / "marker").unlink()
                    collision.rmdir()
                else:
                    self.assertTrue(collision.is_symlink())
                    self.assertEqual(os.readlink(collision), str(self.root / "absent-target"))
                    collision.unlink()
                self.assert_prior(old)
                self.assert_no_stages()

    def test_current_snapshot_survives_rotation_even_after_clock_moves_backwards(self):
        for day in range(24, 29):
            self.old_snapshot(f"202610{day}-154500")
        self.shell('backup_existing\nprintf "%s\\n" "$LAST_CONFIG_BACKUP_DIR" > "$FIXTURE/latest"\n')
        latest = Path((self.root / "latest").read_text().strip())
        self.assertTrue(latest.is_dir())
        self.assertEqual((latest / "settings.json").read_text(), "current state\n")
        self.assertEqual(len(self.completed()), 5)
        self.assertFalse((self.config.parent / (self.config.name + ".backup.20261024-154500")).exists())
        self.assert_no_stages()

    def test_rotation_uses_the_literal_config_namespace(self):
        for case, (name, other_name) in enumerate((("config[1]", "config1"),
                                                  ("config?", "configA"),
                                                  ("config*", "config-extra"))):
            for keep in (0, 1, 5):
                with self.subTest(config=name, keep=keep):
                    self.config = self.root / f"case-{case}-{keep}" / name
                    self.config.mkdir(parents=True)
                    (self.config / "settings.json").write_text("current state\n")
                    prior = [self.old_snapshot(f"202610{day:02d}-154500") for day in range(1, 7)]
                    others = []
                    for day in range(1, 7):
                        path = self.config.parent / f"{other_name}.backup.202610{day:02d}-154500"
                        path.mkdir()
                        (path / "settings.json").write_text("other instance recovery state\n")
                        others.append(path)
                    self.shell('backup_existing\n', CONFIG_BACKUP_KEEP_COUNT=str(keep))
                    for path in others:
                        self.assertEqual((path / "settings.json").read_text(), "other instance recovery state\n")
                    self.assertEqual(len(self.completed()), keep)
                    # The fresh snapshot counts toward retention; remove only
                    # the oldest selected-instance copies, never a peer's.
                    self.assertEqual([p for p in prior if p.exists()], prior[7 - keep:])
                    self.assertEqual((self.config / "settings.json").read_text(), "current state\n")
                    self.assert_no_stages()

    def test_rotation_keeps_embedded_newlines_inside_each_path(self):
        self.config = self.config.parent / "config\nselected"
        self.config.mkdir()
        (self.config / "settings.json").write_text("current state\n")
        # Splitting find's output used this unrelated live directory as the
        # first deletion target, rather than the complete snapshot pathname.
        other = self.config.parent / "config"
        other.mkdir()
        (other / "settings.json").write_text("other live instance state\n")
        prior = [self.old_snapshot(f"202610{day:02d}-154500") for day in range(1, 7)]
        self.shell('backup_existing\n')
        self.assertEqual((other / "settings.json").read_text(), "other live instance state\n")
        self.assertEqual(len(self.completed()), 5)
        self.assertEqual([p for p in prior if p.exists()], prior[2:])
        self.assertEqual((self.config / "settings.json").read_text(), "current state\n")
        self.assert_no_stages()

    def test_snapshot_namespace_preserves_trailing_newlines_in_name_and_parent(self):
        for component in ("name", "parent", "both"):
            for newlines in ("\n", "\n\n"):
                for keep in (0, 1, 5):
                    with self.subTest(component=component, newlines=len(newlines), keep=keep):
                        namespace = self.root / f"case-{component}-{len(newlines)}-{keep}"
                        parent = namespace / ("parent" + (newlines if component != "name" else ""))
                        name = "config" + (newlines if component != "parent" else "")
                        self.config = parent / name
                        self.config.mkdir(parents=True)
                        (self.config / "settings.json").write_text("selected live state\n")
                        prior = [self.old_snapshot(f"202610{day:02d}-154500") for day in range(1, 7)]
                        # This is the distinct namespace the old command
                        # substitutions selected by stripping pathname bytes.
                        peer = namespace / "parent" / "config"
                        peer.mkdir(parents=True)
                        (peer / "settings.json").write_text("peer live state\n")
                        peers = []
                        for day in range(1, 7):
                            path = peer.parent / f"config.backup.202610{day:02d}-154500"
                            path.mkdir()
                            (path / "settings.json").write_text("peer recovery state\n")
                            peers.append(path)
                        self.shell(r'''
backup_existing
printf '%s' "$LAST_CONFIG_BACKUP_DIR" > "$FIXTURE/latest"
''', CONFIG_BACKUP_KEEP_COUNT=str(keep))
                        latest = Path((self.root / "latest").read_text())
                        self.assertEqual(latest.parent, parent)
                        self.assertTrue(latest.name.startswith(name + ".backup."))
                        for path in peers:
                            self.assertEqual((path / "settings.json").read_text(), "peer recovery state\n")
                        self.assertEqual((peer / "settings.json").read_text(), "peer live state\n")
                        self.assertEqual((self.config / "settings.json").read_text(), "selected live state\n")
                        self.assertEqual(len(self.completed()), keep)
                        self.assertEqual([p for p in prior if p.exists()], prior[7 - keep:])
                        if keep:
                            self.assertEqual((latest / "settings.json").read_text(), "selected live state\n")
                        else:
                            self.assertFalse(latest.exists())
                        self.assert_no_stages()

    def test_relative_option_shaped_name_and_trailing_slashes_keep_their_namespace(self):
        self.config = self.root / "-n"
        self.config.mkdir()
        (self.config / "settings.json").write_text("selected live state\n")
        self.shell(r'''
CONFIG_DIR='-n///'
backup_existing
test -f "$LAST_CONFIG_BACKUP_DIR/settings.json"
''')
        self.assertEqual(len(self.completed()), 1)
        self.assertEqual((self.completed()[0] / "settings.json").read_text(), "selected live state\n")
        self.assert_no_stages()

    def test_path_resolution_failure_stops_before_copy_or_rotation(self):
        old = self.old_snapshot()
        for utility in ("dirname", "basename"):
            with self.subTest(utility=utility):
                result = self.shell(r'''
case "$UTILITY" in
    dirname) dirname() { printf 'partial\n'; return 23; } ;;
    basename) basename() { printf 'partial\n'; return 23; } ;;
esac
mktemp() { echo unexpected-allocation > "$FIXTURE/unexpected"; return 90; }
if backup_existing; then exit 91; fi
test -z "$LAST_CONFIG_BACKUP_DIR"
''', UTILITY=utility)
                self.assertIn("Could not resolve configuration backup", result.stderr)
                self.assertFalse((self.root / "unexpected").exists())
                self.assert_prior(old)
                self.assertEqual(self.completed(), [old])
                self.assert_no_stages()

    def test_rotation_skips_files_links_and_an_empty_inventory(self):
        prefix = self.config.name + ".backup."
        outside = self.root / "other instance"
        outside.mkdir()
        (outside / "settings.json").write_text("other live instance state\n")
        link = self.config.parent / (prefix + "20261001-154500")
        link.symlink_to(outside, target_is_directory=True)
        broken = self.config.parent / (prefix + "20261002-154500")
        broken.symlink_to(self.root / "absent")
        regular = self.config.parent / (prefix + "20261003-154500")
        regular.write_text("not a snapshot directory\n")
        self.shell(r'''
prune_config_backups "$(dirname "$CONFIG_DIR")" "$(basename "$CONFIG_DIR").backup."
prune_config_backups "$FIXTURE/absent-parent" "absent.backup."
''', CONFIG_BACKUP_KEEP_COUNT="0")
        self.assertTrue(link.is_symlink())
        self.assertTrue(broken.is_symlink())
        self.assertEqual(regular.read_text(), "not a snapshot directory\n")
        self.assertEqual((outside / "settings.json").read_text(), "other live instance state\n")

    def test_incomplete_copy_is_not_another_invocations_rotation_candidate(self):
        self.shell(r'''
cp() {
    local stage="${@: -1}" peer="$CONFIG_DIR.backup.20261024-154500.peer"
    test -d "$stage" || return 91
    mkdir "$peer"
    CONFIG_BACKUP_KEEP_COUNT=1 prune_config_backups "$(dirname "$CONFIG_DIR")" "$(basename "$CONFIG_DIR").backup." "$peer"
    test -d "$stage" || return 92
    command cp "$@"
}
backup_existing
test -f "$LAST_CONFIG_BACKUP_DIR/settings.json"
''')
        self.assert_no_stages()

    def test_discard_removes_only_this_invocations_new_snapshot(self):
        old = self.old_snapshot()
        self.shell(r'''
backup_existing
current="$LAST_CONFIG_BACKUP_DIR"
discard_last_config_backup
test -z "$LAST_CONFIG_BACKUP_DIR"
test ! -e "$current"
''')
        self.assert_prior(old)
        self.assertEqual(self.completed(), [old])
        self.assert_no_stages()

    def test_missing_config_clears_stale_ownership_without_deleting_prior_snapshot(self):
        old = self.old_snapshot()
        self.shell(r'''
LAST_CONFIG_BACKUP_DIR="$CONFIG_DIR.backup.$STAMP"
CONFIG_DIR="$FIXTURE/absent-config"
backup_existing
test -z "$LAST_CONFIG_BACKUP_DIR"
discard_last_config_backup
''')
        self.assert_prior(old)
        self.assert_no_stages()

    def test_read_only_config_parent_uses_the_existing_hardened_unit_fallback(self):
        if os.geteuid() == 0:
            self.skipTest("root bypasses ordinary write permissions")
        self.config.parent.chmod(0o500)
        try:
            self.shell('backup_existing\nprintf "%s\\n" "$LAST_CONFIG_BACKUP_DIR" > "$FIXTURE/latest"\n')
            latest = Path((self.root / "latest").read_text().strip())
            self.assertEqual(latest.parent, self.install / "config-backups")
            self.assertEqual((latest / "settings.json").read_text(), "current state\n")
            self.assertEqual(list(latest.parent.glob(".*.backup.*")), [])
        finally:
            self.config.parent.chmod(0o700)


    def test_unmarked_prefix_directories_and_legacy_copies_are_never_deleted(self):
        for keep in (0, 1, 5):
            with self.subTest(keep=keep):
                self.config = self.root / f"unmarked-{keep}" / "config"
                self.config.mkdir(parents=True)
                (self.config / "settings.json").write_text("selected live state\n")
                peers = [self.old_snapshot("000-live", owned=False),
                         self.old_snapshot("19000101-000000.Abc123", owned=False)]
                legacy = [self.old_snapshot(f"202610{day:02d}-000000", owned=False)
                          for day in range(1, 7)]
                for peer in peers:
                    (peer / ".encryption.key").write_text("synthetic peer key\n")
                result = self.shell('backup_existing\n', CONFIG_BACKUP_KEEP_COUNT=str(keep))
                self.assertIn("unrecognised", result.stderr)
                for path in peers + legacy:
                    self.assertEqual((path / "settings.json").read_text(), "prior recovery state\n")
                for peer in peers:
                    self.assertEqual((peer / ".encryption.key").read_text(), "synthetic peer key\n")
                self.assertEqual(len(self.completed()), len(peers + legacy) + min(keep, 1))
                self.assertEqual((self.config / "settings.json").read_text(), "selected live state\n")
                self.assert_no_stages()

    def test_only_exact_bounded_regular_ownership_receipts_admit_rotation(self):
        for kind in ("source", "destination", "identity", "version", "truncated",
                     "trailing", "extra-null", "oversized", "link", "directory"):
            with self.subTest(kind=kind):
                old = self.old_snapshot("invalid-" + kind)
                receipt = Path(str(old) + ".pulse-snapshot")
                if kind == "source":
                    self.write_receipt(old, source=self.root / "other-config")
                elif kind == "destination":
                    self.write_receipt(old, destination=self.root / "other-copy")
                elif kind == "identity":
                    self.write_receipt(old, identity="0:0")
                elif kind == "version":
                    receipt.write_bytes(receipt.read_bytes().replace(b"-v1", b"-v2", 1))
                elif kind == "truncated":
                    receipt.write_bytes(receipt.read_bytes()[:-1])
                elif kind == "trailing":
                    receipt.write_bytes(receipt.read_bytes() + b"unexpected")
                elif kind == "extra-null":
                    receipt.write_bytes(receipt.read_bytes() + b"\0")
                elif kind == "oversized":
                    receipt.write_bytes(b"x" * 16385)
                else:
                    saved = self.root / ("receipt-" + kind)
                    receipt.rename(saved)
                    if kind == "link":
                        receipt.symlink_to(saved)
                    else:
                        receipt.mkdir()
                self.shell('backup_existing\n', CONFIG_BACKUP_KEEP_COUNT="0")
                self.assert_prior(old)
                self.assertTrue(receipt.exists())
                self.assert_no_stages()

    def test_receipt_is_private_and_does_not_change_the_copied_configuration(self):
        (self.config / ".pulse-snapshot").write_text("existing private source file\n")
        self.shell('backup_existing\nprintf "%s" "$LAST_CONFIG_BACKUP_DIR" > "$FIXTURE/latest"\n')
        latest = Path((self.root / "latest").read_text())
        receipt = Path(str(latest) + ".pulse-snapshot")
        observed = latest.stat()
        self.assertEqual(receipt.read_bytes(), b"\0".join((b"pulse-config-snapshot-v1",
                         os.fsencode(self.config), os.fsencode(latest),
                         f"{observed.st_dev}:{observed.st_ino}".encode())) + b"\0")
        self.assertEqual(stat.S_IMODE(receipt.stat().st_mode), 0o600)
        self.assertEqual((latest / ".pulse-snapshot").read_bytes(),
                         (self.config / ".pulse-snapshot").read_bytes())
        self.assertEqual((latest / ".env").read_bytes(), (self.config / ".env").read_bytes())
        self.assert_no_stages()

    def test_receipt_preparation_failure_stops_without_publication_or_rotation(self):
        for kind in ("stat", "write", "collision"):
            with self.subTest(kind=kind):
                old = self.old_snapshot("prepare-" + kind)
                result = self.shell(r'''
case "$KIND" in
  stat) stat() { return 23; } ;;
  write) printf() { [[ "$1" != pulse-config-snapshot-v1* ]] || return 23; builtin printf "$@"; } ;;
  collision) mktemp() {
    local stage
    stage=$(command mktemp "$@") || return
    local name="${stage##*/}"
    local final="${stage%/*}/${name#.}"
    printf 'foreign receipt\n' > "$final.pulse-snapshot"
    printf '%s' "$final.pulse-snapshot" > "$FIXTURE/collision-receipt"
    printf '%s\n' "$stage"
  } ;;
esac
if backup_existing; then exit 90; fi
test -z "$LAST_CONFIG_BACKUP_DIR"
''', KIND=kind)
                self.assertIn("Could not prepare configuration backup ownership receipt", result.stderr)
                self.assert_prior(old)
                self.assert_no_stages()
                if kind == "collision":
                    receipt = Path((self.root / "collision-receipt").read_text())
                    self.assertEqual(receipt.read_text(), "foreign receipt\n")
                    self.assertFalse(Path(str(receipt).removesuffix(".pulse-snapshot")).exists())

    def test_receipt_publication_failures_retain_the_copy_without_rotation(self):
        for kind in ("failure", "skipped", "file", "directory", "link"):
            with self.subTest(kind=kind):
                old = self.old_snapshot("publish-" + kind)
                result = self.shell(r'''
mv() {
    local destination="${@: -1}"
    if [[ "$destination" == *.pulse-snapshot ]]; then
        printf '%s' "${destination%.pulse-snapshot}" > "$FIXTURE/retained-copy"
        case "$KIND" in
          failure) return 23 ;;
          skipped) return 0 ;;
          file) printf 'foreign receipt\n' > "$destination" ;;
          directory) mkdir "$destination"; printf 'foreign receipt\n' > "$destination/retained" ;;
          link) ln -s "$FIXTURE/missing-receipt" "$destination" ;;
        esac
    fi
    command mv "$@"
}
if backup_existing; then exit 90; fi
test -z "$LAST_CONFIG_BACKUP_DIR"
''', KIND=kind)
                self.assertIn("no snapshots were rotated", result.stderr)
                copy = Path((self.root / "retained-copy").read_text())
                self.assertEqual((copy / "settings.json").read_text(), "current state\n")
                self.assertEqual((copy / ".env").read_bytes(), (self.config / ".env").read_bytes())
                self.assert_prior(old)
                self.assert_no_stages()
                receipt = Path(str(copy) + ".pulse-snapshot")
                if kind == "file":
                    self.assertEqual(receipt.read_text(), "foreign receipt\n")
                elif kind == "directory":
                    self.assertEqual((receipt / "retained").read_text(), "foreign receipt\n")
                elif kind == "link":
                    self.assertEqual(os.readlink(receipt), str(self.root / "missing-receipt"))
                else:
                    self.assertFalse(receipt.exists())

    def test_repeated_real_publications_rotate_receipts_and_preserve_legacy_copies(self):
        for keep in (0, 1, 5):
            with self.subTest(keep=keep):
                self.config = self.root / f"real-{keep}" / "config"
                self.config.mkdir(parents=True)
                (self.config / "settings.json").write_text("selected live state\n")
                legacy = self.old_snapshot("20260101-000000", owned=False)
                self.shell(r'''
for day in 01 02 03 04 05 06 07 08; do
    date() { printf '202610%s-000000\n' "$day"; }
    backup_existing
done
''', CONFIG_BACKUP_KEEP_COUNT=str(keep))
                self.assert_prior(legacy)
                copies = [p for p in self.completed() if p != legacy]
                self.assertEqual(len(copies), keep)
                receipts = list(self.config.parent.glob("*.pulse-snapshot"))
                self.assertEqual(len(receipts), keep)
                self.assertEqual({Path(str(p) + ".pulse-snapshot") for p in copies}, set(receipts))
                for copy in copies:
                    self.assertEqual((copy / "settings.json").read_text(), "selected live state\n")
                self.assert_no_stages()

    def test_replaced_snapshot_cannot_be_deleted_by_rotation_or_discard(self):
        for action in ("rotate", "discard"):
            with self.subTest(action=action):
                result = self.shell(r'''
backup_existing
current="$LAST_CONFIG_BACKUP_DIR"
command mv -- "$current" "$FIXTURE/owned-original-$ACTION"
mkdir -- "$current"
printf 'replacement live state\n' > "$current/settings.json"
if [[ "$ACTION" == rotate ]]; then
    CONFIG_BACKUP_KEEP_COUNT=0 prune_config_backups "${current%/*}" "${current##*/}"
else
    discard_last_config_backup
    test -z "$LAST_CONFIG_BACKUP_DIR"
fi
test "$(cat "$current/settings.json")" = 'replacement live state'
test "$(cat "$FIXTURE/owned-original-$ACTION/settings.json")" = 'current state'
''', ACTION=action)
                self.assertIn("unverified" if action == "discard" else "unrecognised", result.stderr)
                self.assert_no_stages()

    def test_every_update_entry_stops_on_failed_backup_even_in_an_or_list(self):
        for flow in ("pinned", "menu-update", "menu-reinstall"):
            with self.subTest(flow=flow):
                result = self.shell(r'''
CURRENT_VERSION=v6.5.0
FORCE_VERSION=""
FORCE_CHANNEL=stable
IN_CONTAINER=true
IN_DOCKER=false
[[ "$FLOW" != pinned ]] || FORCE_VERSION=v6.6.0
print_header() { :; }
check_root() { :; }
detect_os() { :; }
check_docker_environment() { :; }
check_existing_installation() { return 0; }
check_proxmox_host() { return 1; }
resolve_latest_release_tag_for_channel() { printf 'v6.6.0\n'; }
detect_service_name() { printf 'pulse\n'; }
safe_read() { local selection=1; [[ "$FLOW" != menu-reinstall ]] || selection=2; printf -v "$2" '%s' "$selection"; }
offer_existing_auto_updates() { :; }
run_upgrade_readiness_preflight() { :; }
backup_existing() { printf 'backup refused\n' >&2; return 23; }
create_user() { echo user > "$FIXTURE/unexpected"; }
download_pulse() { echo download > "$FIXTURE/unexpected"; exit 0; }
status=0
main || status=$?
test "$status" = 1
''', FLOW=flow)
                self.assertIn("backup refused", result.stderr)
                self.assertFalse((self.root / "unexpected").exists())


if __name__ == "__main__":
    unittest.main()
