#!/usr/bin/env python3
"""Actual installer timer discovery and refresh, confined to temporary files.

The server/install producers and systemd manager operations are doubles. The
optional real systemctl inventory uses --root, never the host's unit manager.
"""

import hashlib
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
        # The one-unit read discovers the timer; the two-unit read admits
        # replacement only after checking the effective masks of both units.
        if [[ "$#" == 7 ]]; then
            if [[ "$GUARD_INVENTORY" == real ]]; then
                exec "$REAL_SYSTEMCTL" --root="$FIXTURE/root" "$@"
            fi
            if [[ "$GUARD_INVENTORY" == hang ]]; then exec sleep 30; fi
            printf '%s' "$GUARD_ROWS"
            exit "$GUARD_EXIT"
        fi
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
    refresh-unavailable)
        command() {
            [[ "$*" != '-v systemctl' ]] || return 1
            builtin command "$@"
        }
        refresh_auto_updates
        exit 0 ;;
    refresh) refresh_auto_updates; exit 0 ;;
    setup) ENABLE_AUTO_UPDATES=true; setup_auto_updates; exit 0 ;;
    assets)
        result=0
        install_auto_update_assets || result=$?
        exit "$result" ;;
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
                 config_enabled=True, mask_location="configured", guard_rows=None,
                 guard_exit=0, guard_inventory=None):
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
            service_name = f"{service}-update.service"
            timer = units / timer_name
            updater = units / service_name
            installed_units = units
            if mask and mask_location != "configured":
                installed_units = fixture / "root/usr/lib/systemd/system"
                installed_units.mkdir(parents=True)
                if mask_location == "persistent-other":
                    # Caller-selected destinations are not the directories in
                    # systemd's effective search path. Neither is the mask.
                    destination = fixture / "root/opt/selected-units"
                    destination.mkdir(parents=True)
                    timer = destination / timer_name
                    updater = destination / service_name
            (installed_units / timer_name).write_text(
                "[Timer]\nOnCalendar=daily\n[Install]\nWantedBy=timers.target\n")
            (installed_units / service_name).write_text("[Service]\nExecStart=/bin/true\n")
            if state == "enabled":
                wants = units / "timers.target.wants"
                wants.mkdir()
                (wants / timer_name).symlink_to(
                    "/" + str((installed_units / timer_name).relative_to(fixture / "root")))
            masked = None
            if mask:
                mask_name = timer_name if mask == "timer" else service_name
                if mask_location == "configured":
                    masked = timer if mask == "timer" else updater
                else:
                    mask_dir = (fixture / "root/run/systemd/system"
                                if mask_location == "runtime" else units)
                    mask_dir.mkdir(parents=True, exist_ok=True)
                    masked = mask_dir / mask_name
            if masked:
                if masked.exists() or masked.is_symlink():
                    masked.unlink()
                masked.symlink_to("/dev/null")
            systemctl = tools / "systemctl"
            systemctl.write_text(SYSTEMCTL)
            systemctl.chmod(0o755)
            if rows is None:
                rows = f"{timer_name} {state} enabled\n"
            if guard_rows is None:
                timer_state = "masked" if mask == "timer" else state
                service_state = "masked" if mask == "service" else "disabled"
                guard_rows = f"{timer_name} {timer_state} enabled\n{service_name} {service_state} enabled\n"
            env = {**os.environ, "INSTALLER": str(INSTALLER), "FIXTURE": str(fixture),
                   "FLOW": flow, "ROWS": rows, "QUERY_EXIT": str(query_exit),
                   "GUARD_ROWS": guard_rows, "GUARD_EXIT": str(guard_exit),
                   "GUARD_INVENTORY": guard_inventory or inventory,
                   "INVENTORY": inventory, "REAL_SYSTEMCTL": REAL_SYSTEMCTL or "",
                   "PATH": f"{tools}:{os.environ['PATH']}", "PULSE_SERVICE_NAME": service,
                   "PULSE_INSTALL_DIR": str(install), "PULSE_CONFIG_DIR": str(config),
                   "PULSE_UPDATE_SERVICE_PATH": str(updater), "PULSE_UPDATE_TIMER_PATH": str(timer),
                   "PULSE_AUTO_UPDATE_DEST": str(helper)}

            def assets():
                result = {}
                for path in sorted(fixture.rglob("*")):
                    if path.name in ("calls", "prompts"):
                        continue
                    mode = path.lstat().st_mode
                    if path.is_symlink():
                        value = {"mode": mode, "link": os.readlink(path)}
                    elif path.is_file():
                        value = {"mode": mode, "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}
                    else:
                        value = {"mode": mode}
                    result[str(path.relative_to(fixture))] = value
                return result

            def effective_states():
                if inventory != "real":
                    return None
                observed = subprocess.run(
                    [REAL_SYSTEMCTL, f"--root={fixture / 'root'}", "list-unit-files",
                     "--no-legend", "--no-pager", "--full", "--", timer_name, service_name],
                    capture_output=True, text=True, timeout=5)
                self.assertEqual(observed.returncode, 0, observed.stdout + observed.stderr)
                return dict(row.split()[:2] for row in observed.stdout.splitlines())

            before_assets = assets()
            before_states = effective_states()
            result = subprocess.run(["bash", "-c", HARNESS], env=env,
                                    capture_output=True, text=True, timeout=12)
            def lines(name):
                path = fixture / name
                return path.read_text().splitlines() if path.exists() else []
            evidence = {"result": result, "helper": helper.read_text(),
                    "settings": (config / "system.json").read_text(), "original_settings": settings,
                    "calls": lines("calls"), "prompts": lines("prompts"),
                    "before_assets": before_assets, "after_assets": assets(),
                    "before_states": before_states, "after_states": effective_states(),
                    "timer_masked": timer.is_symlink() and os.readlink(timer) == "/dev/null",
                    "service_masked": updater.is_symlink() and os.readlink(updater) == "/dev/null",
                    "timer_enabled": (units / "timers.target.wants" / timer_name).is_symlink()}
            if os.environ.get("PULSE_TIMER_TRACE") == "1":
                def digest(value):
                    return hashlib.sha256(json.dumps(value, sort_keys=True).encode()).hexdigest()
                print(json.dumps({"flow": flow, "state": state, "service": service,
                                  "mask": mask, "mask_location": mask_location, "inventory": inventory,
                                  "guard_inventory": guard_inventory or inventory, "guard_exit": guard_exit,
                                  "exit": result.returncode, "calls": evidence["calls"],
                                  "before_states": before_states, "after_states": evidence["after_states"],
                                  "before_assets_sha256": digest(before_assets),
                                  "after_assets_sha256": digest(evidence["after_assets"])}, indent=2))
            return evidence

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

    def assert_preserved(self, evidence, *, status=0):
        r = evidence["result"]
        self.assertEqual(r.returncode, status, r.stdout + r.stderr)
        self.assertIn("stale-helper", evidence["helper"])
        self.assertEqual(evidence["before_assets"], evidence["after_assets"])
        self.assertEqual(evidence["before_states"], evidence["after_states"])
        self.assertFalse(any(not call.startswith("list-unit-files") for call in evidence["calls"]))

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
                self.assert_preserved(e)
                self.assertTrue(e[f"{mask}_masked"])
                self.assertIn("stale-helper", e["helper"])
                self.assertIn("masked", e["result"].stdout)
                self.assertEqual(e["settings"], e["original_settings"])
                self.assertFalse(any(call == "daemon-reload" for call in e["calls"]))

    @unittest.skipUnless(REAL_SYSTEMCTL, "systemctl filesystem inventory is unavailable")
    def test_all_consumers_preserve_effective_masks_outside_destinations(self):
        for flow in ("version", "source", "update", "reinstall", "fresh"):
            for location, expected in (("runtime", "masked-runtime"), ("persistent-other", "masked")):
                for mask in ("timer", "service"):
                    with self.subTest(flow=flow, location=location, mask=mask):
                        e = self.exercise(flow=flow, state="disabled", mask=mask,
                                          mask_location=location, inventory="real", config_enabled=False)
                        self.assertEqual(e["before_states"][f"pulse-update.{mask}"], expected)
                        self.assert_preserved(e)
                        self.assertIn(expected, e["result"].stdout)

    @unittest.skipUnless(REAL_SYSTEMCTL, "systemctl filesystem inventory is unavailable")
    def test_asset_repair_and_explicit_setup_cannot_replace_effectively_masked_units(self):
        for flow in ("assets", "setup"):
            for mask in ("timer", "service"):
                with self.subTest(flow=flow, mask=mask):
                    e = self.exercise(flow=flow, state="disabled", mask=mask,
                                      mask_location="runtime", inventory="real", config_enabled=False)
                    self.assert_preserved(e, status=1 if flow == "assets" else 0)
                    self.assertIn("masked-runtime", e["result"].stdout)

    def test_failed_unknown_and_duplicate_effective_inventory_cannot_admit_writes(self):
        for rows, code in (("pulse-update.timer disabled enabled\n", 1),
                           ("pulse-update.timer not-understood enabled\n", 0),
                           ("pulse-update.timer\n", 0),
                           ("pulse-updateXtimer disabled enabled\n", 0),
                           ("pulse-update.timer disabled enabled\npulse-update.timer disabled enabled\n", 0),
                           ("", 1)):
            with self.subTest(rows=rows, code=code):
                self.assert_preserved(self.exercise(flow="version", guard_rows=rows, guard_exit=code))

    def test_effective_inventory_is_bounded_and_missing_systemctl_preserves_assets(self):
        self.assert_preserved(self.exercise(flow="version", guard_inventory="hang"))
        e = self.exercise(flow="refresh-unavailable")
        self.assert_preserved(e)
        self.assertEqual(e["calls"], [])

    def test_successful_absent_or_unmasked_inventory_keeps_asset_repair_working(self):
        for rows in ("", "pulse-update.timer disabled enabled\npulse-update.service disabled enabled\n"):
            with self.subTest(rows=rows):
                self.assert_refreshed(self.exercise(flow="assets", state="disabled", guard_rows=rows), False)

    def test_effective_inventory_names_are_exact_full_custom_instance_names(self):
        service = "pulse-" + "department-" * 12
        e = self.exercise(flow="version", state="disabled", service=service)
        self.assert_refreshed(e, False)
        self.assertEqual(e["calls"][-2],
                         f"list-unit-files --no-legend --no-pager --full -- {service}-update.timer {service}-update.service")
        self.assertTrue(e["calls"][:-2])
        self.assertTrue(all(call == f"list-unit-files --no-legend --no-pager --full -- {service}-update.timer"
                            for call in e["calls"][:-2]))

    @unittest.skipUnless(REAL_SYSTEMCTL, "systemctl filesystem inventory is unavailable")
    def test_real_systemctl_inventory_and_complete_flow_use_only_fixture_root(self):
        for state, enabled in (("enabled", True), ("disabled", False)):
            with self.subTest(state=state):
                self.assert_discovery(self.exercise(inventory="real", state=state), True)
                self.assert_refreshed(self.exercise(flow="version", inventory="real", state=state), enabled)


if __name__ == "__main__":
    unittest.main()
