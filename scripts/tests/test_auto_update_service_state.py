#!/usr/bin/env python3
"""Exercise the real updater with private files and a confined manager double.

No host manager, network, application build or published installer is used.
The versioned executable and signature verifier are synthetic; existing Go
transaction tests separately exercise real local SSH signatures and metadata.
"""

import json
import os
from pathlib import Path
import shutil
import shlex
import signal
import subprocess
import sys
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
    if after and mode == "post-install-hang":
        print("LoadState=loaded\nActiveState=active", flush=True)
        time.sleep(30)
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
    payload = "ActiveState=" + state + "\nLoadState=" + os.environ["LOAD_STATE"] + "\n"
    phase = "post-stop" if stopped else ("post-install" if after else "initial")
    fault = os.environ.get("OBSERVATION_FAULT", "") if phase == os.environ.get("OBSERVATION_PHASE") else ""
    if fault == "stdout-blank": payload += "\n"
    if fault == "stdout-unterminated": payload = payload.rstrip("\n")
    sys.stdout.write(payload)
    sys.stdout.flush()
    if fault == "stderr-warning": sys.stderr.write("private fixture manager diagnostic\n")
    if fault == "stderr-blank": sys.stderr.write("\n")
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
if [[ "$REAL_LOG" != true ]]; then
log() {
    printf '[%s] %s\n' "$1" "${*:2}"
    if [[ "${2:-}" == 'installer: '* ]]; then
        local count=0 status_name=""
        if [[ -f "$FIXTURE/collector-count" ]]; then read -r count < "$FIXTURE/collector-count"; fi
        ((count += 1))
        printf '%s\n' "$count" > "$FIXTURE/collector-count"
        status_name="COLLECTOR_EXIT_$count"
        return "${!status_name:-$COLLECTOR_EXIT}"
    fi
}
fi
# This transaction fixture has a simulated manager and shell-script binaries.
# The separate process controls below exercise the production /proc comparison.
verify_update_running_executable() {
    local fixture_version=""
    IFS= read -r fixture_version < "$INSTALL_DIR/VERSION"
    printf '%s\n' "$fixture_version" >> "$FIXTURE/runtime-probes"
    return "$RUNTIME_EXIT"
}
detect_service_name() { printf '%s\n' "$SERVICE_NAME"; }
# Advance the polling clock instead of waiting; manager/timeout deadlines stay
# real. Dedicated UpdateServiceDeadlines controls below use actual time/sleep.
sleep() { SECONDS=$((SECONDS + $1)); }
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
    printf '%s\000' "$#" "$@" >> "$FIXTURE/curl-calls"
    local out="" url=""
    while (( $# )); do
        case "$1" in -o) out="$2"; shift ;; https://*) url="$1" ;; esac
        shift
    done
    printf '%s\n' "$url" >> "$FIXTURE/downloads"
    if [[ "$DOWNLOAD_FAULT" == installer:* && "$url" != *.sshsig ]] ||
       [[ "$DOWNLOAD_FAULT" == signature:* && "$url" == *.sshsig ]]; then
        printf 'partial unverified input\n' > "$out"
        return "${DOWNLOAD_FAULT#*:}"
    fi
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


LOGGER = r'''#!/bin/sh
set -eu
# Keep every external sink invocation and exact argv, without starting Python
# for each of the matrix's 528 installer lines. Counted NUL frames preserve
# empty arguments, whitespace and newlines; no host journal is contacted.
printf '%s\000' "$#" "$@" >> "$FIXTURE/logger-calls"
[ "$#" -ge 5 ] || exit 97
[ "$1" = -t ] && [ "$2" = pulse-custom@department-auto-update ] && [ "$3" = -p ] || exit 97
case "$4" in user.info|user.warn|user.error) ;; *) exit 97 ;; esac
case "$5" in
    'installer: '*)
        count=0
        if [ -f "$FIXTURE/collector-count" ]; then
            IFS= read -r count < "$FIXTURE/collector-count"
        fi
        count=$((count + 1))
        printf '%s\n' "$count" > "$FIXTURE/collector-count"
        while IFS=' ' read -r line status; do
            if [ "$line" = "$count" ]; then exit "$status"; fi
        done < "$FIXTURE/collector-failures"
        exit "$COLLECTOR_EXIT"
        ;;
esac
'''


def read_logger_calls(path):
    if not path.exists():
        return []
    fields = path.read_bytes().split(b"\0")
    if fields.pop() != b"":
        raise ValueError("unterminated logger argument frame")
    calls, offset = [], 0
    while offset < len(fields):
        size = fields[offset]
        if not size.isdigit():
            raise ValueError("invalid logger argument count")
        count = int(size)
        offset += 1
        if count > len(fields) - offset:
            raise ValueError("incomplete logger argument frame")
        calls.append([arg.decode("utf-8") for arg in fields[offset:offset + count]])
        offset += count
    return calls


class UpdaterFixture(unittest.TestCase):
    def run_fixture(self, *, mode="normal", state="active", load="loaded", success=False, flow="update",
                    installer_exit=None, collector_exit=0, installer_output=None, collector_failures=None,
                    real_log=False, runtime_exit=0, installer_state=None, download_fault="",
                    observation_fault="", observation_phase="initial"):
        with tempfile.TemporaryDirectory(prefix="pulse-updater-state-") as directory:
            f = Path(directory)
            for name in ("tools", "tmp", "install/bin", "config"):
                (f / name).mkdir(parents=True)
            (f / "manager.py").write_text(MANAGER)
            # Use the suite's selected interpreter, without site startup, for
            # this stdlib-only model. Re-resolving python3 for every manager
            # observation can select a different, slower host installation.
            (f / "tools/systemctl").write_text(
                "#!/bin/sh\nexec " + shlex.quote(sys.executable) + " -I -S "
                + shlex.quote(str(f / "manager.py")) + ' "$@"\n')
            (f / "tools/systemctl").chmod(0o755)
            # The production log() remains intact in real-log controls. Only
            # its external sink is confined; no host journal is contacted.
            (f / "tools/logger").write_text(LOGGER)
            (f / "tools/logger").chmod(0o755)
            (f / "collector-failures").write_text(
                "".join(f"{line} {status}\n" for line, status in (collector_failures or {}).items()))
            old = b"#!/bin/sh\necho 'Pulse v6.5.0'\n"
            new = b"#!/bin/sh\necho 'Pulse v6.6.0'\n"
            (f / "install/bin/pulse").write_bytes(old)
            (f / "install/bin/pulse").chmod(0o751)
            (f / "install/VERSION").write_text("v6.5.0\n")
            (f / "config/system.json").write_text('operator settings\n')
            (f / "config/agent-id").write_text('original identity\n')
            (f / "state").write_text(state + "\n")
            (f / "new-pulse").write_bytes(new)
            (f / "installer-output").write_text("fixture installation output\n" if installer_output is None else installer_output)
            (f / "installer").write_text(
                'set -eu\n'
                'echo attempted > "$FIXTURE/installer-ran"\n'
                'cp "$FIXTURE/new-pulse" "$PULSE_INSTALL_DIR/bin/pulse"\n'
                'echo v6.6.0 > "$PULSE_INSTALL_DIR/VERSION"\n'
                'echo "$INSTALLER_STATE" > "$FIXTURE/state"\n'
                'cat "$FIXTURE/installer-output"\n'
                'echo completed > "$FIXTURE/installer-finished"\n'
                'exit "$INSTALLER_EXIT"\n')
            env = dict(os.environ)
            env.update(FIXTURE=str(f), UPDATER=str(UPDATER), MODE=mode, FLOW=flow,
                       PULSE_INSTALL_DIR=str(f / "install"), PULSE_CONFIG_DIR=str(f / "config"),
                       PULSE_SERVICE_NAME="pulse-custom@department", LOAD_STATE=load,
                       INSTALLER_STATE=installer_state if installer_state is not None else ("active" if mode in ("stop-noop", "stop-error", "stop-hang", "after-error", "readback-error", "readback-empty") else "inactive"),
                       INSTALLER_EXIT=str(installer_exit) if installer_exit is not None else ("0" if success else "23"),
                       COLLECTOR_EXIT=str(collector_exit),
                       REAL_LOG="true" if real_log else "false", RUNTIME_EXIT=str(runtime_exit),
                       DOWNLOAD_FAULT=download_fault,
                       OBSERVATION_FAULT=observation_fault, OBSERVATION_PHASE=observation_phase,
                       PATH=str(f / "tools") + os.pathsep + os.environ["PATH"])
            for line, status in (collector_failures or {}).items():
                env[f"COLLECTOR_EXIT_{line}"] = str(status)
            start = time.monotonic()
            result = subprocess.run(["bash", "-c", HARNESS], env=env, capture_output=True, timeout=15)
            calls = [json.loads(line) for line in (f / "calls").read_text().splitlines()] if (f / "calls").exists() else []
            backups = list((f / "tmp").glob("pulse-backup.*"))
            observation = dict(exit=result.returncode, output=result.stdout.decode(), error=result.stderr.decode(),
                               elapsed=time.monotonic() - start, calls=calls,
                               downloaded=(f / "downloads").exists(), temporary=(f / "temp-requests").exists(),
                               installed=(f / "installer-ran").exists(), stopped=(f / "stop-ran").exists(),
                               installer_finished=(f / "installer-finished").exists(),
                               runtime_probes=(f / "runtime-probes").read_text().splitlines()
                               if (f / "runtime-probes").exists() else [],
                               collected=int((f / "collector-count").read_text()) if (f / "collector-count").exists() else 0,
                               logger_calls=read_logger_calls(f / "logger-calls"),
                               curl_calls=read_logger_calls(f / "curl-calls"),
                               temporary_files=[p.name for p in (f / "tmp").iterdir()],
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


class UpdateDownloads(UpdaterFixture):
    def assert_download_policy(self, calls):
        self.assertTrue(calls)
        for args in calls:
            self.assertEqual(args[:10], ["-q", "-fsSL", "--connect-timeout", "5",
                                         "--max-time", "60", "--proto", "=https",
                                         "--proto-redir", "=https"])
            self.assertEqual(len(args), 13)
            self.assertTrue(args[10].startswith("https://github.com/rcourtman/Pulse/releases/download/v6.6.0/"))
            self.assertEqual(args[11], "-o")

    def test_both_complete_downloads_keep_the_exact_signed_installer_handoff(self):
        result = self.run_fixture(success=True, installer_state="active")
        self.assertEqual(result["exit"], 0, result)
        self.assertTrue(result["installer_finished"])
        self.assert_download_policy(result["curl_calls"])
        self.assertEqual([args[10].rsplit("/", 1)[1] for args in result["curl_calls"]],
                         ["install.sh", "install.sh.sshsig"])
        self.assertEqual(result["temporary_files"], [])

    def test_partial_failed_downloads_never_execute_or_mutate_the_install(self):
        for asset in ("installer", "signature"):
            for status in (7, 22, 28):  # Connection, HTTP refusal, timeout.
                for state in ("active", "inactive"):
                    with self.subTest(asset=asset, status=status, state=state):
                        result = self.run_fixture(state=state, download_fault=f"{asset}:{status}")
                        self.assertEqual(result["exit"], 1, result)
                        self.assertFalse(result["installed"])
                        self.assertFalse(result["stopped"])
                        self.assertFalse(result["started"])
                        self.assertEqual(result["state"], state)
                        self.assertEqual(result["binary"], b"#!/bin/sh\necho 'Pulse v6.5.0'\n")
                        self.assertEqual(result["version"], "v6.5.0\n")
                        self.assertEqual(result["temporary_files"], [])
                        self.assertEqual(len(result["curl_calls"]), 1 if asset == "installer" else 2)
                        self.assert_download_policy(result["curl_calls"])
                        self.assertIn("Failed to download installer", result["output"])


class UpdateLogSinks(UpdaterFixture):
    def test_logger_double_keeps_complete_argument_frames_and_exit_sequence(self):
        with tempfile.TemporaryDirectory(prefix="pulse-updater-logger-") as directory:
            f = Path(directory)
            logger = f / "logger"
            logger.write_text(LOGGER)
            logger.chmod(0o755)
            (f / "collector-failures").write_text("1 73\n2 42\n5 73\n")
            env = dict(os.environ, FIXTURE=str(f), COLLECTOR_EXIT="0")
            prefix = ["-t", "pulse-custom@department-auto-update", "-p", "user.info"]
            calls = []
            for message, status in (("installer: first", 73), ("installer: second", 42),
                                    ("installer: quoted \" and \\ and\ttab\nnext ü", 0),
                                    ("installer: later", 0), ("installer: final", 73)):
                args = prefix + [message, "", "separate argument"]
                calls.append(args)
                r = subprocess.run([str(logger)] + args, env=env, capture_output=True, timeout=5)
                self.assertEqual(r.returncode, status, r.stderr)
            invalid = prefix[:3] + ["user.invalid", "invalid priority"]
            calls.append(invalid)
            r = subprocess.run([str(logger)] + invalid, env=env, capture_output=True, timeout=5)
            self.assertEqual(r.returncode, 97)
            self.assertEqual(read_logger_calls(f / "logger-calls"), calls)
            self.assertEqual((f / "collector-count").read_text(), "5\n")
            for malformed in (b"5\0one\0", b"-1\0", b"no-count\0", b"1\0unterminated"):
                with self.subTest(malformed=malformed):
                    (f / "malformed").write_bytes(malformed)
                    with self.assertRaises(ValueError):
                        read_logger_calls(f / "malformed")

    # Keep the expensive real-collector matrix separate from manager timeout
    # controls, without widening their deadline or dropping any combinations.
    def test_real_log_retains_each_sink_failure_in_conditional_calls(self):
        for logger_exit, closed_stdout, expected in ((0, False, 0), (73, False, 73),
                                                      (0, True, 1), (73, True, 73)):
            with self.subTest(logger_exit=logger_exit, closed_stdout=closed_stdout):
                with tempfile.TemporaryDirectory(prefix="pulse-updater-log-") as directory:
                    f = Path(directory)
                    (f / "tools").mkdir()
                    # Only the external logger is substituted, not log().
                    (f / "tools/logger").write_text(
                        '#!/bin/sh\n'
                        'printf "%s\\n" "$*" > "$FIXTURE/logger-args"\n'
                        'exit "$LOGGER_EXIT"\n')
                    (f / "tools/logger").chmod(0o755)
                    env = dict(os.environ, FIXTURE=str(f), UPDATER=str(UPDATER),
                               PULSE_SERVICE_NAME="pulse-custom@department", LOGGER_EXIT=str(logger_exit),
                               PATH=str(f / "tools") + os.pathsep + os.environ["PATH"])
                    redirect = " 1>&-" if closed_stdout else ""
                    script = ('set -euo pipefail\nsource "$UPDATER"\nstatus=0\n'
                              'log info "sink control"' + redirect + ' || status=$?\n'
                              'printf "OBSERVED_STATUS=%s\\n" "$status"\n')
                    r = subprocess.run(["bash", "-c", script], env=env, capture_output=True, timeout=5)
                    self.assertEqual(r.returncode, 0)
                    self.assertIn(f"OBSERVED_STATUS={expected}\n", r.stdout.decode())
                    self.assertEqual((f / "logger-args").read_text(),
                                     "-t pulse-custom@department-auto-update -p user.info sink control\n")
                    self.assertEqual("[info] sink control" in r.stdout.decode(), not closed_stdout)

    def test_real_logger_deadline_preserves_stdout_exit_and_owned_cleanup(self):
        # Exercise the production log(), not a simulated timeout. The logger
        # owns a pipe-holding child; both must stop even if they ignore TERM.
        for ignore_term, expected in ((False, 124), (True, 137)):
            with self.subTest(ignore_term=ignore_term):
                with tempfile.TemporaryDirectory(prefix="pulse-updater-log-deadline-") as directory:
                    f = Path(directory)
                    (f / "tools").mkdir()
                    logger = f / "tools/logger"
                    logger.write_text(f"#!{sys.executable} -IS\n" + r'''import json, os, signal, subprocess, sys, time
from pathlib import Path
f = Path(os.environ["FIXTURE"])
ignore = os.environ["IGNORE_TERM"] == "true"
if ignore:
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
child = subprocess.Popen([sys.executable, "-IS", "-c",
    "import signal,time; " +
    ("signal.signal(signal.SIGTERM, signal.SIG_IGN); " if ignore else "") +
    "time.sleep(60)"])
(f / "logger-observation.json").write_text(json.dumps({
    "args": sys.argv[1:], "pids": [os.getpid(), child.pid], "group": os.getpgrp()}))
child.wait()
''')
                    logger.chmod(0o755)
                    env = dict(os.environ, FIXTURE=str(f), UPDATER=str(UPDATER),
                               IGNORE_TERM=str(ignore_term).lower(),
                               PULSE_SERVICE_NAME="pulse-custom@department",
                               PATH=str(f / "tools") + os.pathsep + os.environ["PATH"])
                    script = ('set -euo pipefail\nsource "$UPDATER"\nstatus=0\n'
                              'log info "journal deadline control" || status=$?\n'
                              'printf "OBSERVED_STATUS=%s\\n" "$status"\n')
                    started = time.monotonic()
                    process = subprocess.Popen(["bash", "-c", script], env=env,
                                               stdout=subprocess.PIPE,
                                               stderr=subprocess.PIPE, text=True, start_new_session=True)
                    try:
                        stdout, stderr = process.communicate(timeout=10)
                    except subprocess.TimeoutExpired:
                        # Only this isolated fixture's session/group is owned.
                        # A deadline regression must fail, never leave readers.
                        observation = f / "logger-observation.json"
                        groups = {process.pid}
                        if observation.exists():
                            groups.add(json.loads(observation.read_text())["group"])
                        for group in groups:
                            try:
                                os.killpg(group, signal.SIGKILL)
                            except ProcessLookupError:
                                pass
                        process.communicate(timeout=3)
                        self.fail("journal write exceeded its five-second deadline and KILL backstop")
                    elapsed = time.monotonic() - started
                    self.assertEqual(process.returncode, 0, stderr)
                    self.assertIn(f"OBSERVED_STATUS={expected}\n", stdout)
                    self.assertIn("[info] journal deadline control\n", stdout)
                    self.assertGreaterEqual(elapsed, 4.5)
                    self.assertLess(elapsed, 9.5)
                    observed = json.loads((f / "logger-observation.json").read_text())
                    self.assertEqual(observed["args"], ["-t", "pulse-custom@department-auto-update",
                                                       "-p", "user.info", "journal deadline control"])
                    # Pipe EOF alone is not termination. Read back both owned
                    # readers; zombies cannot run and may await their reaper.
                    deadline = time.monotonic() + 0.5
                    for pid in observed["pids"]:
                        while True:
                            try:
                                if sys.platform.startswith("linux"):
                                    state = Path(f"/proc/{pid}/stat").read_text().rsplit(") ", 1)[1].split()[0]
                                else:
                                    probe = subprocess.run(["ps", "-p", str(pid), "-o", "stat="],
                                                           capture_output=True, text=True, timeout=2)
                                    if probe.returncode == 1 and not probe.stdout and not probe.stderr:
                                        break
                                    self.assertEqual(probe.returncode, 0, probe.stderr)
                                    state = probe.stdout.strip()[0]
                            except (FileNotFoundError, ProcessLookupError):
                                break
                            if state == "Z":
                                break
                            if time.monotonic() >= deadline:
                                self.fail(f"owned journal reader {pid} remains in state {state}")
                            time.sleep(0.01)

    def test_real_logger_failure_drains_installer_and_preserves_recovery(self):
        # More than a pipe buffer, plus an unterminated final line. These run
        # production log(), perform_update and its collector together: a green
        # log() double cannot establish that a real sink failure is retained.
        lines = ["opening"] + ["x" * 2048 for _ in range(64)] + ["final installer evidence"]
        for state in ("active", "inactive"):
            for failures in ({}, {1: 73}, {1: 73, 2: 42}, {len(lines): 73}, {1: 124}, {1: 137}):
                with self.subTest(state=state, failures=failures):
                    r = self.run_fixture(state=state, success=True, real_log=True,
                                         installer_output="\n".join(lines), collector_failures=failures)
                    self.assertTrue(r["installer_finished"])
                    self.assertEqual(r["collected"], len(lines))
                    self.assertTrue("installer: final installer evidence" in r["output"])
                    installer_calls = [args for args in r["logger_calls"] if args[4].startswith("installer: ")]
                    self.assertEqual(len(installer_calls), len(lines))
                    self.assertTrue(all(args[:4] == ["-t", "pulse-custom@department-auto-update", "-p", "user.info"]
                                        for args in installer_calls))
                    first_failure = next(iter(failures.values()), 0)
                    expected = (f"Installer pipeline failed (installer exit: 0; log collector exit: {first_failure})"
                                if failures else "Installer completed; verifying update (installer exit: 0; log collector exit: 0)")
                    self.assertTrue(expected in r["output"], "real logger exit was not retained by the collector")
                    self.assertEqual(r["exit"], 1 if failures else 0)
                    self.assertEqual("Update successfully installed and verified" in r["output"], not failures)
                    self.assertEqual(r["binary"], b"#!/bin/sh\necho 'Pulse v6.5.0'\n" if failures
                                     else b"#!/bin/sh\necho 'Pulse v6.6.0'\n")
                    self.assertEqual(r["version"], "v6.5.0\n" if failures else "v6.6.0\n")
                    self.assertEqual(r["started"], state == "active")
                    self.assertEqual(r["backups"], [])


class UpdateServiceState(UpdaterFixture):
    def test_poll_timeout_retains_backup_without_start_or_rollback(self):
        result = self.run_fixture(mode="post-install-hang", success=True)
        self.assertEqual(result["exit"], 1, result)
        self.assertTrue(result["installed"] and result["installer_finished"], result)
        self.assertFalse(result["stopped"] or result["started"], result)
        self.assertEqual(result["binary"], b"#!/bin/sh\necho 'Pulse v6.6.0'\n")
        self.assertEqual(result["version"], "v6.6.0\n")
        self.assertEqual(result["backups"], [b"#!/bin/sh\necho 'Pulse v6.5.0'\n"])
        self.assertIn("Update service state is uncertain; backup retained", result["output"])
        self.assertNotIn("Restoring from backup", result["output"])
        self.assertNotIn("Update successfully installed and verified", result["output"])

    def test_manager_diagnostics_refuse_initial_update_and_restart(self):
        for fault in ("stderr-warning", "stderr-blank", "stdout-blank", "stdout-unterminated"):
            for flow, state in (("update", "active"), ("update", "inactive"), ("backstop", "inactive")):
                with self.subTest(fault=fault, flow=flow, state=state):
                    r = self.run_fixture(observation_fault=fault, flow=flow, state=state, success=True)
                    # The backstop retains the caller's status, but no unknown
                    # observation is consent to install, restart or allocate.
                    self.assertEqual(r["exit"], 0 if flow == "backstop" else 1, r)
                    self.assertFalse(r["installed"] or r["downloaded"] or r["temporary"], r)
                    self.assertFalse(r["started"] or r["stopped"], r)
                    self.assertEqual(r["state"], state, r)
                    self.assertEqual(r["backups"], [], r)
                    self.assertEqual(r["binary"], b"#!/bin/sh\necho 'Pulse v6.5.0'\n", r)
                    self.assertEqual(len(r["calls"]), 1, r)  # No more favourable read.
                    self.assertNotIn("private fixture manager diagnostic", r["output"] + r["error"])

    def test_manager_diagnostics_after_install_retain_backup_without_mutation(self):
        for fault in ("stderr-warning", "stderr-blank", "stdout-blank", "stdout-unterminated"):
            for state in ("active", "inactive"):
                with self.subTest(fault=fault, state=state):
                    r = self.run_fixture(observation_fault=fault, observation_phase="post-install",
                                         state=state, success=True, installer_state="active")
                    self.assertEqual(r["exit"], 1, r)
                    self.assertTrue(r["installed"] and r["installer_finished"], r)
                    self.assertFalse(r["started"] or r["stopped"], r)
                    self.assertEqual(r["state"], "active", r)
                    self.assertEqual(r["version"], "v6.6.0\n", r)
                    self.assertEqual(r["backups"], [b"#!/bin/sh\necho 'Pulse v6.5.0'\n"], r)
                    self.assertEqual(r["runtime_probes"], [], r)
                    self.assertEqual(len(r["calls"]), 2, r)
                    self.assertNotIn("Update successfully installed and verified", r["output"])
                    self.assertNotIn("private fixture manager diagnostic", r["output"] + r["error"])

    def test_manager_diagnostics_after_stop_retain_backup_without_restart(self):
        for fault in ("stderr-warning", "stderr-blank", "stdout-blank", "stdout-unterminated"):
            for state in ("active", "inactive"):
                with self.subTest(fault=fault, state=state):
                    r = self.run_fixture(observation_fault=fault, observation_phase="post-stop",
                                         state=state, installer_state="active")
                    self.assertEqual(r["exit"], 1, r)
                    self.assertTrue(r["installed"] and r["stopped"], r)
                    self.assertFalse(r["started"], r)
                    self.assertEqual(r["state"], "inactive", r)
                    # The stop was authorised by the previous valid read;
                    # incomplete readback permits neither restore nor restart.
                    self.assertEqual(r["version"], "v6.6.0\n", r)
                    self.assertEqual(r["backups"], [b"#!/bin/sh\necho 'Pulse v6.5.0'\n"], r)
                    self.assertEqual(len(r["calls"]), 4, r)
                    self.assertIn("manual recovery required", r["output"])
                    self.assertNotIn("Previous installation restored", r["output"])
                    self.assertNotIn("private fixture manager diagnostic", r["output"] + r["error"])

    def test_success_cannot_claim_unverified_running_executable(self):
        # A matching new disk version and an active manager are insufficient.
        # Preserve the currently active service; uncertainty grants no stop,
        # second start or rollback, including a previously inactive unit.
        for prior_state in ("active", "inactive"):
            with self.subTest(prior_state=prior_state):
                r = self.run_fixture(state=prior_state, success=True,
                                     installer_state="active", runtime_exit=2)
                self.assertEqual(r["exit"], 1)
                self.assertEqual(r["runtime_probes"], ["v6.6.0"])
                self.assertFalse(r["started"] or r["stopped"])
                self.assertEqual(r["state"], "active")
                self.assertEqual(r["version"], "v6.6.0\n")
                self.assertEqual(r["backups"], [b"#!/bin/sh\necho 'Pulse v6.5.0'\n"])
                self.assertIn("Running Pulse executable is unverified; backup retained", r["output"])
                self.assertNotIn("Update successfully installed and verified", r["output"])
                self.assertNotIn("Restoring from backup", r["output"])

    def test_rollback_keeps_backup_until_restored_running_executable_matches(self):
        r = self.run_fixture(state="active", success=False, runtime_exit=2)
        self.assertEqual(r["exit"], 1)
        self.assertEqual(r["runtime_probes"], ["v6.5.0"])
        self.assertTrue(r["started"])
        self.assertEqual(r["state"], "active")
        self.assertEqual(r["version"], "v6.5.0\n")
        self.assertEqual(r["backups"], [b"#!/bin/sh\necho 'Pulse v6.5.0'\n"])
        self.assertIn("Restored running Pulse executable is unverified; backup retained", r["output"])
        self.assertNotIn("Previous installation restored", r["output"])

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

    def test_early_collector_failure_survives_later_success_and_drains_installer(self):
        # Exceed a pipe buffer so an early collector exit would interrupt the
        # installer rather than merely miss its final evidence. No host
        # installer or logging service is invoked by this confined fixture.
        lines = ["opening"] + ["x" * 2048 for _ in range(64)] + ["final installer evidence"]
        for state in ("active", "inactive"):
            for failures in ({1: 73}, {1: 73, 2: 42}):
                with self.subTest(state=state, failures=failures):
                    r = self.run_fixture(state=state, success=True, installer_output="\n".join(lines) + "\n",
                                         collector_failures=failures)
                    self.assertIn("Installer pipeline failed (installer exit: 0; log collector exit: 73)", r["output"], r)
                    self.assertTrue(r["installer_finished"], r)
                    self.assertEqual(r["collected"], len(lines), r)
                    self.assertIn("installer: final installer evidence", r["output"], r)
                    self.assertNotIn("Update successfully installed and verified", r["output"], r)
                    self.assertEqual(r["exit"], 1, r)
                    self.assertEqual(r["binary"], b"#!/bin/sh\necho 'Pulse v6.5.0'\n", r)
                    self.assertEqual(r["version"], "v6.5.0\n", r)
                    self.assertEqual(r["started"], state == "active", r)
                    self.assertEqual(r["backups"], [], r)

    def test_unterminated_final_installer_line_is_collected_and_its_failure_retained(self):
        for status in (0, 73):
            with self.subTest(status=status):
                r = self.run_fixture(state="inactive", success=True, installer_output="opening\nfinal evidence without newline",
                                     collector_failures={2: status})
                self.assertIn("installer: final evidence without newline", r["output"], r)
                self.assertTrue(r["installer_finished"], r)
                self.assertEqual(r["collected"], 2, r)
                self.assertEqual(r["exit"], 0 if status == 0 else 1, r)
                self.assertEqual(r["version"], "v6.6.0\n" if status == 0 else "v6.5.0\n", r)
                self.assertFalse(r["started"], r)
                self.assertEqual(r["backups"], [], r)
                expected = ("Installer completed; verifying update (installer exit: 0; log collector exit: 0)"
                            if status == 0 else "Installer pipeline failed (installer exit: 0; log collector exit: 73)")
                self.assertIn(expected, r["output"], r)

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



PROCESS_MANAGER = r'''#!/usr/bin/env python3
import os, shutil, sys, time
from pathlib import Path
f = Path(os.environ["FIXTURE"])
args = sys.argv[1:]
expected = ["show", "pulse-custom@department", "--no-pager", "--property=LoadState", "--property=ActiveState", "--property=MainPID"]
if args != expected: sys.exit(97)
count = int((f / "observations").read_text()) + 1 if (f / "observations").exists() else 1
(f / "observations").write_text(str(count))
mode = os.environ["IDENTITY_MODE"]
if mode == "error": sys.exit(1)
if mode == "hang": time.sleep(30)
if mode.startswith("disk-drift:") and count == 2:
    installed = Path(os.environ["SELECTED_BINARY"])
    fault = mode.partition(":")[2]
    if fault in ("different", "same-copy"):
        replacement = f / "drift-replacement"
        shutil.copyfile("/bin/sleep" if fault == "different" else sys.executable, replacement)
        replacement.chmod(0o700)
        replacement.replace(installed)
    elif fault == "nonexecutable": installed.chmod(0o600)
    elif fault == "missing": installed.unlink()
    elif fault == "wrong-symlink":
        installed.unlink()
        installed.symlink_to("/bin/sleep")
    elif fault == "directory":
        installed.unlink()
        installed.mkdir()
    else: sys.exit(97)
    (f / "identity-drift-applied").write_text(fault)
pid = os.environ["OWNED_PID"]
if mode == "pid-zero": pid = "0"
if mode == "pid-negative": pid = "-1"
if mode == "pid-path": pid = "../self"
if mode == "pid-missing-process": pid = "999999999999"
if mode == "pid-moved" and count > 1: pid = os.environ["DIFFERENT_PID"]
load = "not-found" if mode == "not-found" else ("masked" if mode == "masked" else "loaded")
state = "inactive" if mode == "inactive" or (mode == "state-moved" and count > 1) else "active"
# Deliberately reverse property order, as systemd does not promise an order.
payload = "MainPID=" + pid + "\n"
if mode != "missing": payload += "ActiveState=" + state + "\n"
payload += "LoadState=" + load + "\n"
fault = ""
if mode.startswith("diagnostic:"):
    _, selected_fault, selected_read = mode.split(":")
    if count == int(selected_read): fault = selected_fault
if fault == "stdout-blank": payload += "\n"
if fault == "stdout-unterminated": payload = payload.rstrip("\n")
sys.stdout.write(payload)
sys.stdout.flush()
if fault == "stderr-warning": sys.stderr.write("private fixture manager diagnostic\n")
if fault == "stderr-blank": sys.stderr.write("\n")
if mode == "duplicate": print("MainPID=" + pid)
if mode == "extra": print("Unknown=value")
'''


@unittest.skipUnless(sys.platform.startswith("linux"), "Linux /proc executable identity; transaction controls remain portable")
class RunningExecutable(unittest.TestCase):
    def run_identity(self, mode="normal", binary_mode="same"):
        with tempfile.TemporaryDirectory(prefix="pulse-owned-process-") as directory:
            f = Path(directory); (f / "tools").mkdir(); (f / "install/bin").mkdir(parents=True)
            installed = f / "install/bin/pulse"
            shutil.copyfile(sys.executable, installed); installed.chmod(0o700)
            # Match UpdaterFixture's stdlib-only manager launch. Repeated site
            # startup and PATH interpreter resolution do not exercise identity
            # admission, but can consume the entire unchanged Go watchdog.
            manager = f / "manager.py"; manager.write_text(PROCESS_MANAGER)
            tool = f / "tools/systemctl"
            tool.write_text("#!/bin/sh\nexec " + shlex.quote(sys.executable) + " -I -S "
                            + shlex.quote(str(manager)) + ' "$@"\n')
            tool.chmod(0o700)
            # Run the copied executable, then replace its path: /proc/PID/exe
            # still observes the actual old executable, not the new path bytes.
            child = subprocess.Popen([str(installed), "-I", "-S", "-c", "import time; print('ready', flush=True); time.sleep(30)"],
                                     stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            try:
                self.assertEqual(child.stdout.readline(), b"ready\n")
                if binary_mode in ("different", "same-copy", "nonexecutable"):
                    replacement = f / "replacement"
                    shutil.copyfile("/bin/sleep" if binary_mode == "different" else sys.executable, replacement)
                    replacement.chmod(0o600 if binary_mode == "nonexecutable" else 0o700)
                    replacement.replace(installed)
                elif binary_mode == "symlink":
                    installed.unlink(); installed.symlink_to(sys.executable)
                elif binary_mode == "legacy":
                    installed.rename(f / "install/pulse")
                elif binary_mode == "missing":
                    installed.unlink()
                elif binary_mode == "broken-primary":
                    installed.unlink(); installed.symlink_to(f / "missing")
                    shutil.copyfile(sys.executable, f / "install/pulse")
                env = dict(os.environ, FIXTURE=str(f), UPDATER=str(UPDATER),
                           PULSE_INSTALL_DIR=str(f / "install"), OWNED_PID=str(child.pid),
                           DIFFERENT_PID=str(os.getpid()), IDENTITY_MODE=mode,
                           SELECTED_BINARY=str(f / "install/pulse" if binary_mode == "legacy" else installed),
                           PATH=str(f / "tools") + os.pathsep + os.environ["PATH"])
                script = ('set -euo pipefail\nsource "$UPDATER"\nlog() { :; }\n'
                          'status=0\nverify_update_running_executable pulse-custom@department || status=$?\n'
                          'printf "IDENTITY_STATUS=%s\\n" "$status"\nexit "$status"\n')
                started = time.monotonic()
                r = subprocess.run(["bash", "-c", script], env=env, capture_output=True, timeout=10)
                if mode.startswith("disk-drift:"):
                    self.assertEqual((f / "identity-drift-applied").read_text(), mode.partition(":")[2])
                return r, time.monotonic() - started, int((f / "observations").read_text())
            finally:
                child.terminate()
                child.communicate(timeout=5)
                self.assertIsNotNone(child.returncode)

    def test_actual_process_not_replaced_disk_path_is_checked(self):
        for binary in ("same", "same-copy", "symlink", "legacy", "different", "nonexecutable", "missing", "broken-primary"):
            with self.subTest(binary=binary):
                r, _, observations = self.run_identity(binary_mode=binary)
                accepted = binary in ("same", "same-copy", "symlink", "legacy")
                self.assertEqual(r.returncode, 0 if accepted else 2, r.stderr)
                self.assertIn(b"IDENTITY_STATUS=0" if accepted else b"IDENTITY_STATUS=2", r.stdout)
                self.assertEqual(observations, 2 if accepted else 1)

    def test_complete_stable_main_pid_tuple_is_required(self):
        for mode in ("normal", "masked", "error", "missing", "duplicate", "extra", "inactive", "not-found",
                     "pid-zero", "pid-negative", "pid-path", "pid-missing-process", "pid-moved", "state-moved"):
            with self.subTest(mode=mode):
                r, _, observations = self.run_identity(mode=mode)
                accepted = mode in ("normal", "masked")
                self.assertEqual(r.returncode, 0 if accepted else 2, r.stderr)
                self.assertEqual(observations, 2 if accepted or mode in ("pid-moved", "state-moved") else 1)

    def test_final_manager_read_cannot_hide_installed_executable_drift(self):
        # The first real /proc comparison succeeds. Change only the selected
        # installation path during the second manager read, leaving the owned
        # process/PID active. Equal-byte atomic replacement remains supported.
        for binary in ("same", "legacy", "symlink"):
            for fault in ("different", "nonexecutable", "missing", "wrong-symlink", "directory", "same-copy"):
                # Never chmod the shared interpreter behind a symlink.
                if binary == "symlink" and fault == "nonexecutable":
                    continue
                with self.subTest(binary=binary, fault=fault):
                    result, _, observations = self.run_identity(mode="disk-drift:" + fault, binary_mode=binary)
                    expected = 0 if fault == "same-copy" else 2
                    self.assertEqual(result.returncode, expected, result.stderr)
                    self.assertIn(f"IDENTITY_STATUS={expected}".encode(), result.stdout)
                    self.assertEqual(observations, 2)

    def test_manager_diagnostics_refuse_either_running_identity_read(self):
        # These read the actual private child's /proc executable before and
        # after the unchanged byte comparison, not a simulated version file.
        for fault in ("stderr-warning", "stderr-blank", "stdout-blank", "stdout-unterminated"):
            for reading in (1, 2):
                with self.subTest(fault=fault, reading=reading):
                    r, _, observations = self.run_identity(mode=f"diagnostic:{fault}:{reading}")
                    self.assertEqual(r.returncode, 2, r.stderr)
                    self.assertIn(b"IDENTITY_STATUS=2", r.stdout)
                    self.assertEqual(observations, reading)
                    self.assertNotIn(b"private fixture manager diagnostic", r.stdout + r.stderr)

    def test_identity_read_timeout_is_bounded_and_owned_process_is_reaped(self):
        r, elapsed, observations = self.run_identity(mode="hang")
        self.assertEqual(r.returncode, 2)
        self.assertEqual(observations, 1)
        self.assertLess(elapsed, 8)


# Exclusion uses real file locks, independent of the manager double above.
LOCK_HARNESS = r'''
source "$UPDATER"
log() { printf '[%s] %s\n' "$1" "${*:2}"; }
detect_service_name() { echo pulse-fixture; }
read_update_service_state() {
    echo observed >> "$FIXTURE/manager-$LABEL"
    if [[ "$STATE_FAULT" == true ]]; then return 1; fi
    echo inactive
}
verify_release_signature() { :; }
download_update_asset() {
    echo downloaded >> "$FIXTURE/download-$LABEL"
    if [[ "$1" == *.sshsig ]]; then printf 'fixture signature\n' > "$2"
    else cp "$FIXTURE/installer" "$2"; fi
}
mktemp() {
    local -a args=("$@")
    local last=$((${#args[@]} - 1))
    if [[ "${args[last]}" == /tmp/* ]]; then
        args[last]="$FIXTURE/tmp/${args[last]##*/}"
    fi
    command mktemp "${args[@]}"
}
if [[ "$LOCK_FAULT" == unavailable ]]; then
    flock() { return 69; }
elif [[ "$LOCK_FAULT" == missing ]]; then
    command() {
        if [[ "$1" == -v && "$2" == flock ]]; then return 1; fi
        builtin command "$@"
    }
fi
status=0
perform_update v6.6.0 || status=$?
printf 'STATUS=%s\n' "$status"
[[ -z "$(trap -p RETURN)" ]] || { echo 'RETURN trap leaked'; exit 98; }
# Keep this caller alive after the function returned, so process exit cannot
# hide a leaked descriptor in either the early or full RETURN trap.
if [[ "$HOLD_RETURN" == true ]]; then
    touch "$FIXTURE/returned-$LABEL"
    for ((i=0; i<250; i++)); do
        [[ -f "$FIXTURE/release-return-$LABEL" ]] && break
        sleep 0.02
    done
    [[ -f "$FIXTURE/release-return-$LABEL" ]] || exit 97
fi
exit "$status"
'''

LOCK_DESCRIPTOR_PROBE = r'''
import errno, os
from pathlib import Path
installed = Path(os.environ["PULSE_INSTALL_DIR"]).stat()
descriptors = Path('/proc/self/fd') if Path('/proc/self/fd').is_dir() else Path('/dev/fd')
for item in descriptors.iterdir():
    try:
        descriptor = os.fstat(int(item.name))
    except OSError as error:
        # Directory enumeration can list its own already-closed descriptor.
        # On macOS, stat('/dev/fd/N') can also fail for a *live* pipe: inspect
        # the actual descriptor, not a pathname alias that must be reopened.
        if error.errno != errno.EBADF:
            raise
        continue
    if (descriptor.st_dev, descriptor.st_ino) == (installed.st_dev, installed.st_ino):
        raise SystemExit('installer inherited the update directory lock')
'''

LOCK_INSTALLER = r'''#!/bin/bash
set -euo pipefail
# A long-lived installer child must not inherit the updater's lock descriptor.
python3 - <<'CHECK'
''' + LOCK_DESCRIPTOR_PROBE + r'''
CHECK
printf '%s\n' "$LABEL" >> "$FIXTURE/entered"
if [[ "$BLOCK" == true ]]; then
    touch "$FIXTURE/ready"
    # Bounded even if its test controller fails; all owned children are reaped.
    for ((i=0; i<250; i++)); do
        [[ -f "$FIXTURE/release" ]] && break
        sleep 0.02
    done
    [[ -f "$FIXTURE/release" ]] || exit 97
fi
cp "$FIXTURE/new-pulse" "$PULSE_INSTALL_DIR/bin/pulse"
printf 'v6.6.0\n' > "$PULSE_INSTALL_DIR/VERSION"
exit "$INSTALLER_EXIT"
'''


class UpdateTransactionLock(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(shutil.which("flock"), "flock is required for the actual directory-lock controls")
        self.temporary = tempfile.TemporaryDirectory(prefix="pulse-update-lock-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.processes = []
        self.addCleanup(self.stop_children)
        self.first = self.fixture("first")

    def stop_children(self):
        for process in self.processes:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGKILL)
            process.communicate(timeout=5)

    def fixture(self, name):
        f = self.root / name
        for directory in (f / "install/bin", f / "config", f / "tmp"):
            directory.mkdir(parents=True)
        for path, content in ((f / "install/bin/pulse", "#!/bin/sh\necho 'Pulse v6.5.0'\n"),
                              (f / "new-pulse", "#!/bin/sh\necho 'Pulse v6.6.0'\n"),
                              (f / "installer", LOCK_INSTALLER)):
            path.write_text(content)
            path.chmod(0o700)
        (f / "install/VERSION").write_text("v6.5.0\n")
        (f / "config/system.json").write_text("saved settings\n")
        (f / "config/agent-id").write_text("saved identity\n")
        return f

    def start(self, fixture, label, *, block=False, install=None, installer_exit=0, lock_fault="", state_fault=False, hold_return=False):
        env = dict(os.environ, UPDATER=str(UPDATER), FIXTURE=str(fixture), LABEL=label,
                   BLOCK="true" if block else "false", HOLD_RETURN="true" if hold_return else "false",
                   INSTALLER_EXIT=str(installer_exit),
                   LOCK_FAULT=lock_fault, STATE_FAULT="true" if state_fault else "false", PULSE_INSTALL_DIR=str(install or fixture / "install"),
                   PULSE_CONFIG_DIR=str(fixture / "config"), PULSE_SERVICE_NAME="pulse-fixture")
        process = subprocess.Popen(["bash", "-c", LOCK_HARNESS], env=env, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, text=True, start_new_session=True)
        self.processes.append(process)
        return process

    def finish(self, process, expected):
        output, error = process.communicate(timeout=10)
        self.assertEqual(process.returncode, expected, output + error)
        self.assertIn(f"STATUS={expected}", output)
        self.assertNotIn("trap leaked", output)
        return output + error

    def wait_marker(self, path):
        deadline = time.monotonic() + 4
        while not path.exists() and time.monotonic() < deadline:
            time.sleep(0.02)
        self.assertTrue(path.exists(), "transaction did not reach " + path.name)

    def wait_ready(self):
        self.wait_marker(self.first / "ready")

    def release(self, process):
        (self.first / "release").touch()
        self.finish(process, 0)

    def assert_saved(self, fixture):
        self.assertEqual((fixture / "config/system.json").read_text(), "saved settings\n")
        self.assertEqual((fixture / "config/agent-id").read_text(), "saved identity\n")
        self.assertEqual(list((fixture / "tmp").iterdir()), [])

    def probe_descriptors(self, *, directory=None, fault=""):
        # Exercise the exact Python body embedded in the installer. Keep an
        # unrelated pipe open, including on macOS where its /dev/fd pathname
        # cannot necessarily be stat'ed, and optionally pass a real directory.
        reader, writer = os.pipe()
        descriptors = [reader, writer]
        if directory is not None:
            descriptors.append(os.open(directory, os.O_RDONLY))
        try:
            wrapper = r'''
import errno, os, sys
from pathlib import Path
from unittest.mock import patch
real_stat, real_iterdir, real_fstat = Path.stat, Path.iterdir, os.fstat
pipe_fd = int(os.environ['PROBE_PIPE_FD'])
fault = os.environ['PROBE_FAULT']
# A closed descriptor is a real EBADF, independent of this host's fd filesystem.
closed_fd = os.dup(pipe_fd)
os.close(closed_fd)
def entries(path):
    if str(path) in ('/proc/self/fd', '/dev/fd'):
        return iter([*(entry for entry in real_iterdir(path) if entry.name != str(closed_fd)),
                     path / str(closed_fd)])
    return real_iterdir(path)
def descriptor_path_stat(path, *args, **kwargs):
    if path.parent == Path('/proc/self/fd') or path.parent == Path('/dev/fd'):
        if path.name == str(pipe_fd):
            raise OSError(errno.EBADF, 'fixture: live pipe pathname cannot be reopened')
    return real_stat(path, *args, **kwargs)
def descriptor_stat(fd):
    if fault == 'unexpected' and fd == pipe_fd:
        raise OSError(errno.EIO, 'fixture: unexpected descriptor observation failure')
    return real_fstat(fd)
with patch.object(Path, 'iterdir', entries), patch.object(Path, 'stat', descriptor_path_stat), \
     patch.object(os, 'fstat', descriptor_stat):
    exec(sys.argv[1], {})
'''
            env = dict(os.environ, PULSE_INSTALL_DIR=str(self.first / "install"),
                       PROBE_PIPE_FD=str(reader), PROBE_FAULT=fault)
            return subprocess.run([sys.executable, "-c", wrapper, LOCK_DESCRIPTOR_PROBE],
                                  env=env, pass_fds=descriptors, capture_output=True, text=True, timeout=5)
        finally:
            for descriptor in descriptors:
                os.close(descriptor)

    def test_descriptor_probe_handles_live_pipes_and_closed_entries(self):
        result = self.probe_descriptors(directory=self.first / "tmp")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_descriptor_probe_rejects_real_inherited_installation_directory(self):
        result = self.probe_descriptors(directory=self.first / "install")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("installer inherited the update directory lock", result.stderr)

    def test_descriptor_probe_preserves_unexpected_observation_errors(self):
        result = self.probe_descriptors(fault="unexpected")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unexpected descriptor observation failure", result.stderr)

    def competing(self, alias=False):
        first = self.start(self.first, "holder", block=True)
        self.wait_ready()
        destination = self.first / "install"
        if alias:
            destination = self.root / "same directory alias"
            destination.symlink_to(self.first / "install", target_is_directory=True)
        second = self.start(self.first, "contender", install=destination)
        output = self.finish(second, 1)
        self.assertIn("lock", output)
        self.assertFalse((self.first / "manager-contender").exists())
        self.assertFalse((self.first / "download-contender").exists())
        self.assertEqual((self.first / "entered").read_text().splitlines(), ["holder"])
        self.assertEqual((self.first / "install/VERSION").read_text(), "v6.5.0\n")
        self.release(first)
        self.assertEqual((self.first / "install/VERSION").read_text(), "v6.6.0\n")
        self.assert_saved(self.first)

    def test_same_directory_refuses_competing_transaction(self):
        self.competing()

    def test_directory_alias_cannot_start_a_second_transaction(self):
        self.competing(alias=True)

    def test_independent_installations_do_not_block_each_other(self):
        first = self.start(self.first, "holder", block=True)
        self.wait_ready()
        independent = self.fixture("independent")
        self.finish(self.start(independent, "independent"), 0)
        self.assertIsNone(first.poll())
        self.assertEqual((independent / "install/VERSION").read_text(), "v6.6.0\n")
        self.release(first)
        self.assert_saved(self.first)
        self.assert_saved(independent)

    def test_lock_is_released_after_success_and_failed_installation(self):
        for status in (0, 23):
            with self.subTest(installer_exit=status):
                f = self.fixture("sequential-" + str(status))
                first = self.start(f, "first", installer_exit=status, hold_return=True)
                self.wait_marker(f / "returned-first")
                self.assertIsNone(first.poll())
                self.assertEqual((f / "install/VERSION").read_text(), "v6.6.0\n" if status == 0 else "v6.5.0\n")
                self.finish(self.start(f, "next"), 0)
                (f / "release-return-first").touch()
                self.finish(first, 0 if status == 0 else 1)
                self.assertEqual((f / "entered").read_text().splitlines(), ["first", "next"])
                self.assert_saved(f)

    def test_service_observation_refusal_releases_the_early_lock(self):
        first = self.start(self.first, "unobserved", state_fault=True, hold_return=True)
        self.wait_marker(self.first / "returned-unobserved")
        self.assertIsNone(first.poll())
        self.assertFalse((self.first / "download-unobserved").exists())
        self.assertFalse((self.first / "entered").exists())
        self.assertEqual((self.first / "install/VERSION").read_text(), "v6.5.0\n")
        self.finish(self.start(self.first, "next"), 0)
        (self.first / "release-return-unobserved").touch()
        self.finish(first, 1)
        self.assert_saved(self.first)

    def test_non_directory_or_missing_installation_never_starts_a_transaction(self):
        for destination in (self.first / "config/agent-id", self.first / "missing"):
            with self.subTest(destination=destination.name):
                self.finish(self.start(self.first, "refused", install=destination), 1)
                for marker in ("manager-refused", "download-refused", "entered"):
                    self.assertFalse((self.first / marker).exists(), marker)
                self.assertEqual((self.first / "install/VERSION").read_text(), "v6.5.0\n")
                self.assert_saved(self.first)

    def test_unavailable_or_missing_lock_has_no_service_or_installer_action(self):
        for fault in ("unavailable", "missing"):
            with self.subTest(fault=fault):
                f = self.fixture(fault)
                output = self.finish(self.start(f, "refused", lock_fault=fault), 1)
                self.assertIn("lock", output)
                for marker in ("manager-refused", "download-refused", "entered"):
                    self.assertFalse((f / marker).exists(), marker)
                self.assertEqual((f / "install/VERSION").read_text(), "v6.5.0\n")
                self.assert_saved(f)


class UpdateAssetPublication(unittest.TestCase):
    """Real optional asset writes, always confined to ordinary temporary files."""

    HARNESS = r'''
source "$INSTALLER"
record() { printf '%s\n' "$*" >> "$FIXTURE/operations"; }
print_info() { echo "$*"; }
print_warn() { echo "WARN: $*"; }
print_success() { echo "SUCCESS: $*"; }
safe_systemctl() { record "manager $*"; }
download_auto_update_script() { record unexpected-download; return 98; }
systemctl() { return 99; } # Presence only; inventory below never uses the host.
unit_file_inventory() {
    record inventory
    if [[ "$FAULT" == inventory ]]; then return 73; fi
    local state=disabled
    if [[ "$FAULT" == mask || ( "$FAULT" == late-mask && -e "$FIXTURE/rendered" ) ]]; then state=masked-runtime; fi
    printf 'pulse-update.timer %s\npulse-update.service disabled\n' "$state"
}
migrate_auto_update_assets_outside_sandbox() { record migration; return 1; }
last_arg() { for last; do :; done; printf '%s' "$last"; }
mktemp() {
    local path; path=$(last_arg "$@")
    record "allocate $path"
    case "$FAULT:$path" in
        allocate-helper:*.sh.staged.*|allocate-service:*.service.staged.*|allocate-timer:*.timer.staged.*) return 74 ;;
    esac
    command mktemp "$@"
}
cat_count=0
cat() {
    cat_count=$((cat_count + 1))
    record "render $cat_count"
    touch "$FIXTURE/rendered"
    if [[ "$FAULT" == "render-$cat_count" || ( "$FAULT" == cleanup && "$cat_count" == 1 ) ]]; then printf 'partial unit\n'; return 75; fi
    if [[ "$FAULT" == during-render-directory && "$cat_count" == 1 ]]; then
        command mv "$PULSE_UPDATE_SERVICE_PATH" "$FIXTURE/displaced-service"
        mkdir "$PULSE_UPDATE_SERVICE_PATH"
    fi
    command cat "$@"
}
chmod() {
    local path; path=$(last_arg "$@")
    record "mode $1 $path"
    case "$FAULT:$path" in
        mode-service:*.service.staged.*|mode-timer:*.timer.staged.*) return 76 ;;
    esac
    if [[ "$FAULT" == mode-helper && "$path" == *.sh.staged.* && "$path" != *.repo.* ]]; then return 76; fi
    command chmod "$@"
}
mv() {
    local path; path=$(last_arg "$@")
    record "publish $path"
    case "$FAULT:$path" in
        publish-helper:*/pulse-auto-update.sh|publish-service:*/pulse-update.service|publish-timer:*/pulse-update.timer) return 77 ;;
    esac
    if [[ "$FAULT" == after-admission-directory && "$path" == "$PULSE_UPDATE_SERVICE_PATH" ]]; then
        command mv "$path" "$FIXTURE/displaced-service"
        mkdir "$path"
    fi
    # All three live swaps must be exact non-directory renames; the separate
    # helper repository rewrite is also real but not one of these boundaries.
    if [[ "$STRICT_SWAP" == true && ( "$path" == "$PULSE_AUTO_UPDATE_DEST" || "$path" == "$PULSE_UPDATE_SERVICE_PATH" || "$path" == "$PULSE_UPDATE_TIMER_PATH" ) ]]; then
        [[ "$#" == 4 && "$1" == -fT && "$2" == -- && "$3" == "$4".staged.* ]] || return 96
        local wanted=644
        [[ "$path" != "$PULSE_AUTO_UPDATE_DEST" ]] || wanted=755
        [[ "$(python3 -c 'import os,stat,sys; print(oct(stat.S_IMODE(os.stat(sys.argv[1]).st_mode))[2:])' "$3")" == "$wanted" ]] || return 97
    fi
    command mv "$@"
}
rm() {
    record cleanup
    [[ "$FAULT" != cleanup ]] || return 78
    command rm "$@"
}
umask 077
status=0
# Real callers suppress errexit for the whole producer. Keep that context.
if [[ "$CALLER" == setup ]]; then
    ENABLE_AUTO_UPDATES=true
    selected_update_channel() { echo stable; }
    setup_auto_updates || status=$?
elif [[ "$CALLER" == refresh ]]; then
    refresh_auto_updates || status=$?
else
    install_auto_update_assets || status=$?
fi
printf 'RESULT=%s\n' "$status"
exit "$status"
'''

    def exercise(self, *, fault="none", odd=None, caller="assets", strict=False, foreign=False):
        installer = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh")).resolve()
        with tempfile.TemporaryDirectory(prefix="pulse-update-publication-") as directory:
            f = Path(directory)
            for name in ("install/scripts", "bin", "units", "config"):
                (f / name).mkdir(parents=True)
            paths = [f / "bin/pulse-auto-update.sh", f / "units/pulse-update.service", f / "units/pulse-update.timer"]
            original = [b"#!/bin/bash\necho old helper\n", b"previous complete service\n", b"previous complete timer\n"]
            for path, data in zip(paths, original):
                path.write_bytes(data)
                path.chmod(0o750 if path == paths[0] else 0o640)
            (f / "install/scripts/pulse-auto-update.sh").write_bytes(b"#!/bin/bash\necho new helper\n")
            saved = b'{"autoUpdateEnabled":false,"updateChannel":"beta","operatorSetting":"retained"}\n'
            (f / "config/system.json").write_bytes(saved)
            target = f / "unrelated"
            target.write_bytes(b"unrelated saved bytes\n")
            if odd:
                index, kind = odd
                path = paths[index]
                path.unlink()
                if kind == "directory": path.mkdir()
                elif kind == "fifo": os.mkfifo(path)
                elif kind == "file-link": path.symlink_to(target)
                elif kind == "directory-link": path.symlink_to(f / "config", target_is_directory=True)
                elif kind == "dangling": path.symlink_to(f / "absent")
                elif kind == "mask": path.symlink_to("/dev/null")
                else: raise ValueError(kind)
            if foreign:
                Path(str(paths[1]) + ".tmp").symlink_to(target)
                Path(str(paths[2]) + ".tmp").write_bytes(b"another invocation's temp bytes\n")
            env = dict(os.environ, FIXTURE=str(f), INSTALLER=str(installer), FAULT=fault, CALLER=caller,
                       STRICT_SWAP=str(strict).lower(), PULSE_INSTALL_DIR=str(f / "install"), PULSE_CONFIG_DIR=str(f / "config"),
                       PULSE_AUTO_UPDATE_DEST=str(paths[0]), PULSE_UPDATE_SERVICE_PATH=str(paths[1]),
                       PULSE_UPDATE_TIMER_PATH=str(paths[2]), GITHUB_REPO="rcourtman/Pulse", PULSE_SERVICE_NAME="pulse")
            result = subprocess.run(["bash", "-c", self.HARNESS], env=env, capture_output=True, text=True, timeout=15)
            snapshots = []
            for path in paths:
                kind = "link" if path.is_symlink() else ("directory" if path.is_dir() else ("file" if path.is_file() else "other"))
                snapshots.append(dict(kind=kind, data=path.read_bytes() if kind == "file" else None,
                                      mode=(path.stat().st_mode & 0o777) if kind == "file" else None,
                                      entries=sorted(p.name for p in path.iterdir()) if kind == "directory" else [],
                                      link=os.readlink(path) if kind == "link" else None))
            return dict(exit=result.returncode, output=result.stdout + result.stderr, snapshots=snapshots, original=original,
                        ops=(f / "operations").read_text().splitlines() if (f / "operations").exists() else [],
                        target=target.read_bytes(), config=(f / "config/system.json").read_bytes(), saved=saved,
                        stages=[p.name for parent in (f / "bin", f / "units") for p in parent.iterdir() if ".staged." in p.name],
                        displaced=(f / "displaced-service").read_bytes() if (f / "displaced-service").exists() else None,
                        foreign_link=Path(str(paths[1]) + ".tmp").is_symlink(),
                        foreign_data=Path(str(paths[2]) + ".tmp").read_bytes() if foreign and Path(str(paths[2]) + ".tmp").is_file() else None)

    def assert_preserved(self, evidence, except_index=None):
        for i, (snapshot, original) in enumerate(zip(evidence["snapshots"], evidence["original"])):
            if i != except_index:
                self.assertEqual(snapshot["data"], original, evidence)
                self.assertEqual(snapshot["mode"], 0o750 if i == 0 else 0o640, evidence)
        self.assertEqual(evidence["config"], evidence["saved"], evidence)
        self.assertEqual(evidence["target"], b"unrelated saved bytes\n", evidence)
        self.assertFalse(any(op.startswith("manager ") for op in evidence["ops"]), evidence)
        self.assertEqual(evidence["stages"], [], evidence)

    def test_nonregular_destinations_refuse_before_staging_or_helper_replacement(self):
        for index in range(3):
            for kind in ("directory", "fifo", "file-link", "directory-link", "dangling", "mask"):
                with self.subTest(asset=index, kind=kind):
                    e = self.exercise(odd=(index, kind))
                    self.assertNotEqual(e["exit"], 0, e)
                    self.assert_preserved(e, except_index=index)
                    self.assertFalse(any(op.startswith(("allocate ", "publish ", "render ", "mode ")) for op in e["ops"]), e)
                    if kind == "directory": self.assertEqual(e["snapshots"][index]["entries"], [], e)
                    if "link" in kind or kind in ("dangling", "mask"): self.assertEqual(e["snapshots"][index]["kind"], "link", e)

    def test_foreign_fixed_temp_paths_survive_private_staging_and_exact_renames(self):
        e = self.exercise(foreign=True, strict=True)
        self.assertEqual(e["exit"], 0, e)
        self.assertTrue(e["foreign_link"], e)
        self.assertEqual(e["target"], b"unrelated saved bytes\n", e)
        self.assertEqual(e["foreign_data"], b"another invocation's temp bytes\n", e)
        self.assertEqual([s["kind"] for s in e["snapshots"]], ["file"] * 3, e)
        self.assertEqual([s["mode"] for s in e["snapshots"]], [0o755, 0o644, 0o644], e)
        self.assertIn(b'GITHUB_REPO="rcourtman/Pulse"\n', e["snapshots"][0]["data"])
        self.assertIn(b"OnCalendar=*-*-* 02:00:00\n", e["snapshots"][2]["data"])
        self.assertEqual([line for line in e["snapshots"][2]["data"].splitlines() if line.startswith(b"OnCalendar=")], [b"OnCalendar=*-*-* 02:00:00"])
        self.assertEqual([op for op in e["ops"] if op.startswith("manager ")], ["manager daemon-reload"], e)
        self.assertEqual(e["config"], e["saved"], e)
        self.assertEqual(e["stages"], [], e)

    def test_preparation_failures_preserve_all_three_assets_and_preferences(self):
        for fault in ("allocate-helper", "allocate-service", "allocate-timer", "render-1", "render-2", "mode-helper", "mode-service", "mode-timer"):
            with self.subTest(fault=fault):
                e = self.exercise(fault=fault)
                self.assertNotEqual(e["exit"], 0, e)
                self.assert_preserved(e)

    def test_publication_failure_stops_without_claiming_a_three_file_rollback(self):
        for index, fault in enumerate(("publish-helper", "publish-service", "publish-timer")):
            with self.subTest(fault=fault):
                e = self.exercise(fault=fault)
                self.assertNotEqual(e["exit"], 0, e)
                for i, s in enumerate(e["snapshots"]):
                    if i < index: self.assertNotEqual(s["data"], e["original"][i], e)
                    else: self.assertEqual(s["data"], e["original"][i], e)
                self.assertEqual(e["config"], e["saved"], e)
                managers = [op for op in e["ops"] if op.startswith("manager ")]
                self.assertEqual(managers, ["manager daemon-reload"] if index == 2 else [], e)
                if index > 0: self.assertIn("publication is unconfirmed", e["output"], e)
                self.assertEqual(e["stages"], [], e)

    def test_changed_destinations_and_new_effective_masks_never_report_success(self):
        for fault in ("during-render-directory", "after-admission-directory", "late-mask"):
            with self.subTest(fault=fault):
                e = self.exercise(fault=fault)
                self.assertNotEqual(e["exit"], 0, e)
                self.assertFalse(any(op.startswith("manager ") for op in e["ops"]), e)
                self.assertEqual(e["stages"], [], e)
                self.assertEqual(e["config"], e["saved"], e)
                if fault != "late-mask":
                    self.assertEqual(e["snapshots"][1]["kind"], "directory", e)
                    self.assertEqual(e["snapshots"][1]["entries"], [], e)
                    self.assertEqual(e["displaced"], e["original"][1], e)
                    self.assertEqual(e["snapshots"][2]["data"], e["original"][2], e)
                if fault != "after-admission-directory":
                    self.assertEqual(e["snapshots"][0]["data"], e["original"][0], e)

    def test_cleanup_failure_retains_owned_stages_and_primary_failure(self):
        e = self.exercise(fault="cleanup")
        self.assertNotEqual(e["exit"], 0, e)
        self.assertEqual([s["data"] for s in e["snapshots"]], e["original"], e)
        self.assertEqual(e["config"], e["saved"], e)
        self.assertTrue(e["stages"], e)
        self.assertIn("Could not remove staged auto-update asset", e["output"], e)
        self.assertFalse(any(op.startswith("manager ") for op in e["ops"]), e)

    def test_both_optional_callers_complete_without_enable_or_start_after_refusal(self):
        for caller in ("setup", "refresh"):
            for fault, odd in (("none", (1, "directory")), ("mask", None), ("inventory", None), ("allocate-timer", None)):
                with self.subTest(caller=caller, fault=fault, odd=odd):
                    e = self.exercise(caller=caller, fault=fault, odd=odd)
                    self.assertEqual(e["exit"], 0, e) # Optional setup must not strand monitoring.
                    self.assert_preserved(e, except_index=1 if odd else None)
                    self.assertIn("WARN:", e["output"], e)
                    self.assertNotIn("SUCCESS:", e["output"], e)



class UpdateServiceDeadlines(unittest.TestCase):
    def exercise(self, budget="2", mode="active", *, read_only=False, late_timer=False, model_clock=False):
        with tempfile.TemporaryDirectory(prefix="pulse-updater-deadline-") as directory:
            f = Path(directory)
            (f / "tools").mkdir()
            (f / "manager.py").write_text(r'''
import json, os, signal, subprocess, sys, time
from pathlib import Path
f = Path(os.environ["FIXTURE"])
args = sys.argv[1:]
with (f / "calls").open("a") as out:
    out.write(json.dumps(args) + "\n")
mode = os.environ["DEADLINE_MODE"]
count = len((f / "calls").read_text().splitlines())
if mode == "stubborn":
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    child = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(60)"])
    (f / "owned").write_text(json.dumps({"group": os.getpgrp(), "pids": [os.getpid(), child.pid]}))
    # A plausible partial observation must not pass after the manager hangs.
    print("LoadState=loaded\nActiveState=active", flush=True)
    child.wait()
    sys.exit(97)
if mode.startswith("slow-"):
    time.sleep(1.35)
if mode == "remaining" and count > 1:
    time.sleep(4)
state = "active" if mode in ("active", "slow-active", "diagnostic", "error") or mode == "delayed" and count > 1 else "inactive"
print("LoadState=loaded\nActiveState=" + state)
if mode == "diagnostic":
    print("fixture manager diagnostic", file=sys.stderr)
if mode == "error":
    sys.exit(4)
''')
            (f / "tools/systemctl").write_text(
                "#!/bin/sh\nexec " + shlex.quote(sys.executable) + " -I -S "
                + shlex.quote(str(f / "manager.py")) + ' "$@"\n')
            (f / "tools/timeout").write_text(
                '#!/bin/sh\nprintf "%s\\n" "$*" >> "$FIXTURE/timeouts"\n'
                # Only the explicit late-success control models a timer that
                # returns a complete observation past its requested deadline.
                'if [ "$LATE_TIMER" = true ]; then sleep 1.35; shift 3; exec "$@"; fi\n'
                'exec "$REAL_TIMEOUT" "$@"\n')
            for tool in (f / "tools").iterdir():
                tool.chmod(0o700)
            originals = {f / "saved-settings": b"private fixture settings\n",
                         f / "saved-identity": b"private fixture identity\n"}
            for path, value in originals.items():
                path.write_bytes(value)
                path.chmod(0o600)
            env = {k: v for k, v in os.environ.items() if not k.startswith("PULSE_")}
            env.update(UPDATER=str(UPDATER), FIXTURE=str(f), BUDGET=budget,
                       DEADLINE_MODE=mode, LATE_TIMER="true" if late_timer else "false",
                       REAL_TIMEOUT=shutil.which("timeout"),
                       PATH=str(f / "tools") + os.pathsep + env["PATH"])
            function = "read_update_service_state" if read_only else "wait_for_service_active"
            script = ('set -euo pipefail\nsource "$UPDATER"\n'
                      'log() { printf "[%s] %s\\n" "$1" "${*:2}" >&2; }\n'
                      # Only the remaining-budget argument control uses a
                      # deterministic clock: real SECONDS has one-second
                      # quantization and can expire before a second probe.
                      + ('unset SECONDS; SECONDS=0\n'
                         'sleep() { SECONDS=$((SECONDS + $1)); }\n' if model_clock else '') +
                      'status=0\n' + function + ' pulse-deadline@fixture "$BUDGET" || status=$?\n'
                      'printf "STATUS=%s\\n" "$status"\nexit "$status"\n')
            start = time.monotonic()
            process = subprocess.Popen(["bash", "-c", script], cwd=f, env=env,
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                       text=True, start_new_session=True)
            exceeded = False
            try:
                stdout, stderr = process.communicate(timeout=8)
            except subprocess.TimeoutExpired:
                exceeded = True
                # Clean only these recorded fixture-owned groups, never a host
                # manager. EOF normally proves timeout killed both pipe holders.
                groups = [process.pid]
                if (f / "owned").exists():
                    groups.append(json.loads((f / "owned").read_text())["group"])
                for group in groups:
                    self.assertGreater(group, 1)
                    self.assertNotEqual(group, os.getpgrp())
                    try:
                        os.killpg(group, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                stdout, stderr = process.communicate(timeout=3)
            result = dict(code=process.returncode, elapsed=time.monotonic() - start,
                          stdout=stdout, stderr=stderr, exceeded=exceeded,
                          calls=[json.loads(line) for line in (f / "calls").read_text().splitlines()]
                          if (f / "calls").exists() else [],
                          timeouts=[shlex.split(line) for line in (f / "timeouts").read_text().splitlines()]
                          if (f / "timeouts").exists() else [],
                          owned=json.loads((f / "owned").read_text()) if (f / "owned").exists() else None)
            self.assertEqual({p: p.read_bytes() for p in originals}, originals)
            self.assertTrue(all(p.stat().st_mode & 0o777 == 0o600 for p in originals))
            self.assertFalse(exceeded, result)
            for args in result["calls"]:
                self.assertEqual(args, ["show", "pulse-deadline@fixture", "--no-pager",
                                        "--property=LoadState", "--property=ActiveState"])
            return result

    def test_slow_reads_count_toward_one_polling_budget(self):
        result = self.exercise(mode="slow-inactive")
        self.assertIn(result["code"], (1, 2), result)
        self.assertLess(result["elapsed"], 3.6, result)
        self.assertLessEqual(len(result["calls"]), 2, result)

    def test_complete_active_response_after_deadline_is_uncertain(self):
        result = self.exercise("1", late_timer=True)
        self.assertEqual(result["code"], 2, result)
        self.assertLess(result["elapsed"], 2.8, result)

    def test_last_manager_probe_uses_only_remaining_budget(self):
        result = self.exercise(mode="remaining", model_clock=True)
        self.assertEqual(result["code"], 2, result)
        self.assertLess(result["elapsed"], 3.6, result)
        self.assertEqual([args[2] for args in result["timeouts"]], ["2", "1"], result)

    def test_timeout_kills_partial_manager_and_pipe_holding_child(self):
        result = self.exercise("1", mode="stubborn")
        self.assertEqual(result["code"], 2, result)
        self.assertLess(result["elapsed"], 3.5, result)
        self.assertEqual(len(result["calls"]), 1, result)
        self.assertEqual(len(result["owned"]["pids"]), 2, result)
        self.assertEqual(result["timeouts"][0][:3], ["-k", "1", "1"], result)

    def test_fast_and_delayed_active_states_still_complete(self):
        for mode, calls in (("active", 1), ("delayed", 2)):
            with self.subTest(mode=mode):
                result = self.exercise("4", mode=mode)
                self.assertEqual(result["code"], 0, result)
                self.assertEqual(len(result["calls"]), calls, result)
                self.assertLess(result["elapsed"], 3.5, result)

    def test_settled_inactive_timeout_is_not_manager_uncertainty(self):
        result = self.exercise(mode="inactive")
        self.assertEqual(result["code"], 1, result)
        self.assertLess(result["elapsed"], 3.6, result)

    def test_invalid_budgets_never_query_the_manager(self):
        for budget in ("", "0", "-1", "1x", "1+1"):
            with self.subTest(budget=budget):
                result = self.exercise(budget)
                self.assertEqual(result["code"], 2, result)
                self.assertEqual(result["calls"], [], result)
                self.assertEqual(result["timeouts"], [], result)

    def test_read_budget_and_failed_observations_remain_closed(self):
        for budget in ("", "0", "6", "1+1"):
            with self.subTest(budget=budget):
                result = self.exercise(budget, read_only=True)
                self.assertNotEqual(result["code"], 0, result)
                self.assertEqual(result["calls"], [], result)
        for mode in ("diagnostic", "error"):
            with self.subTest(mode=mode):
                result = self.exercise("2", mode=mode)
                self.assertEqual(result["code"], 2, result)
                self.assertEqual(len(result["calls"]), 1, result)


if __name__ == "__main__":
    unittest.main()
