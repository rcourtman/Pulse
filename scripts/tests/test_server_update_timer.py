#!/usr/bin/env python3
"""Actual installer timer discovery and refresh, confined to temporary files.

The server/install producers and systemd manager operations are doubles. The
optional real systemctl inventory uses --root, never the host's unit manager.
"""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh"))
REAL_SYSTEMCTL = shutil.which("systemctl")

SYSTEMCTL = r'''#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$FIXTURE/calls"
case "$1" in
    list-unit-files)
        if [[ "$INVENTORY" == real ]]; then
            exec "$REAL_SYSTEMCTL" --root="$FIXTURE/root" "$@"
        fi
        if [[ "$INVENTORY" == hang ]]; then exec sleep 30; fi
        printf '%s' "$ROWS"
        exit "$QUERY_EXIT"
        ;;
    daemon-reload) exit 0 ;;
    *) echo "Unexpected manager operation: $*" >&2; exit 97 ;;
esac
'''

HARNESS = r'''
source "$INSTALLER"
IN_CONTAINER=true
IN_DOCKER=false
CURRENT_VERSION=v6.5.0
print_header() { :; }
print_info() { echo "$*"; }
print_warn() { echo "$*"; }
print_success() { echo "$*"; }
print_error() { echo "$*" >&2; }
check_root() { :; }
detect_os() { :; }
check_docker_environment() { :; }
check_existing_installation() { [[ "$FLOW" != fresh ]]; }
detect_service_name() { echo "$SERVICE_NAME"; }
run_upgrade_readiness_preflight() { :; }
backup_existing() { :; }
create_user() { :; }
install_dependencies() { :; }
setup_directories() { :; }
download_pulse() { :; }
build_from_source() { :; }
setup_update_command() { :; }
ensure_systemd_service_installed() { :; }
install_systemd_service() { :; }
start_pulse() { :; }
create_marker_file() { :; }
print_completion() { :; }
resolve_latest_release_tag_for_channel() {
    if [[ "$1" == stable ]]; then echo v6.6.0; else echo v6.7.0-rc.1; fi
}
safe_read() {
    printf '%s\n' "$1" >> "$FIXTURE/prompts"
    if [[ "$2" == enable_updates ]]; then
        printf -v "$2" '%s' n
    elif [[ "$FLOW" == reinstall ]]; then
        printf -v "$2" '%s' 3
    else
        printf -v "$2" '%s' 1
    fi
}
case "$FLOW" in
    query)
        if update_timer_exists; then echo PRESENT; else echo ABSENT; fi
        exit 0 ;;
    unavailable)
        command() {
            [[ "$*" != '-v systemctl' ]] || return 1
            builtin command "$@"
        }
        if update_timer_exists; then echo PRESENT; else echo ABSENT; fi
        exit 0 ;;
    version) FORCE_VERSION=v6.6.0 ;;
    source) BUILD_FROM_SOURCE=true ;;
    update|reinstall|fresh) : ;;
    *) exit 98 ;;
esac
main
'''


