#!/usr/bin/env python3
"""Keep existing PMG guidance tied to the actual navigation and read limits.

Static source/document checks only: no gateway, mail, credentials, network,
service, notification or live diagnostic operation is performed.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/MAIL_GATEWAY.md"


def frontend(path):
    return (ROOT / "frontend-modern/src" / path).read_text(encoding="utf-8")


def plain(text):
    return " ".join(text.replace("**", "").split())


class PMGDocsTest(unittest.TestCase):
    def setUp(self):
        self.text = DOC.read_text(encoding="utf-8")
        self.guide = plain(self.text)

    def test_shipped_guide_matches(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/MAIL_GATEWAY.md").read_bytes())

    def test_connection_instructions_match_existing_picker_and_form(self):
        picker = frontend("components/Settings/InfrastructureSourcePicker.tsx")
        authentication = frontend("components/Settings/NodeModalAuthenticationSection.tsx")
        self.assertIn("Show more sources", picker)
        self.assertIn("Show more sources", self.guide)
        self.assertIn("Settings → Infrastructure → Add infrastructure", self.guide)
        self.assertNotIn("**Add Node**", self.text)
        self.assertIn("Username & Password", authentication)
        self.assertIn("Username & Password", self.guide)
        self.assertIn("modalProps.nodeType !== 'pmg'", authentication)
        for phrase in ("not the PVE/PBS API-token setup", "minimum read permissions",
                       "password only in the private settings form",
                       "do not put it in a command, URL or issue report",
                       "Keep HTTPS and certificate verification enabled"):
            self.assertIn(phrase, self.guide)

    def test_navigation_and_dataset_detail_match_the_current_surface(self):
        model = frontend("features/proxmox/proxmoxPageModel.ts")
        route = re.search(r"id: 'mail', label: '([^']+)', path: '([^']+)'", model)
        self.assertIsNotNone(route)
        self.assertIn(f"**Proxmox → {route[1]}** (`{route[2]}`)", self.text)
        table = frontend("features/proxmox/ProxmoxMailGatewayTable.tsx")
        self.assertIn("No instances match current filters", table)
        self.assertIn("clear its search and status filters", self.guide)
        self.assertNotIn("on the **Infrastructure** page", self.text)
        drawer = frontend("features/proxmox/ProxmoxMailGatewayDrawer.tsx")
        self.assertIn(".slice(0, 8)", drawer)
        self.assertIn("top eight reported domains, not the entire domain inventory", self.guide)
        self.assertIn("stats()?.timeframe", drawer)
        self.assertIn("compare the same window in PMG", self.guide)
        self.assertIn("Expand the intended gateway's row", self.guide)

    def test_documented_thresholds_exist_and_keep_their_units(self):
        config = (ROOT / "internal/alerts/config/types.go").read_text()
        fields = config.split("type PMGThresholdConfig struct {", 1)[1].split("\n}", 1)[0]
        for key in ("queueTotalWarning", "deferredQueueWarn", "holdQueueWarn",
                    "oldestMessageWarnMins", "quarantineSpamWarn", "quarantineVirusWarn",
                    "quarantineGrowthWarnPct", "quarantineGrowthWarnMin"):
            self.assertIn(f'json:"{key}"', fields)
        self.assertNotRegex(fields, r'json:"(?:spamRate|deliveryFailures)')
        self.assertIn("<ThresholdsTableProxmoxPMGSection", frontend(
            "components/Alerts/ThresholdsTableProxmoxTab.tsx"))
        title = re.search(r"ALERT_THRESHOLDS_SECTION_TITLE_PMG = '([^']+)'",
                          frontend("utils/alertThresholdsPresentation.ts"))
        self.assertIsNotNone(title)
        self.assertIn(f"**{title[1]}**", self.text)
        for phrase in ("Alerts → Thresholds → Proxmox", "oldest-message age in minutes",
                       "percentage plus minimum message growth",
                       "do not expose a user-configurable spam-rate or delivery-failure threshold"):
            self.assertIn(phrase, self.guide)
        self.assertIn("m.checkPMGAnomalies(pmg, pmgDefaults)",
                      (ROOT / "internal/alerts/pmg.go").read_text())
        self.assertIn("connectivity and mail anomaly checks", self.guide)

    def test_partial_collection_is_not_a_healthy_zero(self):
        poll = (ROOT / "internal/monitoring/monitor_pbs_pmg.go").read_text()
        poll = poll.split("func (m *Monitor) pollPMGInstance(", 1)[1]
        self.assertLess(poll.index('pmgInst.ConnectionHealth = "healthy"'),
                        poll.index("client.GetMailStatistics"))
        self.assertIn("Healthy is not complete collection", self.guide)
        self.assertIn("Some missing fields can display as zero or an empty section", self.guide)
        self.assertIn("does not prove that every dataset was read", self.guide)
        self.assertIn("Unavailable evidence is unknown, not zero", self.guide)

    def test_opt_out_guidance_does_not_invent_scope_enforcement(self):
        poll = (ROOT / "internal/monitoring/monitor_pbs_pmg.go").read_text()
        poll = poll.split("func (m *Monitor) pollPMGInstance(", 1)[1]
        # Do not lock in the runtime defect. If the owner adds these gates,
        # the caveat can be reviewed/removed with that containing correction.
        if any(f"if instanceCfg.{flag}" not in poll for flag in (
            "MonitorMailStats", "MonitorQueues", "MonitorQuarantine",
        )):
            self.assertIn("requests can still run with their options off", self.guide)
            self.assertIn("not evidence that those requests stopped", self.guide)
        self.assertIn("if instanceCfg.Disabled", poll)
        self.assertLess(poll.index("if instanceCfg.Disabled"), poll.index("client.GetVersion"))
        for phrase in ("pause the PMG connection", "does not cancel a request already in flight",
                       "arrange independent coverage", "only the connection you deliberately paused"):
            self.assertIn(phrase, self.guide)

    def test_diagnostics_preserve_mail_privacy_and_separate_failure_classes(self):
        for phrase in ("refusal is not a password verdict", "Certificate validation error",
                       "401/403", "specific failed request", "configured polling interval",
                       "Do not send test mail", "release quarantine",
                       "Do not enable Debug", "bounded Pulse log readers",
                       "addresses, subjects, message contents, full API responses"):
            self.assertIn(phrase, self.guide)
        self.assertNotRegex(self.text, r"```(?:bash|sh|powershell|http)\b")
        self.assertIn("Pulse's own alert email uses a separate SMTP destination", self.guide)
        self.assertIn("does not guarantee a notification was routed or received", self.guide)
        self.assertIn("TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing", self.text)


if __name__ == "__main__":
    unittest.main()
