#!/usr/bin/env python3
"""Keep licence recovery help distinct from runtime installation and account proof.

This checks shipped guidance against existing controls, not an activation,
private download or installed-runtime recovery.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "### Activation succeeds but Pro features are unavailable"
FRAGMENT = "PULSE_PRO.md#activation-succeeds-but-pro-features-are-unavailable"


def section(text):
    return " ".join(text.split(HEADING + "\n", 1)[1].split("\n### ", 1)[0].split())


class LicenseRecoveryDocsTest(unittest.TestCase):
    def test_shipped_guides_match_and_short_answer_reaches_recovery(self):
        for name in ("PULSE_PRO", "FAQ"):
            with self.subTest(guide=name):
                source = (ROOT / f"docs/{name}.md").read_bytes()
                self.assertEqual(source, (ROOT / f"frontend-modern/public/docs/{name}.md").read_bytes())
        self.assertIn(FRAGMENT, (ROOT / "docs/FAQ.md").read_text())

    def test_active_key_is_not_a_private_runtime_install(self):
        text = section((ROOT / "docs/PULSE_PRO.md").read_text())
        for phrase in (
            "accepted key and an installed Pro runtime are separate requirements",
            "intended Pulse instance and organisation",
            "applying it again does not install the private Pro runtime",
            "https://pulserelay.pro/download.html",
            "AUTO_UPDATE.md#manual-update-methods",
            "preserve the configuration, data, keys and service identity",
            "Do not replace an existing private image with the public image",
            "buy another licence",
            "check the running edition, loaded plan",
            "does not upgrade installed agents",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)
        presentation = (ROOT / "frontend-modern/src/utils/licensePresentation.ts").read_text()
        self.assertIn("Pro runtime missing", presentation)
        self.assertIn("They can accept the license, but Pro-only runtime hooks", presentation)
        self.assertIn("privateRuntimeAction:", presentation)

    def test_failed_reads_and_rejected_keys_do_not_prompt_destructive_reset(self):
        text = section((ROOT / "docs/PULSE_PRO.md").read_text())
        for phrase in (
            "key-format error, an unavailable licence service and a build without a validation key",
            "Do not use **Clear key** as a diagnostic reset",
            "delete licence files",
            "read is unavailable, not proof that the licence was removed",
            "original error and time",
            "session cookies and full configuration private",
            "Do not paste them into shell commands, URLs or public reports",
            "locally redacted error; unknown is valid",
            "private account/support route",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)
        controls = (ROOT / "frontend-modern/src/utils/licensePresentation.ts").read_text()
        for label in ("Manual key recovery", "Apply key", "Clear key"):
            self.assertIn(label, controls)
        handlers = (ROOT / "internal/api/licensing_handlers.go").read_text()
        self.assertIn("This Pulse build cannot verify Pro licenses", handlers)
        self.assertIn("The license server is temporarily unavailable", handlers)


if __name__ == "__main__":
    unittest.main()
