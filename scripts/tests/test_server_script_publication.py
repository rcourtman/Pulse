#!/usr/bin/env python3
"""Real installer-script publication with private files and inert service doubles.

The archive and source consumers use synthetic payloads, not release acceptance.
No host service, transport, ownership or customer state is operated here.
"""
import os
from pathlib import Path
import stat
import subprocess
import tarfile
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh")).resolve()
NAMES = ("install-container-agent.sh", "install-docker.sh", "install.sh", "install.ps1")

HARNESS = r'''
set -euo pipefail
source "$INSTALLER"
print_info() { printf 'INFO %s\n' "$*"; }
print_warn() { printf 'WARN %s\n' "$*" >&2; }
print_error() { printf 'ERROR %s\n' "$*" >&2; }
print_success() { printf 'SUCCESS %s\n' "$*"; }
record() { printf '%s\n' "$*" >> "$FIXTURE/events"; }
cp() {
    local source="${@: -2:1}" destination="${@: -1}"
    if [[ "$FAULT" == copy && "$source" == */input/scripts/install.ps1 ]]; then
        printf partial > "$destination"
        return 23
    fi
    [[ "$FAULT" != sidecar-copy || "$source" != */install.ps1.sshsig ]] || return 23
    [[ "$FAULT" != preserve-copy || "$source" != "$INSTALL_DIR/scripts/." ]] || return 23
    command cp "$@"
}
chmod() {
    [[ "$FAULT" != mode || "${@: -1}" != */install.ps1 ]] || return 24
    [[ "$FAULT" != sidecar-mode || "${@: -1}" != */install.ps1.sshsig ]] || return 24
    command chmod "$@"
}
chown() {
    [[ "$FAULT" != owner || "${@: -1}" != */install.ps1 ]] || return 25
    [[ "$FAULT" != sidecar-owner || "${@: -1}" != */install.ps1.sshsig ]] || return 25
    # No actual ownership change, even inside the fixture.
    :
}
mkdir() {
    [[ "$FAULT" != directory || "$*" != *scripts* ]] || return 26
    command mkdir "$@"
}
mktemp() {
    [[ "$FAULT" != allocation || "$*" != *pulse-scripts-* ]] || return 27
    command mktemp "$@"
}
mv() {
    local source="${@: -2:1}" destination="${@: -1}"
    if [[ "$source" == */next && "$destination" == "$INSTALL_DIR/scripts" ]]; then
        record publish
        case "$FAULT" in
            publish|rollback) return 28 ;;
            publish-skipped) return 0 ;;
            conflict) mkdir "$destination"; printf foreign > "$destination/foreign"; return 0 ;;
        esac
    fi
    [[ "$FAULT" != rollback || "$source" != */previous ]] || return 29
    if [[ "$source" == "$INSTALL_DIR/scripts" ]]; then
        [[ "$FAULT" != reserve ]] || return 30
        [[ "$FAULT" != reserve-skipped ]] || return 0
        if [[ "$FAULT" == reserve-conflict ]]; then
            command mv "$@"
            command mkdir "$INSTALL_DIR/scripts"
            printf foreign > "$INSTALL_DIR/scripts/foreign"
            return 0
        fi
    fi
    command mv "$@"
}
if [[ "$FLOW" == direct ]]; then
    deploy_agent_scripts "$FIXTURE/input" || exit "$?"
elif [[ "$FLOW" == plain ]]; then
    deploy_agent_scripts "$FIXTURE/input"
else
    CURRENT_VERSION=v6.5.0
    FORCE_VERSION=v6.6.0
    ARCHIVE_OVERRIDE="$FIXTURE/input.tgz"
    detect_service_name() { printf 'pulse-script-fixture\n'; }
    detect_pulse_architecture() { printf 'amd64\n'; }
    ensure_update_disk_headroom() { :; }
    run_upgrade_readiness_preflight() { :; }
    verify_release_signature() { :; } # synthetic archive only
    validate_pulse_binary_architecture() { :; }
    install_additional_agent_binaries() { :; }
    install_binary_symlink() { :; }
    restore_selinux_contexts() { :; }
    systemctl() {
        record "$*"
        case "$1" in
            cat|enable) return 0 ;;
            show) if [[ -e "$FIXTURE/stopped" || "$PRIOR" == false ]]; then echo inactive; else echo active; fi ;;
            stop) touch "$FIXTURE/stopped" ;;
            start) touch "$FIXTURE/restarted" ;;
            is-active) [[ -e "$FIXTURE/restarted" ]] ;;
            *) return 98 ;;
        esac
    }
    timeout() { shift; "$@"; }
    sleep() { :; }
    # Inject the same failure at the actual extracted script, not only a
    # direct helper. Archive installation is itself always an if/OR callee.
    cp() {
        local source="${@: -2:1}" destination="${@: -1}"
        if [[ "$FAULT" == copy && "$source" == */scripts/install.ps1 ]]; then
            printf partial > "$destination"; return 23
        fi
        [[ "$FAULT" != sidecar-copy || "$source" != */scripts/install.ps1.sshsig ]] || return 23
        command cp "$@"
    }
    if [[ "$FLOW" == archive ]]; then
        download_pulse
    fi
    if [[ "$FLOW" != archive ]]; then
        apt-get() { :; }
        go() { echo 'go version go1.99.0 linux/amd64'; }
        npm() { :; }
        make() { :; }
        git() {
            if [[ "$1" == clone ]]; then
                local target="${@: -1}"
                command cp -a "$FIXTURE/input" "$target"
                command mkdir "$target/frontend-modern"
                command cp "$FIXTURE/input/bin/pulse" "$target/pulse"
            else
                echo abc1234
            fi
        }
        if [[ "$FLOW" == source ]]; then
            build_from_source fixture || exit "$?"
        else
            IN_CONTAINER=true
            IN_DOCKER=false
            FORCE_VERSION=""
            FORCE_CHANNEL=stable
            FRONTEND_PORT=7655
            BUILD_FROM_SOURCE=false
            [[ "$FLOW" != main-pinned ]] || FORCE_VERSION=v6.6.0
            [[ "$FLOW" != main-source ]] || BUILD_FROM_SOURCE=true
            for fn in print_header check_root detect_os check_docker_environment offer_existing_auto_updates backup_existing install_dependencies create_user setup_directories setup_update_command install_systemd_service ensure_systemd_service_installed setup_auto_updates refresh_auto_updates; do
                eval "$fn() { :; }"
            done
            check_proxmox_host() { return 1; }
            check_existing_installation() { [[ "$FLOW" != main-fresh ]]; }
            resolve_latest_release_tag_for_channel() { echo v6.6.0; }
            safe_read() { local selection=1; [[ "$FLOW" != main-reinstall ]] || selection=2; printf -v "$2" '%s' "$selection"; }
            update_timer_exists() { return 1; }
            create_marker_file() { record marker; }
            print_completion() { record completion; }
            main || exit "$?"
        fi
    fi
fi
record completion
'''


