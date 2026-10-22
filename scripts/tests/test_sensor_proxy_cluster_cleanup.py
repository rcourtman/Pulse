#!/usr/bin/env python3
"""Exercise exact shell discovery/caller code without a host, cluster or SSH.

Only synthetic local key files are changed. Every cluster, DNS, hostname and
SSH command is a closed recording fake; lifecycle/removal phases are adapters.
The ordinary smoke inventory discovers this suite without needing Go.
"""

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = Path(os.environ.get("PULSE_SENSOR_PROXY_TEST_SCRIPT",
                             ROOT / "scripts/uninstall-sensor-proxy.sh"))
BASH = shutil.which("bash")
ORIGINAL_KEYS = "ssh-ed25519 AAAAfixture operator\nssh-ed25519 AAAAlegacy # pulse-managed-key\n"
PRESERVED_KEYS = "ssh-ed25519 AAAAfixture operator\n"

FAKE = r'''import json
import os
from pathlib import Path
import sys
import time

root = Path(os.environ["FIXTURE"])
tool = Path(sys.argv[0]).name
args = sys.argv[1:]
with (root / "calls").open("a") as output:
    output.write(json.dumps([tool, *args]) + "\n")
config = json.loads((root / "scenario.json").read_text())
if tool == "pvesh" and args == ["get", "/nodes", "--output-format", "json"]:
    time.sleep(config.get("inventory_delay", 0))
    sys.stdout.write(config["inventory"])
    sys.exit(config.get("inventory_exit", 0))
if tool == "getent" and len(args) == 2 and args[0] in ("ahostsv4", "hosts"):
    entry = config.get("resolution", {}).get(args[1], {})
    time.sleep(entry.get("delay", 0))
    value = entry.get(args[0], [2, ""])
    sys.stdout.write(value[1])
    sys.exit(value[0])
if tool == "hostname":
    values = {"-I": "192.0.2.1\n", "-f": "node-a.example.test\n", "": "node-a\n"}
    if len(args) <= 1 and (args[0] if args else "") in values:
        sys.stdout.write(values[args[0] if args else ""])
        sys.exit(0)
if tool == "ssh":
    if len([arg for arg in args if arg.startswith("root@")]) != 1:
        sys.exit(91)
    sys.exit(config.get("ssh_exit", 0))
# pvecm and every other unexpected provider/action command must never run.
sys.exit(90)
'''

SHELL = r'''source "$1"
shift
if [[ "$TEST_MODE" == main ]]; then
    mark() { printf '%s\n' "$1" >>"$FIXTURE/phases"; }
    disable_legacy_units() { mark services; }
    cleanup_stale_sensor_proxy_mounts() { mark mounts; }
    remove_legacy_files() { mark files; }
    remove_proxmox_access() { mark access; }
    systemctl_if_available() { mark reload; }
    main "$@"
else
    [[ "$TEST_MODE" != local ]] || LOCAL_ONLY=true
    cleanup_cluster_authorized_keys
fi
'''


class SensorProxyClusterCleanupTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        # Restrict PATH so an accidental new command cannot use the host.
        for name in ("python3", "timeout", "sort", "readlink", "mktemp", "grep",
                     "cmp", "chmod", "chown", "mv", "rm"):
            path = shutil.which(name)
            if not path:
                self.fail(f"required fixture dependency missing: {name}")
            (self.bin / name).symlink_to(path)
        for name in ("pvesh", "pvecm", "getent", "ssh", "hostname", "pct", "pveum", "systemctl"):
            path = self.bin / name
            path.write_text(f"#!{sys.executable}\n" + FAKE)
            path.chmod(0o700)
        self.keys = self.root / "authorized_keys"
        self.known = self.root / "known_hosts"
        self.known.write_text("192.0.2.2 ssh-ed25519 AAAAfixture\n")
        self.env = {
            "PATH": str(self.bin), "FIXTURE": str(self.root), "HOME": str(self.root),
            "TMPDIR": str(self.root), "LC_ALL": "C", "TEST_MODE": "cluster",
            "PULSE_SENSOR_PROXY_AUTHORIZED_KEYS_PATH": str(self.keys),
            "PULSE_SENSOR_PROXY_SSH_KNOWN_HOSTS_PATH": str(self.known),
        }

    def scenario(self, nodes=None, **overrides):
        value = {
            "inventory": json.dumps(nodes if nodes is not None else
                                    [{"node": "node-a"}, {"node": "node-b"}]),
            "resolution": {
                "node-a": {"ahostsv4": [0, "192.0.2.1 STREAM node-a\n192.0.2.1 DGRAM\n"]},
                "node-b": {"ahostsv4": [0, "192.0.2.2 STREAM node-b\n"]},
            },
        }
        value.update(overrides)
        return value

    def invoke(self, config=None, mode="cluster", args=(), env=None):
        self.keys.write_text(ORIGINAL_KEYS)
        self.keys.chmod(0o600)
        for name in ("calls", "phases"):
            (self.root / name).unlink(missing_ok=True)
        (self.root / "scenario.json").write_text(json.dumps(config or self.scenario()))
        result = subprocess.run([BASH, "-c", SHELL, "bash", str(SCRIPT), *args],
                                env=dict(self.env, TEST_MODE=mode, **(env or {})),
                                cwd=self.root, text=True, capture_output=True, timeout=25)
        self.assertEqual(self.keys.read_text(), PRESERVED_KEYS, result.stdout + result.stderr)
        self.assertEqual(self.keys.stat().st_mode & 0o777, 0o600)
        self.calls = [json.loads(line) for line in (self.root / "calls").read_text().splitlines()] \
            if (self.root / "calls").exists() else []
        self.assertFalse(any(call[0] in ("pvecm", "pct", "pveum", "systemctl") for call in self.calls),
                         self.calls)
        return result

    def ssh_calls(self):
        return [call[1:] for call in self.calls if call[0] == "ssh"]

    def assert_unconfirmed(self, result):
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("--local-only separately on every cluster node", result.stderr)
        self.assertEqual(self.ssh_calls(), [], self.calls)
        self.assertNotIn("cleanup complete", result.stdout)

    def test_complete_inventory_cleans_remote_with_strict_trust(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(len(self.ssh_calls()), 1, self.calls)
        args = self.ssh_calls()[0]
        for required in ("root@192.0.2.2", "StrictHostKeyChecking=yes", "UpdateHostKeys=no",
                         "BatchMode=yes", "ConnectTimeout=5", "GlobalKnownHostsFile=none",
                         f"UserKnownHostsFile={self.known.resolve()}"):
            self.assertIn(required, args)
        self.assertEqual([call for call in self.calls if call[0] == "pvesh"],
                         [["pvesh", "get", "/nodes", "--output-format", "json"]])

    def test_duplicate_nodes_and_resolved_aliases_are_cleaned_once(self):
        config = self.scenario([{"node": "node-b"}, {"node": "alias-b"}, {"node": "node-b"}])
        config["resolution"]["alias-b"] = config["resolution"]["node-b"]
        result = self.invoke(config)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(len(self.ssh_calls()), 1)

    def test_local_cleanup_runs_even_when_local_node_is_omitted(self):
        result = self.invoke(self.scenario([{"node": "node-b"}]))
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(len(self.ssh_calls()), 1)

    def test_single_local_node_is_success_without_ssh(self):
        result = self.invoke(self.scenario([{"node": "node-a"}]))
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.ssh_calls(), [])

    def test_empty_invalid_and_partial_json_never_become_single_node_success(self):
        for inventory in ("", "not JSON", "[]", "null", "{}", "[null]", "[{}]",
                          '[{"node":"node-b"},', '[{"node":"node-b"}] []',
                          '[{"node":"node-b","node":"node-a"}]',
                          '[{"node":"node-b"},{"node":3}]'):
            with self.subTest(inventory=inventory):
                self.assert_unconfirmed(self.invoke(self.scenario(inventory=inventory)))
                self.assertFalse(any(call[0] == "getent" for call in self.calls))

    def test_unsafe_node_names_are_rejected_before_resolution(self):
        for node in ("", "-oProxyCommand=fixture", "node b", "node\nnode-b", "node/b", "node@b", "a" * 254):
            with self.subTest(node=node):
                self.assert_unconfirmed(self.invoke(self.scenario([{"node": node}])))
                self.assertFalse(any(call[0] == "getent" for call in self.calls))

    def test_failed_inventory_output_cannot_supply_targets(self):
        self.assert_unconfirmed(self.invoke(self.scenario(inventory_exit=9)))
        self.assertFalse(any(call[0] == "getent" for call in self.calls))

    def test_failed_resolution_of_any_node_prevents_all_remote_actions(self):
        for entry in ({}, {"ahostsv4": [9, "192.0.2.3 STREAM\n"], "hosts": [9, "192.0.2.3 node-c\n"]}):
            with self.subTest(entry=entry):
                config = self.scenario([{"node": "node-b"}, {"node": "node-c"}])
                config["resolution"]["node-c"] = entry
                self.assert_unconfirmed(self.invoke(config))

    def test_empty_and_invalid_resolver_outputs_do_not_supply_ssh_targets(self):
        for output in ("", "\n", "not-an-ip STREAM\n", "192.0.2.999 STREAM\n",
                       "192.0.2.2 STREAM\ninvalid DGRAM\n", "fe80::1%fixture node-b\n"):
            with self.subTest(output=output):
                config = self.scenario()
                config["resolution"]["node-b"] = {"ahostsv4": [0, output]}
                self.assert_unconfirmed(self.invoke(config))

    def test_hosts_fallback_can_resolve_ipv6_without_relaxing_trust(self):
        config = self.scenario()
        config["resolution"]["node-b"] = {"hosts": [0, "2001:db8::2 node-b\n"]}
        result = self.invoke(config)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("root@2001:db8::2", self.ssh_calls()[0])
        self.assertIn("StrictHostKeyChecking=yes", self.ssh_calls()[0])

    def test_fqdn_inventory_is_supported(self):
        config = self.scenario([{"node": "node-b.example.test"}])
        config["resolution"]["node-b.example.test"] = config["resolution"]["node-b"]
        result = self.invoke(config)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(len(self.ssh_calls()), 1)

    def test_missing_dependencies_fail_instead_of_falling_back_to_pvecm(self):
        for dependency in ("pvesh", "python3", "getent", "timeout", "sort"):
            with self.subTest(dependency=dependency):
                path = self.bin / dependency
                saved = self.root / dependency
                path.rename(saved)
                try:
                    result = self.invoke()
                    self.assert_unconfirmed(result)
                    self.assertIn(f"{dependency} is required", result.stderr)
                finally:
                    saved.rename(path)

    def test_explicit_local_mode_requires_no_inventory_resolution_or_ssh(self):
        (self.bin / "pvesh").unlink()
        result = self.invoke(self.scenario(inventory_exit=9), mode="local")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.calls, [])

    def test_remote_key_rejection_remains_an_error_after_local_cleanup(self):
        result = self.invoke(self.scenario(ssh_exit=255))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Unable to verify 192.0.2.2", result.stderr)
        self.assertEqual(len(self.ssh_calls()), 1)

    def test_configured_openssh_trust_is_not_replaced_without_explicit_file(self):
        result = self.invoke(env={"PULSE_SENSOR_PROXY_SSH_KNOWN_HOSTS_PATH": ""})
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(len(self.ssh_calls()), 1)
        self.assertIn("StrictHostKeyChecking=yes", self.ssh_calls()[0])
        self.assertFalse(any(arg.startswith(("UserKnownHostsFile=", "GlobalKnownHostsFile="))
                             for arg in self.ssh_calls()[0]))

    def test_missing_or_empty_provisioned_trust_never_invokes_ssh(self):
        for present in (False, True):
            with self.subTest(present=present):
                self.known.unlink(missing_ok=True)
                if present:
                    self.known.write_text("")
                result = self.invoke()
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("known_hosts file is missing, unreadable, or empty", result.stderr)
                self.assertEqual(self.ssh_calls(), [])

    def test_main_reports_incomplete_discovery_after_safe_local_phases(self):
        result = self.invoke(self.scenario(inventory_exit=9), mode="main", args=("--quiet",))
        self.assert_unconfirmed(result)
        self.assertIn("Local cleanup completed", result.stderr)
        self.assertEqual((self.root / "phases").read_text(), "services\nmounts\nfiles\naccess\nreload\n")

    def test_main_success_requires_the_real_complete_inventory_path(self):
        for args in ((), ("--quiet",)):
            with self.subTest(args=args):
                result = self.invoke(mode="main", args=args)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                if args:
                    self.assertEqual(result.stdout, "")
                else:
                    self.assertIn("Legacy pulse-sensor-proxy cleanup complete", result.stdout)
                self.assertEqual(len(self.ssh_calls()), 1)

    def test_main_local_only_still_skips_remote_discovery(self):
        result = self.invoke(self.scenario(inventory_exit=9), mode="main", args=("--local-only",))
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.calls, [])

    def test_inventory_read_is_bounded_by_the_real_timeout(self):
        self.assert_unconfirmed(self.invoke(self.scenario(inventory_delay=30)))
        self.assertFalse(any(call[0] == "getent" for call in self.calls))

    def test_resolver_reads_are_bounded_by_the_real_timeout(self):
        config = self.scenario()
        config["resolution"]["node-b"] = {"delay": 15}
        self.assert_unconfirmed(self.invoke(config))


if __name__ == "__main__":
    unittest.main()
