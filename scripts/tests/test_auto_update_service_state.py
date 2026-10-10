#!/usr/bin/env python3
"""Exercise the real updater with private files and a confined manager double.

No host manager, network, application build or published installer is used.
The versioned executable and signature verifier are synthetic; existing Go
transaction tests separately exercise real local SSH signatures and metadata.
"""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
UPDATER = Path(os.environ.get("PULSE_AUTO_UPDATER_UNDER_TEST", ROOT / "scripts/pulse-auto-update.sh")).resolve()

MANAGER = r'''#!/usr/bin/env python3
import json, os, sys, time
from pathlib import Path
f = Path(os.environ["FIXTURE"])
args = sys.argv[1:]
with (f / "calls").open("a") as out:
    out.write(json.dumps(args) + "\n")
mode = os.environ["MODE"]
after = (f / "installer-ran").exists()
stopped = (f / "stop-ran").exists()
state = (f / "state").read_text().strip()
if args[0] == "show":
    if not after and mode == "initial-hang": time.sleep(30)
    if not after and mode == "initial-error":
        print("LoadState=loaded\nActiveState=inactive")
        sys.exit(1)
    if stopped and mode == "readback-error": sys.exit(1)
    if stopped and mode == "readback-empty": sys.exit(0)
    if after and not stopped and mode == "after-error": sys.exit(1)
    if after and mode == "success-error": sys.exit(1)
    if not after and mode == "initial-empty": sys.exit(0)
    if not after and mode == "initial-duplicate":
        print("LoadState=loaded\nActiveState=active\nActiveState=inactive")
        sys.exit(0)
    if not after and mode == "initial-extra":
        print("LoadState=loaded\nActiveState=inactive\nUnknown=active")
        sys.exit(0)
    # Reverse property order deliberately: systemd does not promise ordering.
    print("ActiveState=" + state)
    print("LoadState=" + os.environ["LOAD_STATE"])
    sys.exit(0)
if args[0] == "is-active":
    sys.exit(0 if state == "active" and not mode.startswith("initial-") else 1)
if args[0] == "stop":
    (f / "stop-ran").write_text("attempted\n")
    if mode == "stop-hang": time.sleep(30)
    if mode == "stop-error": sys.exit(1)
    if mode != "stop-noop": (f / "state").write_text("inactive\n")
    sys.exit(0)
if args[0] == "start":
    (f / "start-ran").write_text("attempted\n")
    (f / "state").write_text("active\n")
    sys.exit(0)
sys.exit(97)
'''

HARNESS = r'''
source "$UPDATER"
log() {
    printf '[%s] %s\n' "$1" "${*:2}"
    if [[ "${2:-}" == 'installer: '* ]]; then return "$COLLECTOR_EXIT"; fi
}
detect_service_name() { printf '%s\n' "$SERVICE_NAME"; }
sleep() { :; } # Only the liveness wait; timeout/manager deadlines are real.
verify_release_signature() { :; }
mktemp() {
    local -a args=("$@")
    local last=$((${#args[@]} - 1))
    if [[ "${args[last]}" == /tmp/* ]]; then
        args[last]="$FIXTURE/tmp/${args[last]##*/}"
    fi
    printf "%s\n" "${args[*]}" >> "$FIXTURE/temp-requests"
    command mktemp "${args[@]}"
}
curl() {
    local out="" url=""
    while (( $# )); do
        case "$1" in -o) out="$2"; shift ;; https://*) url="$1" ;; esac
        shift
    done
    printf '%s\n' "$url" >> "$FIXTURE/downloads"
    if [[ "$url" == *.sshsig ]]; then printf 'fixture signature\n' > "$out"
    else command cp "$FIXTURE/installer" "$out"; fi
}
if [[ "$FLOW" == backstop ]]; then
    ensure_service_restarted "$SERVICE_NAME" true
    status=0
else
    # Production main uses a conditional call, disabling Bash errexit.
    status=0
    perform_update v6.6.0 || status=$?
fi
printf 'STATUS=%s\n' "$status"
[[ -z "$(trap -p RETURN)" ]] || { echo 'RETURN trap leaked'; exit 98; }
caller() { :; }; caller
exit "$status"
'''


