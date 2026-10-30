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

    def old_snapshot(self, stamp=STAMP):
        path = self.config.parent / (self.config.name + ".backup." + stamp)
        path.mkdir()
        (path / "settings.json").write_text("prior recovery state\n")
        return path

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
        return sorted(self.config.parent.glob(self.config.name + ".backup.*"))

    def assert_no_stages(self):
        self.assertEqual(list(self.config.parent.glob("." + self.config.name + ".backup.*")), [])

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
