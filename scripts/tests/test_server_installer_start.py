#!/usr/bin/env python3
"""Actual server startup and completion paths against a closed service manager.

Only the fixed unit lookup is relocated. Installer transport, user/directory
setup and marker/completion side effects are closed fixtures. No host service,
network, privileged installation or installed recovery is exercised.
"""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh")).resolve()

MANAGER = r'''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
f = Path(os.environ["FIXTURE"])
args = sys.argv[1:]
with (f / "manager-calls").open("a") as out:
    out.write(json.dumps(args) + "\n")
if args == ["cat", "pulse-fixture.service"]:
    sys.exit(1 if os.environ["SCENARIO"] == "missing-unit" else 0)
if len(args) != 2 or args[1] != "pulse-fixture":
    # All reads/writes must be for the fixture's exact unit.
    if args != ["is-active", "--quiet", "pulse-fixture"]:
        sys.exit(97)
verb = args[0]
scenario = os.environ["SCENARIO"]
if verb == "enable":
    sys.exit(1 if scenario == "enable-refused" else 0)
if verb == "start":
    p = f / "starts"
    count = int(p.read_text()) + 1 if p.exists() else 1
    p.write_text(str(count))
    sys.exit(1 if scenario in ("start-refused", "refused-but-active") else 0)
if verb == "is-active":
    if scenario == "read-refused":
        sys.exit(1)
    if scenario in ("active", "refused-but-active", "enable-refused"):
        sys.exit(0)
    if scenario == "delayed-active":
        p = f / "reads"
        count = int(p.read_text()) + 1 if p.exists() else 1
        p.write_text(str(count))
        sys.exit(0 if count >= 3 else 3)
    if scenario == "retry-active" and (f / "starts").read_text() == "2":
        sys.exit(0)
    sys.exit(3)
sys.exit(97)
'''

HARNESS = r'''
source "$INSTALLER"
source "$FIXTURE/relocated-start.sh"
print_info() { printf 'INFO: %s\n' "$*"; }
print_warn() { printf 'WARN: %s\n' "$*" >&2; }
print_error() { printf 'ERROR: %s\n' "$*" >&2; }
print_success() { printf 'SUCCESS: %s\n' "$*"; }
# Keep the production waiter/command statuses, eliminating only wall-clock
# delay. No timeout/sleep can execute an arbitrary or host service operation.
timeout() {
    [[ "$1" == 5 && "$2" == systemctl ]] || return 97
    shift
    "$@"
}
sleep() { [[ "$1" == 1 || "$1" == 3 ]]; }
journalctl() { printf 'unexpected-journal-read\n' >> "$FIXTURE/events"; return 97; }
SERVICE_NAME=pulse-fixture
PULSE_WAS_ACTIVE="$PRIOR_ACTIVE"
if [[ "$FLOW" == start ]]; then
    status=0
    start_pulse || status=$?
    exit "$status"
fi
# Exercise the complete production caller, including under an OR-list where
# Bash's implicit errexit cannot protect marker/completion calls.
record() { printf '%s\n' "$*" >> "$FIXTURE/events"; }
IN_CONTAINER=true IN_DOCKER=false ENABLE_AUTO_UPDATES=false
FORCE_VERSION="" FORCE_CHANNEL=stable BUILD_FROM_SOURCE=false
CURRENT_VERSION=v6.5.0 FRONTEND_PORT=7655
[[ "$FLOW" != pinned ]] || FORCE_VERSION=v6.6.0
[[ "$FLOW" != source ]] || BUILD_FROM_SOURCE=true
check_proxmox_host() { return 1; }
print_header() { :; }; check_root() { :; }; detect_os() { :; }
check_docker_environment() { :; }
check_existing_installation() { [[ "$FLOW" != fresh ]]; }
resolve_latest_release_tag_for_channel() { printf 'v6.6.0\n'; }
read_configured_update_channel() { printf 'stable\n'; }
safe_read() { printf -v "$2" '%s' "$([[ "$FLOW" == reinstall ]] && echo 2 || echo 1)"; }
run_upgrade_readiness_preflight() { record preflight; }
backup_existing() { record backup; }
create_user() { record user; }
setup_directories() { record directories; }
download_pulse() { record download; }
build_from_source() { record source-build; }
detect_service_name() { printf 'pulse-fixture\n'; }
setup_update_command() { record helper; }
offer_existing_auto_updates() { :; }
update_timer_exists() { return 1; }
setup_auto_updates() { record unexpected-timer; return 97; }
refresh_auto_updates() { record unexpected-timer; return 97; }
install_systemd_service() { record unit; }
ensure_systemd_service_installed() { record unit; }
create_marker_file() { record marker; }
print_completion() { record complete; }
if [[ "$SCENARIO" == caller-refused ]]; then
    start_pulse() { record start-refused; return 23; }
fi
status=0
main || status=$?
exit "$status"
'''


