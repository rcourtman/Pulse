#!/usr/bin/env python3
"""Keep startup-import help from implying safe recovery of an existing server.

These are documentation controls, not native import or installed recovery proof.
The production gate/importer and the attempted runtime probe are retained with
the maintainer's outcome; no credential or service is used by these tests.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


def bootstrap(text):
    return " ".join(text.split("### Auto-Import (Bootstrap)\n", 1)[1]
                    .split("\n### ", 1)[0].split())


class BootstrapImportDocsTest(unittest.TestCase):
    def guide(self):
        return bootstrap((ROOT / "docs/CONFIGURATION.md").read_text())

    def assert_gate_and_result(self, text):
        for boundary in ("`nodes.enc`", "`PULSE_DATA_DIR`", "empty data directory",
                         "only exception", "API-token records", "encryption keys",
                         "unknown contents", "all prevent startup import",
                         "Unreadable directories and symbolic links",
                         "checks the destination again before writing",
                         "lock against another running Pulse process",
                         "Do not delete `nodes.enc`",
                         "not proof that import succeeded", "startup continues",
                         "normal configuration loading", "can itself fail"):
            self.assertIn(boundary, text)
        self.assertNotIn("auto-imports on first start", text)
        self.assertNotIn("do not prevent an import attempt", text)

    def assert_transfer_scope(self, text):
        for boundary in ("MIGRATION.md#configuration-transfer", "not a full-state backup",
                         "replaced, not merged", "SSO settings", "not server-side agent",
                         "inventory or enrolment state", "local login credentials",
                         "monitoring history", "identity, credential and TLS trust",
                         "fresh admission", "ordinary reports", "before retiring the source"):
            self.assertIn(boundary, text)
        self.assertNotIn("import restores server-side agent credentials", text)

    def test_canonical_and_served_mirrors_agree(self):
        for name in ("CONFIGURATION.md", "MIGRATION.md"):
            self.assertEqual((ROOT / "docs" / name).read_bytes(),
                             (ROOT / "frontend-modern/public/docs" / name).read_bytes())

    def test_gate_is_not_a_first_run_or_success_guarantee(self):
        self.assert_gate_and_result(self.guide())

    def test_scope_preserves_agent_access_and_excluded_state(self):
        self.assert_transfer_scope(self.guide())

    def test_input_precedence_and_private_provisioning_are_explicit(self):
        text = self.guide()
        for boundary in ("takes precedence over `PULSE_INIT_CONFIG_DATA`",
                         "does **not** fall back", "not supported by server startup",
                         "owner-only", "private credential source", "separate from the backup",
                         "command arguments, URLs, shared Compose files, logs or issue reports",
                         "base64 is not redaction", "no passphrase-file variable",
                         "authenticated Recovery form",
                         "outside the destination data directory",
                         "archive placed inside it", "retains those contents"):
            self.assertIn(boundary, text)
        self.assertNotRegex(text, r"PULSE_INIT_CONFIG_(?:PASSPHRASE_FILE|FORCE)")
        self.assertNotRegex(text, r"```(?:bash|sh|shell|yaml|dotenv)\b")

    def test_cutover_and_retry_boundaries_are_usable_without_a_reset(self):
        text = self.guide()
        for boundary in ("isolated destination", "private backup of the destination",
                         "Isolate the trial destination from agents and notification",
                         "activate monitoring and delivery", "one active installation",
                         "independent outage coverage", "stop the trial destination",
                         "reconcile its state before another attempt", "Do not repeat import",
                         "replay notifications", "remove only the three",
                         "deployment's managed source", "Docker restart alone does not apply"):
            self.assertIn(boundary, text)
        migration = (ROOT / "docs/MIGRATION.md").read_text()
        self.assertIn("CONFIGURATION.md#auto-import-bootstrap", migration)

    def test_discriminates_the_unsafe_promises(self):
        # Removing either boundary restores a real misleading interpretation.
        text = self.guide()
        for changed in (
            text.replace("all prevent startup import", "do not prevent startup import"),
            text.replace("not proof that import succeeded", "proof that import succeeded"),
        ):
            with self.assertRaises(AssertionError):
                self.assert_gate_and_result(changed)
        with self.assertRaises(AssertionError):
            self.assert_transfer_scope(text.replace("replaced, not merged", "merged"))


if __name__ == "__main__":
    unittest.main()
