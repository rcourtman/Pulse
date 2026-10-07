#!/usr/bin/env python3
"""Exercise real replacement paths; external services/builds use local fixtures.

These are ordinary-user installer controls, not native systemd acceptance.
"""

from pathlib import Path
import os
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = ROOT / "install.sh"

SYSTEMCTL = r'''#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$FIXTURE/calls"
case "$1" in
  show)
    if [[ -f "$FIXTURE/stopped" ]]; then
      [[ "$CASE" != post_query_error ]] || exit 1
      case "$CASE" in
        still_active) echo active ;;
        post_transition) echo deactivating ;;
        post_failed) echo failed ;;
        *) echo inactive ;;
      esac
    else
      case "$CASE" in
        query_error) exit 1 ;;
        query_timeout) sleep 10 ;;
        empty) : ;;
        malformed) printf 'active\ninactive\n' ;;
        inactive|failed|activating|deactivating|reloading|unknown) echo "$CASE" ;;
        *) echo active ;;
      esac
    fi
    ;;
  is-active) [[ "$CASE" != inactive && "$CASE" != failed ]] ;;
  stop)
    case "$CASE" in
      stop_error) exit 1 ;;
      stop_timeout) sleep 10 ;;
    esac
    touch "$FIXTURE/stopped"
    ;;
  *) exit 97 ;;
esac
'''

REPLACEMENT = r'''
source "$INSTALLER"
INSTALL_DIR="$FIXTURE/install"
CONFIG_DIR="$FIXTURE/config"
BUILD_FROM_SOURCE_MARKER="$INSTALL_DIR/BUILD_FROM_SOURCE"
BINARY_LINK_PATH="$FIXTURE/link"
CURRENT_VERSION=v6.5.0
PULSE_WAS_ACTIVE=false
print_info() { :; }
print_success() { :; }
print_error() { echo "$*" >&2; }
detect_service_name() { echo pulse-custom; }
# No network, package installation, signature bypass in production or root writes:
# these fixture-only producers supply the inputs to the actual replacement path.
ensure_update_disk_headroom() { return 0; }
detect_pulse_architecture() { echo amd64; }
run_upgrade_readiness_preflight() { return 0; }
resolve_target_release() { LATEST_RELEASE=v6.6.0; }
create_temp_archive_path() { printf '%s\n' "$FIXTURE/archive.tgz"; }
download_release_archive() { touch "$3" "${3}.sshsig"; }
install_pulse_archive() {
  echo REPLACED >> "$FIXTURE/calls"
  cp "$FIXTURE/new-binary" "$INSTALL_DIR/bin/pulse"
  echo v6.6.0 > "$INSTALL_DIR/VERSION"
}
apt-get() { return 0; }
go() { echo 'go version go1.99.0 linux/amd64'; }
npm() { return 0; }
make() { return 0; }
git() {
  if [[ "$1" == clone ]]; then
    local target="${@: -1}"
    mkdir -p "$target/frontend-modern"
    cp "$FIXTURE/new-binary" "$target/pulse"
  else
    echo abc1234
  fi
}
chown() { return 0; }
install_binary_symlink() { echo REPLACED >> "$FIXTURE/calls"; }
if [[ "$FLOW" == archive ]]; then
  download_pulse
else
  build_from_source fixture
fi
printf 'WAS_ACTIVE=%s\n' "$PULSE_WAS_ACTIVE"
'''

STAGING_FAILURE = r'''
source "$INSTALLER"
INSTALL_DIR="$FIXTURE/install"
CONFIG_DIR="$FIXTURE/config"
CURRENT_VERSION=v6.5.0
LATEST_RELEASE=v6.6.0
FORCE_VERSION=""
[[ "$FLOW" != version ]] || FORCE_VERSION=v6.6.0
for fn in print_header print_info check_root detect_os check_docker_environment backup_existing create_user offer_existing_auto_updates; do
  eval "$fn() { :; }"
done
check_proxmox_host() { return 1; }
check_existing_installation() { return 0; }
detect_service_name() { echo pulse-custom; }
resolve_latest_release_tag_for_channel() { echo v6.6.0; }
read_configured_update_channel() { echo stable; }
run_upgrade_readiness_preflight() { return 0; }
safe_read() {
  local selection=1
  [[ "$FLOW" != reinstall ]] || selection=2
  printf -v "$2" '%s' "$selection"
}
curl() { return 0; }
download_pulse() { echo 'staging failed' >&2; exit 42; }
main
'''