class UpdateServiceState(unittest.TestCase):
    def run_fixture(self, *, mode="normal", state="active", load="loaded", success=False, flow="update",
                    installer_exit=None, collector_exit=0):
        with tempfile.TemporaryDirectory(prefix="pulse-updater-state-") as directory:
            f = Path(directory)
            for name in ("tools", "tmp", "install/bin", "config"):
                (f / name).mkdir(parents=True)
            (f / "tools/systemctl").write_text(MANAGER)
            (f / "tools/systemctl").chmod(0o755)
            old = b"#!/bin/sh\necho 'Pulse v6.5.0'\n"
            new = b"#!/bin/sh\necho 'Pulse v6.6.0'\n"
            (f / "install/bin/pulse").write_bytes(old)
            (f / "install/bin/pulse").chmod(0o751)
            (f / "install/VERSION").write_text("v6.5.0\n")
            (f / "config/system.json").write_text('operator settings\n')
            (f / "config/agent-id").write_text('original identity\n')
            (f / "state").write_text(state + "\n")
            (f / "new-pulse").write_bytes(new)
            (f / "installer").write_text(
                'set -eu\n'
                'echo attempted > "$FIXTURE/installer-ran"\n'
                'cp "$FIXTURE/new-pulse" "$PULSE_INSTALL_DIR/bin/pulse"\n'
                'echo v6.6.0 > "$PULSE_INSTALL_DIR/VERSION"\n'
                'echo "$INSTALLER_STATE" > "$FIXTURE/state"\n'
                'printf "fixture installation output\\n"\n'
                'exit "$INSTALLER_EXIT"\n')
            env = dict(os.environ)
            env.update(FIXTURE=str(f), UPDATER=str(UPDATER), MODE=mode, FLOW=flow,
                       PULSE_INSTALL_DIR=str(f / "install"), PULSE_CONFIG_DIR=str(f / "config"),
                       PULSE_SERVICE_NAME="pulse-custom@department", LOAD_STATE=load,
                       INSTALLER_STATE="active" if mode in ("stop-noop", "stop-error", "stop-hang", "after-error", "readback-error", "readback-empty") else "inactive",
                       INSTALLER_EXIT=str(installer_exit) if installer_exit is not None else ("0" if success else "23"),
                       COLLECTOR_EXIT=str(collector_exit),
                       PATH=str(f / "tools") + os.pathsep + os.environ["PATH"])
            start = time.monotonic()
            result = subprocess.run(["bash", "-c", HARNESS], env=env, capture_output=True, timeout=15)
            calls = [json.loads(line) for line in (f / "calls").read_text().splitlines()] if (f / "calls").exists() else []
            backups = list((f / "tmp").glob("pulse-backup.*"))
            observation = dict(exit=result.returncode, output=result.stdout.decode(), error=result.stderr.decode(),
                               elapsed=time.monotonic() - start, calls=calls,
                               downloaded=(f / "downloads").exists(), temporary=(f / "temp-requests").exists(),
                               installed=(f / "installer-ran").exists(), stopped=(f / "stop-ran").exists(),
                               started=(f / "start-ran").exists(), state=(f / "state").read_text().strip(),
                               binary=(f / "install/bin/pulse").read_bytes(), version=(f / "install/VERSION").read_text(),
                               backups=[(b / "pulse-bin").read_bytes() for b in backups],
                               settings=(f / "config/system.json").read_text(), identity=(f / "config/agent-id").read_text())
            self.assertEqual(observation["settings"], "operator settings\n")
            self.assertEqual(observation["identity"], "original identity\n")
            self.assertNotIn("trap leaked", observation["output"])
            for args in calls:
                # All read/mutation calls target the exact explicit service.
                self.assertIn("pulse-custom@department", args)
            return observation

    def test_unknown_initial_state_refuses_before_backup_download_or_installer(self):
        cases = [(m, "active", "loaded") for m in
                 ("initial-error", "initial-empty", "initial-duplicate", "initial-extra")]
        cases += [("normal", s, "loaded") for s in
                  ("activating", "deactivating", "reloading", "refreshing", "maintenance", "future-state")]
        cases += [("normal", "inactive", s) for s in ("not-found", "error", "bad-setting", "future-load")]
        for mode, state, load in cases:
            with self.subTest(mode=mode, state=state, load=load):
                r = self.run_fixture(mode=mode, state=state, load=load, success=True)
                self.assertNotEqual(r["exit"], 0, r)
                self.assertFalse(r["installed"] or r["downloaded"] or r["temporary"], r)
                self.assertFalse(r["started"] or r["stopped"], r)
                self.assertEqual(r["backups"], [], r)
                self.assertIn(b"v6.5.0", r["binary"])

    def test_initial_observation_timeout_is_bounded_without_mutation(self):
        r = self.run_fixture(mode="initial-hang", success=True)
        self.assertNotEqual(r["exit"], 0, r)
        self.assertFalse(r["installed"] or r["started"] or r["stopped"], r)
        self.assertLess(r["elapsed"], 9, r)

    def test_failed_installer_restores_bytes_and_only_prior_active_service(self):
        for state, load in (("active", "loaded"), ("inactive", "loaded"), ("failed", "loaded"), ("inactive", "masked")):
            with self.subTest(state=state, load=load):
                r = self.run_fixture(state=state, load=load)
                self.assertNotEqual(r["exit"], 0, r)
                self.assertTrue(r["installed"], r)
                self.assertIn(b"v6.5.0", r["binary"])
                self.assertEqual(r["version"], "v6.5.0\n", r)
                self.assertEqual(r["started"], state == "active", r)
                self.assertEqual(r["backups"], [], r)

    def test_success_preserves_prior_active_backstop_without_unwanted_start(self):
        for state in ("active", "inactive"):
            with self.subTest(state=state):
                r = self.run_fixture(state=state, success=True)
                self.assertEqual(r["exit"], 0, r)
                self.assertIn(b"v6.6.0", r["binary"])
                self.assertEqual(r["started"], state == "active", r)
                self.assertFalse(r["stopped"], r)
                self.assertEqual(r["backups"], [], r)

    def test_pipeline_failure_records_both_exits_before_unchanged_recovery(self):
        # Distinct simultaneous failures detect swapped or overwritten statuses;
        # collector-only failure must not be misreported as installer failure.
        for installer, collector in ((23, 0), (0, 73), (42, 73)):
            for state, load in (("active", "loaded"), ("inactive", "loaded"),
                                ("failed", "loaded"), ("inactive", "masked")):
                with self.subTest(installer=installer, collector=collector, state=state, load=load):
                    r = self.run_fixture(state=state, load=load, installer_exit=installer,
                                         collector_exit=collector)
                    observed = f"Installer pipeline failed (installer exit: {installer}; log collector exit: {collector})"
                    self.assertIn(observed, r["output"], r)
                    self.assertLess(r["output"].index(observed), r["output"].index("Restoring from backup"), r)
                    self.assertNotIn("Installer completed", r["output"], r)
                    self.assertNotIn("Update successfully installed and verified", r["output"], r)
                    self.assertEqual(r["exit"], 1, r)
                    self.assertTrue(r["installed"], r)
                    self.assertEqual(r["binary"], b"#!/bin/sh\necho 'Pulse v6.5.0'\n", r)
                    self.assertEqual(r["version"], "v6.5.0\n", r)
                    self.assertEqual(r["started"], state == "active", r)
                    self.assertEqual(r["backups"], [], r)

    def test_completed_pipeline_records_exits_without_claiming_verification(self):
        for state in ("active", "inactive"):
            with self.subTest(state=state):
                r = self.run_fixture(state=state, success=True)
                observed = "Installer completed; verifying update (installer exit: 0; log collector exit: 0)"
                self.assertIn(observed, r["output"], r)
                self.assertLess(r["output"].index(observed), r["output"].index("Version verified:"), r)
                self.assertNotIn("Installer pipeline failed", r["output"], r)
                self.assertEqual(r["exit"], 0, r)
                self.assertIn(b"v6.6.0", r["binary"], r)
                self.assertEqual(r["started"], state == "active", r)
                self.assertEqual(r["backups"], [], r)

    def test_uncertain_rollback_stop_retains_backup_without_replacement_or_start(self):
        for mode in ("stop-noop", "stop-error", "readback-error", "readback-empty", "after-error"):
            with self.subTest(mode=mode):
                r = self.run_fixture(mode=mode)
                self.assertNotEqual(r["exit"], 0, r)
                self.assertTrue(r["installed"], r)
                self.assertIn(b"v6.6.0", r["binary"], r)
                self.assertEqual(r["version"], "v6.6.0\n", r)
                self.assertEqual(len(r["backups"]), 1, r)
                self.assertIn(b"v6.5.0", r["backups"][0], r)
                self.assertFalse(r["started"], r)
                if mode == "after-error": self.assertFalse(r["stopped"], r)

    def test_rollback_stop_timeout_is_bounded_and_retains_recovery(self):
        r = self.run_fixture(mode="stop-hang")
        self.assertNotEqual(r["exit"], 0, r)
        self.assertIn(b"v6.6.0", r["binary"], r)
        self.assertEqual(len(r["backups"]), 1, r)
        self.assertFalse(r["started"], r)
        self.assertLess(r["elapsed"], 9, r)

    def test_post_install_observation_error_is_not_a_restart_or_rollback_grant(self):
        for state in ("active", "inactive"):
            with self.subTest(state=state):
                r = self.run_fixture(mode="success-error", state=state, success=True)
                self.assertNotEqual(r["exit"], 0, r)
                self.assertTrue(r["installed"], r)
                self.assertFalse(r["started"] or r["stopped"], r)
                self.assertIn(b"v6.6.0", r["binary"])
                self.assertEqual(len(r["backups"]), 1, r)

    def test_backstop_never_starts_unknown_or_unsettled_state(self):
        for mode, state in (("initial-error", "inactive"), ("initial-empty", "inactive"),
                            ("normal", "activating"), ("normal", "deactivating")):
            with self.subTest(mode=mode, state=state):
                r = self.run_fixture(mode=mode, state=state, flow="backstop")
                self.assertEqual(r["exit"], 0, r) # A RETURN trap must not mask its caller's exit.
                self.assertFalse(r["started"] or r["stopped"] or r["installed"], r)


if __name__ == "__main__":
    unittest.main()
