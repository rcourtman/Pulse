import unittest
from pathlib import Path
import subprocess
import sys


REPO_ROOT = Path(__file__).resolve().parents[2]
TEXT_RUNTIME_SUFFIXES = (".go", ".py", ".sh", ".yml", ".yaml")
ALLOWED_STRICT_HOST_KEY_CHECKING_NO = {
    "internal/hostagent/cluster_sensors.go",
}
ALLOWED_USER_KNOWN_HOSTS_DEV_NULL = {
    "internal/hostagent/cluster_sensors.go",
}


def iter_runtime_files():
    for base in (REPO_ROOT / "internal", REPO_ROOT / "scripts", REPO_ROOT / ".github" / "workflows"):
        if not base.exists():
            continue
        for path in base.rglob("*"):
            if not path.is_file():
                continue
            relative = path.relative_to(REPO_ROOT).as_posix()
            if not relative.endswith(TEXT_RUNTIME_SUFFIXES):
                continue
            if relative.endswith(("_test.go", "_test.py", ".md")):
                continue
            if relative.startswith("scripts/installtests/") or relative.startswith("scripts/tests/"):
                continue
            yield path, relative


class SSHHostKeyPolicyTest(unittest.TestCase):
    def test_runtime_paths_do_not_disable_ssh_host_key_verification(self) -> None:
        strict_host_key_checking_no = []
        user_known_hosts_dev_null = []

        for path, relative in iter_runtime_files():
            content = path.read_text(encoding="utf-8")
            if "StrictHostKeyChecking=no" in content:
                strict_host_key_checking_no.append(relative)
            if "UserKnownHostsFile=/dev/null" in content:
                user_known_hosts_dev_null.append(relative)

        self.assertEqual(
            sorted(strict_host_key_checking_no),
            sorted(ALLOWED_STRICT_HOST_KEY_CHECKING_NO),
            f"unexpected StrictHostKeyChecking=no runtime surfaces: {strict_host_key_checking_no}",
        )
        self.assertEqual(
            sorted(user_known_hosts_dev_null),
            sorted(ALLOWED_USER_KNOWN_HOSTS_DEV_NULL),
            f"unexpected UserKnownHostsFile=/dev/null runtime surfaces: {user_known_hosts_dev_null}",
        )

    def test_dev_agent_deploy_uses_tofu_instead_of_disabling_host_checks(self) -> None:
        script = (REPO_ROOT / "scripts" / "dev-deploy-agent.sh").read_text(encoding="utf-8")

        self.assertNotIn("StrictHostKeyChecking=no", script)
        self.assertIn("StrictHostKeyChecking=accept-new", script)
        self.assertIn("UpdateHostKeys=yes", script)
        self.assertIn("UserKnownHostsFile=$SSH_KNOWN_HOSTS_FILE", script)

    def test_sensor_proxy_uninstall_fails_closed_or_stays_local(self) -> None:
        script = (REPO_ROOT / "scripts" / "uninstall-sensor-proxy.sh").read_text(encoding="utf-8")

        self.assertNotIn("StrictHostKeyChecking=no", script)
        self.assertIn("StrictHostKeyChecking=yes", script)
        self.assertIn("UpdateHostKeys=no", script)
        self.assertIn('UserKnownHostsFile=${known_hosts_path}', script)
        self.assertIn("GlobalKnownHostsFile=none", script)
        self.assertIn("--ssh-known-hosts", script)
        self.assertIn("--local-only", script)

    def test_sensor_proxy_discovery_reaches_strict_remote_cleanup_only_for_complete_targets(self) -> None:
        # Keep the established trust proof executable, not just a string check:
        # complete JSON must reach strict SSH; a missing target or key refusal
        # must remain adverse. All provider/DNS/SSH operations are closed fakes.
        suite = REPO_ROOT / "scripts/tests/test_sensor_proxy_cluster_cleanup.py"
        cases = (
            "test_complete_inventory_cleans_remote_with_strict_trust",
            "test_failed_resolution_of_any_node_prevents_all_remote_actions",
            "test_remote_key_rejection_remains_an_error_after_local_cleanup",
        )
        result = subprocess.run(
            [sys.executable, str(suite),
             *("SensorProxyClusterCleanupTest." + case for case in cases)],
            text=True, capture_output=True, timeout=20,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Ran 3 tests", result.stderr)


if __name__ == "__main__":
    unittest.main()
