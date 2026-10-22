#!/usr/bin/env python3
"""Exercise the real updater with private files and a confined manager double.

No host manager, network, application build or published installer is used.
The versioned executable and signature verifier are synthetic; existing Go
transaction tests separately exercise real local SSH signatures and metadata.
"""

import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
UPDATER = Path(os.environ.get("PULSE_AUTO_UPDATER_UNDER_TEST", ROOT / "scripts/pulse-auto-update.sh")).resolve()

MANAGER = r'''
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
    if after and mode == "wait-read-hang":
        p = f / "wait-reads"
        count = int(p.read_text()) + 1 if p.exists() else 1
        p.write_text(str(count))
        if count == 2:
            time.sleep(8)  # Finite; the remaining one-second read must time out.
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


def write_manager(root):
    # The native runner selected this interpreter. Use its isolated stdlib
    # instead of paying site startup on every synthetic systemctl observation.
    model = root / "manager-model.py"
    model.write_text(MANAGER)
    tool = root / "tools/systemctl"
    tool.write_text("#!/bin/sh\nexec " + shlex.quote(sys.executable)
                    + " -I -S " + shlex.quote(str(model)) + ' "$@"\n')
    tool.chmod(0o755)


class UpdaterTimingResult(unittest.TextTestResult):
    def startTest(self, test):
        self.started = time.monotonic()
        if not hasattr(self, "suite_started"):
            self.suite_started = self.started
        self.subcases = 0
        self.progress("start", test)
        super().startTest(test)

    def addSubTest(self, test, subtest, err):
        self.subcases += 1
        super().addSubTest(test, subtest, err)

    def stopTest(self, test):
        super().stopTest(test)
        self.progress("complete", test)

    def progress(self, phase, test):
        now = time.monotonic()
        self.stream.writeln("Updater fixture timing: " + json.dumps(dict(
            phase=phase, control=test._testMethodName, completed_subcases=self.subcases,
            suite_seconds=round(now - self.suite_started, 6),
            control_seconds=round(now - self.started, 6)), sort_keys=True))
        self.stream.flush()

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
sleep() {
    # The deadline consumer control advances to the final second of the real
    # twenty-second wait. Manager execution and timeout remain real.
    if [[ "$MODE" == wait-read-hang ]]; then SECONDS=$((SECONDS + 19))
    else SECONDS=$((SECONDS + $1)); fi
}
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
            write_manager(f)
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
            self.assertEqual(args[:10], ["--disable", "-fsSL", "--connect-timeout", "5",
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

    def test_real_logger_failure_drains_installer_and_preserves_recovery(self):
        # More than a pipe buffer, plus an unterminated final line. These run
        # production log(), perform_update and its collector together: a green
        # log() double cannot establish that a real sink failure is retained.
        lines = ["opening"] + ["x" * 2048 for _ in range(64)] + ["final installer evidence"]
        for state in ("active", "inactive"):
            for failures in ({}, {1: 73}, {1: 73, 2: 42}, {len(lines): 73}):
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
                    expected = ("Installer pipeline failed (installer exit: 0; log collector exit: 73)"
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
    def test_manager_uses_selected_isolated_interpreter(self):
        with tempfile.TemporaryDirectory(prefix="updater model ") as directory:
            root = Path(directory)
            (root / "tools").mkdir()
            invocations = root / "arguments"
            interpreter = root / "selected python"
            interpreter.write_text('#!/bin/sh\nprintf "%s\\n" "$@" >> '
                                   + shlex.quote(str(invocations)) + '\nexec '
                                   + shlex.quote(sys.executable) + ' "$@"\n')
            interpreter.chmod(0o700)
            with mock.patch.object(sys, "executable", str(interpreter)):
                write_manager(root)
            (root / "sitecustomize.py").write_text(
                'from pathlib import Path\nPath(' + repr(str(root / "site-ran")) + ').touch()\n')
            (root / "state").write_text("active\n")
            env = dict(os.environ, FIXTURE=str(root), PYTHONPATH=str(root), MODE="normal",
                       LOAD_STATE="loaded", OBSERVATION_PHASE="", OBSERVATION_FAULT="")
            args = ["show", "pulse-custom@department", "--no-pager",
                    "--property=LoadState", "--property=ActiveState"]
            result = subprocess.run([str(root / "tools/systemctl"), *args], env=env,
                                    capture_output=True, text=True, timeout=5)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(result.stdout, "ActiveState=active\nLoadState=loaded\n")
            self.assertEqual(result.stderr, "")
            self.assertEqual(invocations.read_text().splitlines(), [
                "-I", "-S", str(root / "manager-model.py"), *args])
            self.assertFalse((root / "site-ran").exists())
            self.assertEqual([json.loads(line) for line in (root / "calls").read_text().splitlines()], [args])

    def test_timing_reports_control_and_cumulative_progress(self):
        import io

        class Control(unittest.TestCase):
            def test_control(self):
                with self.subTest():
                    self.assertTrue(True)

        stream = io.StringIO()
        result = unittest.TextTestRunner(stream=stream, verbosity=2, resultclass=UpdaterTimingResult).run(
            unittest.defaultTestLoader.loadTestsFromTestCase(Control))
        self.assertTrue(result.wasSuccessful())
        records = [json.loads(line.split("timing: ", 1)[1])
                   for line in stream.getvalue().splitlines() if line.startswith("Updater fixture timing: ")]
        self.assertEqual([r["phase"] for r in records], ["start", "complete"])
        self.assertEqual([r["completed_subcases"] for r in records], [0, 1])
        for record in records:
            self.assertEqual(record["control"], "test_control")
            self.assertGreaterEqual(record["suite_seconds"], record["control_seconds"])
        self.assertGreaterEqual(records[-1]["control_seconds"], records[0]["control_seconds"])

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

    def test_startup_deadline_uncertainty_preserves_completed_install_and_backup(self):
        r = self.run_fixture(mode="wait-read-hang", state="active", success=True)
        self.assertEqual(r["exit"], 1, r)
        self.assertLess(r["elapsed"], 4, r)
        self.assertTrue(r["installed"] and r["installer_finished"], r)
        self.assertFalse(r["started"] or r["stopped"], r)
        self.assertEqual(len(r["calls"]), 3, r)  # Initial state and two wait reads; no replay.
        self.assertEqual(r["runtime_probes"], [], r)
        self.assertEqual(r["state"], "inactive", r)
        self.assertEqual(r["binary"], b"#!/bin/sh\necho 'Pulse v6.6.0'\n", r)
        self.assertEqual(r["version"], "v6.6.0\n", r)
        self.assertEqual(r["backups"], [b"#!/bin/sh\necho 'Pulse v6.5.0'\n"], r)
        self.assertIn("Update service state is uncertain; backup retained", r["output"])
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
import os, sys, time
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
            tool = f / "tools/systemctl"; tool.write_text(PROCESS_MANAGER); tool.chmod(0o700)
            # Run the copied executable, then replace its path: /proc/PID/exe
            # still observes the actual old executable, not the new path bytes.
            child = subprocess.Popen([str(installed), "-c", "import time; print('ready', flush=True); time.sleep(30)"],
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
                           PATH=str(f / "tools") + os.pathsep + os.environ["PATH"])
                script = ('set -euo pipefail\nsource "$UPDATER"\nlog() { :; }\n'
                          'status=0\nverify_update_running_executable pulse-custom@department || status=$?\n'
                          'printf "IDENTITY_STATUS=%s\\n" "$status"\nexit "$status"\n')
                started = time.monotonic()
                r = subprocess.run(["bash", "-c", script], env=env, capture_output=True, timeout=10)
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

if __name__ == "__main__":
    unittest.main(testRunner=unittest.TextTestRunner(verbosity=2, resultclass=UpdaterTimingResult))
