#!/usr/bin/env python3
"""Check the existing paid-recovery guide against the actual local controls.

These are documentation/source checks, not payment, mail or installed-runtime
acceptance. The consuming Docs journey has separate browser evidence.
"""

import importlib.util
from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "Already paid but Pro features are unavailable"
SLUG = "already-paid-but-pro-features-are-unavailable"
GUIDES = ("PULSE_PRO", "TROUBLESHOOTING", "FAQ")


def source(relative):
    return (ROOT / relative).read_text(encoding="utf-8")


def recovery():
    return source("docs/PULSE_PRO.md").split(f"### {HEADING}\n", 1)[1].split("\n### ", 1)[0]


class LicenseRecoveryDocsTest(unittest.TestCase):
    def test_the_shipped_guides_match_their_sources(self):
        for guide in GUIDES:
            with self.subTest(guide=guide):
                self.assertEqual(source(f"docs/{guide}.md"),
                                 source(f"frontend-modern/public/docs/{guide}.md"))

    def test_both_existing_symptom_entrypoints_reach_the_same_recovery_section(self):
        for guide in ("TROUBLESHOOTING", "FAQ"):
            with self.subTest(guide=guide):
                self.assertIn(f"](PULSE_PRO.md#{SLUG})", source(f"docs/{guide}.md"))
        spec = importlib.util.spec_from_file_location("public_docs", ROOT / "scripts/check_public_docs.py")
        checker = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(checker)
        self.assertEqual(checker.heading_slug(HEADING), SLUG)
        self.assertEqual(checker.check_links([ROOT / f"docs/{guide}.md" for guide in GUIDES]), [])

    def test_the_route_names_are_the_actual_billing_and_recovery_controls(self):
        prose = " ".join(recovery().split())
        billing = source("frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts")
        controls = source("frontend-modern/src/utils/licensePresentation.ts")
        for label in ("Plans & Billing", "Current plan", "License recovery"):
            with self.subTest(label=label):
                self.assertIn(f"'{label}'", billing)
                self.assertIn(label, prose)
        for label in ("Manual key recovery", "Apply key"):
            with self.subTest(label=label):
                self.assertIn(f"'{label}'", controls)
                self.assertIn(label, prose)

    def test_an_active_plan_does_not_claim_a_private_runtime_is_installed(self):
        prose = " ".join(recovery().split())
        presentation = source("frontend-modern/src/utils/licensePresentation.ts")
        for phrase in ("Pro runtime missing", "public Docker image", "Pro-only runtime hooks"):
            self.assertIn(phrase, prose)
            self.assertIn(phrase, presentation)
        self.assertIn("an accepted key does not install a different runtime", prose)
        self.assertIn("not an installed or running Pro runtime", prose)
        self.assertIn("preserve its persistent data, configuration and keys", prose)
        urls = re.findall(r"https://[^)\s]+", recovery())
        self.assertEqual(urls, ["https://pulserelay.pro/download.html",
                                "https://pulserelay.pro/retrieve-license"])
        for url in urls:
            self.assertIn(f"'{url}'", presentation)

    def test_a_clear_is_not_presented_as_a_refresh_or_cancellation(self):
        prose = " ".join(recovery().split()).replace("**", "")
        self.assertIn("Clear key removes the local activation", prose)
        self.assertIn("not a status refresh or a billing cancellation", prose)
        self.assertIn("clearIdleLabel: 'Clear key'", source("frontend-modern/src/utils/licensePresentation.ts"))
        handler = source("internal/api/licensing_handlers.go").split(
            "func (h *LicenseHandlers) HandleClearLicense", 1)[1].split(
            "// RequireLicenseFeature", 1)[0]
        self.assertIn("persistence.Delete()", handler)
        self.assertIn("persistence.ClearActivationState()", handler)
        self.assertNotIn("stripe", handler.lower())

    def test_failed_or_unknown_results_do_not_send_the_user_back_to_checkout(self):
        prose = " ".join(recovery().split())
        for phrase in ("Payment, local activation and the installed runtime are separate",
                       "does not establish that a payment failed",
                       "Do not buy again, clear a working key or repeat checkout",
                       "failed status refresh is unavailable evidence",
                       "before applying another key"):
            self.assertIn(phrase, " ".join(source("docs/PULSE_PRO.md").split()))
        self.assertNotIn("retry from the v6 license panel", source("docs/PULSE_PRO.md"))

    def test_key_recovery_preserves_credentials_and_avoids_new_diagnostics(self):
        prose = " ".join(recovery().split())
        for phrase in ("verification code private; it is not the activation key",
                       "not a legacy v5 licence key", "supported legacy v5 Pro/Lifetime keys",
                       "not in a command, URL or GitHub report",
                       "`.encryption.key` private and intact",
                       "do not delete them, reinstall or run Diagnostics",
                       "private support route"):
            self.assertIn(phrase, prose)
        self.assertNotIn("```", recovery(), "no new credential or live diagnostic recipe")
        self.assertNotRegex(recovery(), r"ppk_live_[A-Za-z0-9]+|Bearer\s|--insecure|--location")


if __name__ == "__main__":
    unittest.main()
