#!/usr/bin/env python3
"""Keep missing-backup guidance on the real collector and non-destructive path.

No provider calls, setup commands, backups or guest-agent probes are executed.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
GUIDE = "UNIFIED_AGENT.md"
HEADING = "### PVE Backups Not Showing (Recovery)"


def backup_section():
    text = (ROOT / "docs" / GUIDE).read_text(encoding="utf-8")
    return text.split(HEADING, 1)[1].split("\n### ", 1)[0]


class PVEBackupTroubleshootingDocsTest(unittest.TestCase):
    def test_shipped_help_matches_source(self):
        self.assertEqual((ROOT / "docs" / GUIDE).read_bytes(),
                         (ROOT / "frontend-modern/public/docs" / GUIDE).read_bytes())

    def test_explains_actual_server_collection_not_guest_agent_readings(self):
        section = backup_section()
        for phrase in ("server collects PVE backup inventory", "saved Proxmox API",
                       "storage-content requests", "Guest-agent disk readings are not backup inventory",
                       "online nodes", "queryable storage", "backup content"):
            self.assertIn(phrase, section)
        collector = (ROOT / "internal/monitoring/monitor_backups.go").read_text()
        for operation in ("client.GetStorage(ctx, node.Node)",
                          "client.GetStorageContent(ctx, node.Node, storage.Storage)",
                          'nodeEffectiveStatus[node.Node] != "online"',
                          'strings.Contains(storage.Content, "backup")',
                          "storageContentQueryable(storage)"):
            self.assertIn(operation, collector)

    def test_no_destructive_reenrolment_or_blanket_acl_recipe(self):
        section = backup_section()
        shell = "\n".join(re.findall(r"```(?:bash|sh)\n(.*?)```", section, re.DOTALL))
        self.assertNotRegex(shell, r"\brm\b|aclmod|--enable-proxmox|systemctl\s+restart")
        normalised = " ".join(section.split())
        self.assertIn("Do not delete the monitored node, remove registration state or rerun setup", normalised)
        self.assertIn("Setup can rotate an existing API token", normalised)
        for item in ("connection", "credentials", "registration state", "backup/history data"):
            self.assertIn(item, normalised)
        setup = (ROOT / "internal/hostagent/proxmox_setup.go").read_text()
        self.assertIn("rotating token in place", setup)
        self.assertIn('"token", "remove", proxmoxUserPVE, tokenName', setup)

    def test_permissions_are_endpoint_specific_and_keep_privilege_separation(self):
        section = " ".join(backup_section().split())
        for phrase in ("actual rejected endpoint", "installed PVE version",
                       "both user and token ACLs", "scope and inheritance",
                       "Audit-only inventory access does not establish access",
                       "Do not apply blanket storage-administrator ACLs",
                       "disable privilege separation", "administrator token as a test"):
            self.assertIn(phrase, section)
        self.assertIn("TROUBLESHOOTING.md#check-permissions-proxmox", section)

    def test_preserves_source_freshness_and_backup_safety_distinctions(self):
        section = " ".join(backup_section().split())
        for phrase in ("node, storage, guest type/ID and time", "Proxmox → Backups → By date",
                       "Coverage posture is a different reading", "distinct sources",
                       "failed, unavailable or partial read is not an empty inventory",
                       "Keep connection/agent liveness separate from backup collection freshness",
                       "existing collection timestamps where available",
                       "OK task does not prove guest thaw", "Do not run another backup or restore",
                       "outside backups", "without manual guest-agent probes",
                       "Keep token secrets, full ACL listings and private infrastructure details"):
            self.assertIn(phrase, section)
        for target in ("PBS.md#data-source-indicator", "RECOVERY.md#missing-or-inconsistent-evidence",
                       "VM_DISK_MONITORING.md#backup-safety", "TROUBLESHOOTING.md#-getting-help"):
            self.assertIn(target, section)
        safety = (ROOT / "docs/VM_DISK_MONITORING.md").read_text()
        self.assertIn("every filesystem covered by the backup", safety)
        self.assertIn("Pulse monitoring and alerts are unavailable", safety)


if __name__ == "__main__":
    unittest.main()
