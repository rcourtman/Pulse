#!/usr/bin/env python3
"""Exercise actual installer startup admission without controlling any service.

The real start/wait/recovery functions and five main paths consume confined
systemctl responses. Persistent text fixtures never leave this process. These
are failure-propagation controls, not native startup or installed recovery proof.
"""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh")).resolve()

PRELUDE = r'''
set -euo pipefail
source "$INSTALLER"
print_info() { printf 'INFO: %s\n' "$*"; }
print_warn() { printf 'WARN: %s\n' "$*"; }
print_error() { printf 'ERROR: %s\n' "$*" >&2; }
print_success() { printf 'SUCCESS: %s\n' "$*"; }
PULSE_WAS_ACTIVE="$PRIOR_ACTIVE"
STARTS=0
QUERIES=0
sleep() { :; }
timeout() {
    local duration="$1"; shift
    printf '%s %s\n' "$duration" "$*" >> "$FIXTURE/deadlines"
    [[ "$FAULT:$*" != "start-timeout:systemctl start "* ]] || return 124
    [[ "$FAULT:$*" != "query-timeout:systemctl is-active "* ]] || return 124
    "$@"
}
systemctl() {
    printf '%s\n' "$*" >> "$FIXTURE/calls"
    case "$1" in
        cat) [[ "$FAULT" != missing-unit ]] ;;
        enable) [[ "$FAULT" != enable-failure ]] ;;
        start) STARTS=$((STARTS + 1)); [[ "$FAULT" != start-failure && "$FAULT" != start-error-active ]] ;;
        is-active)
            QUERIES=$((QUERIES + 1))
            case "$FAULT" in
                active|enable-failure|start-error-active) return 0 ;;
                delayed) (( QUERIES >= 3 )) ;;
                retry-recovers) (( STARTS >= 2 )) ;;
                query-error) return 5 ;;
                *) return 3 ;;
            esac
            ;;
        *) echo 'Unexpected service operation' >&2; return 98 ;;
    esac
}
journalctl() { printf '%s\n' "$*" >> "$FIXTURE/journal"; }
'''

MAIN = r'''
CURRENT_VERSION=v6.5.0
FORCE_VERSION=""
FORCE_CHANNEL=stable
BUILD_FROM_SOURCE=false
IN_CONTAINER=true
IN_DOCKER=false
[[ "$FLOW" != pinned ]] || FORCE_VERSION=v6.6.0
[[ "$FLOW" != source ]] || BUILD_FROM_SOURCE=true
for fn in print_header check_root detect_os check_docker_environment offer_existing_auto_updates run_upgrade_readiness_preflight backup_existing install_dependencies create_user download_pulse build_from_source setup_directories setup_update_command install_systemd_service ensure_systemd_service_installed setup_auto_updates refresh_auto_updates; do
    eval "$fn() { :; }"
done
check_existing_installation() { [[ "$FLOW" != fresh ]]; }
check_proxmox_host() { return 1; }
resolve_latest_release_tag_for_channel() { printf 'v6.6.0\n'; }
detect_service_name() { printf '%s\n' "$SERVICE_NAME"; }
safe_read() { local selection=1; [[ "$FLOW" != menu-reinstall ]] || selection=2; printf -v "$2" '%s' "$selection"; }
update_timer_exists() { return 1; }
create_marker_file() { printf 'marker\n' >> "$FIXTURE/completion"; }
print_completion() { printf 'complete\n' >> "$FIXTURE/completion"; }
main || exit "$?"
'''


