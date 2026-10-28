#!/usr/bin/env python3
"""Actual removal entries, confined to ordinary-user files and inert services.

The existing Go removal suite owns real quiescence/timeout control. This suite
owns the subsequent filesystem/account/reload boundaries, including conditional
callers for which Bash disables errexit. No installed service or guest is used.
"""

import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh"))

HARNESS = r'''
source "$INSTALLER"
CONFIG_DIR="$FIXTURE/config"
INSTALL_DIR="$FIXTURE/install"
BINARY_LINK_PATH="$FIXTURE/link"
AUTO_UPDATE_DEST="$FIXTURE/updater"
UPDATE_HELPER_PATH="$FIXTURE/helper"
UPDATE_SERVICE_PATH="$FIXTURE/units/$SERVICE-update.service"
UPDATE_TIMER_PATH="$FIXTURE/units/$SERVICE-update.timer"
UPDATE_SERVICE_UNIT="$SERVICE-update.service"
UPDATE_TIMER_UNIT="$SERVICE-update.timer"
SERVICE_NAME="$SERVICE"
SERVICE_NAME_EXPLICIT="$CUSTOM"
case "$LAYOUT" in
    nested) CONFIG_DIR="$INSTALL_DIR/data" ;;
    parent) INSTALL_DIR="$CONFIG_DIR/runtime" ;;
    same) INSTALL_DIR="$CONFIG_DIR" ;;
    symlink) INSTALL_DIR="$FIXTURE/install-link" ;;
    helper-symlink) UPDATE_HELPER_PATH="$FIXTURE/config-link/.key" ;;
    root) CONFIG_DIR=/ ;;
esac
IN_CONTAINER=true
IN_DOCKER=false
CURRENT_VERSION=v6.5.0
FORCE_VERSION=""
for fn in print_header check_root detect_os check_docker_environment; do
    eval "$fn() { :; }"
done
check_existing_installation() { return 0; }
detect_service_name() { echo "$SERVICE"; }
resolve_latest_release_tag_for_channel() { :; }
read_configured_update_channel() { echo stable; }
curl() { :; }
quiesce_pulse_for_removal() { echo QUIESCE >> "$FIXTURE/actions"; }
cleanup_local_sensor_proxy() {
    echo LEGACY >> "$FIXTURE/actions"
    [[ "$FAULT" != legacy ]]
}
id() { [[ "$USER_PRESENT" == true && "$1" == pulse ]]; }
userdel() {
    echo USER_DELETE >> "$FIXTURE/actions"
    [[ "$FAULT" != user ]]
}
systemctl() {
    [[ "$*" == daemon-reload ]] || return 91
    echo RELOAD >> "$FIXTURE/actions"
    [[ "$FAULT" != reload ]]
}
timeout() {
    # This suite's only timeout target is an inert shell service double; the
    # existing Go suite independently exercises the real timeout executable.
    if [[ "$1" == -k ]]; then shift 3; else shift; fi
    "$@"
}
readlink() {
    [[ "$FAULT" != path-read ]] || return 22
    # Do not inspect the host's /etc. Mirror the fixed unit paths inside the
    # fixture; all other canonicalization uses the real coreutils executable.
    local path="${@: -1}"
    case "$path" in
        /etc/systemd/system/*) path="$FIXTURE/units/${path##*/}" ;;
    esac
    command readlink -m -- "$path"
}
rm() {
    local path
    for path in "$@"; do
        case "$path" in -f|-rf|--) continue ;; esac
        case "$path" in
            /etc/systemd/system/*) path="$FIXTURE/units/${path##*/}" ;;
            /var/log/pulse*.log|/opt/pulse.log)
                echo LOG_DELETE >> "$FIXTURE/actions"
                [[ "$FAULT" != log ]] || return 23
                continue ;;
            "$FIXTURE"/*) ;;
            *) echo "UNSAFE_DELETE $path" >> "$FIXTURE/actions"; return 91 ;;
        esac
        echo "DELETE ${path#"$FIXTURE/"}" >> "$FIXTURE/actions"
        if [[ "$FAULT" == delete && "$path" == "$FIXTURE/$TARGET" ]]; then
            echo "fixture deletion failure" >&2
            return 23
        fi
        if [[ "$FAULT" == partial-data && "$path" == "$CONFIG_DIR" ]]; then
            /bin/rm -f -- "$CONFIG_DIR/system.json"
            echo "fixture partial data deletion" >&2
            return 24
        fi
        /bin/rm -rf -- "$path" || return 25
    done
}
safe_read() { printf -v "$2" '%s' 2; }
safe_read_with_default() {
    case "$1" in
        'Remove all configuration'*) printf -v "$2" '%s' "$REMOVE_CONFIG" ;;
        'Remove pulse user'*) printf -v "$2" '%s' "$REMOVE_USER" ;;
        *) return 92 ;;
    esac
}
if [[ "$GUARDED" == true ]]; then
    if [[ "$FLOW" == menu ]]; then main || exit $?; else uninstall_pulse || exit $?; fi
else
    if [[ "$FLOW" == menu ]]; then main; else uninstall_pulse; fi
fi
'''


