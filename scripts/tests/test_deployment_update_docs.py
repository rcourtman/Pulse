#!/usr/bin/env python3
"""Keep the deployment entry point from bypassing the safe update procedures.

These are documentation regression checks, not installed update/restore proof.
The linked Compose recipe has executable controls in test_update_recovery_docs;
the production Docs navigation is exercised by deployment-update-help.cjs.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


def guide(name):
    return (ROOT / "docs" / f"{name}.md").read_text(encoding="utf-8")


def overview():
    return guide("DEPLOYMENT_MODELS")


class DeploymentUpdateDocsTest(unittest.TestCase):
    def test_shipped_overview_matches_the_reviewed_guide(self):
        self.assertEqual(overview(), (ROOT / "frontend-modern/public/docs/DEPLOYMENT_MODELS.md")
                         .read_text(encoding="utf-8"))

    def test_default_paths_are_not_a_complete_backup_inventory(self):
        text = " ".join(overview().split())
        for phrase in ("Default data/config path", "defaults, not a backup inventory",
                       "`PULSE_DATA_DIR`", "external stores", "`PULSE_METRICS_DB_PATH`"):
            self.assertIn(phrase, text)
        self.assertIn("METRICS_HISTORY.md#storage-location", text)
        self.assertIn("AUDIT_LOGGING.md#storage", text)

    def test_backup_and_rollback_claims_preserve_state_and_limits(self):
        text = " ".join(overview().split())
        for phrase in ("running version and edition", "consistent private backup",
                       "matching `.encryption.key` and audit signing key",
                       "stopped, consistent filesystem/volume backup", "SQLite sidecar files",
                       "Do not post configuration files, environment dumps or Helm secret values",
                       "snapshot is not necessarily a complete data backup",
                       "does not undo data migrations", "backup path alone does not establish",
                       "Keep the failed state", "before retrying a failed update"):
            self.assertIn(phrase, text)
        self.assertNotIn("swapping binaries/config safely", text)

    def test_overview_routes_to_the_maintained_procedures_not_a_shortcut(self):
        updates = overview().split("## Updates by Model", 1)[1]
        # A second shorthand can skip durable image selection, backup checks,
        # fail-fast pulling or the actual release/namespace and saved values.
        self.assertNotRegex(updates, r"```(?:bash|sh|shell)\b")
        for link in ("AUTO_UPDATE.md#rollback", "AUTO_UPDATE.md", "DOCKER.md#-updates",
                     "KUBERNETES.md"):
            self.assertIn(f"]({link})", updates)
            self.assertTrue((ROOT / "frontend-modern/public/docs" / link.split("#")[0]).is_file())
        self.assertIn("## 🔄 Updates", guide("DOCKER"))
        self.assertIn("## Rollback", guide("AUTO_UPDATE"))

    def test_deployment_specific_context_survives_the_handoff(self):
        updates = " ".join(overview().split("## Updates by Model", 1)[1].split())
        for phrase in ("inside the container, not on the Proxmox host",
                       "original Compose project", "Persist the exact target",
                       "pulling a different tag alone does not change the configured image",
                       "Paid Pro installs must keep their private image",
                       "only the Pulse service, stopping if the pull fails",
                       "no need to restart unrelated Compose services",
                       "existing Helm release and namespace", "chosen chart version",
                       "saved deployment values", "current PVC, keys and settings",
                       "does not guarantee restoration of Pulse data"):
            self.assertIn(phrase, updates)

    def test_install_entry_point_has_no_unscoped_update_shortcuts(self):
        install = guide("INSTALL").split("#### Manual Update\n", 1)[1].split(
            "\n### Pulse Agent updates", 1)[0]
        for shortcut in ("docker compose pull &&", "docker compose up -d`",
                         "helm repo update &&", "docker stop pulse && docker rm pulse",
                         "re-run your original `docker run` command"):
            with self.subTest(shortcut=shortcut):
                self.assertNotIn(shortcut, install)
        for link in ("DOCKER.md#-updates", "DEPLOYMENT_MODELS.md#kubernetes-helm",
                     "DEPLOYMENT_MODELS.md#updates-by-model",
                     "MIGRATION.md#full-state-recovery", "#rollback"):
            with self.subTest(link=link):
                self.assertIn(f"]({link})", install)
        self.assertIn("`sudo /bin/update`", install)
        self.assertIn("helper is absent or its owner is unknown", install)

    def test_install_update_preserves_storage_edition_and_failure_evidence(self):
        install = " ".join(guide("INSTALL").split("#### Manual Update\n", 1)[1].split(
            "\n### Pulse Agent updates", 1)[0].split())
        for phrase in ("interrupts monitoring and alert delivery",
                       "running version and edition", "deployment definition privately",
                       "every effective data path with its matching keys",
                       "not a complete data backup", "only the Pulse service",
                       "existing release, namespace, saved values and PVC",
                       "chosen chart version and image edition",
                       "pull it successfully before stopping",
                       "same data mount, ports, credentials and other settings",
                       "not a fresh example command", "container's writable layer",
                       "anonymous volume", "`--rm`", "stop here until",
                       "Do not delete or prune volumes", "start a second Pulse",
                       "ordinary monitoring and alert delivery",
                       "check the current state before retrying",
                       "reverting an image restores data",
                       "keep it on the private image or archive"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, install)

    def test_install_shipped_copy_matches_the_checked_update_entry_point(self):
        self.assertEqual(guide("INSTALL"),
                         (ROOT / "frontend-modern/public/docs/INSTALL.md").read_text())


if __name__ == "__main__":
    unittest.main()