class ServerStartAdmission(unittest.TestCase):
    def exercise(self, fault, prior=False, body=None, flow=""):
        with tempfile.TemporaryDirectory(prefix="pulse-server-start-") as temporary:
            fixture = Path(temporary)
            install = fixture / "install with spaces"
            config = fixture / "config with spaces"
            install.mkdir()
            config.mkdir()
            originals = {
                install / "VERSION": b"v6.5.0\n",
                install / "pulse": b"synthetic old executable\n",
                config / ".encryption.key": b"synthetic saved key\n",
                config / "system.json": b'{"preserve":"settings"}\n',
                config / "agent-id": b"saved identity\n",
            }
            for path, content in originals.items():
                path.write_bytes(content)
                path.chmod(0o600)
            env = {k: v for k, v in os.environ.items() if not k.startswith("PULSE_")}
            env.update(INSTALLER=str(INSTALLER), FIXTURE=str(fixture), FAULT=fault,
                       FLOW=flow, PRIOR_ACTIVE=str(prior).lower(),
                       PULSE_SERVICE_NAME="pulse-start-fixture", PULSE_INSTALL_DIR=str(install),
                       PULSE_CONFIG_DIR=str(config))
            if body is None:
                body = 'start_pulse || exit "$?"\nprintf "FLAG=%s\\n" "$PULSE_WAS_ACTIVE"\n'
            result = subprocess.run(["bash", "-c", PRELUDE + body], cwd=fixture,
                                    env=env, capture_output=True, text=True, timeout=10)
            self.assertEqual({p: p.read_bytes() for p in originals}, originals)
            self.assertTrue(all(p.stat().st_mode & 0o777 == 0o600 for p in originals))
            return dict(result=result, calls=(fixture / "calls").read_text().splitlines()
                        if (fixture / "calls").exists() else [],
                        deadlines=(fixture / "deadlines").read_text().splitlines(),
                        completion=(fixture / "completion").read_text().splitlines()
                        if (fixture / "completion").exists() else [],
                        journal=(fixture / "journal").read_text().splitlines()
                        if (fixture / "journal").exists() else [])

    def test_unconfirmed_start_fails_for_fresh_and_prior_active_installations(self):
        for prior in (False, True):
            for fault in ("start-failure", "start-timeout", "inactive", "query-error", "query-timeout"):
                with self.subTest(prior=prior, fault=fault):
                    e = self.exercise(fault, prior)
                    self.assertEqual(e["result"].returncode, 1, e["result"])
                    self.assertNotIn("SUCCESS:", e["result"].stdout)
                    self.assertNotIn("will start automatically", e["result"].stdout)
                    self.assertIn("systemctl status pulse-start-fixture", e["result"].stdout)
                    if not prior:
                        # The fixed timeout double returns before systemctl for this case.
                        self.assertEqual(e["calls"].count("start pulse-start-fixture"),
                                         0 if fault == "start-timeout" else 1)

    def test_confirmed_active_and_delayed_start_consume_liveness_flag(self):
        for prior in (False, True):
            for fault in ("active", "delayed", "start-error-active", "enable-failure"):
                with self.subTest(prior=prior, fault=fault):
                    e = self.exercise(fault, prior)
                    self.assertEqual(e["result"].returncode, 0, e["result"])
                    self.assertIn("SUCCESS:", e["result"].stdout)
                    self.assertIn("FLAG=false\n", e["result"].stdout)
                    self.assertNotIn("will start automatically", e["result"].stdout)
                    self.assertEqual(e["calls"].count("start pulse-start-fixture"), 1)
                    self.assertIn("is-active --quiet pulse-start-fixture", e["calls"])

    def test_retry_is_bounded_and_only_restores_prior_active_service(self):
        for prior in (False, True):
            with self.subTest(prior=prior):
                e = self.exercise("retry-recovers", prior)
                self.assertEqual(e["result"].returncode, 0 if prior else 1, e["result"])
                self.assertEqual(e["calls"].count("start pulse-start-fixture"), 2 if prior else 1)
                self.assertEqual(e["calls"].count("is-active --quiet pulse-start-fixture"),
                                 21 if prior else 20)

    def test_missing_unit_stops_before_enable_or_start(self):
        e = self.exercise("missing-unit")
        self.assertEqual(e["result"].returncode, 1, e["result"])
        self.assertIn("does not exist", e["result"].stderr)
        self.assertEqual(e["calls"], ["cat pulse-start-fixture.service"])

    def test_all_five_main_paths_stop_before_marker_or_completion_in_or_list(self):
        for flow in ("source", "pinned", "menu-update", "menu-reinstall", "fresh"):
            for prior in (False, True):
                with self.subTest(flow=flow, prior=prior):
                    e = self.exercise("inactive", prior, body=MAIN, flow=flow)
                    self.assertEqual(e["result"].returncode, 1, e["result"])
                    self.assertEqual(e["completion"], [], e["result"])
                    self.assertNotIn("SUCCESS:", e["result"].stdout)
                    self.assertIn("is-active --quiet pulse-start-fixture", e["calls"])

    def test_all_five_main_paths_complete_only_after_active_readback(self):
        for flow in ("source", "pinned", "menu-update", "menu-reinstall", "fresh"):
            with self.subTest(flow=flow):
                e = self.exercise("active", body=MAIN, flow=flow)
                self.assertEqual(e["result"].returncode, 0, e["result"])
                self.assertEqual(e["completion"], ["marker", "complete"])
                self.assertIn("is-active --quiet pulse-start-fixture", e["calls"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