class ServerRemovalFailures(unittest.TestCase):
    def exercise(self, flow, *, fault="normal", target="", guarded=True,
                 remove_config="y", remove_user="y", user_present=True,
                 custom=False, layout="separate"):
        with tempfile.TemporaryDirectory(prefix="pulse-removal-failure-") as directory:
            f = Path(directory)
            service = "pulse-other" if custom else "pulse"
            for name in ["install", "config", "units", "other-install", "other-config"]:
                (f / name).mkdir(mode=0o700)
            for name in ["install/pulse", "config/system.json", "config/.key",
                         "other-install/pulse", "other-config/system.json",
                         "other-config/.key", "link", "updater", "helper"]:
                (f / name).write_bytes(("original-" + name).encode())
                (f / name).chmod(0o600)
            for unit in {service + ".service", service + "-update.service",
                         service + "-update.timer", "pulse.service", "pulse-backend.service"}:
                (f / "units" / unit).write_text("original-" + unit)
            (f / "install-link").symlink_to(f / "config", target_is_directory=True)
            (f / "config-link").symlink_to(f / "config", target_is_directory=True)
            config = f / "config"
            if layout == "nested":
                config.rename(f / "install/data")
                config = f / "install/data"
            elif layout == "parent":
                (f / "install").rename(config / "runtime")
            before = self.snapshot(config)
            other_before = (self.snapshot(f / "other-install"), self.snapshot(f / "other-config"))
            if fault == "permission":
                (f / "units").chmod(0o500)
            env = dict(os.environ, INSTALLER=str(INSTALLER), FIXTURE=str(f), FLOW=flow,
                       FAULT=fault, TARGET=target, GUARDED=str(guarded).lower(),
                       REMOVE_CONFIG=remove_config, REMOVE_USER=remove_user,
                       USER_PRESENT=str(user_present).lower(), SERVICE=service,
                       CUSTOM=str(custom).lower(), LAYOUT=layout)
            try:
                result = subprocess.run(["bash", "-c", HARNESS], env=env, text=True,
                                        capture_output=True, timeout=10)
            finally:
                (f / "units").chmod(0o700)
            actions = (f / "actions").read_text().splitlines()
            self.assertFalse(any("UNSAFE" in action for action in actions), actions)
            self.assertEqual(other_before, (self.snapshot(f / "other-install"),
                                            self.snapshot(f / "other-config")))
            return {"result": result, "actions": actions, "before": before,
                    "after": self.snapshot(config),
                    "runtime_present": (f / "install/pulse").exists()}

    @staticmethod
    def snapshot(directory):
        if not directory.exists():
            return None
        return {str(p.relative_to(directory)): (stat.S_IMODE(p.stat().st_mode),
                                                p.read_bytes() if p.is_file() else None)
                for p in [directory, *sorted(directory.rglob("*"))]}

    def assert_incomplete(self, evidence, *, preserves_config=True):
        result = evidence["result"]
        text = result.stdout + result.stderr
        self.assertNotEqual(result.returncode, 0, text + str(evidence["actions"]))
        self.assertNotIn("Pulse removed successfully", text)
        self.assertNotIn("completely uninstalled", text)
        self.assertNotIn("Configuration and data removed", text)
        if preserves_config:
            self.assertEqual(evidence["before"], evidence["after"], text)
            self.assertNotIn("DELETE config", evidence["actions"])
        self.assertFalse(any("start" in action for action in evidence["actions"]))

    def test_each_runtime_deletion_failure_preserves_data_and_stops(self):
        targets = ["units/pulse.service", "units/pulse-update.service",
                   "units/pulse-update.timer", "units/pulse-backend.service",
                   "link", "updater", "helper", "install"]
        for flow in ("menu", "uninstall"):
            for guarded in (False, True):
                for target in targets:
                    with self.subTest(flow=flow, guarded=guarded, target=target):
                        e = self.exercise(flow, fault="delete", target=target, guarded=guarded)
                        self.assert_incomplete(e)
                        self.assertEqual(e["actions"][-1], "DELETE " + target)
                        self.assertIn("Cannot remove", e["result"].stderr)
                        self.assertIn("Some files may already have been removed", e["result"].stderr)

    def test_reload_or_account_failure_preserves_data(self):
        for flow in ("menu", "uninstall"):
            for guarded in (False, True):
                for fault, last in (("reload", "RELOAD"), ("user", "USER_DELETE")):
                    with self.subTest(flow=flow, guarded=guarded, fault=fault):
                        e = self.exercise(flow, fault=fault, guarded=guarded)
                        self.assert_incomplete(e)
                        self.assertEqual(e["actions"][-1], last)
                        self.assertIn("Configuration is retained", e["result"].stderr)

    @unittest.skipIf(os.geteuid() == 0, "requires ordinary-user filesystem permissions")
    def test_real_filesystem_permission_failure_preserves_data(self):
        for flow in ("menu", "uninstall"):
            for guarded in (False, True):
                with self.subTest(flow=flow, guarded=guarded):
                    e = self.exercise(flow, fault="permission", guarded=guarded)
                    self.assert_incomplete(e)
                    self.assertEqual(e["actions"][-1], "DELETE units/pulse.service")
                    self.assertIn("Permission denied", e["result"].stderr)

    def test_full_legacy_cleanup_failure_precedes_config_and_account_deletion(self):
        for guarded in (False, True):
            with self.subTest(guarded=guarded):
                e = self.exercise("uninstall", fault="legacy", guarded=guarded)
                self.assert_incomplete(e)
                self.assertEqual(e["actions"][-1], "LEGACY")
                self.assertNotIn("USER_DELETE", e["actions"])

    def test_menu_log_failure_preserves_data(self):
        for guarded in (False, True):
            with self.subTest(guarded=guarded):
                e = self.exercise("menu", fault="log", guarded=guarded)
                self.assert_incomplete(e)
                self.assertEqual(e["actions"][-1], "LOG_DELETE")

    def test_partial_data_deletion_is_not_completion_or_intact_state(self):
        for flow in ("menu", "uninstall"):
            for guarded in (False, True):
                with self.subTest(flow=flow, guarded=guarded):
                    e = self.exercise(flow, fault="partial-data", guarded=guarded)
                    self.assert_incomplete(e, preserves_config=False)
                    self.assertNotIn("system.json", e["after"])
                    self.assertEqual(e["after"][".key"], e["before"][".key"])
                    self.assertEqual(e["actions"][-1], "DELETE config")
                    self.assertNotIn("Configuration is retained", e["result"].stderr)

    def test_data_is_last_on_success_for_both_entries_and_instances(self):
        for flow in ("menu", "uninstall"):
            for custom in (False, True):
                for guarded in (False, True):
                    for user_present in (False, True):
                        with self.subTest(flow=flow, custom=custom, guarded=guarded,
                                          user_present=user_present):
                            e = self.exercise(flow, custom=custom, guarded=guarded,
                                              user_present=user_present)
                            self.assertEqual(e["result"].returncode, 0, e["result"].stderr)
                            self.assertIsNone(e["after"])
                            self.assertFalse(e["runtime_present"])
                            self.assertEqual(e["actions"][-1], "DELETE config")
                            self.assertEqual("USER_DELETE" in e["actions"], user_present)
                            self.assertIn("RELOAD", e["actions"])
                            self.assertIn("Pulse removed successfully" if flow == "menu" else
                                          "completely uninstalled", e["result"].stdout)
                            if custom:
                                self.assertNotIn("DELETE units/pulse.service", e["actions"])
                                self.assertNotIn("DELETE units/pulse-backend.service", e["actions"])

    def test_menu_preserves_declined_data_and_account(self):
        for user_present in (False, True):
            for guarded in (False, True):
                with self.subTest(user_present=user_present, guarded=guarded):
                    e = self.exercise("menu", remove_config="n", remove_user="n",
                                      user_present=user_present, guarded=guarded)
                    self.assertEqual(e["result"].returncode, 0, e["result"].stderr)
                    self.assertEqual(e["before"], e["after"])
                    self.assertNotIn("DELETE config", e["actions"])
                    self.assertNotIn("USER_DELETE", e["actions"])
                    self.assertIn("Configuration preserved", e["result"].stdout)

    def test_unsafe_or_unreadable_layout_refuses_all_file_deletion(self):
        for flow in ("menu", "uninstall"):
            for layout in ("nested", "parent", "same", "symlink", "helper-symlink", "root", "separate"):
                with self.subTest(flow=flow, layout=layout):
                    e = self.exercise(flow, layout=layout,
                                      fault="path-read" if layout == "separate" else "normal")
                    self.assert_incomplete(e)
                    self.assertEqual(e["actions"], ["QUIESCE"])


if __name__ == "__main__":
    unittest.main()
