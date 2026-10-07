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
    def test_replication_help_uses_existing_row_details_not_backup_commands(self):
        trouble = (ROOT / "docs/TROUBLESHOOTING.md").read_text()
        section = trouble.split("#### Replication jobs are Pending, stale or missing\n", 1)[1].split("\n#### ", 1)[0]
        text = " ".join(section.split())
        self.assertNotIn("```", section)
        table = (ROOT / "frontend-modern/src/features/proxmox/ProxmoxReplicationTable.tsx").read_text()
        # Tie the help's expansion and privacy-safe observation fields to the
        # actual surface, not a speculative job API or native recovery proof.
        self.assertIn("PlatformResourceDetailToggleButton", table)
        for label in ("Guest", "Job", "Route", "Last sync", "Next sync", "Duration", "Failures"):
            with self.subTest(label=label):
                self.assertIn(f"**{label}**", section)
                self.assertRegex(table, rf">{label}</(?:dt|TableHead)>")
        for phrase in ("not PBS backups", "unknown, not an empty healthy result",
                       "does not establish lifecycle or polling recovery",
                       "do not repeat a restart, reboot or update", "widen token permissions",
                       "consistent placeholders", "VM_DISK_MONITORING.md#backup-safety"):
            self.assertIn(phrase, text)
        faq = (ROOT / "docs/FAQ.md").read_text()
        self.assertIn("TROUBLESHOOTING.md#replication-jobs-are-pending-stale-or-missing", faq)
        for name in ("FAQ.md", "TROUBLESHOOTING.md"):
            self.assertEqual((ROOT / "docs" / name).read_bytes(),
                             (ROOT / "frontend-modern/public/docs" / name).read_bytes())

    def test_multi_installation_help_is_mirrored_and_keeps_cosmetic_names_separate(self):
        for name in ("CONFIGURATION.md", "TROUBLESHOOTING.md", "PBS.md"):
            with self.subTest(guide=name):
                self.assertEqual((ROOT / "docs" / name).read_bytes(),
                                 (ROOT / "frontend-modern/public/docs" / name).read_bytes())
        config = (ROOT / "docs/CONFIGURATION.md").read_text()
        section = config.split("### Multiple Proxmox installations\n", 1)[1].split("\n### ", 1)[0]
        text = " ".join(section.split())
        self.assertIn("intended to monitor multiple Proxmox clusters and standalone nodes", text)
        self.assertIn("matching name or VMID alone does not identify the same resource", text)
        self.assertIn("repairs incorrect attribution", text)
        self.assertIn("TROUBLESHOOTING.md#monitoring-is-mixed-between-proxmox-installations", text)
        # Static evidence of existing multi-instance intent, not native recovery.
        helpers = (ROOT / "internal/monitoring/monitor_helpers.go").read_text()
        self.assertIn('fmt.Sprintf("%s:%s:%d", instanceName, node, vmid)', helpers)
        collector = (ROOT / "internal/monitoring/monitor_backups.go").read_text()
        self.assertIn("vm.Instance() == instanceName", collector)
        self.assertIn("vm.Instance == instanceName", collector)

    def test_cross_installation_comparison_uses_existing_visible_agent_evidence(self):
        trouble = (ROOT / "docs/TROUBLESHOOTING.md").read_text()
        section = trouble.split("#### Monitoring is mixed between Proxmox installations\n", 1)[1].split("\n#### ", 1)[0]
        text = " ".join(section.split())
        self.assertNotIn("```", section)
        for boundary in ("existing observations only", "One restored view does not establish",
                         "Keep actual addresses, hostnames, connection IDs and machine identities private",
                         "Do not share a full Agent Doctor report", "Do not rename production nodes",
                         "re-enrol agents", "Do not run live diagnostics, guest-agent probes"):
            self.assertIn(boundary, text)
        model = (ROOT / "frontend-modern/src/components/Settings/infrastructureAgentUpdateCommandsModel.ts").read_text()
        for field in ("Connection: ${connection.id}", "Hostname: ${connection.agentIdentity.hostname}"):
            self.assertIn(field, model)
        page = (ROOT / "frontend-modern/src/components/Settings/InfrastructureAgentDoctorPage.tsx").read_text()
        for label in ("Identity evidence", "Last seen", "Healthy"):
            self.assertIn(label, page)

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


