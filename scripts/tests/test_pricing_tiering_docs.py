#!/usr/bin/env python3
"""Regression checks for the canonical v6 pricing and tiering specification."""

from __future__ import annotations

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRICING_DOC = ROOT / "docs/architecture/v6-pricing-and-tiering.md"


def normalize_prose(text: str) -> str:
    return " ".join(text.split())


class PricingTieringDocsTest(unittest.TestCase):
    def setUp(self) -> None:
        self.raw = PRICING_DOC.read_text(encoding="utf-8")
        self.prose = normalize_prose(self.raw)

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
        self.assertNotRegex(
            self.prose,
            r"Relay\s+is\s+the\s+access\s+service",
        )

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
