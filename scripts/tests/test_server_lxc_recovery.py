#!/usr/bin/env python3
"""Run the real new-LXC installer through confined failure/recovery boundaries.

Every Proxmox, package and service operation is an inert ordinary-user fixture.
The dependency shell itself runs, with /etc redirected to a fixture directory.
This is not a native guest experiment or signed installed-recovery acceptance.
"""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh"))

HARNESS = r'''
source "$INSTALLER"
print_header() { :; }
print_info() { printf '%s\n' "$*"; }
print_success() { printf '%s\n' "$*"; }
print_warn() { printf '%s\n' "$*" >&2; }
print_error() { printf '%s\n' "$*" >&2; }
sleep() { :; }
timeout() { printf 'timeout %s\n' "$1" >> "$FIXTURE/actions"; shift; "$@"; }
safe_read_with_default() { printf -v "$2" '%s' "$3"; }
detect_network_bridges() { echo vmbr0; }
is_bridge_interface() { return 0; }
ip() { printf 'default via 198.51.100.1 dev vmbr0\n'; }
pvesh() {
    [[ "$*" == 'get /cluster/nextid' ]] || return 97
    echo 600
}
qm() { printf 'UNEXPECTED qm\n' >> "$FIXTURE/actions"; return 97; }
pvesm() { printf 'Name Type Status Total Used Available\nlocal dir active 99999999 1 99999998\n'; }
pveam() {
    [[ "$1" == list ]] || return 97
    printf 'NAME SIZE\nlocal:vztmpl/fixture.tar.zst 1\n'
}
ensure_debian_template() { DEBIAN_TEMPLATE=fixture.tar.zst; }
prefetch_pulse_archive_for_container() {
    local staged
    staged=$(create_temp_archive_path "$FIXTURE/shared/pulse-v6.6.0-linux-amd64") || return 1
    printf 'fixture release bytes\n' > "$staged"
    printf 'fixture signature bytes\n' > "${staged}.sshsig"
    printf -v "$1" '%s' "$staged"
}
download_container_installer() {
    local staged
    staged=$(mktemp -d "$FIXTURE/shared/bootstrap.XXXXXX") || return 1
    printf 'fixture installer bytes\n' > "$staged/install.sh"
    printf -v "$1" '%s' "$staged/install.sh"
    printf -v "$2" '%s' "$staged"
}
apt-get() {
    printf 'apt-get %s\n' "$*" >> "$FIXTURE/actions"
    if [[ "$FAULT" == apt_update && "$1" == update ]]; then
        printf 'fixture apt index failed\n' >&2
        return 31
    fi
    if [[ "$FAULT" == apt_install && "$1" == install ]]; then
        printf 'fixture package install failed\n' >&2
        return 32
    fi
}
ln() {
    printf 'timezone\n' >> "$FIXTURE/actions"
    [[ "$FAULT" != optional_settings ]]
}
sysctl() {
    printf 'sysctl\n' >> "$FIXTURE/actions"
    [[ "$FAULT" != optional_settings ]]
}
export -f apt-get ln sysctl
auto_register_pve_node() {
    printf 'register\n' >> "$FIXTURE/actions"
    [[ "$FAULT" != registration ]] || return 49
}
wait_for_pulse_ready() {
    printf 'ready\n' >> "$FIXTURE/actions"
    [[ "$FAULT" != readiness ]] || return 50
}
pct() {
    [[ "$2" == 600 ]] || { printf 'UNEXPECTED target\n' >> "$FIXTURE/actions"; return 97; }
    printf 'pct %s %s\n' "$1" "$2" >> "$FIXTURE/actions"
    case "$1" in
        create)
            [[ "$FAULT" != create ]] || return 40
            [[ "$FAULT" != interrupt_before_create ]] || kill -TERM "$$"
            mkdir "$FIXTURE/container"
            cp "$FIXTURE/original-state" "$FIXTURE/container/state"
            cp "$FIXTURE/original-key" "$FIXTURE/container/key"
            ;;
        start)
            [[ "$FAULT" != start ]] || return 41
            [[ "$FAULT" != interrupt_after_create ]] || kill -TERM "$$"
            ;;
        stop) printf 'STOPPED\n' >> "$FIXTURE/actions" ;;
        destroy)
            # Even an old-source negative control can only move fixture files.
            mv "$FIXTURE/container" "$FIXTURE/removed-container"
            ;;
        push)
            [[ "$FAULT" != interrupt_during_copy ]] || kill -TERM "$$"
            [[ "$FAULT" != script_copy || "$4" != /tmp/install.sh ]] || return 42
            [[ "$FAULT" != archive_copy || "$4" == /tmp/install.sh ]] || return 43
            cp "$3" "$FIXTURE/container/${4##*/}"
            ;;
        exec)
            if [[ "$*" == *'hostname -I'* ]]; then
                [[ "$FAULT" == network ]] || echo 198.51.100.2
            elif [[ "${@: -1}" == *'apt-get update'* ]]; then
                # Execute the actual production dependency shell, not a fake
                # return for the entire stage. No command can touch real /etc.
                local -a args=("${@:4}")
                local dependency_command="${@: -1}"
                dependency_command="${dependency_command//\/etc\//$FIXTURE/guest-etc/}"
                args[${#args[@]}-1]="$dependency_command"
                "${args[@]}"
            elif [[ "${@: -1}" == *'bash /tmp/install.sh'* ]]; then
                printf 'INSTALL\n' >> "$FIXTURE/actions"
                [[ "$FAULT" != installer ]] || return 44
                [[ "$FAULT" != installer_timeout ]] || return 124
                [[ "$FAULT" != interrupt_during_install ]] || kill -INT "$$"
            else
                printf 'UNEXPECTED exec\n' >> "$FIXTURE/actions"
                return 97
            fi
            ;;
        *) printf 'UNEXPECTED pct\n' >> "$FIXTURE/actions"; return 97 ;;
    esac
}
if [[ "$LOCAL_ARCHIVE" == true ]]; then
    ARCHIVE_OVERRIDE="$FIXTURE/pulse-v6.6.0-linux-amd64.tar.gz"
fi
FORCE_VERSION=v6.6.0
create_lxc_container
'''


