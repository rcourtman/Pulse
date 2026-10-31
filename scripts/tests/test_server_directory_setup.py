#!/usr/bin/env python3
"""Execute directory admission with private text files and inert ownership calls.

No system user, manager, real configuration or binary is changed. These controls
prove failure propagation and retained bytes, not native install/rollback.
"""
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh")).resolve()

PRELUDE = r'''
set -euo pipefail
source "$INSTALLER"
print_info() { :; }
print_error() { printf '%s\n' "$*" >&2; }
step() { printf '%s\n' "$1" >> "$FIXTURE/calls"; [[ "$1" != "$FAULT" ]]; }
mkdir() { step "mkdir:${@: -1}" || return 23; command mkdir "$@"; }
chown() { step "chown:${@: -1}" || return 23; }
chmod() { step "chmod:${@: -1}" || return 23; command chmod "$@"; }
chown_install_dir_for_pulse() { step install-owner || return 23; }
cat() { step env-write || return 23; command cat "$@"; }
'''


class DirectorySetup(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="pulse-directory-setup-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.config = self.root / "config with spaces"
        self.install = self.root / "install with spaces"
        self.config.mkdir()
        self.install.mkdir()
        for name in ("alerts.json", "system.json", "extra.json", "nodes.enc", ".env", ".encryption.key"):
            path = self.config / name
            path.write_text("synthetic preserved " + name + "\n")
            path.chmod(0o644)
        for name in ("alerts", "notifications", "audit"):
            path = self.config / name
            path.mkdir()
            (path / "retained").write_text("preserved tree\n")
        (self.install / "VERSION").write_text("v6.5.0\n")
        self.before = {p.relative_to(self.root): p.read_bytes()
                       for p in self.root.rglob("*") if p.is_file()}

    def shell(self, body, fault="", **extra):
        env = {k: v for k, v in os.environ.items() if not k.startswith("PULSE_")}
        env.update(INSTALLER=str(INSTALLER), FIXTURE=str(self.root), FAULT=fault,
                   PULSE_INSTALL_DIR=str(self.install), PULSE_CONFIG_DIR=str(self.config))
        env.update(extra)
        return subprocess.run(["bash", "-c", PRELUDE + body], cwd=self.root,
                              env=env, capture_output=True, text=True, timeout=10)

    def assert_preserved(self):
        for name, content in self.before.items():
            self.assertEqual((self.root / name).read_bytes(), content, str(name))

    def test_every_permission_and_directory_failure_stops_in_conditional_call(self):
        targets = ["mkdir:" + str(self.config), "mkdir:" + str(self.install),
                   "chown:" + str(self.config), "install-owner", "chmod:" + str(self.config)]
        targets += ["chown:" + str(self.config / name) for name in
                    ("alerts.json", "system.json", "extra.json", "nodes.enc",
                     "alerts", "notifications", "audit", ".env", ".encryption.key")]
        targets += ["chmod:" + str(self.config / name) for name in (".env", ".encryption.key")]
        for fault in targets:
            with self.subTest(fault=fault):
                calls = self.root / "calls"
                calls.unlink(missing_ok=True)
                result = self.shell('status=0\nsetup_directories || status=$?\n'
                                    'printf "SETUP_EXIT=%s\\n" "$status"\n', fault=fault)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn("SETUP_EXIT=1\n", result.stdout)
                self.assertIn("Could not", result.stderr)
                events = calls.read_text().splitlines()
                self.assertEqual(events[-1], fault, "continued after failed filesystem operation")
                self.assert_preserved()

    def test_new_environment_write_failure_stops_before_ownership_or_completion(self):
        environment = self.config / ".env"
        environment.unlink()
        del self.before[Path(environment.relative_to(self.root))]
        result = self.shell('status=0\nsetup_directories || status=$?\n'
                            'printf "SETUP_EXIT=%s\\n" "$status"\n', fault="env-write")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("SETUP_EXIT=1\n", result.stdout)
        self.assertIn("environment", result.stderr)
        self.assertEqual((self.root / "calls").read_text().splitlines()[-1], "env-write")
        self.assert_preserved()

    def test_success_preserves_existing_bytes_and_tightens_only_required_modes(self):
        result = self.shell('setup_directories\n')
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assert_preserved()
        self.assertEqual(stat.S_IMODE(self.config.stat().st_mode), 0o700)
        for name in (".env", ".encryption.key"):
            self.assertEqual(stat.S_IMODE((self.config / name).stat().st_mode), 0o600)
        for name in ("alerts.json", "system.json", "extra.json", "nodes.enc"):
            self.assertEqual(stat.S_IMODE((self.config / name).stat().st_mode), 0o644)

    def test_fresh_environment_has_private_mode_and_explicit_mock_default(self):
        environment = self.config / ".env"
        environment.unlink()
        del self.before[Path(environment.relative_to(self.root))]
        result = self.shell('umask 022\nsetup_directories\n')
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("PULSE_MOCK_MODE=false\n", environment.read_text())
        self.assertEqual(stat.S_IMODE(environment.stat().st_mode), 0o600)
        self.assert_preserved()

    def test_all_five_entry_paths_stop_on_directory_failure_even_in_or_list(self):
        for flow in ("source", "pinned", "menu-update", "menu-reinstall", "fresh"):
            with self.subTest(flow=flow):
                result = self.shell(r'''
CURRENT_VERSION=v6.5.0
FORCE_VERSION=""
FORCE_CHANNEL=stable
BUILD_FROM_SOURCE=false
IN_CONTAINER=true
IN_DOCKER=false
[[ "$FLOW" != pinned ]] || FORCE_VERSION=v6.6.0
[[ "$FLOW" != source ]] || BUILD_FROM_SOURCE=true
print_header() { :; }
check_root() { :; }
detect_os() { :; }
check_docker_environment() { :; }
check_existing_installation() { [[ "$FLOW" != fresh ]]; }
check_proxmox_host() { return 1; }
resolve_latest_release_tag_for_channel() { printf 'v6.6.0\n'; }
detect_service_name() { printf 'pulse\n'; }
safe_read() { local selection=1; [[ "$FLOW" != menu-reinstall ]] || selection=2; printf -v "$2" '%s' "$selection"; }
offer_existing_auto_updates() { :; }
run_upgrade_readiness_preflight() { :; }
backup_existing() { :; }
install_dependencies() { :; }
create_user() { :; }
download_pulse() { :; }
# No real build, systemd, updater, process or HOME marker is admitted.
after_failure() { printf '%s\n' unexpected >> "$FIXTURE/unexpected"; }
build_from_source() { after_failure; }
setup_update_command() { after_failure; }
install_systemd_service() { after_failure; }
ensure_systemd_service_installed() { after_failure; }
setup_auto_updates() { after_failure; }
update_timer_exists() { after_failure; }
refresh_auto_updates() { after_failure; }
start_pulse() { after_failure; }
create_marker_file() { after_failure; }
print_completion() { after_failure; }
status=0
main || status=$?
printf 'MAIN_EXIT=%s\n' "$status"
''', FLOW=flow, fault="chown:" + str(self.config))
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn("MAIN_EXIT=1\n", result.stdout)
                self.assertIn("Could not set Pulse directory ownership or permissions", result.stderr)
                self.assertFalse((self.root / "unexpected").exists(), result.stdout + result.stderr)
                self.assert_preserved()


if __name__ == "__main__":
    unittest.main(verbosity=2)
