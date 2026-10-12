#!/usr/bin/env python3
"""Production agent teardown against private systemd doubles, never host units.

The real removal functions delete only owned temporary fixture files. All
systemd/process/credential transport operations are closed doubles. These
controls establish refusal and ordering, not native systemd or installed use.
"""

import os
from pathlib import Path
import shlex
import shutil
import signal
import subprocess
import tempfile
import time
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_AGENT_TEARDOWN_SOURCE", ROOT / "scripts/install.sh"))


def function(source, name):
    marker = name + "() {"
    return marker + source.split(marker, 1)[1].split("\n}\n", 1)[0] + "\n}\n"


def bounded_read_environment(root, child_id):
    # macOS CI prepares GNU timeout outside /usr/bin and /bin. Keep the
    # private systemctl double first without discarding that real prerequisite
    # or replacing it with a simulated timeout. Missing preparation is a fail.
    timeout = shutil.which("timeout")
    if timeout is None:
        raise RuntimeError("GNU timeout missing from the prepared test PATH")
    (root / "timeout").symlink_to(Path(timeout).resolve())
    return dict(os.environ, PATH=str(root) + ":/usr/bin:/bin", CHILD_ID=str(child_id))


DOUBLE = r'''
set -euo pipefail
EXIT_GENERAL=1 AGENT_NAME=pulse-agent
PRIVILEGED_HELPER_NAME=pulse-agent-helper ACTION_RUNNER_NAME=pulse-agent-runner
log_info() { :; }
log_error() { printf '%s\n' "$*"; }
revoke_action_runner_credential() { printf 'revoke\n' >> "$EVENTS"; [[ "$FAULT" != revoke-refused ]]; }
# timeout is exercised separately by a hanging child. All other controls call
# this closed systemctl double, and never invoke a real service manager.
timeout() {
    [[ "$1 $2 $3 $4" == '-k 1 5 systemctl' ]] || return 95
    shift 4
    systemctl "$@"
}
systemctl() {
    local verb="$1" unit="${2:-}" load active enabled
    case "$unit" in pulse-agent.service|pulse-agent-helper.socket|pulse-agent-helper.service|pulse-agent-runner.service) ;;
        *) [[ "$verb" == daemon-reload ]] || return 96 ;;
    esac
    printf '%s:%s\n' "$verb" "$unit" >> "$EVENTS"
    case "$verb" in
        show)
            [[ $# == 3 && "$3" == --property=LoadState,ActiveState,UnitFileState ]] || return 97
            if [[ "$unit" == "$TARGET" ]]; then
                case "$FAULT" in
                    read-refused) printf 'private diagnostic\n' >&2; return 1 ;;
                    read-refused-after-stop) [[ ! -f "$ROOT/stopped" ]] || return 1 ;;
                    read-empty) return 0 ;;
                    read-blank-diagnostic) printf '\n' >&2 ;;
                    read-diagnostic) printf 'private diagnostic\n' >&2 ;;
                    read-duplicate) printf 'ActiveState=inactive\n' ;;
                    read-missing) printf 'LoadState=loaded\nActiveState=inactive\n'; return 0 ;;
                esac
            fi
            read -r load active enabled < "$ROOT/$unit"
            [[ "$enabled" != - ]] || enabled=""
            printf 'UnitFileState=%s\nActiveState=%s\nLoadState=%s\n' "$enabled" "$active" "$load"
            [[ "$FAULT" != read-trailing-blank-diagnostic || "$unit" != "$TARGET" ]] || printf '\n' >&2
            ;;
        stop)
            if [[ "$unit" == "$TARGET" ]]; then
                [[ "$FAULT" != stop-refused ]] || return 1
                [[ "$FAULT" != stop-timeout ]] || return 124
                [[ "$FAULT" != ignored-stop ]] || return 0
            fi
            read -r load active enabled < "$ROOT/$unit"
            printf '%s inactive %s\n' "$load" "$enabled" > "$ROOT/$unit"
            touch "$ROOT/stopped"
            # Earlier activation sources changing during a later stop are not
            # safe just because each individual stop once succeeded.
            if [[ "$unit" == pulse-agent-helper.service ]]; then
                case "$FAULT" in
                    socket-reactivated) printf 'loaded active disabled\n' > "$ROOT/pulse-agent-helper.socket" ;;
                    socket-reenabled) printf 'loaded inactive enabled\n' > "$ROOT/pulse-agent-helper.socket" ;;
                esac
            fi
            if [[ "$unit" == pulse-agent.service && "$FAULT" == runner-reactivated ]]; then
                printf 'loaded active disabled\n' > "$ROOT/pulse-agent-runner.service"
            fi
            ;;
        disable)
            if [[ "$unit" == "$TARGET" ]]; then
                [[ "$FAULT" != disable-refused ]] || return 1
                [[ "$FAULT" != ignored-disable ]] || return 0
            fi
            read -r load active enabled < "$ROOT/$unit"
            case "$enabled" in masked|masked-runtime|static|indirect) ;;
                *) enabled=disabled ;;
            esac
            printf '%s %s %s\n' "$load" "$active" "$enabled" > "$ROOT/$unit"
            ;;
        daemon-reload|reset-failed) return 0 ;;
        *) return 98 ;;
    esac
}
rm() {
    local path
    [[ "$1" == -f || "$1" == -rf ]] || return 99
    for path in "${@:2}"; do
        case "$path" in "$ROOT/artifacts/"*) ;;
            *) printf 'outside fixture deletion refused\n' >&2; return 100 ;;
        esac
    done
    printf 'delete\n' >> "$EVENTS"
    command rm "$@"
}
'''


