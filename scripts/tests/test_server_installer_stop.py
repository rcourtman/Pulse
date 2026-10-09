#!/usr/bin/env python3
"""Exercise real replacement paths; external services/builds use local fixtures.

These are ordinary-user installer controls, not native systemd acceptance.
"""

from pathlib import Path
import os
import itertools
import subprocess
import tempfile
import tarfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh"))

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
  is-active) [[ -f "$FIXTURE/restarted" ]] ;;
  start)
    [[ "$FAULT" != recovery_start ]] || exit 1
    [[ "$FAULT" == recovery_inactive ]] || touch "$FIXTURE/restarted"
    ;;
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
print_info() { echo "$*"; }
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
download_release_archive() {
  command cp "$FIXTURE/input.tgz" "$3"
  [[ "$FAULT" == missing_signature ]] || touch "${3}.sshsig"
}
verify_release_signature() {
  echo SIGNATURE >> "$FIXTURE/calls"
  [[ "$FAULT" != signature ]]
}
validate_pulse_binary_architecture() {
  echo ARCHITECTURE >> "$FIXTURE/calls"
  [[ "$FAULT" != architecture ]]
}
install_additional_agent_binaries() { echo AGENTS >> "$FIXTURE/calls"; }
deploy_agent_scripts() { echo SCRIPTS >> "$FIXTURE/calls"; }
restore_selinux_contexts() { :; }
cp() {
  if [[ "$*" == *'.pulse-stage-'* && "$FAULT" == copy ]]; then
    printf partial > "${@: -1}"
    return 1
  fi
  command cp "$@"
}
chmod() {
  [[ "$FAULT" != permissions ]] || return 1
  command chmod "$@"
}
mktemp() {
  [[ "$FAULT" != extract_directory || "$*" != *pulse-extract-* ]] || return 1
  [[ "$FAULT" != stage_directory || "$*" != *'.pulse-stage-'* ]] || return 1
  local created
  created=$(command mktemp "$@") || return 1
  printf '%s\n' "$created" >> "$FIXTURE/temporary-paths"
  printf '%s\n' "$created"
}
mv() {
  echo RENAME >> "$FIXTURE/calls"
  case "$FAULT" in rename|recovery_start|recovery_inactive) return 1 ;; esac
  command mv "$@"
}
sleep() { :; }

