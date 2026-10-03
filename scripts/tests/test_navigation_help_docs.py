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
