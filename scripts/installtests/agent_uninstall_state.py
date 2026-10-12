#!/usr/bin/env python3
"""Execute the Unix uninstall admission and initial teardown, not a root installer.

Production credential detection, record confirmation and directory-authority
functions remain intact. Only transport and service/process operations are
private doubles; deletion is confined to three owned temporary directories.
The argument parser and token-file ingress also execute before the removal gate.
Root-trust is projected onto the private fixture owner for the shell reader;
these controls are not native installation, TLS or provider acceptance.
"""

import os
from pathlib import Path
import shlex
import stat
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_AGENT_UNINSTALL_SOURCE", ROOT / "scripts/install.sh"))


def function(source, name):
    return name + "() {" + source.split(name + "() {", 1)[1].split("\n}\n", 1)[0] + "\n}\n"


def snapshot(root):
    result = {}
    for path in sorted(root.rglob("*")):
        if path.name == "events":
            continue
        mode = path.lstat().st_mode
        payload = os.readlink(path) if stat.S_ISLNK(mode) else (
            path.read_bytes() if stat.S_ISREG(mode) and mode & stat.S_IRUSR else None)
        result[str(path.relative_to(root))] = (mode, payload, path.lstat().st_size, path.lstat().st_ino)
    return result


class UnixUninstallState(unittest.TestCase):
    source = SOURCE.read_text()

    def exercise(self, credential="runtime", url="", reply="confirmed", authority=True,
                 input_token=False, interactive=False, root_refusal=False, action="uninstall",
                 ingress_only=False, quiescence_refused=False):
        source = self.source
        # Keep the real entry and all initial credential/service/state teardown.
        # Stop before unrelated platform boot/log cleanup; never run it on a host.
        entry = source.split("# --- Uninstall Logic ---\n", 1)[1].split(
            "    # Remove least-privilege helper artifacts.", 1)[0] + "fi\n"
        names = ("collector_credential_state_present", "uninstall_collector_registration",
                 "remove_authorized_runtime_dir", "remove_agent_state_dir",
                 "remove_privileged_helper_state_dir", "remove_safe_profile_state_dir", "fail")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            state, helper, profile = (root / name for name in ("state", "helper", "profile"))
            for directory in (state, helper, profile):
                directory.mkdir(mode=0o700)
                (directory / "identity").write_text("owned-instance\n")
            (state / "connection.env").write_text("PULSE_AGENT_ID='owned-agent'\n")
            (state / "agent-id").write_text("owned-agent\n")
            service = root / "service"
            service.write_text("active\n")
            (root / "unrelated").write_text("do not change\n")
            token = ""
            selected = root / "selected-token"
            if credential == "inline":
                token = "synthetic-collector-credential"
            elif credential != "none":
                path = {"legacy": state / "token", "selected": selected,
                        "helper": helper / "token"}.get(credential, state / "runtime.token")
                if credential == "broken-link":
                    path.symlink_to(root / "missing-token")
                elif credential == "fifo":
                    os.mkfifo(path, 0o600)
                else:
                    path.write_text("" if credential == "empty" else "synthetic-collector-credential\n")
                    path.chmod(0 if credential == "unreadable" else 0o600)
            before = snapshot(root)
            ingress = ""
            arguments = []
            if input_token:
                parser = source.split("# --- Parse Arguments ---\n", 1)[1].split(
                    '\nif [[ "$RETARGET_ONLY"', 1)[0]
                token_reader = source.split("# Read token from file if --token-file was provided\n", 1)[1].split(
                    '\nif [[ -n "$ACTION_TOKEN_FILE_PATH" ]]', 1)[0]
                ingress = parser + "\n" + token_reader
                arguments = {"uninstall": ["--uninstall"], "install": [],
                             "update": ["--update"], "runner": ["--uninstall-action-runner"]}[action]
                arguments += ["--token-file", str(path)]
                if not interactive:
                    arguments.append("--non-interactive")
                if url:
                    arguments += ["--url", url]
                if root_refusal:
                    if os.geteuid() == 0:
                        self.fail("root-refusal control requires the ordinary proof user")
                    ingress += "\n" + source.split("# --- Check Root ---\n", 1)[1].split(
                        '\nif [[ "$SAFE_PROFILE_ACTION" == "inspect" ]]', 1)[0]
            values = {"STATE_DIR": state, "PRIVILEGED_HELPER_STATE_DIR": helper,
                      "PRIVILEGED_HELPER_CREDENTIAL_DIR": helper, "SAFE_PROFILE_STATE_DIR": profile,
                      "STATE_DIR_REMOVAL_AUTHORITY": state if authority else "",
                      "PRIVILEGED_HELPER_STATE_DIR_REMOVAL_AUTHORITY": helper,
                      "SAFE_PROFILE_STATE_DIR_REMOVAL_AUTHORITY": profile,
                      "PULSE_TOKEN": token, "RUNTIME_TOKEN_FILE": selected,
                      "PULSE_URL": url, "AGENT_ID": "owned-agent", "HOSTNAME_OVERRIDE": "owned-host",
                      "EVENTS": root / "events", "SERVICE": service, "REPLY": reply,
                      "QUIESCENCE_REFUSED": str(quiescence_refused).lower()}
            setup = "\n".join(f"{key}={shlex.quote(str(value))}" for key, value in values.items())
            functions = "\n".join(function(source, name) for name in names)
            if input_token:
                functions += "\n" + function(source, "read_collector_token_file_safely")
            # A whitelist protects the fixture even if production cleanup changes.
            shell = r'''
set -euo pipefail
UNINSTALL=true AGENT_NAME=pulse-agent EXIT_GENERAL=1 EXIT_MISSING_ARGS=16
ACTION_RUNNER_NAME=pulse-agent-runner PRIVILEGED_HELPER_NAME=pulse-agent-helper
OUTPUT_FORMAT=text NON_INTERACTIVE=true
TOKEN_FILE_PATH="" UNINSTALL_ACTION_RUNNER=false UPDATE_ONLY=false
RETARGET_ONLY=false PREFLIGHT_ONLY=false SAFE_PROFILE_ACTION=""
log_info() { :; }
log_warn() { printf 'warning:%s\n' "$*"; }
log_error() { printf 'error:%s\n' "$*"; }
# Project only the fixture's ownership onto the legacy root-file boundary.
# The production reader still performs its ordinary bounded read.
collector_lifecycle_binary() { return 1; }
trusted_private_lifecycle_regular_file() {
    [[ -f "$1" && ! -L "$1" ]] || return 1
    local mode="" owner=""
    mode=$(stat -c '%a' "$1" 2>/dev/null || stat -f '%Lp' "$1") || return 1
    owner=$(stat -c '%u' "$1" 2>/dev/null || stat -f '%u' "$1") || return 1
    [[ "$owner" == "$EUID" && "$mode" =~ ^[0-7]{3,4}$ ]] && (( (8#$mode & 8#077) == 0 ))
}
recover_agent_id_from_state_file() { printf 'owned-agent\n'; }
run_collector_lifecycle_command() {
    printf 'request:%s\n' "$*" >> "$EVENTS"
    case "$REPLY" in
        confirmed) printf 'owned-agent\n' ;;
        mismatch) printf 'different-agent\n' ;;
        empty) : ;;
        refused|lost-response) return 1 ;;
        *) return 90 ;;
    esac
}
pkill() { printf 'process-stop\n' >> "$EVENTS"; }
sleep() { :; }
systemctl() { return 91; }
# This fixture checks the caller's refusal gate, not systemd observations.
# agent_systemd_teardown.py executes that real boundary independently.
quiesce_agent_units_for_removal() {
    [[ "$*" == 'pulse-agent-runner.service pulse-agent-helper.socket pulse-agent-helper.service pulse-agent.service' ]] || return 90
    [[ "$QUIESCENCE_REFUSED" == false ]]
}
teardown_action_runner_service() { printf 'runner-stop\n' >> "$EVENTS"; }
teardown_privileged_helper_service() { printf 'helper-stop\n' >> "$EVENTS"; }
teardown_systemd_agent_service() { printf 'service-stop\n' >> "$EVENTS"; printf 'stopped\n' > "$SERVICE"; }
rm() {
    if [[ $# == 2 && "$1" == -f && -n "$TOKEN_FILE_PATH" && "$2" == "$TOKEN_FILE_PATH" ]]; then
        [[ "$2" == "$RUNTIME_TOKEN_FILE" || "$2" == "$STATE_DIR/runtime.token" || "$2" == "$STATE_DIR/token" ]] || return 94
        printf 'input-token-remove\n' >> "$EVENTS"
        command rm "$@"
        return
    fi
    [[ $# == 3 && "$1" == -rf && "$2" == -- ]] || return 92
    [[ "$3" == "$STATE_DIR" || "$3" == "$PRIVILEGED_HELPER_STATE_DIR" || "$3" == "$SAFE_PROFILE_STATE_DIR" ]] || return 93
    printf 'state-remove\n' >> "$EVENTS"
    command rm "$@"
}
'''
            invocation = ""
            if input_token:
                invocation = "UNINSTALL=false NON_INTERACTIVE=false\nset -- " + shlex.join(arguments) + "\n" + ingress + "\n"
                if ingress_only:
                    invocation += "exit 0\n"
            result = subprocess.run(["bash", "-c", shell + "\n" + setup + "\n" + functions +
                                     "\nmain() {\n" + invocation + entry + "}\nmain\n"],
                                    capture_output=True, text=True, timeout=5)
            events = (root / "events").read_text() if (root / "events").exists() else ""
            after = snapshot(root)
            return result, events, before, after

    def test_missing_url_retains_every_credential_representation_without_a_request(self):
        for kind in ("inline", "runtime", "legacy", "selected", "helper", "empty",
                     "unreadable", "broken-link", "fifo"):
            with self.subTest(credential=kind):
                result, events, before, after = self.exercise(kind)
                self.assertEqual(result.returncode, 16, result.stderr)
                self.assertIn("same trusted Pulse instance", result.stdout)
                self.assertNotIn("synthetic-collector-credential", result.stdout + result.stderr)
                self.assertEqual(events, "")
                self.assertEqual(after, before)

    def test_server_refusal_unknown_or_other_identity_retains_service_and_state(self):
        for reply in ("refused", "lost-response", "empty", "mismatch"):
            with self.subTest(reply=reply):
                result, events, before, after = self.exercise(url="https://pulse.invalid", reply=reply)
                self.assertEqual(result.returncode, 1, result.stderr)
                self.assertEqual(events, "request:collector-uninstall --agent-id owned-agent --hostname owned-host\n")
                self.assertEqual(after, before)

    def test_systemd_refusal_precedes_remote_removal_and_every_local_mutation(self):
        for credential in ("runtime", "inline", "none"):
            with self.subTest(credential=credential):
                result, events, before, after = self.exercise(
                    credential, url="https://pulse.invalid", quiescence_refused=True)
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertEqual(events, "")
                self.assertEqual(after, before)
                self.assertIn("every systemd unit stopped and disabled", result.stdout)

    def test_exact_confirmation_precedes_process_service_and_state_teardown(self):
        result, events, before, after = self.exercise(url="https://pulse.invalid")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(events.splitlines(), [
            "request:collector-uninstall --agent-id owned-agent --hostname owned-host",
            "process-stop", "process-stop", "runner-stop", "helper-stop", "service-stop",
            "state-remove", "state-remove", "state-remove"])
        self.assertEqual(set(after), {"service", "unrelated"})
        self.assertEqual(after["service"][1], b"stopped\n")
        self.assertEqual(after["unrelated"], before["unrelated"])

    def test_credential_free_local_only_removal_needs_no_url_or_request(self):
        for url in ("", "https://pulse.invalid"):
            with self.subTest(url=url):
                result, events, before, after = self.exercise("none", url=url)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertNotIn("request:", events)
                self.assertEqual(set(after), {"service", "unrelated"})
                self.assertEqual(after["unrelated"], before["unrelated"])

    def test_server_confirmation_does_not_grant_unknown_path_removal(self):
        result, events, before, after = self.exercise(url="https://pulse.invalid", authority=False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(events.count("state-remove\n"), 2)
        self.assertIn("without exact trusted lifecycle authority", result.stdout)
        for name in ("state", "state/runtime.token", "state/agent-id", "state/connection.env"):
            self.assertEqual(after[name], before[name])

    def assert_retained(self, result, events, before, after, expected_exit, expected_events=""):
        self.assertEqual(result.returncode, expected_exit, result.stdout + result.stderr)
        self.assertNotIn("synthetic-collector-credential", result.stdout + result.stderr)
        self.assertEqual(events, expected_events)
        self.assertEqual(after, before)

    def test_noninteractive_input_token_survives_missing_url_refusal(self):
        for kind in ("selected", "runtime", "legacy"):
            with self.subTest(credential=kind):
                result, events, before, after = self.exercise(kind, input_token=True)
                self.assertIn("same trusted Pulse instance", result.stdout)
                self.assert_retained(result, events, before, after, 16)

    def test_noninteractive_input_token_survives_unconfirmed_removal(self):
        for kind in ("selected", "runtime", "legacy"):
            for reply in ("refused", "lost-response", "empty", "mismatch"):
                with self.subTest(credential=kind, reply=reply):
                    result, events, before, after = self.exercise(
                        kind, url="https://pulse.invalid", reply=reply, input_token=True)
                    self.assert_retained(result, events, before, after, 1,
                                         "request:collector-uninstall --agent-id owned-agent --hostname owned-host\n")

    def test_confirmed_removal_retains_caller_owned_input_token(self):
        result, events, before, after = self.exercise(
            "selected", url="https://pulse.invalid", input_token=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(events.splitlines()[0],
                         "request:collector-uninstall --agent-id owned-agent --hostname owned-host")
        self.assertNotIn("input-token-remove", events)
        self.assertEqual(set(after), {"service", "unrelated", "selected-token"})
        self.assertEqual(after["selected-token"], before["selected-token"])
        self.assertEqual(after["unrelated"], before["unrelated"])

    def test_confirmed_removal_deletes_runtime_token_only_with_authorised_state(self):
        for kind in ("runtime", "legacy"):
            with self.subTest(credential=kind):
                result, events, before, after = self.exercise(
                    kind, url="https://pulse.invalid", input_token=True)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(events.splitlines()[0],
                                 "request:collector-uninstall --agent-id owned-agent --hostname owned-host")
                self.assertNotIn("input-token-remove", events)
                self.assertEqual(set(after), {"service", "unrelated"})
                self.assertEqual(after["unrelated"], before["unrelated"])

    def test_noninteractive_input_token_survives_root_refusal(self):
        for action in ("uninstall", "install", "update", "runner"):
            for kind in ("selected", "runtime", "legacy"):
                with self.subTest(action=action, credential=kind):
                    result, events, before, after = self.exercise(
                        kind, input_token=True, root_refusal=True, action=action)
                    self.assertIn("This script must be run as root", result.stdout)
                    self.assert_retained(result, events, before, after, 1)

    def test_interactive_input_token_is_read_only_at_ingress(self):
        # Compare interactive ingress without invoking the unrelated TTY exit
        # prompt in a subprocess with no controlling terminal.
        result, events, before, after = self.exercise(
            "selected", input_token=True, interactive=True, ingress_only=True)
        self.assert_retained(result, events, before, after, 0)


if __name__ == "__main__":
    unittest.main()
