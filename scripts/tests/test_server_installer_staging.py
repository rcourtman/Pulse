#!/usr/bin/env python3
"""Execute installer boundary policy with confined UID/ownership doubles.

Filesystem creation, modes, symlinks and retained payloads are real. Only stat's
UID/device facts and chown are modelled: this is not a second-UID/root or native
systemd proof. The ordinary replacement tests exercise transport/stop/cleanup.
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
INSTALL_DIR="$FIXTURE/ancestor/install"
print_error() { printf '%s\n' "$*" >&2; }
print_warn() { printf '%s\n' "$*" >&2; }
stat() {
    local path="${@: -1}" owner=0 mode=""
    if [[ "$path" == "$INSTALL_DIR" && ! -f "$FIXTURE/owned" ]]; then owner=1000; fi
    if [[ "$path" == "$FIXTURE/ancestor" ]]; then
        [[ "$FAULT" != ancestor_owner ]] || owner=1000
        [[ "$FAULT" != sticky_service_owner ]] || owner=1000
    fi
    case "$2" in
        '%u') printf '%s\n' "$owner" ;;
        '%u %g %a') printf '%s 1000 %s\n' "$owner" "$(command stat -c '%a' -- "$path")" ;;
        '%u %a')
            [[ "$FAULT" != stat_failure ]] || return 1
            mode=$(command stat -c '%a' -- "$path") || return 1
            [[ "$FAULT" != ancestor_mode || "$path" != "$FIXTURE/ancestor" ]] || mode=777
            [[ "$FAULT" != sticky_service_owner || "$path" != "$FIXTURE/ancestor" ]] || mode=1777
            printf '%s %s\n' "$owner" "$mode" ;;
        *) command stat "$@" ;;
    esac
}
chown() {
    printf '%s\n' "$*" >> "$FIXTURE/chowns"
    [[ "$FAULT" != chown_failure ]] || return 1
    if [[ "$1" == -h && "$2" == root:pulse ]]; then
        if [[ "$FAULT" == swap_symlink ]]; then
            mv "$INSTALL_DIR" "$FIXTURE/renamed-install"
            ln -s "$FIXTURE/outside" "$INSTALL_DIR"
        fi
        [[ "$FAULT" == owner_readback ]] || touch "$FIXTURE/owned"
    elif [[ "$1" == -h && "$2" == 1000:1000 && -f "$FIXTURE/owned" ]]; then
        command rm -- "$FIXTURE/owned"
    fi
}
chmod() {
    [[ "$FAULT" != chmod_failure ]] || return 1
    [[ "$FAULT" != chmod_noop ]] || return 0
    command chmod "$@"
}
# Treat any candidate age sweep as old enough, without touching real clocks.
# The repaired installer must never use it to delete another run's payload.
find() { printf '%s\n' "$1"; }
case "$FAULT" in
    relative) INSTALL_DIR=relative/install ;;
    root) INSTALL_DIR=/ ;;
    dotdot) INSTALL_DIR="$FIXTURE/ancestor/../install" ;;
    double_slash) INSTALL_DIR="$FIXTURE//ancestor/install" ;;
esac
cd "$FIXTURE" || exit 97
parent=$(prepare_server_staging_parent) || exit 41
printf '%s\n' "$parent" > "$FIXTURE/parent"
if [[ "$FLOW" == ownership ]]; then
    mkdir "$INSTALL_DIR/bin" "$INSTALL_DIR/.local"
    archive=$(create_temp_archive_path "$parent/.pulse-v6.6.0-linux-amd64")
    printf verified > "$archive"
    printf signature > "${archive}.sshsig"
    stage=$(mktemp -d "$parent/.pulse-stage-XXXXXX")
    printf executable > "$stage/pulse"
    chown_install_dir_for_pulse || exit 42
fi
'''


class ServerInstallerStagingTest(unittest.TestCase):
    def exercise(self, fault="", flow="prepare"):
        with tempfile.TemporaryDirectory() as temporary:
            fixture = Path(temporary)
            (fixture / "ancestor/install/.pulse-extract-AbC123").mkdir(parents=True)
            (fixture / "ancestor/install/.pulse-extract-AbC123/live-payload").write_text("keep")
            (fixture / "outside").mkdir()
            (fixture / "outside/keep").write_text("untouched")
            original_mode = 0o700 if fault == "private_install" else 0o755
            (fixture / "ancestor/install").chmod(original_mode)
            if fault == "final_symlink":
                install = fixture / "ancestor/install"
                install.rename(fixture / "old-install")
                install.symlink_to(fixture / "outside", target_is_directory=True)
            if fault == "ancestor_symlink":
                (fixture / "ancestor").rename(fixture / "old-ancestor")
                (fixture / "ancestor").symlink_to(fixture / "old-ancestor", target_is_directory=True)
            env = {**os.environ, "FIXTURE": str(fixture), "INSTALLER": str(INSTALLER),
                   "FAULT": fault, "FLOW": flow}
            result = subprocess.run(["bash", "-c", HARNESS], env=env, text=True,
                                    capture_output=True, timeout=10)
            install = fixture / "ancestor/install"
            observation = {
                "chowns": (fixture / "chowns").read_text().splitlines() if (fixture / "chowns").exists() else [],
                "parent": (fixture / "parent").read_text().strip() if (fixture / "parent").exists() else None,
                "install": str(install),
                "mode": stat.S_IMODE(install.stat().st_mode) if install.exists() else None,
                "payload": (install / ".pulse-extract-AbC123/live-payload").exists(),
                "outside": (fixture / "outside/keep").read_text(),
                "model_root_owned": (fixture / "owned").exists(),
                "original_mode": original_mode,
            }
            return result, observation

    def test_root_sticky_parent_preserves_group_home_writes_and_live_payload(self):
        result, observed = self.exercise(flow="ownership")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(observed["mode"], 0o1775)
        self.assertEqual(observed["parent"], observed["install"])
        self.assertTrue(observed["payload"], "age alone cannot establish abandoned staging")
        self.assertTrue(any(line.startswith("-h root:pulse -- ") for line in observed["chowns"]))
        service_chowns = [line for line in observed["chowns"] if line.startswith("-R pulse:pulse ")]
        self.assertEqual(sorted(Path(line.split(" ", 2)[2]).name for line in service_chowns), [".local", "bin"])
        self.assertNotIn("pulse:pulse " + observed["install"], observed["chowns"])

    def test_private_install_does_not_gain_other_account_read_or_search(self):
        result, observed = self.exercise("private_install", flow="ownership")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(observed["mode"], 0o1770)
        self.assertTrue(observed["payload"])

    def test_unsafe_ancestry_is_rejected_before_any_ownership_change(self):
        for fault in ("ancestor_owner", "ancestor_mode", "sticky_service_owner",
                      "stat_failure", "ancestor_symlink", "final_symlink",
                      "relative", "root", "dotdot", "double_slash"):
            with self.subTest(fault=fault):
                result, observed = self.exercise(fault)
                self.assertEqual(result.returncode, 41, result.stderr)
                self.assertIsNone(observed["parent"])
                self.assertEqual(observed["chowns"], [])
                self.assertEqual(observed["outside"], "untouched")

    def test_failed_or_substituted_parent_is_not_returned_for_staging(self):
        for fault in ("chown_failure", "chmod_failure", "chmod_noop", "owner_readback", "swap_symlink"):
            with self.subTest(fault=fault):
                result, observed = self.exercise(fault)
                self.assertEqual(result.returncode, 41, result.stderr)
                self.assertIsNone(observed["parent"])
                self.assertEqual(observed["outside"], "untouched")
                if fault != "swap_symlink":
                    self.assertEqual(observed["mode"], observed["original_mode"])
                    self.assertFalse(observed["model_root_owned"])


if __name__ == "__main__":
    unittest.main()