class LXCRecovery(unittest.TestCase):
    def run_install(self, fault="none", *, no_timeout=False, local_archive=False):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        (root / "shared").mkdir()
        (root / "guest-etc").mkdir()
        for name in ("profile", "bash.bashrc", "sysctl.conf"):
            (root / "guest-etc" / name).write_text("")
        (root / "original-state").write_bytes(b"saved fixture state\n")
        (root / "original-key").write_bytes(b"synthetic fixture key\n")
        (root / "original-key").chmod(0o600)
        archive = root / "pulse-v6.6.0-linux-amd64.tar.gz"
        archive.write_bytes(b"caller-owned archive\n")
        Path(str(archive) + ".sshsig").write_bytes(b"caller-owned signature\n")
        env = dict(os.environ, INSTALLER=str(INSTALLER), FIXTURE=str(root),
                   FAULT=fault, LOCAL_ARCHIVE=str(local_archive).lower())
        env["PULSE_CONTAINER_TIMEOUT"] = "0" if no_timeout else "300"
        result = subprocess.run(["bash", "-c", HARNESS], env=env,
                                capture_output=True, text=True, timeout=10,
                                start_new_session=True)
        actions = (root / "actions").read_text()
        self.assertNotIn("UNEXPECTED", actions, result.stdout + result.stderr)
        self.assertNotIn("pct stop", actions)
        self.assertNotIn("pct destroy", actions)
        self.assertEqual(list((root / "shared").iterdir()), [], "owned scratch leaked")
        self.assertEqual(archive.read_bytes(), b"caller-owned archive\n")
        self.assertEqual(Path(str(archive) + ".sshsig").read_bytes(),
                         b"caller-owned signature\n")
        if (root / "container").exists():
            self.assertEqual((root / "container/state").read_bytes(),
                             (root / "original-state").read_bytes())
            self.assertEqual((root / "container/key").read_bytes(),
                             (root / "original-key").read_bytes())
            self.assertEqual((root / "container/key").stat().st_mode & 0o777, 0o600)
        return root, result, actions

    def assert_retained_failure(self, root, result, code):
        output = result.stdout + result.stderr
        self.assertEqual(result.returncode, code, output)
        self.assertTrue((root / "container").is_dir(), output)
        self.assertEqual(output.count("Container 600 has been retained"), 1, output)
        self.assertIn("pct status 600", output)
        self.assertIn("pct enter 600", output)
        self.assertIn("Do not rerun the host installer", output)
        self.assertIn("an interrupted command may still be running", output)
        self.assertNotIn("Pulse installation complete!", output)

    def test_required_dependency_failures_stop_before_install(self):
        for fault, code, diagnostic in (
                ("apt_update", 31, "fixture apt index failed"),
                ("apt_install", 32, "fixture package install failed")):
            with self.subTest(fault=fault):
                root, result, actions = self.run_install(fault)
                self.assert_retained_failure(root, result, code)
                self.assertIn(diagnostic, result.stderr)
                self.assertIn(f"Failed to install dependencies in container (exit {code})",
                              result.stderr)
                self.assertNotIn("timezone", actions)
                self.assertNotIn("INSTALL\n", actions)
                if fault == "apt_update":
                    self.assertNotIn("apt-get install", actions)

    def test_start_network_and_copy_failures_keep_recovery_state(self):
        for fault, diagnostic in (
                ("start", "Failed to start container"),
                ("network", "Container network failed to obtain an IP address"),
                ("script_copy", "Failed to copy install script"),
                ("archive_copy", "Failed to copy Pulse release archive")):
            with self.subTest(fault=fault):
                root, result, actions = self.run_install(fault, local_archive=True)
                self.assert_retained_failure(root, result, 1)
                self.assertIn(diagnostic, result.stderr)
                self.assertNotIn("INSTALL\n", actions)

    def test_inner_installer_exit_is_reported_with_and_without_timeout(self):
        for no_timeout in (False, True):
            with self.subTest(no_timeout=no_timeout):
                root, result, actions = self.run_install("installer", no_timeout=no_timeout)
                self.assert_retained_failure(root, result, 44)
                self.assertIn("Failed to install Pulse inside container", result.stderr)
                self.assertIn("INSTALL\n", actions)
                self.assertNotIn("register", actions)
                self.assertEqual("timeout 300\n" in actions, not no_timeout)

    def test_timeout_retains_copied_inputs_and_requires_state_reconciliation(self):
        root, result, actions = self.run_install("installer_timeout")
        self.assert_retained_failure(root, result, 124)
        output = result.stdout + result.stderr
        self.assertIn("Installation timed out after 300s", output)
        self.assertIn("Only after confirming no installer is still running", output)
        self.assertNotIn("This usually happens", output)
        self.assertEqual((root / "container/install.sh").read_bytes(), b"fixture installer bytes\n")
        self.assertEqual((root / "container/pulse-v6.6.0-linux-amd64.tar.gz").read_bytes(),
                         b"fixture release bytes\n")
        self.assertEqual((root / "container/pulse-v6.6.0-linux-amd64.tar.gz.sshsig").read_bytes(),
                         b"fixture signature bytes\n")
        self.assertNotIn("register", actions)

    def test_interruptions_never_destroy_the_partially_installed_container(self):
        for fault, code in (("interrupt_after_create", 143),
                            ("interrupt_during_copy", 143),
                            ("interrupt_during_install", 130)):
            with self.subTest(fault=fault):
                root, result, _ = self.run_install(fault)
                self.assert_retained_failure(root, result, code)
                self.assertIn("Installation interrupted", result.stderr)

    def test_no_owned_container_is_claimed_before_create_succeeds(self):
        for fault, code in (("create", 1), ("interrupt_before_create", 143)):
            with self.subTest(fault=fault):
                root, result, _ = self.run_install(fault)
                self.assertEqual(result.returncode, code, result.stdout + result.stderr)
                self.assertFalse((root / "container").exists())
                self.assertNotIn("has been retained", result.stdout + result.stderr)
                self.assertNotIn("Pulse installation complete!", result.stdout)

    def test_late_unexpected_failures_keep_state_and_original_exit(self):
        for fault, code in (("registration", 49), ("readiness", 50)):
            with self.subTest(fault=fault):
                root, result, actions = self.run_install(fault)
                self.assert_retained_failure(root, result, code)
                self.assertIn("INSTALL\n", actions)

    def test_success_and_optional_settings_failures_still_complete(self):
        for fault, local_archive in (("none", False), ("optional_settings", False),
                                     ("none", True)):
            with self.subTest(fault=fault, local_archive=local_archive):
                root, result, actions = self.run_install(fault, local_archive=local_archive)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertTrue((root / "container").is_dir())
                self.assertIn("Pulse installation complete!", result.stdout)
                self.assertNotIn("has been retained", result.stdout + result.stderr)
                self.assertIn("apt-get update -qq\napt-get install -y -qq curl wget ca-certificates\n",
                              actions)
                self.assertIn("ready\n", actions)

    def test_exit_handler_does_not_replace_the_primary_failure(self):
        for primary, expected in ((44, 44), (0, 1)):
            with self.subTest(primary=primary):
                env = dict(os.environ, INSTALLER=str(INSTALLER))
                script = r'''
source "$INSTALLER"
cleanup_container_install_inputs() { return 57; }
INSTALL_CONTAINER_CREATED=false
handle_container_install_exit "$1"
'''
                result = subprocess.run(["bash", "-c", script, "bash", str(primary)],
                                        env=env, capture_output=True, text=True,
                                        timeout=5, start_new_session=True)
                self.assertEqual(result.returncode, expected, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
