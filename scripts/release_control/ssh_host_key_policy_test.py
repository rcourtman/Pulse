import unittest
from pathlib import Path


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


if __name__ == "__main__":
    unittest.main()
