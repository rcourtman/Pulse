#!/usr/bin/env python3
"""Exercise agent-install denial guidance through the real completion path.

Readiness, registration, persistence and sleep are closed peers. The actual
waiter, warning, logging, JSON emitter and completion functions run in Bash;
no listener, credential, service manager or installer main is used. These
controls do not establish an installed agent's cause or reporting recovery.
"""

import itertools
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = ROOT / "scripts/install.sh"


def functions():
    source = INSTALLER.read_text()
    result = []
    for name in ("log_info", "log_warn", "log_error", "redact_token", "json_event",
                 "warn_agent_token_rejected", "verify_agent_started", "complete_installation_flow"):
        match = re.search(r"^" + name + r"\(\) \{\n.*?^\}", source, re.M | re.S)
        if match is None:
            raise AssertionError("missing installer function: " + name)
        result.append(match.group())
    return "\n".join(result)


class AgentAuthCompletion(unittest.TestCase):
    def exercise(self, *, registration=2, health=True, upgrade=False,
                 output="text", noninteractive=False, persisted=True, running=True):
        with tempfile.TemporaryDirectory(prefix="pulse-auth-completion-") as temporary:
            fixture = Path(temporary)
            originals = {"identity": b"fixture-agent-id\n",
                         "token": b"synthetic-private-token\n",
                         "connection": b"synthetic-preserved-connection\n"}
            for name, content in originals.items():
                (fixture / name).write_bytes(content)
                (fixture / name).chmod(0o600)
            script = r'''
set -euo pipefail
EXIT_SERVICE_START_FAILED=13
EXIT_AUTH_REJECTED=18
OUTPUT_FORMAT="$TEST_OUTPUT"
NON_INTERACTIVE="$TEST_NONINTERACTIVE"
UPGRADE_MODE="$TEST_UPGRADE"
CONNECTION_INFO_PERSISTED="$TEST_PERSISTED"
SAVED_INSTALL_SCRIPT=''
LOG_FILE="$FIXTURE/absent-log"
PULSE_TOKEN=synthetic-private-token
TOKEN_FILE_PATH="$FIXTURE/token"
record() { printf '%s\n' "$1" >> "$FIXTURE/events"; }
save_connection_info() { [[ "$1" == "$FIXTURE" ]]; record save; }
resolve_agent_health_url() {
    if [[ "$TEST_HEALTH" == true ]]; then printf 'http://127.0.0.1:9191/readyz'; fi
}
sleep() { [[ "$1" == 2 ]]; }
curl() {
    [[ "$*" == '--disable -sf --max-time 2 http://127.0.0.1:9191/readyz' ]] || return 97
    record health
    [[ "$TEST_RUNNING" == true ]]
}
agent_process_running() { record process; [[ "$TEST_RUNNING" == true ]]; }
verify_agent_server_registration_with_retry() { record registration; return "$TEST_REGISTRATION"; }
report_proxmox_registration_outcome() { [[ "$1" == "$FIXTURE" ]]; record proxmox; }
trap 'record cleanup' EXIT
''' + functions() + r'''
complete_installation_flow "$FIXTURE" INSTALL-SUCCESS UPGRADE-SUCCESS fixture-log-command || record returned-failure
record caller-success
'''
            env = dict(os.environ, FIXTURE=str(fixture), TEST_REGISTRATION=str(registration),
                       TEST_HEALTH=str(health).lower(), TEST_UPGRADE=str(upgrade).lower(),
                       TEST_OUTPUT=output, TEST_NONINTERACTIVE=str(noninteractive).lower(),
                       TEST_PERSISTED=str(persisted).lower(), TEST_RUNNING=str(running).lower())
            result = subprocess.run(["bash", "-c", script], cwd=fixture, env=env,
                                    text=True, capture_output=True, timeout=5)
            events = (fixture / "events").read_text().splitlines()
            self.assertEqual({name: (fixture / name).read_bytes() for name in originals}, originals)
            self.assertEqual({name: (fixture / name).stat().st_mode & 0o777 for name in originals},
                             {name: 0o600 for name in originals})
            self.assertEqual(set(path.name for path in fixture.iterdir()), {*originals, "events"})
            self.assertEqual(events.count("cleanup"), 1)
            self.assertEqual(events.count("save"), int(not persisted))
            self.assertNotIn("returned-failure", events)
            self.assertEqual(result.stderr, "")
            self.assertNotIn("synthetic-private-token", result.stdout)
            self.assertNotIn(str(fixture / "token"), result.stdout)
            records = [json.loads(line) for line in result.stdout.splitlines() if line.startswith("{")]
            return result, events, records

    def test_rejection_preserves_state_and_directs_existing_credential_checks(self):
        for health, upgrade, output, noninteractive, persisted in itertools.product(
                (False, True), (False, True), ("text", "json"), (False, True), (False, True)):
            with self.subTest(health=health, upgrade=upgrade, output=output,
                              noninteractive=noninteractive, persisted=persisted):
                result, events, records = self.exercise(health=health, upgrade=upgrade, output=output,
                                                       noninteractive=noninteractive, persisted=persisted)
                self.assertEqual(result.returncode, 18, result.stdout)
                self.assertEqual(events.count("registration"), 1)
                self.assertEqual(events.count("health"), int(health))
                self.assertEqual(events.count("process"), int(not health))
                self.assertNotIn("proxmox", events)
                self.assertNotIn("caller-success", events)
                for unsafe in ("mint a fresh token", "Generate a fresh", "Re-run the full agent install",
                               "until the credential is replaced", "usually means Pulse was"):
                    self.assertNotIn(unsafe, result.stdout)
                for fact in ("HTTP 401/403", "does not identify the cause", "same Pulse instance",
                             "existing credential", "Settings > API Access", "agent:report",
                             "does not require command execution or administrator access",
                             "Do not reinstall", "re-enrol", "delete identity", "private",
                             "installation is not complete"):
                    self.assertIn(fact, result.stdout)
                self.assertNotIn("INSTALL-SUCCESS", result.stdout)
                self.assertNotIn("UPGRADE-SUCCESS", result.stdout)
                self.assertEqual(records, [{"phase": "complete", "code": "auth_rejected",
                                            "message": "Pulse rejected the agent credential", "exitCode": 18}]
                                 if output == "json" else [])

    def test_registered_and_pending_keep_their_existing_completion(self):
        for registration, health, upgrade, output in itertools.product(
                (0, 1), (False, True), (False, True), ("text", "json")):
            with self.subTest(registration=registration, health=health, upgrade=upgrade, output=output):
                result, events, records = self.exercise(registration=registration, health=health,
                                                       upgrade=upgrade, output=output)
                self.assertEqual(result.returncode, 0, result.stdout)
                self.assertEqual(events.count("registration"), 1)
                self.assertIn("proxmox", events)
                self.assertIn("caller-success", events)
                self.assertNotIn("HTTP 401/403", result.stdout)
                self.assertNotIn("authentication failed", result.stdout)
                self.assertIn("UPGRADE-SUCCESS" if upgrade else "INSTALL-SUCCESS", result.stdout)
                self.assertIn("not confirmed yet" if registration == 1 else "registered with Pulse", result.stdout)
                self.assertEqual(records, [{"phase": "complete", "code": "updated" if upgrade else "installed",
                                            "message": "Installation updated" if upgrade else "Installation installed",
                                            "exitCode": 0}] if output == "json" else [])

    def test_missing_startup_is_not_relabelled_as_authentication(self):
        for health, output in itertools.product((False, True), ("text", "json")):
            with self.subTest(health=health, output=output):
                result, events, records = self.exercise(health=health, output=output, running=False)
                self.assertEqual(result.returncode, 13, result.stdout)
                self.assertNotIn("registration", events)
                self.assertNotIn("caller-success", events)
                self.assertNotIn("proxmox", events)
                self.assertNotIn("HTTP 401/403", result.stdout)
                self.assertNotIn("authentication failed", result.stdout)
                self.assertEqual(records, [{"phase": "complete", "code": "installed_unhealthy",
                                            "message": "Agent installed but not responding", "exitCode": 13}]
                                 if output == "json" else [])


if __name__ == "__main__":
    unittest.main()
