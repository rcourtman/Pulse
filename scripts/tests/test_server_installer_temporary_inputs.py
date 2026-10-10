#!/usr/bin/env python3
"""Actual installer staging functions, confined to ordinary-user fixtures.

Transport and service boundaries are doubles; signature verification is real.
Directory modes establish the staging boundary, not a native root/LXC proof.
"""

import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh"))

HARNESS = r'''
source "$INSTALLER"
print_info() { :; }
print_success() { :; }
print_warn() { printf '%s\n' "$*" >&2; }
print_error() { printf '%s\n' "$*" >&2; }
mktemp() {
    [[ "$FAULT" != allocation ]] || return 1
    local template="${@: -1}" created
    local -a args=("$@")
    args[${#args[@]}-1]="$FIXTURE/shared/${template##*/}"
    created=$(command mktemp "${args[@]}") || return 1
    printf '%s\n' "$created" >> "$FIXTURE/created"
    if [[ -d "$created" ]]; then
        stat -c '%a' "$created" >> "$FIXTURE/directory-modes"
    fi
    printf '%s\n' "$created"
}
timeout() { shift; "$@"; }
sleep() { :; }
curl() {
    local url="${@: -1}"
    printf '%s\n' "$url" >> "$FIXTURE/transport"
    case "$url" in
        https://github.com/rcourtman/Pulse/releases/download/v6.6.0/install.sh)
            cat "$FIXTURE/installer-input" || return 1
            [[ "$FAULT" != script_interrupt ]] || kill -TERM "$$"
            [[ "$FAULT" != script_transport ]] ;;
        https://github.com/rcourtman/Pulse/releases/download/v6.6.0/install.sh.sshsig)
            cat "$FIXTURE/installer-input.sig" || return 1
            [[ "$FAULT" != signature_transport ]] ;;
        *) return 97 ;;
    esac
}
resolve_install_script_download_url() {
    [[ "$FAULT" != metadata ]] || return 1
    echo https://github.com/rcourtman/Pulse/releases/download/v6.6.0/install.sh
}
PINNED_RELEASE_SSH_PUBLIC_KEY=$(cat "$FIXTURE/signing-key.pub")
if [[ "$FAULT" == verifier ]]; then
    command() {
        [[ "$*" != '-v ssh-keygen' ]] || return 1
        builtin command "$@"
    }
fi
INSTALL_DIR="$FIXTURE/install"
BUILD_FROM_SOURCE_MARKER="$INSTALL_DIR/BUILD_FROM_SOURCE"
CURRENT_VERSION=v6.5.0
LATEST_RELEASE=v6.6.0
detect_pulse_architecture() { echo amd64; }
ensure_update_disk_headroom() { return 0; }
resolve_target_release() { :; }
download_release_archive() {
    printf '%s\n' "$3" > "$FIXTURE/archive-path"
    printf 'archive fixture' > "$3"
    printf 'signature fixture' > "${3}.sshsig"
    [[ "$FAULT" != archive_transport ]]
}
run_upgrade_readiness_preflight() { [[ "$FAULT" != readiness ]]; }
install_pulse_archive() {
    echo INSTALL >> "$FIXTURE/calls"
    [[ "$FAULT" != archive_admission ]]
}
# Exercise the complete production bootstrap too; no pct, package, service,
# registration, bridge or guest command below can reach a real host/guest.
print_header() { :; }
cleanup_stale_sensor_proxy_mounts() { :; }
safe_read_with_default() { printf -v "$2" '%s' "$3"; }
detect_network_bridges() { echo vmbr0; }
ip() { printf 'default via 198.51.100.1 dev vmbr0\n'; }
ensure_debian_template() { DEBIAN_TEMPLATE=fixture.tar.zst; }
pvesh() { echo 600; }
qm() { return 1; }
pvesm() { printf 'Name Type Status Total Used Available\nlocal dir active 99999999 1 99999998\n'; }
pveam() { printf 'NAME SIZE\nlocal:vztmpl/fixture.tar.zst 1\n'; }
auto_register_pve_node() { :; }
wait_for_pulse_ready() { :; }
pct() {
    case "$1" in
        create|start|stop|destroy) return 0 ;;
        push)
            printf 'PUSH=%s\n' "$4" >> "$FIXTURE/calls"
            [[ -f "$3" ]] || return 1
            [[ "$FAULT" != script_copy || "$4" != /tmp/install.sh ]] || return 1
            if [[ "$4" == /tmp/install.sh ]]; then
                cp "$3" "$FIXTURE/copied-script"
            fi
            return 0 ;;
        exec)
            if [[ "$*" == *'hostname -I'* ]]; then
                echo 198.51.100.2
            elif [[ "$*" == *'bash /tmp/install.sh'* ]]; then
                echo EXECUTE >> "$FIXTURE/calls"
            fi
            return 0 ;;
        *) return 97 ;;
    esac
}
umask "$MASK"
case "$FLOW" in
    reserve)
        path=$(create_temp_archive_path "$FIXTURE/shared/pulse-v6.6.0-linux-amd64")
        printf '%s\n' "$path" > "$FIXTURE/archive-path"
        infer_release_from_archive_name "$path" > "$FIXTURE/inferred"
        ;;
    cleanup)
        path=$(create_temp_archive_path "$FIXTURE/shared/pulse-v6.6.0-linux-amd64")
        printf '%s\n' "$path" > "$FIXTURE/archive-path"
        printf archive > "$path"
        printf signature > "${path}.sshsig"
        [[ "$FAULT" != extra_file ]] || touch "$(dirname "$path")/unowned"
        cleanup_temp_archive_path "$path"
        ;;
    download) download_pulse ;;
    override)
        ARCHIVE_OVERRIDE="$FIXTURE/pulse-v6.6.0-linux-amd64.tar.gz"
        FORCE_VERSION=v6.6.0
        download_pulse
        ;;
    prefetch)
        prefetched=""
        prefetch_pulse_archive_for_container prefetched
        printf '%s\n' "$prefetched" > "$FIXTURE/returned"
        ;;
    bootstrap)
        script_source=""
        container_script_temp_dir=""
        trap 'cleanup_container_install_inputs "${container_script_temp_dir:-}" "" false' EXIT
        download_container_installer script_source container_script_temp_dir
        printf '%s\n' "$script_source" > "$FIXTURE/returned"
        cp "$script_source" "$FIXTURE/copied-script"
        ;;
    interrupted_bootstrap)
        script_source=""
        container_script_temp_dir=""
        trap 'cleanup_container_install_inputs "${container_script_temp_dir:-}" "" false' EXIT
        trap 'exit 143' TERM
        download_container_installer script_source container_script_temp_dir
        kill -TERM "$$"
        ;;
    container_cleanup)
        container_archive_source=""
        prefetch_pulse_archive_for_container container_archive_source
        script_source=""
        container_script_temp_dir=""
        trap 'cleanup_container_install_inputs "${container_script_temp_dir:-}" "$container_archive_source" true' EXIT
        download_container_installer script_source container_script_temp_dir
        exit 42
        ;;
    connected_container) create_lxc_container ;;
    local_container)
        ARCHIVE_OVERRIDE="$FIXTURE/pulse-v6.6.0-linux-amd64.tar.gz"
        create_lxc_container
        ;;
esac
'''


