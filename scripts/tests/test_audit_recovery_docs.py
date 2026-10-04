#!/usr/bin/env python3
"""Keep audit help from treating a failed check as a diagnosis or a reset task.

Signer/key behaviour is exercised separately by TestSignerRecoveryGuidance;
these checks bind those distinctions to the actual canonical and shipped help.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
GUIDES = ("AUDIT_LOGGING", "TROUBLESHOOTING", "CONFIGURATION", "DEPLOYMENT_MODELS")


def read_guide(name: str) -> str:
    return " ".join((ROOT / "docs" / f"{name}.md").read_text(encoding="utf-8").split())


class AuditRecoveryDocsTest(unittest.TestCase):
    def test_shipped_guides_match_the_canonical_evidence(self):
        for name in GUIDES:
            with self.subTest(guide=name):
                self.assertEqual((ROOT / "docs" / f"{name}.md").read_bytes(),
                                 (ROOT / "frontend-modern/public/docs" / f"{name}.md").read_bytes())

    def test_verification_results_are_not_conflated(self):
        guide = read_guide("AUDIT_LOGGING")
        for result in ("**Not checked**", "**Unsigned**", "**Failed**", "**Unavailable**", "**Error**"):
            with self.subTest(result=result):
                self.assertIn(result, guide)
        self.assertIn("not that Pulse has established why it failed", guide)
        self.assertIn("not proof that the event failed verification", guide)
        self.assertIn("does not prove that the history is complete", guide)
        self.assertNotIn("the event data has been tampered with since it was recorded", guide)

    def test_recovery_preserves_history_and_both_keys(self):
        guide = read_guide("AUDIT_LOGGING")
        for boundary in ("consistent private backup", "copying a live `.db` file alone",
                         "including any SQLite sidecar files", "matching `.encryption.key`",
                         "isolated test instance", "without replacing the live data",
                         "or contacting monitored systems and notification destinations",
                         "Do not delete or regenerate a key", "edit audit rows or re-sign old events"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, guide)
        self.assertIn("newly generated signing key does not verify events signed with the previous key", guide)
        self.assertIn("Later enabling signing cannot authenticate it retroactively", guide)

    def test_troubleshooting_does_not_offer_a_live_key_swap(self):
        guide = read_guide("TROUBLESHOOTING")
        self.assertNotIn("restart Pulse to regenerate", guide)
        self.assertNotIn("Restore the previous `.audit-signing.key` from backup to verify", guide)
        self.assertIn("Do not swap an old key into the live instance", guide)
        self.assertIn("cannot authenticate an old unsigned event", guide)
        self.assertIn("AUDIT_LOGGING.md#verification-failures-and-safe-recovery", guide)

    def test_empty_and_gated_are_not_missing_history(self):
        for name in ("TROUBLESHOOTING", "AUDIT_LOGGING"):
            guide = read_guide(name)
            with self.subTest(guide=name):
                self.assertIn("Pulse Pro runtime required", guide)
                self.assertIn("Download Pulse Pro", guide)
                self.assertIn("filters", guide)
                self.assertIn("organisation", guide)
        short = read_guide("TROUBLESHOOTING")
        self.assertIn("A query error is not an empty history", short)
        self.assertIn("Do not change passwords or create tokens merely to populate the panel", short)
        self.assertNotIn("Community plan uses console logging only", short)

    def test_storage_paths_match_the_default_store_not_the_old_root_path(self):
        for name in ("CONFIGURATION", "DEPLOYMENT_MODELS"):
            guide = read_guide(name)
            with self.subTest(guide=name):
                self.assertIn("`audit/audit.db`", guide)
                self.assertIn("`audit/.audit-signing.key`", guide)
                self.assertIn("AUDIT_LOGGING.md#storage", guide)
                self.assertNotRegex(guide, r"(?:\| |\*\*: )`\.audit-signing\.key`")
        guide = read_guide("AUDIT_LOGGING")
        self.assertIn("**inside the audit store directory**", guide)
        self.assertIn("organisation's data directory", guide)
        self.assertIn("runtime may supply a different audit directory or managed signing key", guide)


if __name__ == "__main__":
    unittest.main()
