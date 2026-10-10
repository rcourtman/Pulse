#!/usr/bin/env python3
"""Current-source updater transactions on a disposable GitHub-hosted systemd VM.

The two real, real-embed binaries differ only in synthetic version ldflags.
Installers and archives use an ephemeral test signer, never a release key.
Only download transport and /tmp allocation are redirected to private fixtures;
production signature, replacement, service observation and recovery functions run.
This is not a published-release/LXC, timer-sandbox or browser acceptance test.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import tarfile
import time
import urllib.error
import urllib.request

UNIT = "pulse-source-updater"
TAGS = ("v0.0.1", "v0.0.2")
PORT = 17655
PARSE_MARKER = "# Parse command line arguments\n"


def digest(path):
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def installer_definitions(source):
    if source.count(PARSE_MARKER) != 1:
        raise ValueError("installer definition boundary changed")
    definitions, entry = source.split(PARSE_MARKER)
    if not entry.startswith("while [[ $# -gt 0 ]]; do\n"):
        raise ValueError("installer entry boundary changed")
    return definitions


def hosted_admission(env, uid, source):
    if (uid != 0 or env.get("GITHUB_ACTIONS") != "true"
            or env.get("RUNNER_ENVIRONMENT") != "github-hosted"
            or env.get("GITHUB_REPOSITORY") != "rcourtman/Pulse"
            or not re.fullmatch(r"[0-9a-f]{40}", source)
            or any(Path(p).exists() for p in ("/.dockerenv", "/opt/pulse", "/etc/pulse"))):
        raise ValueError("native updater proof requires a fresh public hosted VM")


# A closed adapter: missing fixtures fail, never fall back to online curl.
TRANSPORT = r'''#!/usr/bin/python3
import os, pathlib, shutil, subprocess, sys
root = pathlib.Path(os.environ["PULSE_NATIVE_ROOT"])
args = sys.argv[1:]
if pathlib.Path(sys.argv[0]).name == "mktemp":
    if args and args[-1].startswith("/tmp/pulse-"):
        args[-1] = str(root / "tmp" / pathlib.Path(args[-1]).name)
    os.execv("/usr/bin/mktemp", ["mktemp", *args])
out = None
url = None
while args:
    arg = args.pop(0)
    if arg == "-o" and args:
        out = pathlib.Path(args.pop(0))
    elif arg == "-fsSL":
        pass
    elif arg.startswith("https://") and url is None:
        url = arg
    else:
        sys.exit(97)
allowed = {"https://github.com/rcourtman/Pulse/releases/latest/download/install.sh": "installer"}
for tag in ("v0.0.1", "v0.0.2"):
    allowed["https://github.com/rcourtman/Pulse/releases/download/" + tag + "/install.sh"] = "installer"
allowed.update({url + ".sshsig": name + ".sshsig" for url, name in list(allowed.items())})
name = allowed.get(url)
if name is None:
    sys.exit(97)
file = root / name
if out is None:
    sys.stdout.buffer.write(file.read_bytes())
else:
    shutil.copyfile(file, out)
'''

INSTALLER = r'''#!/bin/bash
set -euo pipefail
[[ $# == 2 && "$1" == --version ]]
case "$2" in v0.0.1|v0.0.2) ;; *) exit 97 ;; esac
source "$PULSE_NATIVE_ROOT/installer-definitions"
PINNED_RELEASE_SSH_PUBLIC_KEY=$(cat "$PULSE_NATIVE_ROOT/signer/key.pub")
printf 'attempted\n' >> "$PULSE_NATIVE_ROOT/installer-calls"
install_pulse_archive "$PULSE_NATIVE_ROOT/archives/pulse-$2-linux-amd64.tar.gz" "$2"
# Fail after real signed replacement, not before the interesting transaction.
case "${PULSE_NATIVE_FAULT:-}" in installer|restore) exit 23 ;; esac
if [[ "$PULSE_WAS_ACTIVE" == true ]]; then
    systemctl start "$SERVICE_NAME"
fi
'''

UPDATER = r'''
set -euo pipefail
source "$PULSE_NATIVE_UPDATER"
PINNED_RELEASE_SSH_PUBLIC_KEY=$(cat "$PULSE_NATIVE_ROOT/signer/key.pub")
# One explicit restore-copy failure tests retained backup/no unsafe restart.
if [[ "${PULSE_NATIVE_FAULT:-}" == restore ]]; then
    cp() {
        if [[ "${*: -1}" == *.rollback.* ]]; then return 71; fi
        command cp "$@"
    }
fi
status=0
perform_update "$PULSE_NATIVE_TARGET" || status=$?
# Exercise a subsequent function return: the transaction's trap must be gone.
is_prerelease_tag v0.0.3-rc.1
exit "$status"
'''


class NativeLab:
    def __init__(self, root, repo, before, after, source):
        self.root, self.repo = Path(root), Path(repo)
        self.binaries = dict(zip(TAGS, (Path(before), Path(after))))
        self.receipt = {"schema_version": 1, "source_sha": source,
                        "scope": "current-source-synthetic-signed-systemd-transaction",
                        "phases": [], "cleanup": {"complete": False}}
        self.env = {"PATH": str(self.root / "tools") + ":/usr/sbin:/usr/bin:/sbin:/bin",
                    "LANG": "C.UTF-8", "LC_ALL": "C", "HOME": str(self.root),
                    "PULSE_NATIVE_ROOT": str(self.root),
                    "PULSE_NATIVE_UPDATER": str(self.repo / "scripts/pulse-auto-update.sh"),
                    "PULSE_SERVICE_NAME": UNIT, "PULSE_INSTALL_DIR": str(self.root / "install"),
                    "PULSE_CONFIG_DIR": str(self.root / "data"),
                    "PULSE_BINARY_LINK_PATH": str(self.root / "pulse"),
                    "PULSE_UPDATE_HELPER_PATH": str(self.root / "update"),
                    "PULSE_PROFILE_PATH": str(self.root / "profile"),
                    "PULSE_BASHRC_PATH": str(self.root / "bashrc"), "FRONTEND_PORT": str(PORT)}
        self.unit = Path("/etc/systemd/system") / (UNIT + ".service")
        self.owned_unit = False
        self.owned_root = False
        self.owned_user = False

    def run(self, *args, timeout=120, ok=(0,)):
        process = subprocess.Popen(args, env=self.env, stdin=subprocess.DEVNULL,
                                   stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                   start_new_session=True)
        try:
            output, _ = process.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.communicate(timeout=2)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.communicate(timeout=2)
            self.receipt["command_failure"] = {"command": Path(args[0]).name,
                                                "deadline_exceeded": True,
                                                "observed_exit": process.returncode}
            raise RuntimeError("native command deadline exceeded") from None
        # Private in-memory diagnostic for confined controls, never a receipt field.
        self._last_output = output
        result = subprocess.CompletedProcess(args, process.returncode, output)
        if result.returncode not in ok:
            self.receipt["command_failure"] = {"command": Path(args[0]).name,
                                                "observed_exit": result.returncode,
                                                "output_bytes": len(output),
                                                "output_sha256": hashlib.sha256(output).hexdigest()}
            raise RuntimeError(f"native command failed ({result.returncode}): {Path(args[0]).name}")
        return result

    def shell(self, script, **env):
        script = "set -euo pipefail\n" + script
        old = self.env.copy()
        try:
            self.env.update(env)
            return self.run("/bin/bash", "-c", script)
        finally:
            self.env = old

    def state(self):
        result = self.run("systemctl", "show", UNIT + ".service", "--no-pager",
                          "--property=LoadState,ActiveState,UnitFileState,MainPID")
        pairs = dict(line.split("=", 1) for line in result.stdout.decode().splitlines())
        if set(pairs) != {"LoadState", "ActiveState", "UnitFileState", "MainPID"}:
            raise RuntimeError("incomplete native manager observation")
        return pairs

    def prepare(self):
        for path in (self.unit, Path("/run/systemd/system") / self.unit.name):
            if path.exists() or path.is_symlink():
                raise ValueError("native fixture unit already exists")
        if self.state()["LoadState"] != "not-found":
            raise ValueError("native fixture unit is already known to systemd")
        self.root.mkdir(mode=0o755)  # A pulse-owned service must traverse it.
        self.owned_root = True
        for name in ("tools", "tmp", "archives", "signer"):
            (self.root / name).mkdir(mode=0o700 if name == "signer" else 0o755)
        source = (self.repo / "install.sh").read_text()
        (self.root / "installer-definitions").write_text(installer_definitions(source))
        self.receipt["sources"] = {p: digest(self.repo / p) for p in
                                   ("install.sh", "scripts/pulse-auto-update.sh",
                                    "scripts/installtests/auto_update_systemd.py")}
        self.receipt["binaries"] = {tag: digest(binary) for tag, binary in self.binaries.items()}
        if len(set(self.receipt["binaries"].values())) != 2:
            raise ValueError("same executable bytes prove no replacement")
        for tag, binary in self.binaries.items():
            if self.run(str(binary), "--version").stdout.decode().splitlines()[0] != "Pulse " + tag:
                raise ValueError("synthetic binary identity mismatch")
        self.run("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(self.root / "signer/key"))
        for tag, binary in self.binaries.items():
            version = self.root / "VERSION"
            version.write_text(tag + "\n")
            archive = self.root / "archives" / f"pulse-{tag}-linux-amd64.tar.gz"
            with tarfile.open(archive, "w:gz") as tar:
                tar.add(binary, arcname="bin/pulse")
                tar.add(version, arcname="VERSION")
            self.sign(archive)
        (self.root / "installer").write_text(INSTALLER)
        self.sign(self.root / "installer")
        for name in ("curl", "mktemp"):
            path = self.root / "tools" / name
            path.write_text(TRANSPORT)
            path.chmod(0o755)
        # Establish the baseline using real source staging and unit/helper writers.
        self.owned_user = self.run("id", "-u", "pulse", ok=(0, 1)).returncode == 1
        self.owned_unit = True  # Any partial writer output is ours to clean up.
        self.shell('source "$PULSE_NATIVE_ROOT/installer-definitions"; '
                   'PINNED_RELEASE_SSH_PUBLIC_KEY=$(cat "$PULSE_NATIVE_ROOT/signer/key.pub"); '
                   'create_user; setup_directories; '
                   'install_pulse_archive "$PULSE_NATIVE_ROOT/archives/pulse-v0.0.1-linux-amd64.tar.gz" v0.0.1; '
                   'install_systemd_service; setup_update_command')
        data = self.root / "data"
        (data / ".env").write_text("PULSE_TELEMETRY=false\n")
        (data / "system.json").write_text('{"autoUpdateEnabled":false,"updateChannel":"stable"}\n')
        (data / "native-persistence-sentinel").write_text("original persistent data\n")
        self.run("systemctl", "start", self.unit.name)
        self.assert_runtime(TAGS[0], "active")
        time.sleep(2)  # Let asynchronous initial persistence settle once.
        files = (".env", "system.json", ".encryption.key", "native-persistence-sentinel")
        self.baseline_data = {p: digest(data / p) for p in files}
        self.baseline_mode = (self.root / "install/bin/pulse").stat().st_mode & 0o777

    def sign(self, path):
        self.run("ssh-keygen", "-Y", "sign", "-f", str(self.root / "signer/key"),
                 "-n", "pulse-install", str(path))
        Path(str(path) + ".sig").rename(str(path) + ".sshsig")

    def assert_runtime(self, tag, active):
        binary = self.root / "install/bin/pulse"
        if digest(binary) != self.receipt["binaries"][tag]:
            raise RuntimeError("installed executable differs from authenticated fixture")
        if (self.root / "install/VERSION").read_text() != tag + "\n":
            raise RuntimeError("version sidecar differs")
        state = self.state()
        if state["ActiveState"] != active:
            raise RuntimeError("prior activity was not restored")
        if active == "active":
            deadline = time.monotonic() + 30
            while True:
                try:
                    observations = {}
                    for endpoint in ("health", "version"):
                        with urllib.request.urlopen(f"http://127.0.0.1:{PORT}/api/{endpoint}", timeout=2) as response:
                            body = response.read(16385)
                        if len(body) > 16384:
                            raise RuntimeError("native API observation too large")
                        observations[endpoint] = json.loads(body)
                    if observations["version"].get("version", "").lstrip("v") != tag.lstrip("v"):
                        raise RuntimeError("served version differs")
                    if observations["health"].get("status") != "healthy":
                        raise RuntimeError("native application is not healthy")
                    break
                except urllib.error.HTTPError:
                    # Do not reinterpret refusal as a restart gap or retry it.
                    raise RuntimeError("native API request refused") from None
                except OSError:
                    if time.monotonic() >= deadline:
                        raise RuntimeError("native application readiness deadline") from None
                    time.sleep(0.25)
            # Type=simple start can return before exec. Bind the observed
            # process after application readiness, not to that launch race.
            state = self.state()
            if state["ActiveState"] != "active" or state["MainPID"] == "0":
                raise RuntimeError("served application has no active managed process")
            if digest(Path("/proc") / state["MainPID"] / "exe") != digest(binary):
                raise RuntimeError("running executable differs from installed executable")
        elif state["MainPID"] != "0":
            raise RuntimeError("inactive fixture retains a managed process")
        return state

    def phase(self, name, tag, expected_exit=0, active="active", fault="", retain=False, manual=False):
        before_state = self.state()
        self.env.update(PULSE_NATIVE_TARGET=tag, PULSE_NATIVE_FAULT=fault)
        result = self.run("/bin/bash", "-c", '"$PULSE_UPDATE_HELPER_PATH" --version "$PULSE_NATIVE_TARGET"'
                          if manual else UPDATER, ok=(expected_exit,))
        (self.root / (name + ".log")).write_bytes(result.stdout)
        expected_tag = TAGS[1] if retain else (TAGS[0] if expected_exit else tag)
        state = self.assert_runtime(expected_tag, active)
        if state["LoadState"] != before_state["LoadState"] or state["UnitFileState"] != before_state["UnitFileState"]:
            raise RuntimeError("unit mask/enablement changed")
        if {p: digest(self.root / "data" / p) for p in self.baseline_data} != self.baseline_data:
            raise RuntimeError("persistent state changed")
        backups = list((self.root / "tmp").glob("pulse-backup.*"))
        if len(backups) != (1 if retain else 0):
            raise RuntimeError("rollback backup disposition differs")
        if retain and digest(backups[0] / "pulse-bin") != self.receipt["binaries"][TAGS[0]]:
            raise RuntimeError("retained rollback backup differs from baseline")
        if not retain and list((self.root / "install/bin").glob("*.rollback.*")):
            raise RuntimeError("rollback staging leaked")
        if digest(self.root / "install/bin/pulse") == self.receipt["binaries"][TAGS[0]]:
            if (self.root / "install/bin/pulse").stat().st_mode & 0o777 != self.baseline_mode:
                raise RuntimeError("baseline executable mode not restored")
        self.receipt["phases"].append({"name": name, "exit": result.returncode, "passed": True,
                                       "installed_tag": expected_tag, "state": state,
                                       "persistent_hashes_unchanged": True, "backup_retained": retain,
                                       "output_sha256": hashlib.sha256(result.stdout).hexdigest()})
        print("NATIVE_UPDATER_PHASE=" + name, flush=True)

    def exercise(self):
        self.phase("forward", TAGS[1])
        self.phase("manual-reverse", TAGS[0], manual=True)
        self.phase("failed-active", TAGS[1], 1, fault="installer")
        self.run("systemctl", "stop", self.unit.name)
        self.phase("failed-inactive", TAGS[1], 1, active="inactive", fault="installer")
        # A real /dev/null mask, not a manager double returning 'masked'.
        saved_unit = self.unit.read_bytes()
        self.unit.unlink()
        self.run("systemctl", "mask", self.unit.name)
        self.run("systemctl", "daemon-reload")
        self.phase("failed-masked", TAGS[1], 1, active="inactive", fault="installer")
        self.run("systemctl", "unmask", self.unit.name)
        self.unit.write_bytes(saved_unit)
        self.run("systemctl", "daemon-reload")
        self.run("systemctl", "start", self.unit.name)
        installer = self.root / "installer"
        original = installer.read_bytes()
        calls = (self.root / "installer-calls").read_bytes()
        installer.write_bytes(original + b"# tampered after signing\n")
        try:
            self.phase("bad-signature", TAGS[1], 1)
            if (self.root / "installer-calls").read_bytes() != calls:
                raise RuntimeError("invalid signature reached installer")
        finally:
            installer.write_bytes(original)
        # Native stop/byte replacement precede a deliberate failed restore copy.
        self.phase("incomplete-restore", TAGS[1], 1, active="inactive", fault="restore", retain=True)

    def cleanup(self):
        if self.owned_unit:
            state = self.state()
            if state["LoadState"] != "not-found":
                self.run("systemctl", "stop", self.unit.name)
                state = self.state()
            if state["ActiveState"] != "inactive" or state["MainPID"] != "0":
                raise RuntimeError("fixture service did not stop")
            # A failed writer may have left a file not yet known to the manager.
            self.unit.unlink(missing_ok=True)
            self.run("systemctl", "daemon-reload")
            self.run("systemctl", "reset-failed", self.unit.name, ok=(0, 1))
            if self.state()["LoadState"] != "not-found" or self.state()["MainPID"] != "0":
                raise RuntimeError("fixture unit/process remained")
        if self.owned_user:
            if self.run("id", "-u", "pulse", ok=(0, 1)).returncode == 0:
                self.run("userdel", "pulse")
            if self.run("id", "-u", "pulse", ok=(0, 1)).returncode != 1:
                raise RuntimeError("owned fixture account remained")
        if self.owned_root:
            # Delete only this invocation's owned signer and scratch directories.
            shutil.rmtree(self.root)
        self.receipt["cleanup"] = {"complete": True, "owned_unit_removed": self.owned_unit,
                                    "owned_account_removed": self.owned_user,
                                    "owned_fixture_removed": self.owned_root,
                                    "fixture_files_absent": not self.root.exists()}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--before", required=True)
    parser.add_argument("--after", required=True)
    parser.add_argument("--receipt", required=True)
    args = parser.parse_args()
    source = os.environ.get("EXPECTED_SOURCE_SHA", "")
    hosted_admission(os.environ, os.geteuid(), source)
    runner_temp = Path(os.environ["RUNNER_TEMP"]).resolve()
    receipt = Path(args.receipt).resolve()
    if receipt.parent != runner_temp or receipt.exists():
        raise ValueError("receipt must be a new direct runner-temp file")
    # /home is hidden by the real unit's ProtectHome; /tmp and /var/tmp are
    # private under PrivateTmp. Preserve both protections, use a fixed /opt lab.
    root = Path("/opt/pulse-source-native")
    lab = NativeLab(root, Path(__file__).resolve().parents[2], args.before, args.after, source)
    failed = False
    try:
        lab.prepare()
        lab.exercise()
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        failed = True
        lab.receipt["failure"] = {"type": type(error).__name__, "message": str(error)}
    finally:
        try:
            lab.cleanup()
        except (OSError, RuntimeError, subprocess.SubprocessError) as error:
            failed = True
            lab.receipt["cleanup"] = {"complete": False, "error_type": type(error).__name__}
        receipt.write_text(json.dumps(lab.receipt, indent=2) + "\n")
    if failed or len(lab.receipt["phases"]) != 7:
        raise SystemExit("Native updater proof incomplete; inspect the bounded receipt.")
    print("NATIVE_UPDATER_COMPLETE=" + source)


if __name__ == "__main__":
    main()
