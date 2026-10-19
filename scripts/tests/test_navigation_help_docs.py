#!/usr/bin/env python3
"""Keep operator navigation guidance tied to the shipped routes and tab labels.

These checks cover documentation accuracy, not collection or appliance health.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


def doc_text(name: str) -> str:
    return (ROOT / "docs" / name).read_text(encoding="utf-8")


def source(name: str) -> str:
    return (ROOT / "frontend-modern/src" / name).read_text(encoding="utf-8")


class NavigationHelpDocsTest(unittest.TestCase):
    def test_troubleshooting_groups_offer_focused_fragment_navigation(self):
        entry = doc_text("TROUBLESHOOTING.md").split("## Choose the affected workflow\n", 1)[1]
        entry = entry.split("\n## ⚡ Quick Fixes", 1)[0]
        groups = (
            ("Sign-in, access and updates", "sign-in-access-and-updates", 7),
            ("Missing or incorrect readings", "missing-or-incorrect-readings", 6),
            ("Backups and storage", "backups-and-storage", 6),
            ("Notifications and performance", "notifications-and-performance", 5),
        )
        introduction = entry.split("\n### ", 1)[0]
        for label, fragment, count in groups:
            with self.subTest(group=label):
                self.assertIn(f"[{label}](#{fragment})", introduction)
                group = entry.split(f"### {label}\n", 1)[1].split("\n### ", 1)[0]
                self.assertEqual(len(re.findall(r"^- \*\*", group, re.M)), count)
        self.assertEqual(len(re.findall(r"^- \*\*", entry, re.M)), 25)
        # Safety and disclosure containment must be reachable before choosing
        # a category, not buried inside an unrelated symptom group.
        self.assertIn("[Backup safety](VM_DISK_MONITORING.md#backup-safety)", introduction)
        self.assertIn("#if-you-already-posted-private-information", introduction)

    def test_troubleshooting_groups_preserve_existing_destination_links(self):
        entry = doc_text("TROUBLESHOOTING.md").split("## Choose the affected workflow\n", 1)[1]
        entry = entry.split("\n## ⚡ Quick Fixes", 1)[0]
        destinations = set(re.findall(r"\[[^\]]*\]\(([^)]+)\)", entry))
        expected = {
            'AUTO_UPDATE.md#update-failed',
            'CONFIGURATION.md#if-debug-makes-the-ui-unusable',
            'DOCKER.md#check-a-failed-or-pending-workload-update',
            'MAIL_GATEWAY.md#cluster-response-or-backup-lookup-errors',
            'MAIL_GATEWAY.md#troubleshooting-without-changing-the-gateway',
            'OIDC.md#sign-in-fails-after-editing-a-provider',
            'PBS.md#a-pbs-job-looks-healthy-but-its-status-disagrees',
            'PBS.md#backup-activity-has-no-volume-or-fewer-rows-than-expected',
            'PBS.md#duplicate-backups',
            'PBS.md#latest-backup-job-is-unknown',
            'PBS.md#one-pbs-namespace-is-missing-or-stops-changing',
            'PBS.md#pbs-is-active-but-missing-from-monitoring',
            'PBS.md#read-backup-job-identity-and-timing',
            'PULSE_PRO.md#already-paid-but-pro-features-are-unavailable',
            'TROUBLESHOOTING.md#-getting-help',
            'TROUBLESHOOTING.md#a-usage-alert-stays-open-after-usage-drops',
            'TROUBLESHOOTING.md#apprise-api-reports-a-tls-or-connection-error',
            'TROUBLESHOOTING.md#cannot-login--401-unauthorized',
            'TROUBLESHOOTING.md#excessive-cpu-writes-or-database-growth',
            'TROUBLESHOOTING.md#find-the-running-server-and-agent-versions',
            'TROUBLESHOOTING.md#guest-memory-is-high-but-available-memory-is-plentiful',
            'TROUBLESHOOTING.md#i-forgot-my-password',
            'TROUBLESHOOTING.md#if-you-already-posted-private-information',
            'TROUBLESHOOTING.md#memory-use-keeps-growing',
            'TROUBLESHOOTING.md#monitoring-is-mixed-between-proxmox-installations',
            'TROUBLESHOOTING.md#proxmox-or-pbs-certificate-validation-failed',
            'TROUBLESHOOTING.md#raid-array-is-degraded-with-a-spare',
            'TROUBLESHOOTING.md#replication-jobs-are-pending-stale-or-missing',
            'TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing',
            'TROUBLESHOOTING.md#unraid-reports-missing-disks-in-empty-slots',
            'TRUENAS.md#stale-truenas-data',
            'UNIFIED_AGENT.md#agent-is-healthy-locally-but-host-reports-are-stale',
            'UNIFIED_AGENT.md#agent-lookup-returns-http-401-or-403',
            'UNIFIED_AGENT.md#copied-agent-command-fails-before-installation',
            'VM_DISK_MONITORING.md#backup-safety',
            'VM_DISK_MONITORING.md#current-retained-and-unavailable-disk-readings',
        }
        # Preserve both the workflow shortcuts and the existing focused guides.
        self.assertEqual({link for link in destinations if not link.startswith("#")}, expected)

    def test_entry_routes_match_the_registered_platform_routes(self):
        guide = doc_text("TROUBLESHOOTING.md").split("#### Old bookmarks don't work", 1)[1]
        guide = guide.split("### Relay / Mobile", 1)[0]
        links = source("routing/resourceLinks.ts")
        app = source("App.tsx")
        for label, prefix in (("Proxmox", "PROXMOX"), ("Docker", "DOCKER"),
                              ("Kubernetes", "KUBERNETES"), ("TrueNAS", "TRUENAS"),
                              ("vSphere", "VMWARE"), ("Machines", "STANDALONE")):
            with self.subTest(platform=label):
                path = re.search(rf"export const {prefix}_PATH = '([^']+)';", links).group(1)
                tab = re.search(rf"export const {prefix}_DEFAULT_TAB = '([^']+)';", links).group(1)
                self.assertIn(f"| {label} | `{path}/{tab}` |", guide)
                self.assertIn(f'<Route path={{{prefix}_PATH}}', app)
        self.assertNotIn("Legacy URLs", guide)
        self.assertIn("are supported", guide)

    def test_navigation_guidance_preserves_connections_and_separates_proxy_failures(self):
        guide = doc_text("TROUBLESHOOTING.md")
        for phrase in ("rather than Machines", "last\nsuccessful collection",
                       "Do not delete connections or re-enrol agents",
                       "reloading the same URL returns a proxy 404",
                       "REVERSE_PROXY.md", "FAQ.md#how-is-navigation-organised-in-pulse-v6",
                       "is historical,\nnot a guide to the current menu"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, guide)
        # /infrastructure is still an entry route, not an invented missing page.
        self.assertIn('<Route path="/infrastructure" component={RuntimeHomePage}', source("App.tsx"))
        self.assertIn("`/infrastructure` now opens the default workspace", guide)

    def test_truenas_locations_match_the_actual_tab_labels_and_paths(self):
        model = source("features/truenas/truenasPageModel.ts")
        tabs = dict(re.findall(r"label: '([^']+)', path: '(/truenas/[^']+)'", model))
        guide = doc_text("TRUENAS.md")
        for label in ("Overview", "Storage", "Protection", "Apps", "VMs"):
            with self.subTest(tab=label):
                self.assertIn(f"**TrueNAS → {label}** (`{tabs[label]}`)", guide)
                self.assertIn(f"| TrueNAS → {label} |", guide)
        self.assertNotIn("| Unified Page |", guide)
        self.assertNotIn("on the **Infrastructure** page", guide)
        self.assertNotIn("on the **Recovery** page", guide)
        self.assertNotIn("on the **Storage** page", guide)

    def test_general_truenas_help_uses_platform_tabs(self):
        guide = doc_text("TROUBLESHOOTING.md").split("### TrueNAS", 1)[1].split("### Navigation", 1)[0]
        for label in ("Overview", "Storage", "Protection"):
            self.assertIn(f"**TrueNAS → {label}**", guide)
        for path in ("/truenas/storage", "/truenas/protection"):
            self.assertIn(f"`{path}`", guide)
        self.assertIn("TRUENAS.md#stale-truenas-data", guide)

    def test_setup_uses_the_existing_infrastructure_editor(self):
        guide = doc_text("TRUENAS.md")
        self.assertIn("**Settings → Infrastructure**", guide)
        self.assertIn("**Add infrastructure** and choose **TrueNAS**", guide)
        self.assertNotIn("**Settings → TrueNAS**", guide)
        editor = source("components/Settings/InfrastructureWorkspace.tsx")
        self.assertIn("return 'Add infrastructure'", editor)
        self.assertIn("case 'truenas':", editor)
        self.assertIn("<TrueNASCredentialSlot", editor)

    def test_pbs_backups_live_under_proxmox_not_top_level_recovery(self):
        guide = doc_text("PBS.md")
        self.assertGreaterEqual(guide.count("**Proxmox → Backups** (`/proxmox/backups`)"), 2)
        self.assertNotIn("unified Recovery view", guide)
        self.assertNotIn("In the Recovery view", guide)
        self.assertIn("{ id: 'backups', label: 'Backups'", source("features/proxmox/proxmoxPageModel.ts"))
        self.assertIn("PROXMOX_BACKUPS_PATH = `${PROXMOX_PATH}/backups`", source("routing/resourceLinks.ts"))

    def test_all_changed_guides_match_their_shipped_copies(self):
        for name in ("TROUBLESHOOTING.md", "TRUENAS.md", "PBS.md"):
            with self.subTest(guide=name):
                self.assertEqual(doc_text(name),
                                 (ROOT / "frontend-modern/public/docs" / name).read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