class ServerInstallerStart(unittest.TestCase):
    def exercise(self, scenario, *, prior_active=False, flow="start"):
        with tempfile.TemporaryDirectory(prefix="pulse-start-fixture-") as temporary:
            f = Path(temporary)
            tools = f / "tools"
            tools.mkdir()
            manager = tools / "systemctl"
            manager.write_text(MANAGER)
            manager.chmod(0o700)
            source = INSTALLER.read_text()
            marker = "start_pulse() {"
            self.assertEqual(source.count(marker), 1)
            body = marker + source.split(marker, 1)[1].split("\n}\n", 1)[0] + "\n}\n"
            self.assertIn("/etc/systemd/system/", body)
            # No real /etc lookup or systemctl operation is possible.
            (f / "relocated-start.sh").write_text(body.replace("/etc/systemd/system/", str(f / "units") + "/"))
            env = {k: v for k, v in os.environ.items() if not k.startswith("PULSE_")}
            env.update(FIXTURE=str(f), INSTALLER=str(INSTALLER), SCENARIO=scenario,
                       PRIOR_ACTIVE=str(prior_active).lower(), FLOW=flow,
                       PATH=str(tools) + os.pathsep + os.environ["PATH"],
                       PULSE_INSTALL_DIR=str(f / "install"), PULSE_CONFIG_DIR=str(f / "config"))
            r = subprocess.run(["bash", "-c", HARNESS], cwd=f, env=env,
                               capture_output=True, text=True, timeout=10)
            def lines(name):
                return (f / name).read_text().splitlines() if (f / name).exists() else []
            return r, [json.loads(line) for line in lines("manager-calls")], lines("events")

    def test_unconfirmed_fresh_start_is_failure_not_future_boot_or_success(self):
        for scenario in ("start-refused", "inactive", "read-refused"):
            with self.subTest(scenario=scenario):
                r, calls, events = self.exercise(scenario)
                self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
                self.assertNotIn("SUCCESS:", r.stdout)
                self.assertNotIn("will start automatically", r.stdout + r.stderr)
                self.assertIn("systemctl status pulse-fixture", r.stdout + r.stderr)
                self.assertIn("journalctl -u pulse-fixture", r.stdout + r.stderr)
                self.assertEqual(calls.count(["start", "pulse-fixture"]), 1)
                self.assertNotIn("unexpected-journal-read", events)

    def test_actual_active_read_not_start_command_exit_admits_completion(self):
        for scenario in ("active", "delayed-active", "refused-but-active", "enable-refused"):
            with self.subTest(scenario=scenario):
                r, calls, _ = self.exercise(scenario)
                self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
                self.assertIn("SUCCESS: Pulse started successfully", r.stdout)
                self.assertEqual(calls.count(["start", "pulse-fixture"]), 1)
                self.assertIn(["is-active", "--quiet", "pulse-fixture"], calls)
                if scenario == "delayed-active":
                    self.assertEqual(calls.count(["is-active", "--quiet", "pulse-fixture"]), 3)

    def test_prior_active_update_keeps_its_one_existing_start_retry(self):
        for scenario, expected_exit, starts in (("active", 0, 1), ("retry-active", 0, 2),
                                                ("inactive", 1, 2), ("read-refused", 1, 2)):
            with self.subTest(scenario=scenario):
                r, calls, _ = self.exercise(scenario, prior_active=True)
                self.assertEqual(r.returncode, expected_exit, r.stdout + r.stderr)
                self.assertEqual(calls.count(["start", "pulse-fixture"]), starts)
                self.assertEqual("SUCCESS:" in r.stdout, expected_exit == 0)

    def test_missing_unit_never_reaches_enable_or_start(self):
        r, calls, _ = self.exercise("missing-unit")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertEqual(calls, [["cat", "pulse-fixture.service"]])
        self.assertIn("does not exist", r.stdout + r.stderr)

    def test_every_installer_caller_propagates_start_failure_even_in_an_or_list(self):
        for flow in ("fresh", "pinned", "menu", "reinstall", "source"):
            with self.subTest(flow=flow):
                r, _, events = self.exercise("caller-refused", flow=flow)
                self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
                self.assertIn("start-refused", events)
                self.assertNotIn("marker", events)
                self.assertNotIn("complete", events)

    def test_every_installer_caller_requires_real_active_observation_before_completion(self):
        for flow in ("fresh", "pinned", "menu", "reinstall", "source"):
            for scenario, expected_exit in (("inactive", 1), ("active", 0)):
                with self.subTest(flow=flow, scenario=scenario):
                    r, calls, events = self.exercise(scenario, flow=flow)
                    self.assertEqual(r.returncode, expected_exit, r.stdout + r.stderr)
                    self.assertEqual("marker" in events, expected_exit == 0)
                    self.assertEqual("complete" in events, expected_exit == 0)
                    self.assertEqual(calls.count(["start", "pulse-fixture"]), 1)
                    self.assertIn(["is-active", "--quiet", "pulse-fixture"], calls)


if __name__ == "__main__":
    unittest.main()
