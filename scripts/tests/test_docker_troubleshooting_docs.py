#!/usr/bin/env python3
"""Execute the shipped Docker diagnosis against bounded synthetic tools.

This proves command syntax, disclosure and failure handling, not native PVE or
Docker compatibility. No real service, socket or container is touched.
"""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SECRET = "synthetic-docker-docs-secret"


def section(heading):
    guide = (ROOT / "docs/UNIFIED_AGENT.md").read_text()
    marker = "### " + heading + "\n"
    if marker not in guide:
        raise AssertionError(f"missing diagnostic section: {heading}")
    return guide.split(marker, 1)[1].split("\n### ", 1)[0]


def command(heading):
    blocks = re.findall(r"```bash\n(.*?)```", section(heading), re.DOTALL)
    if len(blocks) != 1:
        raise AssertionError("expected one diagnostic recipe")
    return blocks[0]


PCT = r'''#!/usr/bin/python3
import json, os, sys
from pathlib import Path
args = sys.argv[1:]
assert args[:4] == ["exec", "123", "--", "sh"]
assert len(args) == 6 and args[4] == "-c"
Path(os.environ["PCT_RECEIPT"]).write_text(json.dumps(args))
mode = os.environ["PCT_MODE"]
if mode == "locked":
    print("pct: guest is locked", file=sys.stderr)
    sys.exit(255)
if mode == "hang":
    import time
    time.sleep(25)
env = dict(os.environ, PATH=os.environ["GUEST_BIN"])
# Model only the socket-presence builtin. The documented sh program and its
# Docker invocations otherwise execute unchanged, with no host socket access.
probe = """test() {
  [ "$#" = 2 ] && [ "$1" = -S ] && [ "$2" = /var/run/docker.sock ] || exit 91
  [ "$SOCKET_STATE" = present ]
}
"""
os.execve("/bin/sh", ["sh", "-c", probe + args[5]], env)
'''

DOCKER = r'''#!/usr/bin/python3
import json, os, sys
from pathlib import Path
args = sys.argv[1:]
with Path(os.environ["DOCKER_RECEIPT"]).open("a") as output:
    output.write(json.dumps(args) + "\n")
if args == ["version", "--format", "server={{.Server.Version}}"]:
    if os.environ["DOCKER_MODE"] == "daemon-error":
        print("Cannot connect to the Docker daemon", file=sys.stderr)
        sys.exit(1)
    print("server=29.8.2")
elif args == ["ps", "-a", "--format", "{{.State}}"]:
    if os.environ["DOCKER_MODE"] == "list-error":
        print("container listing failed", file=sys.stderr)
        sys.exit(1)
    if os.environ["DOCKER_MODE"] != "empty":
        print("running\nexited")
else:
    raise AssertionError("unexpected diagnostic operation")
'''


