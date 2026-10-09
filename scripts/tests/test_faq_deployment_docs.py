#!/usr/bin/env python3
"""Bind the short deployment answers to source and the safer detailed paths.

Documentation checks, not an installed port change or environment cutover.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]


def faq():
    return (ROOT / "docs/FAQ.md").read_text(encoding="utf-8")


def answer(heading):
    return " ".join(faq().split(f"### {heading}\n", 1)[1].split("\n### ", 1)[0].split())


class FAQDeploymentDocsTest(unittest.TestCase):
    def test_ports_distinguish_listener_mapping_and_server_host(self):
        text = answer("How do I change the port?")
        for phrase in ("`FRONTEND_PORT`", "default **7655**", "`PULSE_AGENT_INGEST_PORT`",
                       "`8080:7655`", "`PULSE_PORT`", "inside the Pulse container",
                       "not on the Proxmox host", "active Pulse service",
                       "service-manager reload", "maintenance window"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)
        config = (ROOT / "internal/config/config.go").read_text()
        self.assertIn('envconfig:"FRONTEND_PORT" default:"7655"', config)
        self.assertIn('envconfig:"PULSE_AGENT_INGEST_PORT"', config)
        compose = (ROOT / "docker-compose.yml").read_text()
        self.assertIn('${PULSE_PORT:-7655}:7655', compose)

    def test_existing_deployment_and_security_survive_port_change(self):
        text = answer("How do I change the port?")
        for phrase in ("existing container manager", "recreate/redeploy",
                       "preserving the same image, data mount and other settings",
                       "`docker restart` does not apply a new port mapping",
                       "Do not delete a data volume", "fresh `docker run` command",
                       "reverse proxy's upstream port separately",
                       "do not expose Pulse directly", "inspections private",
                       "TROUBLESHOOTING.md#port-change-didnt-take-effect"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)

    def test_override_handover_is_scoped_applied_and_verified(self):
        text = answer("Why can't I change settings in the UI?")
        for phrase in ("warning that names an environment variable", "If it is intentional, keep it",
                       "do not remove unrelated authentication or security overrides",
                       "remove only its override", "active deployment source",
                       "Docker require recreation/redeployment, not `docker restart`",
                       "service-manager reload and restart", "Preserve the image, data mount, credentials",
                       "does not change the running process's environment",
                       "check both the warning and effective value",
                       "does not necessarily erase the saved value", "`ALLOWED_ORIGINS`",
                       "no environment warning", "Do not post full environment"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)
        config = (ROOT / "internal/config/config.go").read_text()
        self.assertIn('cfg.EnvOverrides["ALLOWED_ORIGINS"] = true', config)
        form = (ROOT / "frontend-modern/src/components/Settings/DiscoverySettingsForm.tsx").read_text()
        self.assertIn('props.envOverrides().discoveryEnabled || props.savingDiscoverySettings()', form)
        self.assertIn('sectionPresentation().environmentOverrideMessage', form)

    def test_shipped_copy_and_detailed_destinations_exist(self):
        self.assertEqual(faq(), (ROOT / "frontend-modern/public/docs/FAQ.md").read_text())
        for guide, heading in (("TROUBLESHOOTING", "### Port change didn't take effect"),
                               ("TROUBLESHOOTING", "### CORS errors"),
                               ("CONFIGURATION", "### Common Overrides (Environment Variables)")):
            with self.subTest(guide=guide, heading=heading):
                self.assertIn(heading, (ROOT / "docs" / f"{guide}.md").read_text())


if __name__ == "__main__":
    unittest.main()