apt-get() { return 0; }
go() { echo 'go version go1.99.0 linux/amd64'; }
npm() { return 0; }
make() { return 0; }
git() {
  if [[ "$1" == clone ]]; then
    local target="${@: -1}"
    mkdir -p "$target/frontend-modern"
    [[ "$FAULT" == missing_binary ]] || cp "$FIXTURE/new-binary" "$target/pulse"
  else
    [[ "$FAULT" != source_revision ]] || return 1
    [[ "$FAULT" != source_revision_empty ]] || return 0
    echo abc1234
  fi
}
chown() { [[ "$FAULT" != ownership ]]; }
install_binary_symlink() { echo REPLACED >> "$FIXTURE/calls"; }
if [[ "$FLOW" == archive || "$FLOW" == local_archive ]]; then
  if [[ "$FLOW" == local_archive ]]; then
    ARCHIVE_OVERRIDE="$FIXTURE/input.tgz"
    FORCE_VERSION=v6.6.0
  fi
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
    def exercise(self, script, flow, case, fault=""):
        with tempfile.TemporaryDirectory() as temporary:
            fixture = Path(temporary)
            (fixture / "bin").mkdir()
            installed = fixture / "install"
            (installed / "bin").mkdir(parents=True)
            (fixture / "config").mkdir()
            originals = {
                installed / "bin/pulse": b"old executable\n",
                installed / "bin/pulse.old": b"pre-existing recovery binary\n",
                installed / "VERSION": b"v6.5.0\n",
                installed / "BUILD_FROM_SOURCE": b"old branch\n",
                fixture / "config/private-state": b"fixture persistent state\n",
            }
            for path, content in originals.items():
                path.write_bytes(content)
                path.chmod(0o600)
            new_binary = b"#!/bin/sh\necho VERSION_PROBE >> \"$FIXTURE/calls\"\n"
            if fault == "version_timeout":
                new_binary += b"sleep 10\n"
            elif fault == "version_exit":
                new_binary += b"echo v6.6.0; exit 1\n"
            elif fault == "version_empty":
                new_binary += b"printf ' \\t\\n'\n"
            else:
                version = "v6.5.0" if fault == "version_mismatch" else "unknown" if fault == "version_unknown" else "v6.6.0"
                new_binary += ("echo " + version + "\n").encode()
            (fixture / "new-binary").write_bytes(new_binary)
            (fixture / "new-binary").chmod(0o700)
            with tarfile.open(fixture / "input.tgz", "w:gz") as archive:
                if fault != "missing_binary":
                    archive.add(fixture / "new-binary", arcname="bin/pulse")
                version_file = fixture / "candidate-version"
                version_file.write_text("v6.6.0\n")
                archive.add(version_file, arcname="VERSION")
            if fault != "missing_signature":
                (fixture / "input.tgz.sshsig").touch()
            if fault == "extraction":
                (fixture / "input.tgz").write_bytes(b"not a tarball")
            (fixture / "bin/systemctl").write_text(SYSTEMCTL)
            (fixture / "bin/systemctl").chmod(0o700)
            env = {**os.environ, "FIXTURE": str(fixture), "CASE": case,
                   "FLOW": flow, "INSTALLER": str(INSTALLER), "FAULT": fault,
                   "PATH": str(fixture / "bin") + ":" + os.environ["PATH"]}
            result = subprocess.run(["bash", "-c", script], env=env,
                                    capture_output=True, text=True, timeout=30)
            return {
                "result": result,
                "new_binary": new_binary,
                "stage_paths": list(installed.glob("bin/.pulse-stage-*")),
                "temporary_paths_remaining": [p for p in (fixture / "temporary-paths").read_text().splitlines() if Path(p).exists()] if (fixture / "temporary-paths").exists() else [],
                "local_archive_retained": (fixture / "input.tgz").exists(),
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
                    self.assertNotIn("RENAME", observation["calls"])
                    self.assertEqual(observation["stage_paths"], [])
                    self.assertEqual(observation["temporary_paths_remaining"], [])
                    self.assertNotRegex(observation["calls"], r"(?m)^(start|restart)\b")
                    self.assertFalse(observation["archive_retained"])

    def test_confirmed_stop_and_inactive_installs_keep_liveness_attribution(self):
        for flow in ("archive", "source"):
            for case in ("active", "inactive", "failed"):
                with self.subTest(flow=flow, case=case):
                    observation = self.exercise(REPLACEMENT, flow, case)
                    result = observation["result"]
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(observation["files"]["install/bin/pulse"], observation["new_binary"])
                    self.assertEqual(observation["files"]["config/private-state"], b"fixture persistent state\n")
                    self.assertIn("WAS_ACTIVE=" + str(case == "active").lower(), result.stdout)
                    self.assertEqual(observation["calls"].count("stop pulse-custom"), int(case != "inactive"))
                    self.assertFalse(observation["archive_retained"])

    def test_archive_admission_failures_never_stop_or_mutate_pulse(self):
        for flow in ("archive", "local_archive"):
            for fault in ("missing_signature", "signature", "extract_directory", "extraction",
                          "missing_binary", "architecture", "stage_directory", "copy",
                          "permissions", "ownership", "version_exit", "version_timeout",
                          "version_unknown", "version_mismatch"):
                with self.subTest(flow=flow, fault=fault):
                    observation = self.exercise(REPLACEMENT, flow, "active", fault)
                    self.assertNotEqual(observation["result"].returncode, 0)
                    self.assertEqual(observation["files"], observation["originals"])
                    self.assertTrue(all(m == 0o600 for m in observation["modes"].values()))
                    self.assertNotRegex(observation["calls"], r"(?m)^(show|stop|start|restart|RENAME|AGENTS|SCRIPTS)\b")
                    self.assertEqual(observation["stage_paths"], [])
                    self.assertEqual(observation["temporary_paths_remaining"], [])
                    self.assertFalse(observation["archive_retained"])
                    self.assertTrue(observation["local_archive_retained"])

    def test_atomic_rename_failure_restores_only_a_previously_active_service(self):
        for flow, state, fault in itertools.product(
            ("archive", "source"), ("active", "inactive", "failed"),
            ("rename", "recovery_start", "recovery_inactive")
        ):
            with self.subTest(flow=flow, state=state, fault=fault):
                observation = self.exercise(REPLACEMENT, flow, state, fault)
                result = observation["result"]
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(observation["files"], observation["originals"])
                self.assertTrue(all(m == 0o600 for m in observation["modes"].values()))
                self.assertEqual(observation["calls"].count("start pulse-custom"), int(state == "active"))
                self.assertNotIn("AGENTS", observation["calls"])
                self.assertNotIn("SCRIPTS", observation["calls"])
                if state == "active":
                    if fault == "rename":
                        self.assertIn("Previous Pulse binary is running again; the update failed", result.stdout)
                    else:
                        self.assertIn("could not be confirmed running", result.stderr)
                self.assertEqual(observation["stage_paths"], [])
                self.assertEqual(observation["temporary_paths_remaining"], [])
                self.assertFalse(observation["archive_retained"])

    def test_source_admission_failures_never_stop_or_mutate_pulse(self):
        for fault in ("missing_binary", "stage_directory", "copy", "permissions",
                      "ownership", "version_exit", "version_timeout", "version_empty",
                      "source_revision", "source_revision_empty"):
            with self.subTest(fault=fault):
                observation = self.exercise(REPLACEMENT, "source", "active", fault)
                self.assertNotEqual(observation["result"].returncode, 0)
                self.assertEqual(observation["files"], observation["originals"])
                self.assertTrue(all(m == 0o600 for m in observation["modes"].values()))
                self.assertNotRegex(observation["calls"], r"(?m)^(show|stop|start|restart|RENAME|AGENTS|SCRIPTS)\b")
                self.assertEqual(observation["stage_paths"], [])
                self.assertEqual(observation["temporary_paths_remaining"], [])

    def test_source_success_admits_bytes_before_stop_and_commits_once(self):
        observation = self.exercise(REPLACEMENT, "source", "active")
        self.assertEqual(observation["result"].returncode, 0, observation["result"].stderr)
        calls = observation["calls"]
        self.assertLess(calls.index("VERSION_PROBE"), calls.index("stop pulse-custom"))
        self.assertLess(calls.index("stop pulse-custom"), calls.index("RENAME"))
        self.assertEqual(calls.count("RENAME"), 1)
        self.assertEqual(calls.count("VERSION_PROBE"), 1)
        self.assertEqual(observation["files"]["install/bin/pulse"], observation["new_binary"])
        self.assertEqual(observation["modes"]["install/bin/pulse"], 0o755)
        self.assertEqual(observation["files"]["install/bin/pulse.old"], observation["originals"]["install/bin/pulse.old"])
        self.assertEqual(observation["files"]["install/VERSION"], b"fixture-abc1234\n")
        self.assertEqual(observation["files"]["install/BUILD_FROM_SOURCE"], b"fixture\n")
        self.assertEqual(observation["files"]["config/private-state"], b"fixture persistent state\n")
        self.assertEqual(observation["stage_paths"], [])
        self.assertEqual(observation["temporary_paths_remaining"], [])
        self.assertNotIn("start pulse-custom", calls)

    def test_archive_success_admits_bytes_before_stop_and_commits_once(self):
        observation = self.exercise(REPLACEMENT, "archive", "active")
        self.assertEqual(observation["result"].returncode, 0, observation["result"].stderr)
        calls = observation["calls"]
        self.assertLess(calls.index("SIGNATURE"), calls.index("ARCHITECTURE"))
        self.assertLess(calls.index("ARCHITECTURE"), calls.index("VERSION_PROBE"))
        self.assertLess(calls.index("VERSION_PROBE"), calls.index("stop pulse-custom"))
        self.assertLess(calls.index("stop pulse-custom"), calls.index("RENAME"))
        self.assertLess(calls.index("RENAME"), calls.index("AGENTS"))
        self.assertEqual(calls.count("RENAME"), 1)
        self.assertEqual(calls.count("VERSION_PROBE"), 1)
        self.assertEqual(observation["files"]["install/bin/pulse"], observation["new_binary"])
        self.assertEqual(observation["modes"]["install/bin/pulse"], 0o755)
        self.assertEqual(observation["stage_paths"], [])
        self.assertEqual(observation["temporary_paths_remaining"], [])
        self.assertNotIn("start pulse-custom", calls)

    def test_all_manual_update_flows_leave_service_running_when_staging_fails(self):
        for flow in ("version", "update", "reinstall"):
            with self.subTest(flow=flow):
                observation = self.exercise(STAGING_FAILURE, flow, "active")
                self.assertEqual(observation["result"].returncode, 42, observation["result"].stderr)
                self.assertEqual(observation["files"], observation["originals"])
                self.assertNotRegex(observation["calls"], r"(?m)^stop\b")


if __name__ == "__main__":
    unittest.main()