class ServerUpdateTimerTest(unittest.TestCase):
    def exercise(self, *, flow="query", state="enabled", service="pulse",
                 rows=None, query_exit=0, inventory="double", mask="",
                 config_enabled=True):
        with tempfile.TemporaryDirectory() as temporary:
            fixture = Path(temporary)
            config = fixture / "config"
            install = fixture / "install"
            units = fixture / "root/etc/systemd/system"
            tools = fixture / "tools"
            for directory in (config, install / "scripts", units, tools):
                directory.mkdir(parents=True)
            helper = tools / "auto-update"
            helper.write_text("#!/bin/bash\necho stale-helper\n")
            (install / "scripts/pulse-auto-update.sh").write_text(
                "#!/bin/bash\necho current-helper\n")
            settings = json.dumps({"autoUpdateEnabled": config_enabled, "updateChannel": "stable"})
            (config / "system.json").write_text(settings)
            timer_name = f"{service}-update.timer"
            timer = units / timer_name
            updater = units / f"{service}-update.service"
            timer.write_text("[Timer]\nOnCalendar=daily\n[Install]\nWantedBy=timers.target\n")
            updater.write_text("[Service]\nExecStart=/bin/true\n")
            if state == "enabled":
                wants = units / "timers.target.wants"
                wants.mkdir()
                (wants / timer_name).symlink_to(f"../{timer_name}")
            masked = timer if mask == "timer" else updater if mask == "service" else None
            if masked:
                masked.unlink()
                masked.symlink_to("/dev/null")
            systemctl = tools / "systemctl"
            systemctl.write_text(SYSTEMCTL)
            systemctl.chmod(0o755)
            if rows is None:
                rows = f"{timer_name} {state} enabled\n"
            env = {**os.environ, "INSTALLER": str(INSTALLER), "FIXTURE": str(fixture),
                   "FLOW": flow, "ROWS": rows, "QUERY_EXIT": str(query_exit),
                   "INVENTORY": inventory, "REAL_SYSTEMCTL": REAL_SYSTEMCTL or "",
                   "PATH": f"{tools}:{os.environ['PATH']}", "PULSE_SERVICE_NAME": service,
                   "PULSE_INSTALL_DIR": str(install), "PULSE_CONFIG_DIR": str(config),
                   "PULSE_UPDATE_SERVICE_PATH": str(updater), "PULSE_UPDATE_TIMER_PATH": str(timer),
                   "PULSE_AUTO_UPDATE_DEST": str(helper)}
            result = subprocess.run(["bash", "-c", HARNESS], env=env,
                                    capture_output=True, text=True, timeout=12)
            def lines(name):
                path = fixture / name
                return path.read_text().splitlines() if path.exists() else []
            return {"result": result, "helper": helper.read_text(),
                    "settings": (config / "system.json").read_text(), "original_settings": settings,
                    "calls": lines("calls"), "prompts": lines("prompts"),
                    "timer_masked": timer.is_symlink() and os.readlink(timer) == "/dev/null",
                    "service_masked": updater.is_symlink() and os.readlink(updater) == "/dev/null",
                    "timer_enabled": (units / "timers.target.wants" / timer_name).is_symlink()}

    def assert_discovery(self, evidence, present):
        r = evidence["result"]
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertEqual(r.stdout.strip(), "PRESENT" if present else "ABSENT")

    def assert_refreshed(self, evidence, enabled):
        r = evidence["result"]
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("current-helper", evidence["helper"])
        self.assertNotIn("stale-helper", evidence["helper"])
        self.assertEqual(evidence["settings"], evidence["original_settings"])
        self.assertEqual(evidence["timer_enabled"], enabled)
        mutations = [call for call in evidence["calls"] if not call.startswith("list-unit-files")]
        self.assertEqual(mutations, ["daemon-reload"])

    def test_discovers_full_systemctl_rows_regardless_of_enablement(self):
        for state in ("enabled", "enabled-runtime", "disabled", "static", "indirect",
                      "linked", "linked-runtime", "masked", "masked-runtime"):
            with self.subTest(state=state):
                self.assert_discovery(self.exercise(state=state), True)

    def test_exact_unit_field_rejects_prefixes_and_regex_impostors(self):
        for rows in ("", "pulse-update.timer-other enabled enabled\n",
                     "pulse-updateXtimer enabled enabled\n", "other-update.timer enabled enabled\n"):
            with self.subTest(rows=rows):
                self.assert_discovery(self.exercise(rows=rows), False)
        self.assert_discovery(self.exercise(rows="other.timer enabled enabled\n"
                                             "\tpulse-update.timer\tdisabled\tenabled\n"), True)

    def test_query_is_bounded_and_scoped_to_full_exact_instance_name(self):
        service = "pulse-" + "department-" * 12
        e = self.exercise(service=service)
        self.assert_discovery(e, True)
        self.assertEqual(e["calls"], [f"list-unit-files --no-legend --no-pager --full -- {service}-update.timer"])

    def test_failed_query_never_accepts_its_stdout(self):
        self.assert_discovery(self.exercise(query_exit=1), False)

    def test_missing_systemctl_never_runs_a_query(self):
        e = self.exercise(flow="unavailable")
        self.assert_discovery(e, False)
        self.assertEqual(e["calls"], [])

    def test_hung_inventory_times_out_without_manager_mutation(self):
        e = self.exercise(inventory="hang")
        self.assert_discovery(e, False)
        self.assertEqual(len(e["calls"]), 1)

    def test_all_install_flows_reach_real_refresh_without_changing_consent(self):
        for flow in ("version", "source", "update", "reinstall", "fresh"):
            for state, enabled in (("enabled", True), ("disabled", False)):
                with self.subTest(flow=flow, state=state):
                    e = self.exercise(flow=flow, state=state, config_enabled=enabled)
                    self.assert_refreshed(e, enabled)
                    self.assertNotIn("Automatic updates are not configured.", e["result"].stdout)
                    if enabled:
                        self.assertFalse(any("Enable auto-updates?" in p for p in e["prompts"]))

    def test_missing_or_failed_inventory_does_not_refresh_or_enable(self):
        for rows, code in (("", 0), ("pulse-update.timer enabled enabled\n", 1)):
            e = self.exercise(flow="version", rows=rows, query_exit=code)
            self.assertEqual(e["result"].returncode, 0)
            self.assertIn("stale-helper", e["helper"])
            self.assertEqual(e["settings"], e["original_settings"])
            self.assertFalse(any(call == "daemon-reload" for call in e["calls"]))

    def test_timer_and_updater_masks_are_not_replaced_during_refresh(self):
        for mask in ("timer", "service"):
            with self.subTest(mask=mask):
                e = self.exercise(flow="version", mask=mask)
                self.assertEqual(e["result"].returncode, 0)
                self.assertTrue(e[f"{mask}_masked"])
                self.assertIn("stale-helper", e["helper"])
                self.assertIn("masked", e["result"].stdout)
                self.assertEqual(e["settings"], e["original_settings"])
                self.assertFalse(any(call == "daemon-reload" for call in e["calls"]))

    @unittest.skipUnless(REAL_SYSTEMCTL, "systemctl filesystem inventory is unavailable")
    def test_real_systemctl_inventory_and_complete_flow_use_only_fixture_root(self):
        for state, enabled in (("enabled", True), ("disabled", False)):
            with self.subTest(state=state):
                self.assert_discovery(self.exercise(inventory="real", state=state), True)
                self.assert_refreshed(self.exercise(flow="version", inventory="real", state=state), enabled)


if __name__ == "__main__":
    unittest.main()
