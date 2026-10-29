#!/usr/bin/env python3
"""Keep private-registry help faithful to the existing agent credential path.

These are documentation/safety checks, not installed registry acceptance.
Discovered by scripts/tests/run.sh and the documentation CI shards.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


def section():
    return (ROOT / "docs/DOCKER.md").read_text().split(
        "### Private Registries\n", 1
    )[1].split("\n### Disabling Update Features", 1)[0]


def prose():
    return " ".join(section().split())


class DockerRegistryDocsTest(unittest.TestCase):
    def test_shipped_document_matches_source(self):
        self.assertEqual((ROOT / "docs/DOCKER.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/DOCKER.md").read_bytes())

    def test_login_follows_agent_account_not_an_assumed_root_service(self):
        text = prose()
        for fact in ("not on the Pulse server", "service account", "least-privilege",
                     "service environment", "Do not switch the agent to root"):
            self.assertIn(fact, text)
        self.assertNotIn("the installer's systemd service runs the agent as root", text)
        self.assertIn("only for an agent that actually runs as root", text)

    def test_paths_and_cache_match_the_existing_producer(self):
        text = section()
        source = (ROOT / "internal/dockeragent/registry_credentials.go").read_text()
        for name in ("REGISTRY_AUTH_FILE", "DOCKER_CONFIG", "XDG_RUNTIME_DIR"):
            self.assertIn(f'c.getenv("{name}")', source)
            self.assertIn(f"`{name}`", text)
        for path in ("config.json", "auth.json", "$HOME/.docker/config.json",
                     "$XDG_RUNTIME_DIR/containers/auth.json"):
            self.assertIn(path, text)
        self.assertRegex(source, r"credentialCacheTTL\s*=\s*5\s*\*\s*time.Minute")
        self.assertIn("up to five minutes, including missing logins", prose())
        self.assertIn("next ordinary scheduled check", prose())

    def test_container_and_helper_visibility_keep_least_privilege(self):
        text = prose()
        for fact in ("Docker socket access does not expose the host's login files",
                     "container's filesystem", "read-only", "container path",
                     "helper binary and its backing store", "not just `config.json`",
                     "pull-only", "Do not mount an entire home directory",
                     "make credentials world-readable"):
            self.assertIn(fact, text)

    def test_detection_and_workload_actions_are_not_conflated(self):
        text = prose()
        for fact in ("does not require enabling command execution",
                     "not proof that a workload update can pull or recreate",
                     "PULSE_DISABLE_REGISTRY_CREDENTIALS", "--disable-registry-credentials",
                     "no credential-file reads or helper execution"):
            self.assertIn(fact, text)
        self.assertNotIn("daemon, which reads these credentials natively", text)

    def test_error_guidance_does_not_request_secrets_or_weaken_tls(self):
        text = prose()
        for fact in ("not evidence that the image is up to date", "existing short error",
                     "TLS or network failure", "Do not disable TLS verification",
                     "Never share login files, helper output, passwords or tokens",
                     "registry or its authentication endpoint", "not reported to the Pulse server"):
            self.assertIn(fact, text)
        blocks = re.findall(r"```bash\n(.*?)```", section(), re.S)
        self.assertEqual(blocks, ["docker login registry.example.com\n"])
        self.assertNotRegex(text, r"(?i)(?:docker|podman) login[^`\n]*(?:--password(?:\s|=)|\s-p\s)")


if __name__ == "__main__":
    unittest.main()