ZFS_DOC = ROOT / "docs/ZFS_MONITORING.md"


class ZFSMonitoringDocsTest(unittest.TestCase):
    """Keep the existing pool guide passive and scoped, not a native ZFS proof."""

    def text(self):
        return " ".join(ZFS_DOC.read_text(encoding="utf-8").split())

    def test_shipped_zfs_guide_matches_source(self):
        self.assertEqual(ZFS_DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/ZFS_MONITORING.md").read_bytes())

    def test_zfs_permissions_use_the_configured_privilege_separated_token(self):
        text = self.text()
        for distinction in ("user, realm and token ID actually configured",
                            "intersection of user and token permissions",
                            "user-only permission listing does not test Pulse's token",
                            "both scoped ACLs and inherited permissions",
                            "TROUBLESHOOTING.md#check-permissions-proxmox"):
            self.assertIn(distinction, text)
        for boundary in ("Do not grant a role across all nodes",
                         "disable privilege separation", "administrator token",
                         "rerun setup", "maintenance window"):
            self.assertIn(boundary, text)

    def test_zfs_guide_has_no_permission_or_recovery_mutation_recipe(self):
        commands = "\n".join(re.findall(r"`([^`]+)`", ZFS_DOC.read_text()))
        self.assertNotRegex(commands, r"\bpveum\b|aclmod|--privsep|\bsudo\b|"
                                      r"systemctl\s+(?:restart|stop)|"
                                      r"zpool\s+(?:scrub|clear|export|import|replace)")

    def test_zfs_help_keeps_row_capacity_health_and_inventory_distinct(self):
        text = self.text()
        for distinction in ("No storage row", "Storage row present, pool health absent",
                            "Pool health disagrees", "capacity/usage is a separate reading",
                            "missing health is **unknown**, not `ONLINE`",
                            "capacity view does not resolve a pool-health failure",
                            "Agent capacity or dataset readings do not prove the API token",
                            "Datacenter → Storage", "intentionally unregistered pool"):
            self.assertIn(distinction, text)
        # These are inspected collection paths, not permission or appliance acceptance.
        client = (ROOT / "pkg/proxmox/client.go").read_text()
        for endpoint in ('"/nodes/%s/disks/zfs"', '"/nodes/%s/disks/zfs/%s"'):
            self.assertIn(endpoint, client)

    def test_zfs_comparison_keeps_source_identity_and_freshness(self):
        text = self.text()
        for distinction in ("Proxmox installation, node, configured storage and backing pool",
                            "Repeated storage or pool names on different nodes are not one identity",
                            "Use the existing configuration and observations",
                            "last successful collection time",
                            "retained value is not proof of a fresh read",
                            "Keep established native ZFS health checks meanwhile"):
            self.assertIn(distinction, text)

    def test_zfs_logs_reuse_containing_reader_on_the_actual_pulse_host(self):
        text = self.text()
        self.assertNotRegex(ZFS_DOC.read_text(), r"journalctl[^\n]*\|")
        for distinction in ("TROUBLESHOOTING.md#inspect-notification-logs",
                            "original time window", "read inside that container, not the Proxmox host",
                            "for Docker, use the container reader",
                            "matching partial line can mask a failed read",
                            "empty search does not prove collection succeeded",
                            "Do not restart Pulse, enable Debug or run diagnostics"):
            self.assertIn(distinction, text)

    def test_zfs_report_preserves_private_evidence_and_existing_setup(self):
        text = self.text()
        for boundary in ("do not register a pool, rename storage or recreate a connection",
                         "Do not scrub, export/import, clear errors or replace a device",
                         "failing surface (row, capacity, pool health or dataset inventory)",
                         "collection path (API or agent)", "consistent aliases",
                         "token secrets, full permission listings, raw logs",
                         "private paths or serial numbers out of the public thread"):
            self.assertIn(boundary, text)


if __name__ == "__main__":
    unittest.main()
