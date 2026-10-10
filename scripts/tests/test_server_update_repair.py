#!/usr/bin/env python3
"""Exercise both existing-install update paths after config/unit removal.

The production main, directory setup, missing-unit repair and unit renderer run
against private files. Only their fixed unit destination is relocated. Transport,
ownership, activation and timer operations are confined doubles; no host service,
release signature or native installed recovery is qualified here.
"""

import json
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
# Go executes this test from scripts/installtests and passes repoFile's relative
# path. Resolve it before moving the shell into its private working directory.
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh")).resolve()

MANAGER = r'''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
f = Path(os.environ["FIXTURE"])
args = sys.argv[1:]
with (f / "manager-calls").open("a") as out:
    out.write(json.dumps(args) + "\n")
if args == ["list-unit-files", "--no-legend", "--no-pager", "--full", "--",
            "pulse-backend.service", "pulse.service"]:
    # Complete known absence permits recreating the missing default unit.
    sys.exit(0)
if args == ["daemon-reload"]:
    sys.exit(0)
sys.exit(97)
'''

HARNESS = r'''
source "$INSTALLER"
source "$FIXTURE/relocated-units.sh"
FORCE_VERSION="$PIN"
FORCE_CHANNEL=stable
IN_CONTAINER=true
IN_DOCKER=false
ENABLE_AUTO_UPDATES="$AUTO_UPDATES"
print_header() { :; }
print_info() { echo "$*"; }
print_warn() { echo "$*" >&2; }
print_error() { echo "$*" >&2; }
print_success() { echo "$*"; }
check_root() { :; }
check_proxmox_host() { return 1; }
detect_os() { :; }
check_docker_environment() { :; }
safe_read() { return 1; } # The ordinary non-interactive menu selects the upgrade.
resolve_latest_release_tag_for_channel() { echo v6.6.0; }
offer_existing_auto_updates() { :; }
record() { printf '%s\n' "$*" >> "$FIXTURE/events"; }
run_upgrade_readiness_preflight() { record preflight; }
backup_existing() { record backup; }
create_user() { record user; }
download_pulse() { record download; cp "$FIXTURE/new-pulse" "$INSTALL_DIR/bin/pulse"; }
chown() {
    # Exercise production mkdir/chmod/config creation, without needing a host
    # pulse user or allowing any ownership operation outside the fixture.
    local path
    for path in "$@"; do
        case "$path" in -R|pulse:pulse|"$FIXTURE"/*) ;; *) return 97 ;; esac
    done
}
setup_update_command() { test -f "$CONFIG_DIR/.env" || return 96; record helper; }
update_timer_exists() { return 1; }
setup_auto_updates() { test -f "$CONFIG_DIR/.env" || return 96; record timer; }
refresh_auto_updates() { record unexpected-refresh; return 97; }
start_pulse() {
    test -f "$FIXTURE/units/$SERVICE_NAME.service" || return 96
    test -f "$CONFIG_DIR/.env" || return 96
    record "start $SERVICE_NAME"
}
create_marker_file() { record marker; }
print_completion() { record complete; }
if [[ "$UNIT_ERROR" == true ]]; then
    install_systemd_service() { record refused-unit; return 23; }
fi
# Failure propagation must survive an OR-list, not depend on implicit errexit.
status=0
main || status=$?
exit "$status"
'''


class ServerUpdateRepair(unittest.TestCase):
    def exercise(self, *, pinned, automatic, unit_error=False):
        with tempfile.TemporaryDirectory(prefix="pulse-update-repair-") as directory:
            f = Path(directory)
            for name in ["tools", "install/bin", "units"]:
                (f / name).mkdir(parents=True)
            for name, version in [("install/bin/pulse", "v6.5.0"), ("new-pulse", "v6.6.0")]:
                (f / name).write_text("#!/bin/sh\necho 'Pulse " + version + "'\n")
                (f / name).chmod(0o755)
            (f / "tools/systemctl").write_text(MANAGER)
            (f / "tools/systemctl").chmod(0o755)
            source = INSTALLER.read_text()
            functions = []
            for name in ["install_systemd_service", "ensure_systemd_service_installed"]:
                start = source.index(name + "() {")
                end = source.index("\n}\n", start) + 3
                functions.append(source[start:end].replace("/etc/systemd/system/", str(f / "units") + "/"))
            (f / "relocated-units.sh").write_text("\n".join(functions))
            env = {k: v for k, v in os.environ.items() if not k.startswith("PULSE_")}
            env.update(FIXTURE=str(f), INSTALLER=str(INSTALLER),
                       PIN="v6.6.0" if pinned else "",
                       AUTO_UPDATES=str(automatic).lower(), UNIT_ERROR=str(unit_error).lower(),
                       PATH=str(f / "tools") + os.pathsep + os.environ["PATH"],
                       PULSE_INSTALL_DIR=str(f / "install"), PULSE_CONFIG_DIR=str(f / "config"))
            # Config and unit are genuinely absent on entry, not pre-created.
            self.assertFalse((f / "config").exists())
            self.assertEqual(list((f / "units").iterdir()), [])
            result = subprocess.run(["bash", "-c", HARNESS], cwd=f, env=env,
                                    capture_output=True, text=True, timeout=10)
            def lines(name):
                return (f / name).read_text().splitlines() if (f / name).exists() else []
            env_file = f / "config/.env"
            return dict(result=result, events=lines("events"),
                        calls=[json.loads(line) for line in lines("manager-calls")],
                        config_mode=stat.S_IMODE((f / "config").stat().st_mode) if (f / "config").exists() else None,
                        env_mode=stat.S_IMODE(env_file.stat().st_mode) if env_file.exists() else None,
                        env=env_file.read_text() if env_file.exists() else "",
                        unit=(f / "units/pulse.service").read_text() if (f / "units/pulse.service").exists() else "")

    def test_both_update_paths_repair_config_and_unit_before_activation(self):
        for pinned in (True, False):
            for automatic in (True, False):
                with self.subTest(pinned=pinned, automatic=automatic):
                    e = self.exercise(pinned=pinned, automatic=automatic)
                    self.assertEqual(e["result"].returncode, 0, e["result"].stdout + e["result"].stderr)
                    self.assertEqual(e["config_mode"], 0o700)
                    self.assertEqual(e["env_mode"], 0o600)
                    self.assertIn("PULSE_MOCK_MODE=false", e["env"])
                    self.assertIn("EnvironmentFile=", e["unit"])
                    self.assertIn("/install/bin/pulse", e["unit"])
                    self.assertIn(["daemon-reload"], e["calls"])
                    self.assertEqual(e["events"], ["preflight", "backup", "user", "download", "helper"] +
                                     (["timer"] if automatic else []) + ["start pulse", "marker", "complete"])

    def test_failed_unit_repair_stops_timer_start_and_false_completion(self):
        for pinned in (True, False):
            for automatic in (True, False):
                with self.subTest(pinned=pinned, automatic=automatic):
                    e = self.exercise(pinned=pinned, automatic=automatic, unit_error=True)
                    self.assertEqual(e["result"].returncode, 1, e["result"].stdout + e["result"].stderr)
                    self.assertEqual(e["events"], ["preflight", "backup", "user", "download", "helper", "refused-unit"])
                    self.assertEqual(e["unit"], "")
                    self.assertNotIn(["daemon-reload"], e["calls"])


if __name__ == "__main__":
    unittest.main()