class ServerInstallerStopTest(unittest.TestCase):
    def exercise(self, script, flow, case):
        with tempfile.TemporaryDirectory() as temporary:
            fixture = Path(temporary)
            (fixture / "bin").mkdir()
            installed = fixture / "install"
            (installed / "bin").mkdir(parents=True)
            (fixture / "config").mkdir()
            originals = {
                installed / "bin/pulse": b"old executable\n",
                installed / "VERSION": b"v6.5.0\n",
                installed / "BUILD_FROM_SOURCE": b"old branch\n",
                fixture / "config/private-state": b"fixture persistent state\n",
            }
            for path, content in originals.items():
                path.write_bytes(content)
                path.chmod(0o600)
            (fixture / "new-binary").write_bytes(b"new executable\n")
            (fixture / "bin/systemctl").write_text(SYSTEMCTL)
            (fixture / "bin/systemctl").chmod(0o700)
            env = {**os.environ, "FIXTURE": str(fixture), "CASE": case,
                   "FLOW": flow, "INSTALLER": str(INSTALLER),
                   "PATH": str(fixture / "bin") + ":" + os.environ["PATH"]}
            result = subprocess.run(["bash", "-c", script], env=env,
                                    capture_output=True, text=True, timeout=30)
            return {
                "result": result,
                "files": {str(p.relative_to(fixture)): p.read_bytes() if p.exists() else None for p in originals},
                "modes": {str(p.relative_to(fixture)): p.stat().st_mode & 0o777 if p.exists() else None for p in originals},
                "originals": {str(p.relative_to(fixture)): b for p, b in originals.items()},
                "calls": (fixture / "calls").read_text() if (fixture / "calls").exists() else "",
                "archive_retained": (fixture / "archive.tgz").exists(),
            }

    def test_refused_replacements_preserve_installed_bytes_and_modes(self):
        for flow in ("archive", "source"):
            for case in ("stop_error", "stop_timeout", "query_error", "query_timeout",
                         "still_active", "post_query_error", "post_transition", "post_failed",
                         "activating", "deactivating", "reloading", "unknown", "empty", "malformed"):
                with self.subTest(flow=flow, case=case):
                    observation = self.exercise(REPLACEMENT, flow, case)
                    result = observation["result"]
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn("refusing binary replacement", result.stderr)
                    self.assertEqual(observation["files"], observation["originals"])
                    self.assertTrue(all(m == 0o600 for m in observation["modes"].values()))
                    self.assertNotIn("REPLACED", observation["calls"])
                    self.assertNotRegex(observation["calls"], r"(?m)^(start|restart)\b")
                    self.assertFalse(observation["archive_retained"])

    def test_confirmed_stop_and_inactive_installs_keep_liveness_attribution(self):
        for flow in ("archive", "source"):
            for case in ("active", "inactive", "failed"):
                with self.subTest(flow=flow, case=case):
                    observation = self.exercise(REPLACEMENT, flow, case)
                    result = observation["result"]
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(observation["files"]["install/bin/pulse"], b"new executable\n")
                    self.assertEqual(observation["files"]["config/private-state"], b"fixture persistent state\n")
                    self.assertIn("WAS_ACTIVE=" + str(case == "active").lower(), result.stdout)
                    self.assertEqual(observation["calls"].count("stop pulse-custom"), int(case != "inactive"))
                    self.assertFalse(observation["archive_retained"])

    def test_all_manual_update_flows_leave_service_running_when_staging_fails(self):
        for flow in ("version", "update", "reinstall"):
            with self.subTest(flow=flow):
                observation = self.exercise(STAGING_FAILURE, flow, "active")
                self.assertEqual(observation["result"].returncode, 42, observation["result"].stderr)
                self.assertEqual(observation["files"], observation["originals"])
                self.assertNotRegex(observation["calls"], r"(?m)^stop\b")


if __name__ == "__main__":
    unittest.main()
