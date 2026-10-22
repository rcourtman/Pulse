#!/usr/bin/env python3
"""Check recovery guidance against the existing storage owners, not a restore.

No key, encrypted file, database or installation is opened by these checks.
Browser coverage exercises the shipped Docs consumer and its recovery links.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


def read(name):
    return (ROOT / name).read_text(encoding="utf-8")


def recovery():
    text = read("docs/CONFIGURATION.md")
    return " ".join(text.split("### Back up recoverable state\n", 1)[1]
                    .split("\n---", 1)[0].split())


class ConfigurationRecoveryDocsTest(unittest.TestCase):
    def test_current_provider_storage_matches_both_protocols_and_legacy_loader(self):
        source = read("internal/config/persistence.go")
        save = source.split("func (c *ConfigPersistence) SaveSSOConfig(", 1)[1]
        save = save.split("func (c *ConfigPersistence) LoadSSOConfig(", 1)[0]
        self.assertIn("c.crypto.Encrypt(data)", save)
        self.assertIn("c.writeConfigFileLocked(c.ssoFile, data, 0600)", save)
        legacy = source.split("func (c *ConfigPersistence) loadLegacyOIDCConfigAsSSOLocked(", 1)[1]
        legacy = legacy.split("// SaveAIConfig", 1)[0]
        self.assertIn('c.oidcFile, &legacy, "legacy oidc config"', legacy)
        self.assertIn("rewriteEncryptedJSONLocked(c, c.ssoFile", legacy)
        providers = read("internal/config/sso.go")
        for protocol in ('SSOProviderTypeOIDC SSOProviderType = "oidc"',
                         'SSOProviderTypeSAML SSOProviderType = "saml"'):
            self.assertIn(protocol, providers)

    def test_file_tables_do_not_send_current_oidc_recovery_to_legacy_storage(self):
        for name in ("CONFIGURATION", "DEPLOYMENT_MODELS"):
            text = read(f"docs/{name}.md")
            with self.subTest(guide=name):
                legacy = next(line for line in text.splitlines() if "`oidc.enc`" in line)
                current = next(line for line in text.splitlines() if "`sso.enc`" in line)
                self.assertIn("Legacy", legacy)
                self.assertIn("OIDC", current)
                self.assertIn("SAML", current)
                self.assertIn("UI/API", current)

    def test_recovery_set_includes_keys_and_effective_state_not_two_settings_files(self):
        text = recovery()
        for boundary in ("`.env` and `system.json` alone", "not a full backup",
                         "matching `.encryption.key`", "encrypted files", "audit signing key",
                         "active data paths", "organisation directories", "external stores",
                         "deployment-supplied credentials", "private"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)
        self.assertNotIn("Back up `.env` separately from `system.json`.", read("docs/CONFIGURATION.md"))

    def test_full_state_transfer_and_update_snapshots_remain_distinct(self):
        text = recovery()
        for target in ("MIGRATION.md#full-state-recovery", "MIGRATION.md#configuration-transfer",
                       "AUTO_UPDATE.md#what-an-update-snapshot-contains"):
            self.assertIn(target, text)
        self.assertIn("configuration export", text)
        self.assertIn("updater snapshot", text)
        self.assertIn("different, limited scopes", text)
        for name, heading in (("MIGRATION", "Full-state recovery"),
                              ("MIGRATION", "Configuration transfer"),
                              ("AUTO_UPDATE", "What an update snapshot contains")):
            self.assertRegex(read(f"docs/{name}.md"), rf"(?m)^#{{2,4}} {re.escape(heading)}$")

    def test_recovery_preserves_source_and_does_not_reset_or_mutate_live_state(self):
        text = recovery()
        for boundary in ("stopped, consistent", "monitoring and alerts pause",
                         "isolated destination", "before retiring the source",
                         "Do not restore over a running instance", "regenerate keys",
                         "edit encrypted files", "startup import", "not a recovery shortcut",
                         "keep it stopped", "VM_DISK_MONITORING.md#backup-safety"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)
        self.assertNotIn("```", text, "No generic copy, deletion or live-restore recipe")

    def test_provider_save_failure_keeps_existing_admin_recovery_route(self):
        text = recovery()
        self.assertIn("OIDC.md#sign-in-fails-after-editing-a-provider", text)
        self.assertIn("working administrator session", text)
        self.assertIn("not a repair for that sign-in defect", text)

    def test_uncertain_import_guidance_is_not_a_rejection_or_rollback_recipe(self):
        text = read("docs/MIGRATION.md")
        self.assertIn("[unconfirmed-import checks](#import-was-not-confirmed)", text)
        uncertain = " ".join(text.split("### Import was not confirmed\n", 1)[1]
                             .split("\n### ", 1)[0].split())
        for boundary in ("timeout, lost connection or incomplete response",
                         "does not prove the archive was rejected",
                         "working administrator session", "without saving",
                         "Do not sign out, repeat the import",
                         "does not undo an import that reached the server",
                         "stop the cutover", "not a second attempt"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, uncertain)
        # Bind the advice to the real producer's uncertain-write message, not
        # an invented rejection state. This is not execution or restore proof.
        self.assertIn("Could not confirm the import. Settings may already have changed.",
                      read("frontend-modern/src/components/Settings/useBackupTransferFlow.ts"))

    def test_unsent_import_size_checks_do_not_clear_an_earlier_uncertain_write(self):
        text = read("docs/MIGRATION.md")
        self.assertIn("[unsent-import checks](#import-was-not-sent)", text)
        unsent = " ".join(text.split("### Import was not sent\n", 1)[1]
                          .split("\n### ", 1)[0].split())
        for boundary in ("No import request was sent", "1 MiB (1,048,576 bytes)",
                         "before it is read", "complete UTF-8 JSON request",
                         "including encrypted data and the passphrase",
                         "A smaller file can still exceed", "original file intact",
                         "do not split, truncate", "stop the cutover",
                         "not the file or passphrase", "only this attempt",
                         "earlier attempt lost its response",
                         "#import-was-not-confirmed", "does not undo it"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, unsent)
        # These checks bind help to the two existing local guards and server
        # budget. Runtime dispatch behaviour has its own executable coverage.
        producer = read("frontend-modern/src/components/Settings/useBackupTransferFlow.ts")
        self.assertEqual(producer.count("No import request was sent."), 2)
        self.assertIn("const MAX_CONFIG_IMPORT_BYTES = 1024 * 1024;", producer)
        self.assertIn("file.size > MAX_CONFIG_IMPORT_BYTES", producer)
        self.assertIn("new TextEncoder().encode(body).byteLength > MAX_CONFIG_IMPORT_BYTES", producer)
        self.assertIn("http.MaxBytesReader(w, r.Body, 1024*1024)",
                      read("internal/api/configapi/config_export_import_handlers.go"))

    def test_deployment_inventory_links_to_the_same_recovery_checklist(self):
        self.assertIn("CONFIGURATION.md#back-up-recoverable-state", read("docs/DEPLOYMENT_MODELS.md"))

    def test_shipped_guides_match_canonical_bytes(self):
        for name in ("CONFIGURATION", "DEPLOYMENT_MODELS", "MIGRATION"):
            self.assertEqual(read(f"docs/{name}.md"), read(f"frontend-modern/public/docs/{name}.md"))


if __name__ == "__main__":
    unittest.main()