class AgentSystemdTeardown(unittest.TestCase):
    source = SOURCE.read_text()

    def exercise(self, family, fault="", state="loaded active enabled"):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            artifacts = root / "artifacts"
            artifacts.mkdir()
            paths = {
                "PRIVILEGED_HELPER_SOCKET_UNIT": artifacts / "helper.socket",
                "PRIVILEGED_HELPER_SERVICE_UNIT": artifacts / "helper.service",
                "PRIVILEGED_HELPER_SOCKET_PATH": artifacts / "helper.sock",
                "PRIVILEGED_HELPER_CREDENTIAL_DIR": artifacts / "helper-credentials",
                "ACTION_RUNNER_BINARY_PATH": artifacts / "runner",
                "ACTION_RUNNER_SERVICE_UNIT": artifacts / "runner.service",
                "ACTION_RUNNER_CONFIG_DIR": artifacts / "runner-config",
                "ACTION_RUNNER_STATE_DIR": artifacts / "runner-state",
            }
            for name, path in paths.items():
                if name.endswith("DIR"):
                    path.mkdir()
                    (path / "token").write_text("synthetic-recovery-credential\n")
                else:
                    path.write_text("owned recovery material\n")
            agent_unit = artifacts / "agent.service"
            agent_unit.write_text("owned agent unit\n")
            unrelated = root / "unrelated"
            unrelated.write_text("keep\n")
            before = {str(p.relative_to(artifacts)): p.read_bytes()
                      for p in artifacts.rglob("*") if p.is_file()}
            target = {"agent": "pulse-agent.service", "helper": "pulse-agent-helper.socket",
                      "runner": "pulse-agent-runner.service", "batch": "pulse-agent.service"}[family]
            for unit in ("pulse-agent.service", "pulse-agent-helper.socket",
                         "pulse-agent-helper.service", "pulse-agent-runner.service"):
                (root / unit).write_text(state + "\n")
            values = dict(paths, ROOT=root, EVENTS=root / "events", FAULT=fault, TARGET=target)
            setup = "\n".join(f"{key}={shlex.quote(str(value))}" for key, value in values.items())
            names = ("read_agent_removal_unit_state", "confirm_agent_unit_quiescent",
                     "quiesce_agent_units_for_removal", "teardown_systemd_agent_service",
                     "teardown_privileged_helper_service", "teardown_action_runner_service")
            # The same controls can reproduce the older source, which has no
            # observation helper: never insert a substitute implementation.
            functions = "\n".join(function(self.source, name) for name in names
                                  if name + "() {" in self.source)
            calls = {"agent": "teardown_systemd_agent_service " + shlex.quote(str(agent_unit)),
                     "helper": "teardown_privileged_helper_service",
                     "runner": "teardown_action_runner_service",
                     "batch": "quiesce_agent_units_for_removal pulse-agent-runner.service pulse-agent-helper.socket pulse-agent-helper.service pulse-agent.service"}
            # Production fail() exits even if the caller's errexit is disabled.
            functions += "\n" + function(self.source, "fail")
            shell = DOUBLE + "\nOUTPUT_FORMAT=text\n" + setup + "\n" + functions + "\n" + calls[family]
            result = subprocess.run(["bash", "-c", shell], capture_output=True,
                                    text=True, timeout=8)
            events = (root / "events").read_text().splitlines() if (root / "events").exists() else []
            after = {str(p.relative_to(artifacts)): p.read_bytes()
                     for p in artifacts.rglob("*") if p.is_file()}
            self.assertEqual(unrelated.read_text(), "keep\n")
            return result, events, before, after

    def test_unknown_read_stop_and_disable_retain_every_artifact(self):
        faults = ("read-refused", "read-empty", "read-blank-diagnostic", "read-diagnostic",
                  "read-trailing-blank-diagnostic", "read-duplicate", "read-missing", "stop-refused", "stop-timeout",
                  "ignored-stop", "disable-refused", "ignored-disable", "read-refused-after-stop")
        for family in ("agent", "helper", "runner"):
            for fault in faults:
                with self.subTest(family=family, fault=fault):
                    result, events, before, after = self.exercise(family, fault)
                    self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertEqual(after, before)
                    self.assertNotIn("delete", events)
                    self.assertNotIn("revoke", events)
                    self.assertNotIn("private diagnostic", result.stdout + result.stderr)

    def test_unsettled_or_unknown_states_never_authorise_deletion(self):
        for state in ("error active enabled", "loaded activating enabled", "loaded deactivating enabled",
                      "loaded reloading enabled", "loaded unknown enabled", "loaded inactive unknown",
                      "not-found active -", "not-found inactive enabled"):
            for family in ("agent", "helper", "runner"):
                with self.subTest(family=family, state=state):
                    result, events, before, after = self.exercise(family, state=state)
                    self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertEqual(after, before)
                    self.assertNotIn("delete", events)

    def test_helper_socket_and_service_are_reconciled_together(self):
        for fault in ("socket-reactivated", "socket-reenabled"):
            with self.subTest(fault=fault):
                result, events, before, after = self.exercise("helper", fault)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("delete", events)

    def test_known_quiescence_allows_only_owned_teardown(self):
        for family in ("agent", "helper", "runner"):
            for state in ("loaded active enabled", "loaded failed enabled", "loaded inactive disabled",
                          "loaded inactive static", "loaded inactive indirect", "masked inactive masked",
                          "masked inactive masked-runtime", "not-found inactive -"):
                with self.subTest(family=family, state=state):
                    result, events, before, after = self.exercise(family, state=state)
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertLess(len(after), len(before))
                    first_delete = events.index("delete")
                    self.assertTrue(any(event.startswith("show:") for event in events[:first_delete]))
                    if family == "runner":
                        self.assertLess(events.index("revoke"), first_delete)
                        self.assertTrue(any(event.startswith("show:") for event in events[:events.index("revoke")]))
                    if state.startswith("not-found"):
                        self.assertFalse(any(event.startswith(("stop:", "disable:")) for event in events))

    def test_failed_revocation_retains_stopped_runner_recovery_material(self):
        result, events, before, after = self.exercise("runner", "revoke-refused")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("revoke", events)
        self.assertNotIn("delete", events)
        self.assertEqual(after, before)

    def test_full_batch_reconciles_earlier_activation_sources(self):
        for fault in ("", "runner-reactivated", "stop-refused"):
            with self.subTest(fault=fault):
                result, events, before, after = self.exercise("batch", fault)
                self.assertEqual(result.returncode == 0, fault == "", result.stdout + result.stderr)
                self.assertEqual(after, before)
                self.assertNotIn("delete", events)
                self.assertNotIn("revoke", events)

    def test_real_read_timeout_stops_its_owned_child(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            executable = root / "systemctl"
            child_id = root / "child-id"
            executable.write_text("#!/bin/bash\nprintf '%s\\n' \"$$\" > \"$CHILD_ID\"\nexec sleep 60\n")
            executable.chmod(0o700)
            env = bounded_read_environment(root, child_id)
            started = time.monotonic()
            try:
                result = subprocess.run(["bash", "-c", function(self.source, "read_agent_removal_unit_state") +
                                         "\nread_agent_removal_unit_state pulse-agent.service\n"],
                                        env=env, capture_output=True, text=True, timeout=8)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertLess(time.monotonic() - started, 7)
                self.assertEqual(result.stdout, "")
                self.assertTrue(child_id.exists(), "the bounded read never started")
                with self.assertRaises(ProcessLookupError):
                    os.kill(int(child_id.read_text()), 0)
            finally:
                # An adverse result must fail above, not leave our own hanging
                # fixture behind. This does not turn failed cleanup into a pass.
                if child_id.exists():
                    try:
                        os.kill(int(child_id.read_text()), signal.SIGKILL)
                    except ProcessLookupError:
                        pass

    def test_timeout_probe_keeps_prepared_tool_and_refuses_missing_preparation(self):
        timeout = shutil.which("timeout")
        self.assertIsNotNone(timeout, "native CI must supply GNU timeout")
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            prepared = root / "prepared"
            prepared.mkdir()
            (prepared / "timeout").symlink_to(Path(timeout).resolve())
            probe = root / "probe"
            probe.mkdir()
            # A tool prepared in an otherwise empty, non-system PATH must be
            # carried into the isolated probe. Never fall back to a host unit.
            with mock.patch.dict(os.environ, PATH=str(prepared)):
                env = bounded_read_environment(probe, probe / "child-id")
            self.assertEqual((probe / "timeout").resolve(), Path(timeout).resolve())
            version = subprocess.run([str(probe / "timeout"), "--version"], env=env,
                                     capture_output=True, text=True, timeout=3)
            self.assertEqual(version.returncode, 0, version.stdout + version.stderr)
            self.assertIn("GNU coreutils", version.stdout)
            with mock.patch.dict(os.environ, PATH=str(root / "absent")):
                with self.assertRaisesRegex(RuntimeError, "missing from the prepared test PATH"):
                    bounded_read_environment(root, root / "child-id")


if __name__ == "__main__":
    unittest.main(verbosity=2)
