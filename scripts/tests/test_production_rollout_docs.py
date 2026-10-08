#!/usr/bin/env python3
"""Keep production decisions consistent with existing safety and edition help.

These are source/guidance checks, not native backup, installation, licence or
notification acceptance. No provider calls or disruptive operations are run.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/PRODUCTION_SECURITY.md"


def section(heading):
    text = DOC.read_text(encoding="utf-8")
    return " ".join(text.split(heading + "\n", 1)[1].split("\n## ", 1)[0].split())


class ProductionRolloutDocsTest(unittest.TestCase):
    def test_shipped_guide_matches_canonical(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/PRODUCTION_SECURITY.md").read_bytes())

    def test_api_only_keeps_the_guest_agent_backup_boundary(self):
        text = section("## Choose the least-privileged collection path")
        for boundary in ("API-only does not mean no guest-agent commands",
                         "without a Pulse agent installed", "freeze-enabled backups",
                         "VM_DISK_MONITORING.md#backup-safety", "automatic updaters",
                         "stopping only a Pulse agent is not enough",
                         "An OK backup task does not prove guest thaw",
                         "Do not run a backup or guest-agent probe"):
            self.assertIn(boundary, text)
        # The actual server collector calls the Proxmox QGA endpoint. API-only
        # does not imply a passive or guest-agent-free transport.
        collector = (ROOT / "internal/monitoring/monitor_pve_guest_builders.go").read_text()
        client = (ROOT / "pkg/proxmox/client.go").read_text()
        self.assertIn("client.GetVMFSInfo(ctx, res.Node, res.VMID)", collector)
        self.assertIn('"/nodes/%s/qemu/%d/agent/get-fsinfo"', client)

    def test_installation_preserves_paid_edition_and_recoverable_data(self):
        text = section("## Installation and update integrity")
        for boundary in ("version, edition and deployment configuration",
                         "consistent private backup", "matching keys", "external stores",
                         "does not reverse data migrations", "Community builds",
                         "even with an activation key", "continuing Relay",
                         "private Pro image or archive", "Do not replace an installed paid runtime",
                         "For a Community server installation", "pulse-install",
                         "before running it", "same pinned version"):
            self.assertIn(boundary, text)
        install = " ".join((ROOT / "docs/INSTALL.md").read_text().split())
        self.assertIn("Community builds", install)
        self.assertIn("do not include the private Pulse Pro runtime hooks", install)
        self.assertIn("private Pulse Pro Docker image or Linux archive", install)
        deployment = " ".join((ROOT / "docs/DEPLOYMENT_MODELS.md").read_text().split())
        self.assertIn("does not undo data migrations", deployment)
        self.assertIn("Paid Pro installs must keep their private image", deployment)

    def test_retirement_is_not_a_current_remote_web_or_purchase_offer(self):
        text = section("## Community and Pro boundaries")
        for boundary in ("Relay is no longer sold", "31 March 2027",
                         "Existing paired phones keep working until then",
                         "current price", "for as long as their subscription continues",
                         "including after the app retires", "**not the web UI**",
                         "own VPN or tunnel", "ntfy, Gotify or Pushover",
                         "phone's browser", "do not plan a new deployment"):
            self.assertIn(boundary, text)
        self.assertNotIn("Relay and Pro add", text)
        plans = " ".join((ROOT / "docs/PULSE_PRO.md").read_text().split())
        self.assertIn("Relay is no longer sold", plans)
        self.assertIn("Relay connects the app, not the web UI", plans)
        self.assertIn("Those Pro features do not end with the app's retirement", plans)

    def test_current_plan_copy_keeps_free_sso_and_history_distinct(self):
        text = section("## Community and Pro boundaries")
        self.assertIn("seven days of metric history", text)
        self.assertIn("Pro adds 90-day history", text)
        self.assertIn("SSO is included with Community and higher tiers", text)
        features = (ROOT / "pkg/licensing/features.go").read_text()
        free = features.split("var freeFeatures = []string{", 1)[1].split("}", 1)[0]
        self.assertRegex(free, r"\bFeatureSSO\b")
        history = features.split("var TierHistoryDays = map[Tier]int{", 1)[1].split("}", 1)[0]
        self.assertRegex(history, r"TierFree:\s+7,")
        self.assertRegex(history, r"TierPro:\s+90,")

    def test_rollout_does_not_inject_failures_or_hide_faults_on_live_workloads(self):
        text = section("## Production rollout checklist")
        for boundary in ("isolated non-production fixture", "independent monitoring",
                         "recovery path", "Do not induce node loss", "failed backup",
                         "full disk", "alert storm", "on live workloads",
                         "Test notification success does not establish ordinary, grouped or resolved",
                         "every covered filesystem", "workload liveness",
                         "restore only services and timers active before the pause",
                         "monitoring and alerts are unavailable", "independent outage coverage",
                         "matching node names or VMIDs do not establish the same resource",
                         "normal staged operation", "comparable",
                         "database size is not a write rate", "Do not prune history or change polling"):
            self.assertIn(boundary, text)
        self.assertNotIn("Test loss-of-node", text)
        self.assertNotIn("```", text)
        self.assertIn("CONFIGURATION.md#multiple-proxmox-installations", text)
        self.assertIn("TROUBLESHOOTING.md#excessive-cpu-writes-or-database-growth", text)

    def test_scale_and_privilege_limits_remain_explicit(self):
        text = " ".join(DOC.read_text().split())
        for boundary in ("dedicated read-only or narrowly scoped API token",
                         "root` by default", "monitoring-only", "agent:exec",
                         "DISCOVERY_ENABLED=false", "127.0.0.1:9191", "AES-256-GCM",
                         "not a blanket certification", "simulated 500-node",
                         "stage the rollout", "independent security certification"):
            self.assertIn(boundary, text)
        self.assertNotRegex(text, r"(?:--insecure|curl[^\n]*\||--token\s)")


if __name__ == "__main__":
    unittest.main()