class DockerTroubleshootingDocsTest(unittest.TestCase):
    def test_service_account_guidance_does_not_grant_socket_access(self):
        text = section("Permission Denied (Docker)")
        self.assertNotIn("usermod", text)
        self.assertNotRegex(text, r"chmod\s+(?:666|777)|--property=Environment")
        for required in ("root-equivalent", "interactive `$USER`", "summary-only",
                         "collector-owned rootless socket", "unauthenticated TCP"):
            self.assertIn(required, text)
        recipe = command("Permission Denied (Docker)")
        self.assertEqual(recipe.strip().splitlines(), [
            "systemctl show pulse-agent.service --property=User --property=Group --property=SupplementaryGroups",
            "ls -l /var/run/docker.sock",
        ])

    def test_guest_guidance_preserves_opt_in_and_avoids_speculative_mutations(self):
        text = section("Docker visible in one LXC but missing in another")
        for required in ("owning node", "command-capable", "allowlist", "online guest-local agent",
                         "root `pct exec` context", "run this **once**", "Exit `124`",
                         "do not enable debug", "Do not\nchange LXC privilege, `keyctl`"):
            self.assertIn(required, text)
        recipe = command("Docker visible in one LXC but missing in another")
        self.assertNotRegex(recipe, r"(?i)inspect|printenv|usermod|chmod|systemctl|restart|--token|https?://")
        self.assertIn("timeout --kill-after=2s 15s", recipe)
        self.assertNotIn("2>/dev/null", recipe)
        source = (ROOT / "internal/monitoring/docker_detection.go").read_text()
        for operation in ("test -S /var/run/docker.sock", "docker version --format", "docker ps -a"):
            self.assertIn(operation, recipe)
            self.assertIn(operation, source)

    def test_command_channel_guidance_does_not_confuse_scope_with_admission(self):
        text = section("Commands enabled but remote control blocked")
        for required in (
            "no admitted command channel connected", "not a connected session",
            "Automatic updates ready", "Changing Proxmox API permissions cannot",
            "locally on the affected node", "`dial websocket`", "`registration failed`",
            "No matching entry is\ninconclusive", "not the full journal",
            "Omit tokens, cookies, URLs, hostnames, addresses", "Keep saved identity and credentials intact",
            "do not delete state, loosen TLS", "commands disabled remains an alternative",
        ):
            with self.subTest(required=required):
                self.assertIn(required, text)
        for label in ("Connected and registered with Pulse command server",
                      "WebSocket connection failed repeatedly, reconnecting"):
            self.assertIn(label, text)
            self.assertIn(label, (ROOT / "internal/hostagent/commands.go").read_text())
        policy = (ROOT / "internal/api/connections_aggregator.go").read_text()
        self.assertIn("agent reports command execution enabled, but no admitted command channel is connected", policy)
        guest = section("Docker visible in one LXC but missing in another")
        self.assertIn("#commands-enabled-but-remote-control-blocked", guest)
        self.assertIn("do not repeat the guest probe", guest)
        self.assertNotRegex(text, r"--property=Environment|systemctl (?:cat|restart)|--insecure")

    def test_command_channel_journal_check_is_bounded_and_read_only(self):
        # The shared reader has executable deadline, partial-failure and stream
        # controls in test_agent_log_docs; keep this entry point bound to it.
        text = section("Commands enabled but remote control blocked")
        self.assertIn("[bounded agent log reader](#collect-agent-logs-safely)", text)
        self.assertNotIn("journalctl", text)

    def run_guest(self, *, socket="present", docker="ok", pct="ok", cli=True):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            host_bin = root / "host"
            guest_bin = root / "guest"
            host_bin.mkdir()
            guest_bin.mkdir()
            (host_bin / "pct").write_text(PCT)
            (host_bin / "pct").chmod(0o700)
            if cli:
                (guest_bin / "docker").write_text(DOCKER)
                (guest_bin / "docker").chmod(0o700)
            env = dict(os.environ, PATH=f"{host_bin}:{os.environ['PATH']}",
                       GUEST_BIN=str(guest_bin), SOCKET_STATE=socket,
                       DOCKER_MODE=docker, PCT_MODE=pct,
                       PCT_RECEIPT=str(root / "pct.json"),
                       DOCKER_RECEIPT=str(root / "docker.jsonl"),
                       SYNTHETIC_SECRET=SECRET)
            result = subprocess.run(["bash", "-c", command("Docker visible in one LXC but missing in another")],
                                    env=env, capture_output=True, text=True, timeout=22)
            self.assertNotIn(SECRET, result.stdout + result.stderr)
            self.assertEqual(json.loads((root / "pct.json").read_text())[:4], ["exec", "123", "--", "sh"])
            path = root / "docker.jsonl"
            operations = [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []
            return result, operations

    def test_working_guest_reports_only_socket_daemon_and_states(self):
        result, operations = self.run_guest()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertRegex(result.stdout, r"/guest/docker\ndefault_socket=present\nserver=29\.8\.2\nrunning\nexited\n$")
        self.assertEqual(len(operations), 2)

    def test_nondefault_context_does_not_hide_absent_default_socket(self):
        result, _ = self.run_guest(socket="absent")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("default_socket=absent\nserver=29.8.2", result.stdout)

    def test_empty_inventory_is_success_not_daemon_failure(self):
        result, operations = self.run_guest(docker="empty")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(result.stdout.endswith("server=29.8.2\n"))
        self.assertEqual(len(operations), 2)

    def test_daemon_failure_is_visible_and_stops_listing(self):
        result, operations = self.run_guest(docker="daemon-error")
        self.assertEqual(result.returncode, 1)
        self.assertIn("Cannot connect", result.stderr)
        self.assertEqual(len(operations), 1)

    def test_listing_failure_is_visible_and_nonzero(self):
        result, _ = self.run_guest(docker="list-error")
        self.assertEqual(result.returncode, 1)
        self.assertIn("container listing failed", result.stderr)

    def test_missing_cli_stops_before_socket_or_daemon_claim(self):
        result, operations = self.run_guest(cli=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("default_socket=", result.stdout)
        self.assertEqual(operations, [])

    def test_pct_failure_is_not_reported_as_empty_inventory(self):
        result, operations = self.run_guest(pct="locked")
        self.assertEqual(result.returncode, 255)
        self.assertIn("guest is locked", result.stderr)
        self.assertEqual(operations, [])

    def test_hung_guest_is_stopped_by_actual_timeout(self):
        result, operations = self.run_guest(pct="hang")
        self.assertEqual(result.returncode, 124)
        self.assertEqual(operations, [])

    def test_shipped_guide_matches_the_tested_source(self):
        self.assertEqual((ROOT / "docs/UNIFIED_AGENT.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/UNIFIED_AGENT.md").read_bytes())


if __name__ == "__main__":
    unittest.main()
