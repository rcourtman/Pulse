#!/usr/bin/env python3
"""Keep licence recovery help distinct from runtime installation and account proof.

This checks shipped guidance against existing controls, not an activation,
private download or installed-runtime recovery.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
HEADING = "### Activation succeeds but Pro features are unavailable"
FRAGMENT = "PULSE_PRO.md#activation-succeeds-but-pro-features-are-unavailable"
PRICING_DOC = ROOT / "docs/architecture/v6-pricing-and-tiering.md"


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

    def test_license_recovery_points_to_system_plans_and_billing_route(self):
        text = section((ROOT / "docs/PULSE_PRO.md").read_text())
        self.assertIn(
            "Open **Settings → System → Plans & Billing** (`/settings/pulse-intelligence/billing/plan`, **Plan** tab)",
            text,
        )
        self.assertNotIn("Open **Plans & Billing → Plan**", text)
        self.assertNotIn("/settings/system-billing/plan", text)
        catalog = (ROOT / "frontend-modern/src/components/Settings/settingsNavCatalog.ts").read_text(encoding="utf-8")
        self.assertIn("id: 'system-billing'", catalog)
        billing = (ROOT / "frontend-modern/src/components/Settings/selfHostedBillingPresentation.ts").read_text(encoding="utf-8")
        self.assertIn("navLabel: 'Plans & Billing'", billing)
        self.assertIn("planTabLabel: 'Plan'", billing)
        pricing = (ROOT / "frontend-modern/src/utils/pricingHandoff.ts").read_text(encoding="utf-8")
        self.assertIn("export const SELF_HOSTED_PRO_BILLING_ROUTE = '/settings/pulse-intelligence/billing';", pricing)
        self.assertIn("export const SELF_HOSTED_PRO_BILLING_PLAN_ROUTE = `${SELF_HOSTED_PRO_BILLING_ROUTE}/plan`;", pricing)
        nav = (ROOT / "frontend-modern/src/components/Settings/settingsNavigationModel.ts").read_text(encoding="utf-8")
        self.assertIn("case 'system-billing':\n      return SELF_HOSTED_PRO_BILLING_PLAN_ROUTE;", nav)


class PricingTieringDocsTest(unittest.TestCase):
    def setUp(self) -> None:
        self.raw = PRICING_DOC.read_text(encoding="utf-8")
        self.prose = " ".join(self.raw.split())

    def test_relay_is_retired_and_not_offered_as_web_remote_access(self) -> None:
        self.assertIn("29 September 2026", self.prose)
        self.assertIn("31 March 2027", self.prose)
        self.assertIn("Relay only ever connected the Pulse Mobile app", self.prose)
        self.assertIn("never provided remote access to the Pulse web UI", self.prose)
        self.assertIn("receive Pro entitlements for as long as their subscription", self.prose)
        self.assertIn("nobody moves onto a retired Relay price", self.prose)
        self.assertIn("Relay is retired and is neither a target offer nor a downgrade option from Pro", self.prose)

        self.assertNotIn("[Buy Relay]", self.raw)
        self.assertNotIn("Community to Relay or Pro", self.prose)
        self.assertNotIn("| Pro to Relay |", self.raw)
        self.assertNotIn("Want Pulse outside your LAN? Upgrade to Relay", self.prose)
        self.assertNotRegex(self.prose, r"Relay\s+is\s+the\s+access\s+service")

    def test_business_and_pro_seat_terms_are_documented(self) -> None:
        self.assertIn("### Business ($399/year, annual only)", self.raw)
        self.assertIn("20 August 2026", self.prose)
        self.assertIn("3 named team seats", self.prose)
        self.assertIn("Unlimited named team seats", self.prose)
        self.assertIn("Next-business-day target", self.prose)
        self.assertIn("price_1U6WQuBrHBocJIGH0ThY3XAc", self.raw)
        self.assertIn("price_1U6WQvBrHBocJIGHxMjt6t41", self.raw)
        self.assertIn("price_1U6WQvBrHBocJIGHYqXOjFWm", self.raw)

    def test_msp_solo_and_self_serve_portal_checkout_are_documented(self) -> None:
        self.assertIn("### MSP Solo ($69/month or $690/year)", self.raw)
        self.assertIn("60-day, two-workspace evaluation", self.prose)
        self.assertIn("direct in-app Stripe checkout from **Plan** in the provider portal", self.prose)
        self.assertIn("Solo, Starter, Growth, and Scale", self.prose)
        self.assertNotIn("MSP is an assisted preview, not a public self-serve checkout path", self.prose)

    def test_required_paid_claim_audit_patterns_remain_intact(self) -> None:
        required_patterns = (
            r"monitored systems are not the paid\s+gate",
            r"Customer-specific Relay URL.*standard outbound relay service",
            r"Self-hosted trial acquisition.*No",
            r"Metric history was a Pro pillar until 17 November 2026",
        )
        for pattern in required_patterns:
            self.assertRegex(
                self.raw,
                re.compile(pattern, flags=re.IGNORECASE | re.DOTALL),
                msg=f"missing required paid-claim pattern: {pattern}",
            )

    def test_self_service_lifecycle_anchor_preserved(self) -> None:
        self.assertIn("### 10. Self-service lifecycle contract", self.raw)


if __name__ == "__main__":
    unittest.main()
