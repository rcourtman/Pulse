#!/usr/bin/env python3
"""Agent removal boundaries on a fresh public hosted systemd runner.

Use the committed installer functions unchanged, with fixed private unit names
and synthetic recovery files. systemctl, socket activation and deletion are real;
runner credential revocation is a closed fixture, not TLS/server acceptance.
This is not installation of a signed release or Windows/other-manager proof.
Never run this lab on a worker, self-hosted runner or an existing Pulse host.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import signal
import socket
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]
LAB = Path("/opt/pulse-agent-removal-native")
UNIT_DIR = Path("/etc/systemd/system")
AGENT = "pulse-removal-native-agent"
HELPER = "pulse-removal-native-helper"
RUNNER = "pulse-removal-native-runner"
UNITS = (RUNNER + ".service", HELPER + ".socket", HELPER + ".service", AGENT + ".service")
FUNCTIONS = ("read_agent_removal_unit_state", "confirm_agent_unit_quiescent",
             "quiesce_agent_units_for_removal", "teardown_systemd_agent_service",
             "teardown_privileged_helper_service", "teardown_action_runner_service")
PHASES = ("active-agent", "activated-helper", "confirmed-runner",
          "refused-agent-stop", "refused-helper-stop", "unavailable-manager",
          "refused-runner-revocation", "batch-reactivation", "absent-and-repeated")


def definitions(source):
    result = []
    for name in FUNCTIONS:
        marker = name + "() {"
        if source.count(marker) != 1:
            raise ValueError("installer function boundary changed")
        body = source.split(marker, 1)[1]
        if "\n}\n" not in body:
            raise ValueError("installer function end missing")
        result.append(marker + body.split("\n}\n", 1)[0] + "\n}\n")
    return "\n".join(result)


def admission(env, uid, source):
    if (uid != 0 or env.get("GITHUB_ACTIONS") != "true"
            or env.get("RUNNER_ENVIRONMENT") != "github-hosted"
            or env.get("GITHUB_REPOSITORY") != "rcourtman/Pulse"
            or not re.fullmatch(r"[0-9a-f]{40}", source)
            or any(Path(p).exists() for p in
                   ("/.dockerenv", "/opt/pulse", "/etc/pulse", "/etc/pulse-agent",
                    "/etc/pulse-agent-runner", "/var/lib/pulse-agent", LAB))):
        raise ValueError("agent removal proof requires a fresh public hosted VM")


def state(output):
    values = {}
    for line in output.decode("ascii").splitlines():
        key, separator, value = line.partition("=")
        if not separator or key not in {"LoadState", "ActiveState", "UnitFileState", "MainPID"} or key in values:
            raise ValueError("independent unit observation incomplete")
        values[key] = value
    if not {"LoadState", "ActiveState", "UnitFileState"} <= values.keys():
        raise ValueError("independent unit observation missing")
    return values


def pid_alive(pid):
    if not re.fullmatch(r"[1-9][0-9]*", pid):
        raise ValueError("native process identity unavailable")
    try:
        os.kill(int(pid), 0)
    except ProcessLookupError:
        return False
    # Permission/read errors stay failures; never manufacture process absence.
    return True


class Lab:
    def __init__(self, source, installer):
        self.source = source
        self.definitions = definitions(installer)
        self.owned_units = set()
        self.owned_pids = set()
        self.owned_root = False
        self.env = {"PATH": "/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL": "C", "HOME": str(LAB)}
        self.receipt = {"schema_version": 1, "source_sha": source,
                        "scope": "current-source-native-systemd-removal-boundary",
                        "installer_sha256": hashlib.sha256(installer.encode()).hexdigest(),
                        "definitions_sha256": hashlib.sha256(self.definitions.encode()).hexdigest(),
                        "credential_transport": "closed-synthetic-revocation-only",
                        "phases": [], "cleanup": {"complete": False}}

    def run(self, *args, ok=(0,), timeout=20):
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
            self.receipt["command_failure"] = {"deadline_exceeded": True,
                                                "observed_exit": process.returncode}
            raise RuntimeError("owned command deadline exceeded") from None
        result = subprocess.CompletedProcess(args, process.returncode, output)
        if result.returncode not in ok:
            self.receipt["command_failure"] = {"command": Path(args[0]).name,
                                                "observed_exit": result.returncode,
                                                "output_bytes": len(output),
                                                "output_sha256": hashlib.sha256(output).hexdigest()}
            raise RuntimeError("native command failed")
        return result

    def observe(self):
        observed = {unit: state(self.run("systemctl", "show", unit,
                                         "--property=LoadState,ActiveState,UnitFileState,MainPID").stdout)
                    for unit in UNITS}
        for unit in self.owned_units:
            pid = observed[unit].get("MainPID", "0")
            if pid != "0":
                if not re.fullmatch(r"[1-9][0-9]*", pid):
                    raise ValueError("native process identity unavailable")
                self.owned_pids.add(pid)
        return observed

    def prepare(self):
        # Reject loaded or vendor-provided units as well as on-disk collisions.
        if any(Path(UNIT_DIR / unit).exists() or Path(UNIT_DIR / unit).is_symlink() for unit in UNITS):
            raise ValueError("native unit name already owned")
        if any(v["LoadState"] != "not-found" for v in self.observe().values()):
            raise ValueError("native unit already loaded")
        LAB.mkdir(mode=0o700)
        self.owned_root = True

    def install_unit(self, unit, text):
        if unit not in UNITS:
            raise ValueError("unit outside fixed lab")
        with (UNIT_DIR / unit).open("x") as stream:
            self.owned_units.add(unit)
            stream.write(text)
        self.run("systemctl", "daemon-reload")
        self.run("systemctl", "enable", "--now", unit)

    def service(self, unit, extra=""):
        self.install_unit(unit, "[Unit]\nDescription=Pulse owned removal fixture\n" +
                          ("RefuseManualStop=yes\n" if extra == "refuse" else "") +
                          "[Service]\nType=simple\nExecStart=/usr/bin/sleep infinity\n" +
                          ("ExecStop=/usr/bin/systemctl start " + RUNNER + ".service\n"
                           if extra == "reactivate" else "") +
                          "[Install]\nWantedBy=multi-user.target\n")

    def helper(self, directory, refuse=False):
        # The pending connection genuinely starts the paired service. The
        # synthetic process does not implement the privileged-helper protocol.
        with (UNIT_DIR / (HELPER + ".service")).open("x") as stream:
            self.owned_units.add(HELPER + ".service")
            stream.write("[Service]\nExecStart=/usr/bin/sleep infinity\n")
        self.install_unit(HELPER + ".socket",
                          "[Unit]\nDescription=Pulse owned socket fixture\n" +
                          ("RefuseManualStop=yes\n" if refuse else "") +
                          "[Socket]\nListenStream=" + str(directory / "helper.sock") +
                          "\n[Install]\nWantedBy=sockets.target\n")
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as connection:
            connection.settimeout(2)
            connection.connect(str(directory / "helper.sock"))
        deadline = time.monotonic() + 5
        while self.observe()[HELPER + ".service"]["ActiveState"] != "active":
            if time.monotonic() >= deadline:
                raise RuntimeError("real helper socket activation was not observed")
            time.sleep(0.05)

    def shell(self, directory, command, *, revoke=True, unavailable=False):
        values = {"AGENT_NAME": AGENT, "PRIVILEGED_HELPER_NAME": HELPER, "ACTION_RUNNER_NAME": RUNNER,
                  "PRIVILEGED_HELPER_SOCKET_UNIT": UNIT_DIR / (HELPER + ".socket"),
                  "PRIVILEGED_HELPER_SERVICE_UNIT": UNIT_DIR / (HELPER + ".service"),
                  "PRIVILEGED_HELPER_SOCKET_PATH": directory / "helper.sock",
                  "PRIVILEGED_HELPER_CREDENTIAL_DIR": directory / "helper-credentials",
                  "ACTION_RUNNER_SERVICE_UNIT": UNIT_DIR / (RUNNER + ".service"),
                  "ACTION_RUNNER_BINARY_PATH": directory / "runner",
                  "ACTION_RUNNER_CONFIG_DIR": directory / "runner-config",
                  "ACTION_RUNNER_STATE_DIR": directory / "runner-state"}
        prefix = "set -euo pipefail\nEXIT_GENERAL=1\n" + "\n".join(
            key + "=" + shlex.quote(str(value)) for key, value in values.items()) + "\n"
        prefix += 'fail() { exit "${2:-1}"; }\nlog_info() { :; }\nlog_error() { :; }\n'
        prefix += "revoke_action_runner_credential() { printf 'attempted\\n' >> " + shlex.quote(str(directory / "revoke")) + "; " + ("return 0" if revoke else "return 1") + "; }\n"
        if unavailable:
            # This adapter directs the real systemctl to a deliberately absent
            # user manager. It proves read-failure retention, not a system-bus
            # incident. No failed system-manager command is replayed.
            # timeout execs its target; a shell function would not intercept it.
            tools = directory / "unavailable-manager-tools"
            tools.mkdir()
            manager = shutil.which("systemctl", path=self.env["PATH"])
            if manager is None:
                raise ValueError("native manager executable unavailable")
            adapter = tools / "systemctl"
            adapter.write_text("#!/bin/sh\nexec env XDG_RUNTIME_DIR=" +
                               shlex.quote(str(directory / "no-manager")) +
                               " DBUS_SESSION_BUS_ADDRESS=" + shlex.quote("unix:path=" + str(directory / "no-bus")) +
                               " " + shlex.quote(manager) + ' --user "$@"\n')
            adapter.chmod(0o700)
            prefix += "PATH=" + shlex.quote(str(tools)) + ':"$PATH"\n'
        return self.run("/bin/bash", "-c", prefix + self.definitions + command, ok=(0, 1), timeout=40)

    def snapshot(self, directory):
        paths = [directory / "runner"] + [UNIT_DIR / unit for unit in self.owned_units]
        for name in ("helper-credentials", "runner-config", "runner-state"):
            paths.extend((directory / name).rglob("*"))
        return {str(path): hashlib.sha256(path.read_bytes()).hexdigest()
                for path in paths if path.is_file() and path.name != "revoke"}

    def exercise(self, phase):
        directory = LAB / phase
        directory.mkdir(mode=0o700)
        for name in ("helper-credentials", "runner-config", "runner-state"):
            (directory / name).mkdir(mode=0o700)
            (directory / name / "token").write_text("synthetic-recovery-material\n")
        (directory / "runner").write_text("synthetic-runner-not-executable\n")
        if phase == "batch-reactivation":
            self.service(RUNNER + ".service")
            self.helper(directory)
            self.service(AGENT + ".service", "reactivate")
        elif "helper" in phase:
            self.helper(directory, phase == "refused-helper-stop")
        elif "runner" in phase:
            self.service(RUNNER + ".service")
        elif phase != "absent-and-repeated":
            self.service(AGENT + ".service", "refuse" if phase == "refused-agent-stop" else "")
        before, files_before = self.observe(), self.snapshot(directory)
        for unit, observed in before.items():
            if unit in self.owned_units:
                if (observed["LoadState"] != "loaded" or observed["ActiveState"] != "active"
                        or observed["UnitFileState"] not in {"enabled", "static"}
                        or (unit.endswith(".service") and not pid_alive(observed.get("MainPID", "0")))):
                    raise RuntimeError("intended active fixture was not observed")
            elif observed["LoadState"] != "not-found" or observed["ActiveState"] != "inactive":
                raise RuntimeError("unowned native unit appeared")
        agent_command = "teardown_systemd_agent_service " + shlex.quote(str(UNIT_DIR / (AGENT + ".service")))
        if phase == "batch-reactivation":
            command = "quiesce_agent_units_for_removal " + " ".join(UNITS) + " || fail refused 1\nprintf mutation > " + shlex.quote(str(directory / "mutation"))
        elif phase == "absent-and-repeated":
            command = (agent_command + "\nteardown_privileged_helper_service\nteardown_action_runner_service\n") * 2
        elif "helper" in phase:
            command = "teardown_privileged_helper_service"
        elif "runner" in phase:
            command = "teardown_action_runner_service"
        else:
            command = agent_command
        result = self.shell(directory, command, revoke=phase != "refused-runner-revocation",
                            unavailable=phase == "unavailable-manager")
        after = self.observe()
        refused = phase.startswith("refused-") or phase in {"unavailable-manager", "batch-reactivation"}
        record = {"name": phase, "command_exit": result.returncode, "before": before, "after": after,
                  "recovery_files_unchanged": self.snapshot(directory) == files_before,
                  "revocation_attempted": (directory / "revoke").exists(),
                  "revocation_attempts": len((directory / "revoke").read_text().splitlines()) if (directory / "revoke").exists() else 0,
                  "prior_process_alive": {unit: pid_alive(v["MainPID"]) for unit, v in before.items()
                                          if v.get("MainPID", "0") != "0"}, "passed": False}
        self.receipt["phases"].append(record)
        if result.returncode != (1 if refused else 0):
            raise RuntimeError("native removal verdict differs")
        if refused:
            if not record["recovery_files_unchanged"] or (directory / "mutation").exists():
                raise RuntimeError("unconfirmed removal erased recovery material")
            if phase in {"refused-agent-stop", "unavailable-manager"} and (
                    after[AGENT + ".service"]["ActiveState"] != "active" or
                    before[AGENT + ".service"]["MainPID"] != after[AGENT + ".service"]["MainPID"] or
                    not record["prior_process_alive"][AGENT + ".service"]):
                raise RuntimeError("refused collector changed process identity")
            if phase == "batch-reactivation" and after[RUNNER + ".service"]["ActiveState"] != "active":
                raise RuntimeError("batch reactivation adverse control did not occur")
            if phase == "refused-runner-revocation" and (
                    not record["revocation_attempted"] or after[RUNNER + ".service"]["ActiveState"] != "inactive" or
                    after[RUNNER + ".service"]["UnitFileState"] != "disabled"):
                raise RuntimeError("revocation refusal lacks stopped retained runner")
        else:
            targets = (UNITS if phase == "absent-and-repeated" else
                       (HELPER + ".socket", HELPER + ".service") if "helper" in phase else
                       (RUNNER + ".service",) if "runner" in phase else (AGENT + ".service",))
            for unit in targets:
                if (after[unit]["LoadState"] != "not-found" or after[unit]["ActiveState"] != "inactive"
                        or after[unit].get("MainPID", "0") != "0" or (UNIT_DIR / unit).exists()):
                    raise RuntimeError("native successful removal left unit or process")
                if record["prior_process_alive"].get(unit, False):
                    raise RuntimeError("native successful removal left its prior process alive")
            if "helper" in phase and any((directory / n).exists() for n in ("helper.sock", "helper-credentials")):
                raise RuntimeError("helper recovery material not removed after confirmation")
            if "runner" in phase and (not record["revocation_attempted"] or
                    any((directory / n).exists() for n in ("runner", "runner-config", "runner-state"))):
                raise RuntimeError("confirmed runner removal incomplete")
            if phase == "absent-and-repeated" and record["revocation_attempts"] != 1:
                raise RuntimeError("repeated absent removal repeated synthetic revocation")
        record["passed"] = True

    def cleanup_units(self):
        # Withdraw only our own refusal/ExecStop fixtures before stopping them.
        # Never repair, stop or unlink a pre-existing unit.
        for unit in self.owned_units:
            path = UNIT_DIR / unit
            if path.exists():
                text = path.read_text()
                text = re.sub(r"(?m)^(RefuseManualStop|ExecStop)=.*\n", "", text)
                path.write_text(text)
        if self.owned_units:
            self.run("systemctl", "daemon-reload")
        for unit in UNITS:
            if unit in self.owned_units:
                self.run("systemctl", "disable", unit, ok=(0, 1))
                self.run("systemctl", "stop", unit, ok=(0, 5))
                (UNIT_DIR / unit).unlink(missing_ok=True)
        if self.owned_units:
            self.run("systemctl", "daemon-reload")
            self.run("systemctl", "reset-failed", *sorted(self.owned_units), ok=(0, 1))
            if any(v["LoadState"] != "not-found" or v["ActiveState"] != "inactive" or v.get("MainPID", "0") != "0"
                   for v in self.observe().values()):
                raise RuntimeError("owned native unit cleanup incomplete")
            if any(pid_alive(pid) for pid in self.owned_pids):
                raise RuntimeError("owned native process cleanup incomplete")
        self.owned_units.clear()
        self.owned_pids.clear()

    def cleanup(self):
        self.cleanup_units()
        if self.owned_root:
            shutil.rmtree(LAB)
            self.owned_root = False
        self.receipt["cleanup"] = {"complete": True, "owned_units_absent": True,
                                    "owned_fixture_absent": not LAB.exists()}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--receipt", required=True)
    args = parser.parse_args()
    source = os.environ.get("EXPECTED_SOURCE_SHA", "")
    admission(os.environ, os.geteuid(), source)
    receipt = Path(args.receipt).resolve()
    if receipt.parent != Path(os.environ["RUNNER_TEMP"]).resolve() or receipt.exists():
        raise ValueError("receipt must be a new direct runner-temp file")
    # The ordinary runner checks the exact clean checkout before sudo. Do not
    # change root's Git ownership/configuration to read a runner-owned checkout.
    # Bind the actual installer and extracted bytes in the terminal receipt too.
    lab = Lab(source, (ROOT / "scripts/install.sh").read_text())
    failed = False
    try:
        lab.prepare()
        for phase in PHASES:
            lab.exercise(phase)
            lab.cleanup_units()
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        failed = True
        lab.receipt["failure"] = {"type": type(error).__name__}
    finally:
        try:
            lab.cleanup()
        except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
            failed = True
            lab.receipt["cleanup"] = {"complete": False, "error_type": type(error).__name__}
        receipt.write_text(json.dumps(lab.receipt, indent=2) + "\n")
    if failed or len(lab.receipt["phases"]) != len(PHASES):
        raise SystemExit("Native agent removal proof incomplete; inspect the bounded receipt.")
    print("NATIVE_AGENT_REMOVAL_COMPLETE=" + source)


if __name__ == "__main__":
    main()
