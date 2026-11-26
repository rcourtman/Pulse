#!/usr/bin/env python3
"""Keep Availability Help aligned with Availability UI, DNS, HTTPS TLS, and retirement code.

Static source/document checks only: no probe, DNS query, network request, or live
alert operation is performed.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/AVAILABILITY.md"
MIRROR = ROOT / "frontend-modern/public/docs/AVAILABILITY.md"
PAGE = ROOT / "frontend-modern/src/features/availability/AvailabilityPageSurface.tsx"
SLOT = (
    ROOT
    / "frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/AvailabilityTargetSlot.tsx"
)
DNS_SECTION = (
    ROOT
    / "frontend-modern/src/components/Settings/ConnectionEditor/CredentialSlots/AvailabilityDnsConfigSection.tsx"
)
PRESETS = (
    ROOT
    / "frontend-modern/src/components/Settings/ConnectionEditor/availabilityTargetPresets.ts"
)
HISTORY = ROOT / "frontend-modern/src/utils/availabilityHistoryPresentation.ts"
PROBE_GO = ROOT / "internal/availabilityprobe/probe.go"
POLLER_GO = ROOT / "internal/monitoring/availability_poller.go"


def plain(text: str) -> str:
    return " ".join(text.replace("**", "").replace("`", "").split())


class AvailabilityHelpDocsTest(unittest.TestCase):
    def setUp(self) -> None:
        self.doc = DOC.read_text(encoding="utf-8")
        self.guide = plain(self.doc)
        self.page = PAGE.read_text(encoding="utf-8")
        self.slot = SLOT.read_text(encoding="utf-8")
        self.dns_section = DNS_SECTION.read_text(encoding="utf-8")
        self.presets = PRESETS.read_text(encoding="utf-8")
        self.history = HISTORY.read_text(encoding="utf-8")
        self.probe_go = PROBE_GO.read_text(encoding="utf-8")
        self.poller_go = POLLER_GO.read_text(encoding="utf-8")

    def test_shipped_guide_matches(self) -> None:
        self.assertEqual(DOC.read_bytes(), MIRROR.read_bytes())

    def test_top_level_page_controls_history_ranges_and_retirement_match_source(self) -> None:
        for token in ("Add check", "Test now", "Pause", "Resume", "Edit"):
            with self.subTest(control=token):
                self.assertIn(token, self.page)
                self.assertIn(token, self.guide)

        for label in ("24h", "7d", "30d", "1y"):
            with self.subTest(range_label=label):
                self.assertIn(f"label: '{label}'", self.history)
                self.assertIn(label, self.guide)

        self.assertIn("refreshAvailabilitySchedule(localTasks)", self.poller_go)
        self.assertIn("retires its pending local poll from the scheduler queue", self.guide)

    def test_dns_resolution_labels_match_ui_components(self) -> None:
        self.assertIn('<option value="dns">DNS resolution</option>', self.slot)
        self.assertIn("'DNS server address'", self.slot)
        self.assertIn("DNS resolver (Pi-hole / AdGuard / Unbound)", self.presets)
        for label in (
            "Query hostname",
            "Record type",
            "Transport",
            "UDP (default)",
            "TCP",
            "Expected answer (optional)",
            "NOERROR",
        ):
            with self.subTest(dns_label=label):
                self.assertIn(label, self.dns_section)
                self.assertIn(label, self.guide)

        for phrase in (
            "DNS resolution",
            "DNS resolver (Pi-hole / AdGuard / Unbound)",
            "DNS server address",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, self.guide)

    def test_https_certificate_pinning_and_refusal_match_source(self) -> None:
        for label in (
            "Monitor TLS certificate validity",
            "Expiry warning (days)",
            "Pinned certificate fingerprint (optional)",
            "Allow an unverified certificate",
            "presented certificate SHA-256",
        ):
            with self.subTest(tls_label=label):
                self.assertIn(label, self.slot)
                self.assertIn(label, self.guide)

        self.assertIn("contract.TLSFingerprintSHA256", self.probe_go)
        self.assertIn("contract.InsecureSkipVerify || !contract.SendsStoredValues()", self.probe_go)
        self.assertIn("httpCertificateRefusal", self.probe_go)
        self.assertIn("does not send them to an untrusted certificate", self.guide)

    def test_build_stage_push_checks_not_mentioned(self) -> None:
        self.assertNotIn("push check", self.guide.lower())
        self.assertNotIn("push checks", self.guide.lower())


if __name__ == "__main__":
    unittest.main()
