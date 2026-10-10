#!/usr/bin/env python3
"""Bind removal help to actual lifecycle boundaries, using only private doubles.

Copied Unix commands run a recorder, not sudo or an installer. The extracted
runner dispatch uses shell doubles; no service, token, host or provider changes.
Windows checks are source-bound, not native PowerShell/service proof.
"""

import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/UNIFIED_AGENT.md"


def section(text=None):
    value = DOC.read_text() if text is None else text
    return value.split("## Uninstall\n", 1)[1].split("\n## Migration Notes", 1)[0]


def compact(value):
    return " ".join(value.replace("**", "").split())


class AgentUninstallDocsTest(unittest.TestCase):
    # Supports read-only actual-parent discrimination without editing source.
    text = None

    def setUp(self):
        self.help = section(self.text)
        self.words = compact(self.help)
        self.installer = (ROOT / "scripts/install.sh").read_text()
        self.removal = self.installer.split("# --- Uninstall Logic ---", 1)[1].split(
            "# --- Validation ---", 1)[0]

    def test_destructive_scope_and_evidence_warning_match_real_cleanup(self):
        for fact in ("destructive removal", "not a repair for missing readings",
                     "saved identity", "connection/runtime credentials", "local agent logs",
                     "platform boot entries", "bounded private log reader", "before removal",
                     "independent monitoring", "credential files or full service configuration"):
            self.assertIn(fact, self.words)
        for operation in ('remove_agent_state_dir "$STATE_DIR"',
                          "teardown_action_runner_service", "teardown_privileged_helper_service",
                          "remove_privileged_helper_state_dir", "remove_safe_profile_state_dir",
                          'rm -f /var/log/pulse-agent.log', 'GO_SCRIPT="/boot/config/go"',
                          "remove_qnap_autorun_block"):
            self.assertIn(operation, self.removal)
        self.assertIn('Remove-Item $StateDir -Recurse -Force',
                      (ROOT / "scripts/install.ps1").read_text())

    def test_saved_state_trust_and_explicit_path_authority_are_not_bypassed(self):
        for fact in ("same trusted Pulse instance", "on the affected agent host",
                     "top-level `install.sh`", "Uninstallation needs no new token",
                     "saved connection and credential state", "Do not reinstall or re-enrol",
                     "installer requires root", "--state-dir", "guessed path",
                     "known Pulse URL and an existing collector credential",
                     "authenticated server confirmation before local collector teardown"):
            self.assertIn(fact, self.words)
        self.assertIn('if [[ $EUID -ne 0', self.installer)
        lifecycle = self.installer.split("# --- Installed Lifecycle State Recovery ---", 1)[1].split(
            "# --- CA Certificate Validation ---", 1)[0]
        self.assertIn('"$UNINSTALL" == "true"', lifecycle)
        self.assertIn('recover_connection_state "$lifecycle_conn_env"', lifecycle)
        self.assertLess(self.removal.index("uninstall_collector_registration"),
                        self.removal.index("pkill"))
        self.assertIn('collector_credential_state_present', self.removal)
        self.assertIn('"${STATE_DIR_REMOVAL_AUTHORITY:-}"', self.installer)

    def test_runner_only_is_separate_and_stop_does_not_establish_revocation(self):
        for fact in ("Linux systemd", "standalone option leaves the collector installed and running",
                     "Do not combine it with `--uninstall`, `--update` or `--enable-action-runner`",
                     "stops and disables the runner before revoking its separate credential",
                     "retains the runner's local recovery material", "stopped is not revoked",
                     "Do not delete that material or bypass TLS", "legacy collector command authority"):
            self.assertIn(fact, self.words)
        runner = self.installer.split("teardown_action_runner_service() {", 1)[1].split(
            "\nteardown_openrc_agent_service", 1)[0]
        self.assertLess(runner.index('systemctl stop'), runner.index('revoke_action_runner_credential'))
        self.assertLess(runner.index('revoke_action_runner_credential'), runner.index('rm -f'))
        self.assertIn('every local artifact was retained', runner)
        self.assertIn('fail "Action runner removal requires a successful credential revocation', runner)

    def test_partial_failures_platform_differences_and_unknowns_remain_explicit(self):
        for fact in ("Windows removes the service first", "failed server notification",
                     "completion message does not prove server-side revocation",
                     "failure can leave a partial result", "before a later runner-revocation failure",
                     "host's service manager", "same-instance", "Settings → Infrastructure",
                     "before deciding on another action", "Retained-state warnings",
                     "intentional path protections", "blindly repeat", "private for safe recovery"):
            self.assertIn(fact, self.words)
        source = (ROOT / "scripts/install.ps1").read_text().split(
            "# --- Uninstall Logic ---", 1)[1].split("# --- Validation", 1)[0]
        self.assertLess(source.index("Remove-PulseService"), source.index('Uri         = "$Url/api/agents/agent/uninstall"'))
        self.assertLess(source.index('# Ignore errors during uninstall', source.index('Uri         = "$Url/api/agents/agent/uninstall"')),
                        source.index('Remove-Item $StateDir -Recurse -Force'))
        commands = re.findall(r"```powershell\n(.*?)```", self.help, re.DOTALL)
        self.assertEqual(commands, ['& $installerFile -Uninstall $true\n'])
        self.assertIn('[bool]$Uninstall = $false', (ROOT / "scripts/install.ps1").read_text())
        self.assertNotIn('Remove-Item', self.help)

    def test_copied_unix_commands_select_only_the_requested_route_and_keep_failures(self):
        commands = re.findall(r"```bash\n(.*?)```", self.help, re.DOTALL)
        self.assertEqual(len(commands), 2)
        for command, expected in zip(commands, ("--uninstall", "--uninstall-action-runner")):
            self.assertEqual(shlex.split(command), ["sudo", "bash",
                "$HOME/.config/pulse/agent-install.sh", expected])
            for installer_exit, sudo_exit in ((0, 0), (17, 0), (0, 23)):
                with self.subTest(route=expected, installer_exit=installer_exit,
                                  sudo_exit=sudo_exit), tempfile.TemporaryDirectory() as temporary:
                    home = Path(temporary)
                    private = home / ".config/pulse"
                    private.mkdir(parents=True, mode=0o700)
                    tools = home / "tools"
                    tools.mkdir(mode=0o700)
                    sudo = tools / "sudo"
                    sudo.write_text('#!/bin/sh\nif [ "$SUDO_EXIT" -ne 0 ]; then exit "$SUDO_EXIT"; fi\nexec "$@"\n')
                    sudo.chmod(0o700)
                    installer = private / "agent-install.sh"
                    installer.write_text('''#!/bin/bash
python3 - "$@" <<'END'
import json, os, sys
from pathlib import Path
Path(os.environ["ARGV_RECEIPT"]).write_text(json.dumps(sys.argv[1:]))
END
exit "$INSTALLER_EXIT"
''')
                    installer.chmod(0o600)
                    receipt = private / "argv.json"
                    env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}",
                               ARGV_RECEIPT=str(receipt), INSTALLER_EXIT=str(installer_exit),
                               SUDO_EXIT=str(sudo_exit))
                    result = subprocess.run(["bash", "-c", command], env=env,
                                            capture_output=True, timeout=10)
                    self.assertEqual(result.returncode, sudo_exit or installer_exit, result.stderr)
                    if sudo_exit:
                        self.assertFalse(receipt.exists())
                    else:
                        self.assertEqual(json.loads(receipt.read_text()), [expected])
                    self.assertEqual(result.stdout, b"")
                    self.assertEqual(result.stderr, b"")

    def test_actual_runner_dispatch_does_not_fall_through_to_collector_lifecycle(self):
        branch = self.installer.split('if [[ "$UNINSTALL_ACTION_RUNNER" == "true" ]]; then', 1)[1].split(
            "\n# --- URL Normalization ---", 1)[0]
        branch = 'if [[ "$UNINSTALL_ACTION_RUNNER" == "true" ]]; then' + branch
        # Execute the real dispatcher with doubles and a fallthrough witness,
        # not the root-only installer or its service teardown functions.
        for uninstall, update, enabled, teardown_exit in ((False, False, False, 0),
                (True, False, False, 0), (False, True, False, 0),
                (False, False, True, 0), (False, False, False, 19)):
            with self.subTest(uninstall=uninstall, update=update, enabled=enabled,
                              teardown_exit=teardown_exit):
                script = f'''set -e
UNINSTALL_ACTION_RUNNER=true
UNINSTALL={str(uninstall).lower()}
UPDATE_ONLY={str(update).lower()}
ACTION_RUNNER_ENABLED={str(enabled).lower()}
EXIT_MISSING_ARGS=7
teardown_action_runner_service() {{ printf 'runner-only\\n'; return {teardown_exit}; }}
log_info() {{ :; }}
fail() {{ exit "$2"; }}
{branch}
printf 'collector-fallthrough\\n'
'''
                result = subprocess.run(["bash", "-c", script], capture_output=True, timeout=10)
                self.assertEqual(result.returncode, 7 if uninstall or update or enabled else teardown_exit)
                self.assertNotIn(b"collector-fallthrough", result.stdout)
                self.assertEqual(result.stdout, b"" if uninstall or update or enabled else b"runner-only\n")

    def test_shipped_mirror_matches_the_canonical_help(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/UNIFIED_AGENT.md").read_bytes())


if __name__ == "__main__":
    unittest.main()