class ServerScriptPublication(unittest.TestCase):
    def setUp(self):
        self.fixture_count = 0

    def tearDown(self):
        print(f"Executed {self.fixture_count} confined fixture(s)")

    def exercise(self, fault="", flow="direct", prior=True, existing=True, sidecars=True, target="directory"):
        self.fixture_count += 1
        with tempfile.TemporaryDirectory(prefix="pulse-script-publication-") as temporary:
            f = Path(temporary)
            install = f / "install"
            for directory in [install / "bin", f / "config", f / "input/scripts", f / "input/bin"]:
                directory.mkdir(parents=True)
            original = {}
            if existing:
                (install / "scripts").mkdir()
                for name in [n + suffix for n in NAMES for suffix in ("", ".sig", ".sshsig")] + ["custom-helper"]:
                    p = install / "scripts" / name
                    p.write_bytes(("old-" + name).encode())
                    p.chmod(0o640)
                    original[name] = p.read_bytes()
            for name in NAMES:
                (f / "input/scripts" / name).write_bytes(("new-" + name).encode())
                if sidecars:
                    for suffix in (".sig", ".sshsig"):
                        (f / "input/scripts" / (name + suffix)).write_bytes(("new-" + name + suffix).encode())
            outside = f / "outside"
            outside.write_bytes(b"unrelated file")
            if target == "file-link":
                (install / "scripts/install.ps1").unlink()
                (install / "scripts/install.ps1").symlink_to(outside)
            elif target == "directory-link":
                (install / "scripts").rename(f / "original-scripts")
                (install / "scripts").symlink_to(f / "original-scripts")
            binary = f / "input/bin/pulse"
            binary.write_text("#!/bin/sh\necho 'Pulse v6.6.0'\n")
            binary.chmod(0o755)
            (install / "bin/pulse").write_bytes(b"old executable")
            (install / "VERSION").write_text("v6.5.0\n")
            saved_state = {
                f / "config/.encryption.key": b"synthetic saved key",
                f / "config/.env": b"PULSE_MOCK_MODE=false\n",
                f / "config/system.json": b'{"preserve":"configuration"}\n',
                f / "config/agent-id": b"synthetic saved identity",
                f / "config.backup.keep/.encryption.key": b"synthetic recovery key",
            }
            for path, content in saved_state.items():
                path.parent.mkdir(exist_ok=True)
                path.write_bytes(content)
                path.chmod(0o600)
            with tarfile.open(f / "input.tgz", "w:gz") as archive:
                for child in (f / "input").iterdir():
                    archive.add(child, arcname=child.name)
            (f / "input.tgz.sshsig").write_text("synthetic sidecar")
            env = {k: v for k, v in os.environ.items() if not k.startswith("PULSE_")}
            env.update(INSTALLER=str(INSTALLER), FIXTURE=str(f), FLOW=flow, FAULT=fault,
                       PRIOR=str(prior).lower(), PULSE_INSTALL_DIR=str(install),
                       PULSE_CONFIG_DIR=str(f / "config"), PULSE_SERVICE_NAME="pulse-script-fixture")
            result = subprocess.run(["bash", "-c", HARNESS], cwd=f, env=env,
                                    text=True, capture_output=True, timeout=15)
            for path, content in saved_state.items():
                self.assertEqual(path.read_bytes(), content)
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
            self.assertEqual(outside.read_bytes(), b"unrelated file")
            live = install / "scripts"
            contents = {p.name: p.read_bytes() for p in live.iterdir()} if live.is_dir() else {}
            modes = {p.name: stat.S_IMODE(p.stat().st_mode) for p in live.iterdir()} if live.is_dir() else {}
            retained = {str(p.relative_to(install)): p.read_bytes()
                        for p in install.glob(".pulse-scripts-*/previous/*")}
            events = (f / "events").read_text().splitlines() if (f / "events").exists() else []
            return dict(result=result, original=original, contents=contents, modes=modes,
                        stages=list(install.glob(".pulse-scripts-*")), retained=retained,
                        events=events, restarted=(f / "restarted").exists())

    def test_preparation_failures_preserve_the_complete_old_bundle(self):
        for fault in ("copy", "mode", "owner", "directory", "allocation", "sidecar-copy", "sidecar-mode", "sidecar-owner", "preserve-copy"):
            for flow in ("plain", "direct"):
                with self.subTest(fault=fault, flow=flow):
                    e = self.exercise(fault, flow)
                    self.assertNotEqual(e["result"].returncode, 0, e["result"].stdout + e["result"].stderr)
                    self.assertEqual(e["contents"], e["original"])
                    self.assertTrue(all(mode == 0o640 for mode in e["modes"].values()))
                    self.assertEqual(e["stages"], [])
                    self.assertNotIn("completion", e["events"])

    def test_failed_or_skipped_publication_restores_old_bundle(self):
        for fault in ("publish", "publish-skipped", "reserve", "reserve-skipped"):
            for existing in ((True,) if fault.startswith("reserve") else (False, True)):
                with self.subTest(fault=fault, existing=existing):
                    e = self.exercise(fault, existing=existing)
                    self.assertNotEqual(e["result"].returncode, 0)
                    self.assertEqual(e["contents"], e["original"])
                    self.assertEqual(e["stages"], [])

    def test_failed_restore_retains_original_bundle_and_names_its_location(self):
        for fault in ("rollback", "conflict", "reserve-conflict"):
            with self.subTest(fault=fault):
                e = self.exercise(fault)
                self.assertNotEqual(e["result"].returncode, 0)
                self.assertEqual({k.split("/previous/")[1]: v for k, v in e["retained"].items()}, e["original"])
                self.assertEqual(len(e["stages"]), 1)
                self.assertIn(str(e["stages"][0]) + "/previous", e["result"].stderr)
                self.assertEqual(e["contents"], {"foreign": b"foreign"} if "conflict" in fault else {})

    def test_symlinked_script_is_replaced_without_writing_its_target(self):
        e = self.exercise(target="file-link")
        self.assertEqual(e["result"].returncode, 0, e["result"].stderr)
        self.assertEqual(e["contents"]["install.ps1"], b"new-install.ps1")

    def test_symlinked_bundle_directory_is_not_replaced(self):
        e = self.exercise(target="directory-link")
        self.assertEqual(e["result"].returncode, 2, e["result"].stderr)
        self.assertEqual(e["contents"], e["original"])
        self.assertEqual(e["stages"], [])

    def test_success_publishes_scripts_and_both_sidecars_preserving_other_files(self):
        for existing in (False, True):
            with self.subTest(existing=existing):
                e = self.exercise(existing=existing)
                self.assertEqual(e["result"].returncode, 0, e["result"].stderr)
                for name in NAMES:
                    for suffix in ("", ".sig", ".sshsig"):
                        self.assertEqual(e["contents"][name + suffix], ("new-" + name + suffix).encode())
                        self.assertEqual(e["modes"][name + suffix], 0o755 if not suffix else 0o644)
                if existing:
                    self.assertEqual(e["contents"]["custom-helper"], b"old-custom-helper")
                    self.assertEqual(e["modes"]["custom-helper"], 0o640)
                self.assertEqual(e["stages"], [])

    def test_archive_source_and_main_consumers_complete_with_a_whole_bundle(self):
        for flow in ("archive", "source", "main-pinned", "main-update", "main-reinstall", "main-fresh", "main-source"):
            with self.subTest(flow=flow):
                e = self.exercise(flow=flow)
                self.assertEqual(e["result"].returncode, 0, e["result"].stdout + e["result"].stderr)
                self.assertIn("completion", e["events"])
                for name in NAMES:
                    for suffix in ("", ".sig", ".sshsig"):
                        self.assertEqual(e["contents"][name + suffix], ("new-" + name + suffix).encode())
                self.assertEqual(e["stages"], [])

    def test_unsigned_source_scripts_do_not_inherit_stale_release_sidecars(self):
        e = self.exercise(sidecars=False)
        self.assertEqual(e["result"].returncode, 0, e["result"].stderr)
        for name in NAMES:
            self.assertEqual(e["contents"][name], ("new-" + name).encode())
            self.assertNotIn(name + ".sig", e["contents"])
            self.assertNotIn(name + ".sshsig", e["contents"])

    def test_archive_and_source_consumers_fail_and_only_recover_prior_active_service(self):
        for flow in ("archive", "source"):
            for fault in ("copy", "mode", "owner", "sidecar-copy", "sidecar-mode", "sidecar-owner"):
                for prior in (False, True):
                    with self.subTest(flow=flow, fault=fault, prior=prior):
                        e = self.exercise(fault, flow, prior)
                        self.assertNotEqual(e["result"].returncode, 0, e["result"].stdout + e["result"].stderr)
                        self.assertEqual(e["contents"], e["original"])
                        self.assertEqual(e["restarted"], prior)
                        self.assertNotIn("completion", e["events"])
                        self.assertNotIn("Version verified:", e["result"].stdout)
                        self.assertNotIn("Successfully built and installed", e["result"].stdout)

    def test_uncertain_bundle_restore_does_not_restart_the_server(self):
        for flow in ("archive", "source"):
            for fault in ("rollback", "conflict", "reserve-conflict"):
                with self.subTest(flow=flow, fault=fault):
                    e = self.exercise(fault, flow)
                    self.assertNotEqual(e["result"].returncode, 0)
                    self.assertFalse(e["restarted"])
                    self.assertIn("Pulse has not been restarted", e["result"].stderr)
                    self.assertNotIn("completion", e["events"])
        for flow in ("archive", "source"):
            with self.subTest(flow=flow, fault="conflict", existing=False):
                e = self.exercise("conflict", flow, existing=False)
                self.assertNotEqual(e["result"].returncode, 0)
                self.assertFalse(e["restarted"])
                self.assertEqual(e["contents"], {"foreign": b"foreign"})
                self.assertIn("Pulse has not been restarted", e["result"].stderr)

    def test_every_main_consumer_stops_before_markers_or_completion(self):
        for flow in ("main-pinned", "main-update", "main-reinstall", "main-fresh", "main-source"):
            with self.subTest(flow=flow):
                e = self.exercise("copy", flow)
                self.assertNotEqual(e["result"].returncode, 0, e["result"].stdout + e["result"].stderr)
                self.assertEqual(e["contents"], e["original"])
                self.assertNotIn("marker", e["events"])
                self.assertNotIn("completion", e["events"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
