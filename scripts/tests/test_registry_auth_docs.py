#!/usr/bin/env python3
"""Keep private-registry help honest about credential and action boundaries.

These are documentation/source consistency checks, not a registry, credential
helper, Docker daemon or installed private-pull acceptance test.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]


def guidance():
    text = (ROOT / "docs/DOCKER.md").read_text()
    return " ".join(text.split("### Private Registries\n", 1)[1]
                    .split("\n### ", 1)[0].replace("**", "").split())


class RegistryAuthDocsTest(unittest.TestCase):
    def test_discloses_authentication_beyond_the_host_and_registry_hostname(self):
        text = guidance()
        for fact in (
            "Authentication is not host-local",
            "send a stored password or token to the registry or its HTTPS token service",
            "token service can have a different hostname",
            "auth.docker.io",
            "Stored login entries are not included in the update-check report",
        ):
            with self.subTest(fact=fact):
                self.assertIn(fact, text)
        self.assertNotIn("Credentials never leave the host", text)
        registry = (ROOT / "internal/dockeragent/registry.go").read_text()
        self.assertIn('req.SetBasicAuth(creds.Username, creds.Secret)', registry)
        self.assertIn('form.Set("refresh_token", creds.Secret)', registry)

    def test_does_not_assume_root_or_require_secret_arguments_or_broader_access(self):
        text = guidance()
        for boundary in (
            "account running the agent",
            "not necessarily visible to a system service or container",
            "interactive prompt",
            "Do not switch a least-privilege agent to root",
            "relax credential-file permissions",
            "credential-helper output and passwords or tokens private",
            "Configured helpers execute as the agent's account",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)
        for unsafe in ("sudo docker login", "--password ", "--password=", "docker login -p"):
            self.assertNotIn(unsafe, text)

    def test_credential_opt_out_is_not_an_offline_or_monitoring_only_promise(self):
        text = guidance()
        for boundary in (
            "prevent credential-file reads and helper execution for detection",
            "PULSE_DISABLE_REGISTRY_CREDENTIALS=true",
            "--disable-registry-credentials",
            "Public checks can continue anonymously",
            "private checks requiring a login can fail authentication",
            "not stop all registry traffic, revoke a login or disable container actions",
            "#disabling-update-features",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)

    def test_detection_does_not_promise_authenticated_private_pulls(self):
        text = guidance()
        for boundary in (
            "Detection and pulling an update are separate",
            "does not pass detection's stored registry login to the Docker API",
            "private pull can fail even when its digest check succeeds",
            "not repeated Pulse actions to test the login",
            "#check-a-failed-or-pending-workload-update",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)
        self.assertNotIn("daemon, which reads these credentials natively", text)

    def test_shipped_guide_matches_the_repository_document(self):
        self.assertEqual((ROOT / "docs/DOCKER.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/DOCKER.md").read_bytes())


if __name__ == "__main__":
    unittest.main()
