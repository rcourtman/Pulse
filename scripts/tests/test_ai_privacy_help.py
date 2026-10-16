#!/usr/bin/env python3
"""Keep the provider disclosure reachable and distinct from local-only controls.

These are documentation regressions, not a privacy audit or an outbound trace.
The existing modelboundary tests exercise the corresponding sanitation floor.
"""
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]


class AIPrivacyHelpTest(unittest.TestCase):
    def setUp(self):
        self.guide = (ROOT / "docs/AI.md").read_text()
        self.privacy = " ".join(self.guide.split("## Privacy\n", 1)[1].split("\n---", 1)[0].split())

    def test_overview_does_not_equate_byok_with_local_inference(self):
        overview = self.guide.split("### Assistant And MCP", 1)[0]
        self.assertNotIn("complete privacy", self.guide)
        self.assertNotIn("entirely on your infrastructure", self.guide)
        self.assertIn("not keeping inference local", overview)
        self.assertIn("[where AI context goes](#privacy)", overview)

    def test_all_three_destinations_and_the_limits_are_disclosed(self):
        for route in ("Local model endpoint under your control", "Hosted model with your own key or subscription",
                      "External agent using Pulse MCP"):
            with self.subTest(route=route):
                self.assertIn(route, self.privacy)
        for boundary in ("does not forward requests", "retention or training policy",
                         "conversation and tool results", "names and addresses can still be shared",
                         "not blanket anonymisation", "never paste credentials",
                         "read-only explanation", "Review external MCP clients"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, self.privacy)

    def test_action_and_telemetry_controls_do_not_claim_to_stop_context(self):
        for boundary in ("limit actions, not model requests", "Disabling usage telemetry does not disable AI",
                         "or external MCP access", "local model does not turn telemetry off",
                         "PULSE_TELEMETRY=false", "rotating pseudonymous install ID",
                         "Patrol mode and governed Pulse Intelligence operations",
                         "enabled by default and can be disabled any time",
                         "run a provider test merely"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, self.privacy)
        self.assertIn('ariaLabel="Enable Pulse Intelligence"',
                      (ROOT / "frontend-modern/src/components/Settings/AISettings.tsx").read_text())
        self.assertIn("PULSE_TELEMETRY=false", (ROOT / "internal/telemetry/telemetry.go").read_text())

    def test_same_page_disclosure_link_and_shipped_bytes(self):
        self.assertEqual(self.guide.count("## Privacy\n"), 1)
        self.assertEqual(self.guide.count("### Usage telemetry is separate\n"), 1)
        self.assertIn("[outbound usage telemetry](#usage-telemetry-is-separate)", self.guide)
        self.assertIn("[Privacy disclosure](PRIVACY.md)", self.guide)
        self.assertTrue((ROOT / "docs/PRIVACY.md").is_file())
        self.assertEqual((ROOT / "docs/AI.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/AI.md").read_bytes())


if __name__ == "__main__":
    unittest.main()
