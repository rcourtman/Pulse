#!/usr/bin/env python3
"""Guardrails for first-wave localized public docs."""

from __future__ import annotations

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]


def read(rel: str) -> str:
    return (ROOT / rel).read_text(encoding="utf-8")


LOCALIZED_DOCS = (
    "docs/i18n/de/README.md",
    "docs/i18n/es/README.md",
)

MACHINE_FACING_STRINGS = (
    "export PULSE_VERSION=vX.Y.Z",
    "curl -fsSLO \"https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh\"",
    "curl -fsSLO \"https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh.sshsig\"",
    "ssh-keygen -Y verify",
    "bash install.sh --version \"${PULSE_VERSION}\"",
    "docker run -d",
    "rcourtman/pulse:vX.Y.Z",
    "image: ${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}",
    "PULSE_AUTH_USER=admin",
    "PULSE_AUTH_PASS=secret123",
    "docker exec pulse /app/pulse bootstrap-token",
    "kubectl exec -it <pod> -- /app/pulse bootstrap-token",
    "sudo pulse bootstrap-token",
    "Settings → Infrastructure → Install on a host",
    "Settings → Plans → Existing purchases",
    "ppk_live_",
    "http://<your-ip>:7655",
    "/install.sh",
    "https://pulserelay.pro/download.html",
)


class LocalizedPublicDocsTest(unittest.TestCase):
    def test_public_indexes_link_to_localized_getting_started_docs(self) -> None:
        root_readme = read("README.md")
        docs_index = read("docs/README.md")

        self.assertIn("[Deutsch](docs/i18n/de/README.md)", root_readme)
        self.assertIn("[Español](docs/i18n/es/README.md)", root_readme)
        self.assertIn("[Deutsch](i18n/de/README.md)", docs_index)
        self.assertIn("[Español](i18n/es/README.md)", docs_index)

    def test_localized_docs_preserve_machine_facing_strings(self) -> None:
        for rel in LOCALIZED_DOCS:
            content = read(rel)
            for value in MACHINE_FACING_STRINGS:
                with self.subTest(path=rel, value=value):
                    self.assertIn(value, content)

    def test_localized_docs_point_back_to_canonical_english_docs(self) -> None:
        for rel in LOCALIZED_DOCS:
            content = read(rel)
            with self.subTest(path=rel):
                self.assertIn("[docs/README.md](../../README.md)", content)
                self.assertIn("[Installation Guide](../../INSTALL.md)", content)
                self.assertIn("[Configuration](../../CONFIGURATION.md)", content)
                self.assertIn("[Troubleshooting](../../TROUBLESHOOTING.md)", content)
                self.assertIn("[Agent Security](../../AGENT_SECURITY.md)", content)
                self.assertIn("[Plans and entitlements](../../PULSE_PRO.md)", content)

    def test_localized_docs_do_not_reintroduce_paid_capacity_claims(self) -> None:
        forbidden = (
            "unlimited",
            "uncapped",
            "no cap",
            "no-cap",
            "sin límite",
            "ilimitado",
            "unbegrenzt",
        )
        for rel in LOCALIZED_DOCS:
            content = read(rel).lower()
            for phrase in forbidden:
                with self.subTest(path=rel, phrase=phrase):
                    self.assertNotIn(phrase, content)


if __name__ == "__main__":
    unittest.main()