class ServerInstallerTemporaryInputsTest(unittest.TestCase):
    def exercise(self, flow, fault="", mask="000"):
        with tempfile.TemporaryDirectory() as temporary:
            fixture = Path(temporary)
            (fixture / "shared").mkdir(mode=0o777)
            (fixture / "shared").chmod(0o1777)
            (fixture / "install").mkdir()
            (fixture / "install/BUILD_FROM_SOURCE").write_text("old marker")
            (fixture / "installer-input").write_text("#!/bin/bash\n# inert signed fixture\n")
            key = fixture / "signing-key"
            subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(key)],
                           check=True, capture_output=True)
            subprocess.run(["ssh-keygen", "-q", "-Y", "sign", "-f", str(key),
                            "-n", "pulse-install", str(fixture / "installer-input")],
                           check=True, capture_output=True)
            if fault == "bad_signature":
                (fixture / "installer-input").write_text("unsigned changed fixture\n")
            if fault == "missing_signature":
                (fixture / "installer-input.sig").unlink()
            override = fixture / "pulse-v6.6.0-linux-amd64.tar.gz"
            override.write_bytes(b"caller archive")
            Path(str(override) + ".sshsig").write_bytes(b"caller signature")
            env = {**os.environ, "INSTALLER": str(INSTALLER), "FIXTURE": str(fixture),
                   "FLOW": flow, "FAULT": fault, "MASK": mask}
            command = ["bash", "-c", HARNESS]
            if flow == "local_container":
                command.append(str(fixture / "installer-input"))
            result = subprocess.run(command, env=env, text=True,
                                    capture_output=True, timeout=15)
            observed = {}
            for name in ("archive-path", "returned", "inferred", "calls", "transport", "directory-modes"):
                p = fixture / name
                observed[name] = p.read_text().splitlines() if p.exists() else []
            archive_path = Path(observed["archive-path"][0]) if observed["archive-path"] else None
            observed["archive_parent_is_private"] = bool(archive_path and archive_path.parent != fixture / "shared"
                and archive_path.parent.is_dir() and stat.S_IMODE(archive_path.parent.stat().st_mode) == 0o700)
            observed["shared_entries"] = sorted(p.name for p in (fixture / "shared").iterdir())
            observed["unowned"] = sorted(str(p.relative_to(fixture / "shared"))
                                         for p in (fixture / "shared").rglob("unowned"))
            observed["copied"] = (fixture / "copied-script").read_bytes() if (fixture / "copied-script").exists() else None
            observed["caller-script"] = (fixture / "installer-input").read_bytes() if (fixture / "installer-input").exists() else None
            observed["override"] = (override.read_bytes(), Path(str(override) + ".sshsig").read_bytes())
            return result, observed

    def test_archive_and_sidecar_reserve_a_private_parent(self):
        for mask in ("000", "022", "077"):
            with self.subTest(umask=mask):
                r, o = self.exercise("reserve", mask=mask)
                self.assertEqual(r.returncode, 0, r.stderr)
                self.assertTrue(o["archive_parent_is_private"], o)
                self.assertEqual(o["inferred"], ["v6.6.0"])

    def test_allocation_failure_cannot_return_an_unreserved_path(self):
        r, o = self.exercise("reserve", "allocation")
        self.assertNotEqual(r.returncode, 0)
        self.assertEqual(o["archive-path"], [])
        self.assertEqual(o["shared_entries"], [])
        self.assertEqual(o["directory-modes"], [])

    def test_cleanup_removes_archive_sidecar_and_owned_empty_directory(self):
        r, o = self.exercise("cleanup")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(o["shared_entries"], [])

    def test_bootstrap_staging_is_private_even_with_a_permissive_umask(self):
        for mask in ("000", "022", "077"):
            with self.subTest(umask=mask):
                r, o = self.exercise("bootstrap", mask=mask)
                self.assertEqual(r.returncode, 0, r.stderr)
                self.assertEqual(o["directory-modes"], ["700"])

    def test_cleanup_never_recursively_erases_an_unexpected_file(self):
        r, o = self.exercise("cleanup", "extra_file")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(len(o["unowned"]), 1, o)

    def test_download_success_and_each_failure_clean_owned_inputs(self):
        for fault in ("", "archive_transport", "readiness", "archive_admission", "allocation"):
            with self.subTest(fault=fault):
                r, o = self.exercise("download", fault)
                self.assertEqual(r.returncode, 1 if fault else 0, r.stderr)
                self.assertEqual(o["shared_entries"], [], o)
                self.assertEqual(o["calls"], ["INSTALL"] if fault in ("", "archive_admission") else [])

    def test_explicit_archive_and_sidecar_are_never_owned_or_deleted(self):
        for fault in ("", "readiness", "archive_admission"):
            with self.subTest(fault=fault):
                r, o = self.exercise("override", fault)
                self.assertEqual(r.returncode, 1 if fault else 0, r.stderr)
                self.assertEqual(o["override"], (b"caller archive", b"caller signature"))
                self.assertEqual(o["shared_entries"], [])

    def test_prefetch_retains_private_archive_until_handoff(self):
        r, o = self.exercise("prefetch")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertTrue(o["archive_parent_is_private"], o)
        self.assertEqual(o["returned"], o["archive-path"])

    def test_prefetch_failure_releases_its_private_inputs(self):
        r, o = self.exercise("prefetch", "archive_transport")
        self.assertNotEqual(r.returncode, 0)
        self.assertEqual(o["shared_entries"], [])

    def test_bootstrap_checks_real_signature_before_copy(self):
        r, o = self.exercise("bootstrap")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(o["copied"], b"#!/bin/bash\n# inert signed fixture\n")
        self.assertEqual(o["caller-script"], b"#!/bin/bash\n# inert signed fixture\n")
        self.assertEqual(o["transport"], [
            "https://github.com/rcourtman/Pulse/releases/download/v6.6.0/install.sh",
            "https://github.com/rcourtman/Pulse/releases/download/v6.6.0/install.sh.sshsig"])
        self.assertEqual(o["shared_entries"], [])

    def test_bootstrap_fails_closed_and_cleans_every_owned_input(self):
        for fault in ("metadata", "verifier", "allocation", "script_transport",
                      "signature_transport", "missing_signature", "bad_signature"):
            with self.subTest(fault=fault):
                r, o = self.exercise("bootstrap", fault)
                self.assertNotEqual(r.returncode, 0, r.stdout + r.stderr)
                self.assertIsNone(o["copied"])
                self.assertEqual(o["shared_entries"], [], o)
                if fault in ("metadata", "verifier", "allocation"):
                    self.assertEqual(o["transport"], [])

    def test_interrupted_bootstrap_cleans_inputs_without_a_copy(self):
        r, o = self.exercise("interrupted_bootstrap")
        self.assertEqual(r.returncode, 143, r.stderr)
        self.assertIsNone(o["copied"])
        self.assertEqual(o["shared_entries"], [])

    def test_container_failure_cleans_both_owned_staging_directories(self):
        r, o = self.exercise("container_cleanup")
        self.assertEqual(r.returncode, 42, r.stderr)
        self.assertEqual(o["shared_entries"], [], o)

    def test_production_container_bootstrap_copies_only_the_verified_installer(self):
        r, o = self.exercise("connected_container")
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertEqual(o["copied"], b"#!/bin/bash\n# inert signed fixture\n")
        self.assertIn("EXECUTE", o["calls"])
        self.assertEqual(o["shared_entries"], [])

    def test_production_container_never_copies_invalid_or_unavailable_signed_input(self):
        for fault in ("bad_signature", "script_transport", "signature_transport", "missing_signature", "script_interrupt"):
            with self.subTest(fault=fault):
                r, o = self.exercise("connected_container", fault)
                self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
                self.assertIsNone(o["copied"])
                self.assertNotIn("EXECUTE", o["calls"])
                self.assertEqual(o["shared_entries"], [])

    def test_production_container_failed_script_copy_cleans_prefetched_inputs_too(self):
        r, o = self.exercise("connected_container", "script_copy")
        self.assertEqual(r.returncode, 1, r.stdout + r.stderr)
        self.assertIsNone(o["copied"])
        self.assertNotIn("EXECUTE", o["calls"])
        self.assertEqual(o["shared_entries"], [])

    def test_production_local_installer_and_archive_remain_caller_owned(self):
        r, o = self.exercise("local_container")
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertEqual(o["transport"], [])
        self.assertEqual(o["override"], (b"caller archive", b"caller signature"))
        self.assertEqual(o["copied"], b"#!/bin/bash\n# inert signed fixture\n")
        self.assertEqual(o["caller-script"], b"#!/bin/bash\n# inert signed fixture\n")
        self.assertEqual(o["shared_entries"], [])


if __name__ == "__main__":
    unittest.main()
