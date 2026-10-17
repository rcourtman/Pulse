#!/usr/bin/env python3
"""Connected asset/setup/refresh controls; only manager and ownership are doubles."""
from __future__ import annotations

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh"))


class AutoUpdateReload(unittest.TestCase):
    def exercise(self, caller="setup", fault="", conditional=False, enabled="disabled"):
        source = SOURCE.read_text()
        functions = ""
        for name in ("safe_systemctl", "unit_file_inventory", "repo_web_url",
                     "configure_auto_update_script_repo", "auto_update_dir_writable",
                     "auto_update_helper_is_sane", "auto_update_units_refreshable",
                     "reload_auto_update_manager", "install_auto_update_assets",
                     "setup_auto_updates", "refresh_auto_updates"):
            start = name + "() {"
            if start not in source and name == "reload_auto_update_manager":
                continue  # The exact parent has no strict reload helper.
            functions += start + source.split(start, 1)[1].split("\n}\n", 1)[0] + "\n}\n"
        with tempfile.TemporaryDirectory(prefix="pulse-update-reload-") as temporary:
            root = Path(temporary)
            for directory in ("bin", "config", "install/scripts", "units", "helpers"):
                (root / directory).mkdir(parents=True)
            saved = root / "config/system.json"
            saved.write_text('{"autoUpdateEnabled":false,"updateChannel":"stable","keep":"café"}\n')
            before = saved.read_bytes()
            saved.chmod(0o640)
            (root / "unrelated").write_text("preserve me\n")
            (root / "install/scripts/pulse-auto-update.sh").write_text("#!/bin/bash\n# new helper\n")
            for path in ("helpers/pulse-auto-update.sh", "units/pulse-fixture-update.service",
                         "units/pulse-fixture-update.timer"):
                (root / path).write_text("old installed asset\n")
            (root / "enabled").write_text(enabled)
            (root / "active").write_text("inactive")
            manager = root / "bin/systemctl"
            manager.write_text(r'''#!/usr/bin/env bash
set -eu
printf 'manager:%s\n' "$*" >> "$FIXTURE/events"
case "$1" in
    list-unit-files)
        [[ "$*" == 'list-unit-files --no-legend --no-pager --full -- pulse-fixture-update.timer pulse-fixture-update.service' ]] || exit 97
        printf 'pulse-fixture-update.timer %s\npulse-fixture-update.service static\n' "$(cat "$FIXTURE/enabled")"
        ;;
    daemon-reload)
        [[ "$#" == 1 ]] || exit 97
        # An error can follow acceptance. The caller must neither retry nor
        # infer old files survived; this witness is not a real systemd result.
        printf 'observed\n' > "$FIXTURE/reload-observed"
        case "$FAULT" in
            failed) printf 'SYSTEMCTL_PRIVATE\n' >&2; exit 7 ;;
            signal) kill -TERM "$$" ;;
            hang) trap '' TERM; exec sleep 30 ;;
        esac
        ;;
    enable) printf enabled > "$FIXTURE/enabled" ;;
    start) printf active > "$FIXTURE/active" ;;
    is-enabled) cat "$FIXTURE/enabled"; echo ;;
    is-active) cat "$FIXTURE/active"; echo ;;
    *) exit 98 ;;
esac
''')
            manager.chmod(0o755)
            setup = r'''
set -euo pipefail
CONFIG_DIR="$FIXTURE/config" INSTALL_DIR="$FIXTURE/install"
SERVICE_NAME=pulse-fixture GITHUB_REPO=rcourtman/Pulse ENABLE_AUTO_UPDATES=true
AUTO_UPDATE_DEST="$FIXTURE/helpers/pulse-auto-update.sh"
UPDATE_SERVICE_PATH="$FIXTURE/units/pulse-fixture-update.service"
UPDATE_TIMER_PATH="$FIXTURE/units/pulse-fixture-update.timer"
print_info() { :; }
print_warn() { printf 'WARN: %s\n' "$*"; }
print_success() { printf 'SUCCESS: %s\n' "$*"; }
selected_update_channel() { echo rc; }
chown() { printf 'owner\n' >> "$FIXTURE/events"; }
migrate_auto_update_assets_outside_sandbox() { echo unexpected-migration >&2; return 97; }
'''
            call = {"setup": "setup_auto_updates", "refresh": "refresh_auto_updates",
                    "assets": "install_auto_update_assets", "legacy": "safe_systemctl daemon-reload"}[caller]
            if conditional or caller == "assets":
                invoke = f"result=0; {call} || result=$?\n"
            else:
                invoke = f"{call}\nresult=$?\n"
            invoke += 'printf "FLAG:%s\\n" "$ENABLE_AUTO_UPDATES"\nexit "$result"\n'
            result = subprocess.run(["bash", "-c", setup + functions + invoke], cwd=root,
                env=dict(os.environ, FIXTURE=str(root), FAULT=fault,
                         PATH=str(root / "bin") + os.pathsep + os.environ["PATH"]),
                capture_output=True, text=True, timeout=10)
            return result, dict(
                before=before, final=saved.read_bytes(), mode=saved.stat().st_mode & 0o777,
                enabled=(root / "enabled").read_text(), active=(root / "active").read_text(),
                events=(root / "events").read_text().splitlines(),
                helper=(root / "helpers/pulse-auto-update.sh").read_text(),
                service=(root / "units/pulse-fixture-update.service").read_text(),
                timer=(root / "units/pulse-fixture-update.timer").read_text(),
                staged=list(root.rglob("*.staged.*")) + list(root.rglob("*.tmp")) + list(root.rglob(".system.json.*")),
                unrelated=(root / "unrelated").read_text(),
                reload_observed=(root / "reload-observed").exists())

    def assert_unconfirmed(self, result, observed, caller, enabled="disabled"):
        self.assertEqual(result.returncode, 1 if caller == "assets" else 0, result.stdout + result.stderr)
        self.assertIn("systemd reload was not confirmed", result.stdout)
        self.assertIn("Auto-update files were installed", result.stdout)
        self.assertIn("No settings write or timer enable/start was attempted", result.stdout)
        self.assertNotIn("SUCCESS:", result.stdout)
        self.assertNotIn("SYSTEMCTL_PRIVATE", result.stdout + result.stderr)
        self.assertEqual(observed["before"], observed["final"])
        self.assertEqual(observed["mode"], 0o640)
        self.assertEqual(observed["enabled"], enabled)
        self.assertEqual(observed["active"], "inactive")
        self.assertEqual(observed["events"], [
            "manager:list-unit-files --no-legend --no-pager --full -- pulse-fixture-update.timer pulse-fixture-update.service",
            "manager:daemon-reload"])
        self.assertTrue(observed["reload_observed"])
        self.assertIn("# new helper", observed["helper"])
        self.assertIn("ExecStart=", observed["service"])
        self.assertIn("OnCalendar=*-*-* 02:00:00", observed["timer"])
        self.assertEqual(observed["staged"], [])
        self.assertEqual(observed["unrelated"], "preserve me\n")
        self.assertIn("FLAG:false" if caller == "setup" else "FLAG:true", result.stdout)

    def test_failed_reload_stops_all_three_callers_without_retry_or_rollback(self):
        for caller in ("assets", "setup", "refresh"):
            for conditional in (False, True):
                for enabled in ("disabled", "enabled"):
                    with self.subTest(caller=caller, conditional=conditional, enabled=enabled):
                        r, o = self.exercise(caller, "failed", conditional, enabled)
                        self.assert_unconfirmed(r, o, caller, enabled)

    def test_interrupted_reload_stops_setup_before_consent_or_activation(self):
        for conditional in (False, True):
            with self.subTest(conditional=conditional):
                r, o = self.exercise("setup", "signal", conditional)
                self.assert_unconfirmed(r, o, "setup")

    def test_reload_deadline_kills_an_unresponsive_manager_without_replay(self):
        for caller in ("setup", "refresh"):
            with self.subTest(caller=caller):
                r, o = self.exercise(caller, "hang", True)
                self.assert_unconfirmed(r, o, caller)

    def test_confirmed_reload_precedes_settings_and_timer_activation(self):
        r, o = self.exercise()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("SUCCESS: Automatic updates enabled", r.stdout)
        self.assertEqual(o["enabled"], "enabled")
        self.assertEqual(o["active"], "active")
        self.assertEqual(o["events"][1:], ["manager:daemon-reload", "owner",
            "manager:enable pulse-fixture-update.timer", "manager:is-enabled -- pulse-fixture-update.timer",
            "manager:start pulse-fixture-update.timer", "manager:is-active -- pulse-fixture-update.timer"])
        self.assertEqual(o["staged"], [])

    def test_confirmed_refresh_preserves_settings_and_timer_state(self):
        r, o = self.exercise("refresh", enabled="enabled")
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertEqual(o["final"], o["before"])
        self.assertEqual(o["enabled"], "enabled")
        self.assertEqual(o["active"], "inactive")
        self.assertEqual(len(o["events"]), 2)
        self.assertEqual(o["staged"], [])

    def test_legacy_service_reload_tolerance_is_unchanged(self):
        r, o = self.exercise("legacy", "failed")
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertEqual(o["events"], ["manager:daemon-reload"])
        self.assertEqual(o["helper"], "old installed asset\n")


if __name__ == "__main__":
    unittest.main()
