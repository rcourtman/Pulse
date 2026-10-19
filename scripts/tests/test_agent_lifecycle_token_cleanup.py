#!/usr/bin/env python3
"""Closed-peer checks for the installer's temporary lifecycle credentials.

Only extracted production functions run. The lifecycle binary, identity lookup
and trust check are inert peers; every file and self-directed signal belongs to
the disposable fixture. This is not installed registration/removal acceptance.
"""

import itertools
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


INSTALLER = Path(__file__).resolve().parents[2] / "scripts/install.sh"
TOKEN = "synthetic-lifecycle-secret"
COMMANDS = ("collector-verify-registration", "collector-uninstall", "collector-reduce-authority")


def functions():
    source = INSTALLER.read_text()
    result = []
    for name in ("cleanup", "prepare_collector_lifecycle_token_file", "run_collector_lifecycle_command"):
        match = re.search(r"^" + name + r"\(\) \{\n.*?^\}", source, re.M | re.S)
        if match is None:
            raise AssertionError("missing installer function: " + name)
        result.append(match.group())
    return "\n".join(result)


class AgentLifecycleTokenCleanup(unittest.TestCase):
    def exercise(self, *, command=COMMANDS[0], stored="temporary", outcome="0",
                 substitution=False, phase="command", failure=""):
        with tempfile.TemporaryDirectory(prefix="pulse-lifecycle-cleanup-") as temporary:
            root = Path(temporary)
            (root / "tmp").mkdir()
            (root / "state").mkdir()
            originals = {"state/runtime.token": b"retained-runtime-secret",
                         "configured.token": b"retained-configured-secret",
                         "state/token": b"retained-bootstrap-secret"}
            selected = {"runtime": "state/runtime.token", "configured": "configured.token",
                        "bootstrap": "state/token"}.get(stored)
            # Higher priority stored files exist only when selected. The lower
            # priority bootstrap remains to prove runtime/configured precedence.
            present = {name: value for name, value in originals.items()
                       if name == selected or (selected and name == "state/token")}
            for name, content in present.items():
                (root / name).write_bytes(content)
                (root / name).chmod(0o600)
            (root / "caller.tmp").write_text("owned by caller, not the lifecycle scope")
            script = r'''
set -euo pipefail
TMP_FILES=("$FIXTURE/caller.tmp")
AGENT_STAGING_DIRS=()
STATE_DIR="$FIXTURE/state"
RUNTIME_TOKEN_FILE="$FIXTURE/configured.token"
PULSE_URL=https://fixture.invalid
PULSE_TOKEN=synthetic-lifecycle-secret
LEAST_PRIVILEGE_USER=fixture-user
CURL_CA_BUNDLE="$FIXTURE/ca.pem"
SERVER_FINGERPRINT=synthetic-pin
record() { builtin printf '%s\n' "$1" >> "$FIXTURE/events"; }
collector_lifecycle_binary() { builtin printf '%s\n' lifecycle_peer; }
trusted_lifecycle_regular_file() { [[ "$TEST_FAILURE" != trust && "$*" == 'lifecycle_peer 755' ]]; }
id() { [[ "$*" == '-u fixture-user' ]]; builtin printf '%s\n' 1234; }
mktemp() { [[ "$TEST_FAILURE" != mktemp ]] && command mktemp "$@"; }
chmod() { [[ "$TEST_FAILURE" != chmod ]] && command chmod "$@"; }
rm() {
    if [[ "$TEST_FAILURE" == cleanup && "${*: -1}" == "$TMPDIR/"* ]]; then
        record cleanup-refused
        return 1
    fi
    command rm "$@"
}
printf() {
    builtin printf "$@"
    if [[ "${1:-}" == '%s' && "${2:-}" == "$PULSE_TOKEN" ]]; then
        [[ "$TEST_FAILURE" != write ]] || return 1
        if [[ "$TEST_PHASE" == write ]]; then
            kill -"$TEST_OUTCOME" "$BASHPID"
        fi
    fi
}
lifecycle_peer() {
    record invoked
    [[ "$#" == 15 && "$1" == "$TEST_COMMAND" && "$2" == --url && "$3" == "$PULSE_URL" &&
       "$4" == --token-file && "$6" == --token-owner-uid && "$7" == 1234 &&
       "$8" == --cacert && "$9" == "$CURL_CA_BUNDLE" && "${10}" == --server-fingerprint &&
       "${11}" == "$SERVER_FINGERPRINT" && "${12}" == --agent-id && "${13}" == fixture-agent &&
       "${14}" == --hostname && "${15}" == fixture-host ]] || return 91
    [[ -f "$FIXTURE/caller.tmp" ]] || return 92
    [[ -f "$5" && ! -L "$5" && "$(stat -c %a "$5")" == 600 ]] || return 93
    case "$TEST_STORED" in
        temporary) [[ "$5" == "$TMPDIR/"* && "$(cat "$5")" == "$PULSE_TOKEN" ]] || return 94 ;;
        runtime) [[ "$5" == "$STATE_DIR/runtime.token" ]] || return 95 ;;
        configured) [[ "$5" == "$RUNTIME_TOKEN_FILE" ]] || return 96 ;;
        bootstrap) [[ "$5" == "$STATE_DIR/token" ]] || return 97 ;;
    esac
    builtin printf '%s\n' "$@" > "$FIXTURE/argv"
    case "$TEST_OUTCOME" in
        INT|TERM) kill -"$TEST_OUTCOME" "$BASHPID" ;;
        *) builtin printf '%s\n' fixture-last-seen; return "$TEST_OUTCOME" ;;
    esac
}
''' + functions() + r'''
trap 'record caller-exit; cleanup' EXIT
trap 'record caller-int; exit 130' INT
trap 'record caller-term; exit 143' TERM
before_traps=$(trap -p)
rc=0
if [[ "$TEST_SUBSTITUTION" == true ]]; then
    result=$(run_collector_lifecycle_command "$TEST_COMMAND" --agent-id fixture-agent --hostname fixture-host) || rc=$?
else
    run_collector_lifecycle_command "$TEST_COMMAND" --agent-id fixture-agent --hostname fixture-host || rc=$?
fi
[[ "$(trap -p)" == "$before_traps" ]] || exit 98
[[ -f "$FIXTURE/caller.tmp" ]] || exit 99
record caller-continued
exit "$rc"
'''
            env = dict(os.environ, FIXTURE=str(root), TMPDIR=str(root / "tmp"),
                       TEST_COMMAND=command, TEST_STORED=stored, TEST_OUTCOME=outcome,
                       TEST_SUBSTITUTION=str(substitution).lower(), TEST_PHASE=phase,
                       TEST_FAILURE=failure)
            result = subprocess.run(["bash", "-c", script], cwd=root, env=env,
                                    capture_output=True, text=True, timeout=5)
            events = (root / "events").read_text().splitlines()
            expected_exit = {"INT": 130, "TERM": 143}.get(outcome)
            if expected_exit is None:
                expected_exit = int(outcome)
            if failure and (failure != "cleanup" or expected_exit == 0):
                expected_exit = 1
            self.assertEqual(result.returncode, expected_exit,
                             result.stdout + result.stderr)
            self.assertEqual(result.stderr, "")
            self.assertNotIn(TOKEN, result.stdout)
            self.assertFalse((root / "caller.tmp").exists(), "caller EXIT cleanup did not run")
            self.assertEqual(events.count("caller-exit"), 1)
            self.assertEqual({name: (root / name).read_bytes() for name in present}, present)
            self.assertEqual({name: (root / name).stat().st_mode & 0o777 for name in present},
                             {name: 0o600 for name in present})
            leftovers = list((root / "tmp").iterdir())
            if failure == "cleanup":
                self.assertEqual(len(leftovers), 1)
                self.assertEqual(leftovers[0].read_text(), TOKEN)
                self.assertEqual(leftovers[0].stat().st_mode & 0o777, 0o600)
            else:
                self.assertEqual(leftovers, [], "temporary lifecycle credential survived shell exit")
            argv = (root / "argv").read_text() if (root / "argv").exists() else ""
            for secret in (TOKEN, *[value.decode() for value in originals.values()]):
                self.assertNotIn(secret, argv)
            if phase == "command" and failure not in ("trust", "mktemp", "chmod", "write"):
                self.assertEqual(events.count("invoked"), 1)
            else:
                self.assertNotIn("invoked", events)
            return events

    def test_returns_and_signals_clean_only_owned_temporary_credentials(self):
        for command, stored, outcome, substitution in itertools.product(
                COMMANDS, ("temporary", "runtime", "configured", "bootstrap"),
                ("0", "1", "2", "37", "INT", "TERM"), (False, True)):
            with self.subTest(command=command, stored=stored, outcome=outcome,
                              substitution=substitution):
                self.exercise(command=command, stored=stored, outcome=outcome,
                              substitution=substitution)

    def test_interrupt_during_token_write_is_owned_before_secret_bytes(self):
        for outcome, substitution in itertools.product(("INT", "TERM"), (False, True)):
            with self.subTest(outcome=outcome, substitution=substitution):
                self.exercise(outcome=outcome, substitution=substitution, phase="write")

    def test_caller_traps_and_temporary_files_survive_inner_scope(self):
        for stored in ("temporary", "runtime", "configured", "bootstrap"):
            with self.subTest(stored=stored):
                events = self.exercise(stored=stored)
                self.assertEqual(events, ["invoked", "caller-continued", "caller-exit"])

    def test_preparation_failures_never_call_lifecycle_or_leave_credentials(self):
        for failure, substitution in itertools.product(("trust", "mktemp", "chmod", "write"),
                                                       (False, True)):
            with self.subTest(failure=failure, substitution=substitution):
                self.exercise(failure=failure, substitution=substitution)

    def test_cleanup_failure_cannot_report_success_or_hide_original_failure(self):
        for outcome, substitution in itertools.product(("0", "2", "37", "INT", "TERM"),
                                                       (False, True)):
            with self.subTest(outcome=outcome, substitution=substitution):
                self.exercise(failure="cleanup", outcome=outcome, substitution=substitution)


if __name__ == "__main__":
    unittest.main()
