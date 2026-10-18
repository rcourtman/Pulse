#!/usr/bin/env python3
"""Keep the hosted migration entry point within the actual export/agent scope.

These check instructions, not installed Cloud recovery or provider access.
The real encrypted-archive control is TestConfigurationMigrationArchiveScope.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]


def guide():
    return (ROOT / "docs/CLOUD.md").read_text(encoding="utf-8")


def section(text, heading, level):
    return " ".join(text.split(f"{level} {heading}\n", 1)[1].split(f"\n{level} ", 1)[0].split())


class CloudMigrationDocsTest(unittest.TestCase):
    def test_in_app_guide_matches_canonical(self):
        self.assertEqual(guide(), (ROOT / "frontend-modern/public/docs/CLOUD.md").read_text())

    def test_export_scope_is_not_a_portable_installation_promise(self):
        text = section(guide(), "Migrating To/From Cloud", "##")
        for boundary in (
            "not the full installation", "Metrics and audit history",
            "incidents and queued notifications", "agent inventory and enrolment state",
            "profiles and assignments", "TrueNAS, vSphere and Machine Availability",
            "local login credentials or deployment overrides", "exporting version",
            "MIGRATION.md#configuration-transfer", "do not reveal the original tokens",
            "Hosted automatic backups and this downloaded export are different things",
            "Do not assume access to the hosted filesystem", "MIGRATION.md#full-state-recovery",
            "Do not mix encrypted files and keys", "Keep exports, passphrases and credentials private",
        ):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)
        self.assertNotIn("fully portable", guide())
        self.assertIn("not a complete copy of the hosted installation",
                      section(guide(), "Data & Privacy", "##"))

    def test_both_directions_preserve_access_source_and_destination(self):
        for heading in ("Self-Hosted → Cloud", "Cloud → Self-Hosted"):
            text = section(guide(), heading, "###")
            with self.subTest(direction=heading):
                for boundary in ("destination-local administrator access", "notification destinations",
                                 "API-token records", "not", "merge", "passphrase", "export",
                                 "excluded connections and settings", "before retiring"):
                    self.assertIn(boundary, text)
        hosted_exit = section(guide(), "Cloud → Self-Hosted", "###")
        self.assertIn("do not cancel it or discard the source", hosted_exit)
        self.assertIn("verify your Pro licence separately", hosted_exit)

    def test_retargeting_preserves_identity_and_does_not_inherit_endpoint_trust(self):
        text = section(guide(), "Migrating To/From Cloud", "##")
        self.assertEqual(text.count("UNIFIED_AGENT.md#moving-pulse-to-a-new-address"), 2)
        for boundary in ("saved identity and credential", "host's platform",
                         "rather than creating a replacement token", "HTTPS verification enabled",
                         "installer download", "agent connection", "certificate pin",
                         "not automatically trust for the new endpoint", "fresh ordinary reports",
                         "agent admission", "before enabling remote actions"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)
        self.assertNotIn("Update agent `--url` flags", text)
        self.assertNotIn("```", text, "No generic credential, reinstall or hosted-filesystem recipe")

    def test_import_errors_and_cutover_do_not_trigger_destructive_diagnostics(self):
        text = section(guide(), "Migrating To/From Cloud", "##")
        for boundary in ("Import can activate monitoring", "single-active cutover before importing",
                         "retain independent monitoring", "does not grant hosted filesystem",
                         "stop before importing", "reload/apply failure",
                         "settings may already have been written", "before repeating it",
                         "stop the trial destination", "intact source", "avoid two active writers",
                         "Do not induce alerts, replay a queue or run a workload action",
                         "not proof of full recovery", "Missing history", "does not prove collection failed"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)


if __name__ == "__main__":
    unittest.main()
