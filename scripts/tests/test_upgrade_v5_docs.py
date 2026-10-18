#!/usr/bin/env python3
"""Keep the historical upgrade entry point from recommending destructive recovery."""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
GUIDE = "UPGRADE_v5.md"


def upgrade_guide():
    return (ROOT / "docs" / GUIDE).read_text(encoding="utf-8")


def section(heading, text=None):
    return (upgrade_guide() if text is None else text).split(heading, 1)[1].split("\n### ", 1)[0]


class UpgradeV5DocsTest(unittest.TestCase):
    def assert_current_upgrade_routes(self, text):
        paths = text.split("## Upgrade Paths", 1)[1].split("## Post-Upgrade Checklist", 1)[0]
        self.assertNotRegex(paths, r"```(?:bash|sh|shell)\b",
                            "Historical shortcuts must not bypass current update safeguards")
        for boundary in ("INSTALL.md#manual-update", "helper's ownership",
                         "existing service and data/config paths", "absent or unknown helper",
                         "inside the existing Pulse LXC", "AUTO_UPDATE.md#a-helper-selects-a-helm-chart-release",
                         "UNIFIED_AGENT.md#private-file-installation-linux-macos-and-nas",
                         "DOCKER.md#-updates", "explicit published image tag", "mounted data",
                         "private Pro image", "unrelated services", "INSTALL.md#1-kubernetes-helm",
                         "release, namespace, values and persistent volumes"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, " ".join(paths.split()))

    def assert_safe_proxy_retirement_entry(self, text):
        cleanup = section("### Sensor proxy removal", text)
        self.assertNotRegex(cleanup, r"```(?:bash|sh|shell)\b",
                            "Use the existing inspected procedure, not an inline privileged shortcut")
        for boundary in ("can stop and start running LXCs", "including Pulse",
                         "does not restrict cleanup to one container", "authorised Proxmox administrator",
                         "private recovery evidence", "schedule their maintenance",
                         "Do not pipe a web response into a privileged shell",
                         "TEMPERATURE_MONITORING.md#legacy-cleanup-if-upgrading",
                         "prior-active-state recording", "verified restoration",
                         "preserves proxy state, configuration, logs and the service account",
                         "Do not add `--purge` or `--remove-proxmox-access`",
                         "tokens still used by current connections", "Neither is required",
                         "not a cluster-wide uninstall"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, " ".join(cleanup.split()))

    def test_upgrade_paths_use_current_owned_procedures_not_mutating_shortcuts(self):
        self.assert_current_upgrade_routes(upgrade_guide())

    def test_proxy_retirement_preserves_access_evidence_and_workload_scope(self):
        self.assert_safe_proxy_retirement_entry(upgrade_guide())

    def test_configuration_export_is_not_the_full_upgrade_recovery_backup(self):
        before = upgrade_guide().split("## Before You Upgrade", 1)[1].split("## Upgrade Paths", 1)[0]
        for boundary in ("private, consistent backup", "active data, encryption key",
                         "configuration export", "not a full-state backup", "excludes history",
                         "agent enrolment state", "MIGRATION.md#full-state-recovery",
                         "do not copy live data", "does not require another first-time setup",
                         "actually", "published software release", "private Pro runtime"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, " ".join(before.split()))

    def test_missing_temperature_does_not_require_deployment_or_development_ssh_changes(self):
        temperature = section("### Temperature monitoring in containers")
        self.assertNotIn("PULSE_DEV_ALLOW_CONTAINER_SSH=", temperature)
        self.assertNotRegex(temperature, r"```(?:bash|sh|shell)\b")
        for boundary in ("TEMPERATURE_MONITORING.md#recommended-pulse-agent-proxmox",
                         "verified HTTPS and token-file", "Do not move a working server",
                         "enable development-only container SSH", "reinstall an agent",
                         "just to diagnose", "restricted SSH collection is a separate path"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, " ".join(temperature.split()))

    def test_shipped_help_matches_source(self):
        self.assertEqual((ROOT / "docs" / GUIDE).read_bytes(),
                         (ROOT / "frontend-modern/public/docs" / GUIDE).read_bytes())

    def test_historical_page_routes_current_upgrades_without_downgrading(self):
        intro = " ".join(upgrade_guide().split("## Before You Upgrade", 1)[0].split())
        for phrase in ("historical guide", "v4-to-v5 transition", "AUTO_UPDATE.md",
                       "INSTALL.md", "do not downgrade to v5"):
            self.assertIn(phrase, intro)

    def test_existing_login_recovery_does_not_reset_auth_or_expose_bootstrap_token(self):
        auth = " ".join(section("### Bootstrap token on fresh auth setup").split())
        for phrase in ("Do not delete `.env`", "repeat first-time setup",
                       "TROUBLESHOOTING.md#i-forgot-my-password",
                       "preserving the existing configuration and data",
                       "genuinely needs initial setup", "not a replacement for an existing password",
                       "enter it in the setup screen", "do not post it in a report",
                       "put it in a URL"):
            self.assertIn(phrase, auth)
        self.assertNotIn("for example by deleting", auth)

    def test_missing_backups_do_not_trigger_acl_mutation_or_reenrolment(self):
        backup = section("### Backups not showing (PVE)")
        self.assertNotRegex(backup, r"aclmod|Quick fix|Delete the node|--enable-proxmox")
        shell = "\n".join(re.findall(r"```(?:bash|sh)\n(.*?)```", backup, re.DOTALL))
        self.assertEqual(shell, "")
        backup = " ".join(backup.split())
        for phrase in ("empty backup table does not establish a permission failure",
                       "actual rejected endpoint", "installed PVE version",
                       "configured service user/token", "both user and token ACLs",
                       "scope and inheritance", "Authentication failure is not proof",
                       "audit-only access is not proof", "Do not grant blanket storage-administrator",
                       "disable privilege separation", "substitute an administrator token",
                       "Do not delete the monitored node, remove registration state or rerun setup",
                       "Setup can rotate an existing API token", "connection, credentials",
                       "registration state and backup/history data", "outside backups"):
            self.assertIn(phrase, backup)

    def test_backup_source_freshness_and_safety_route_survive_upgrade(self):
        backup = " ".join(section("### Backups not showing (PVE)").split())
        for phrase in ("server reads PVE backup inventory", "saved Proxmox API connection",
                       "host agent does not prove", "node, storage, guest type/ID and time",
                       "Proxmox → Backups → By date", "connection and filters",
                       "Direct PBS and PVE passthrough are distinct sources",
                       "Coverage posture is not the same as archive presence",
                       "failed or partial read is not an empty inventory",
                       "do not establish backup freshness", "original redacted error and time",
                       "Do not run another backup, restore, guest-agent probe or restart",
                       "OK backup task does not prove guest thaw", "observe ordinary polling",
                       "Keep token secrets, full ACL listings and private infrastructure details"):
            self.assertIn(phrase, backup)
        for target in ("VM_DISK_MONITORING.md#backup-safety",
                       "UNIFIED_AGENT.md#pve-backups-not-showing-recovery",
                       "TROUBLESHOOTING.md#check-permissions-proxmox",
                       "TROUBLESHOOTING.md#-getting-help"):
            self.assertIn(target, backup)


if __name__ == "__main__":
    unittest.main()
