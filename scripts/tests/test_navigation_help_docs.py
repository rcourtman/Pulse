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
        self.assertGreaterEqual(len(tabs), 7)
        self.assertIn("Services", tabs)
        self.assertIn("Shares", tabs)
        guide = doc_text("TRUENAS.md")
        for label, path in tabs.items():
            with self.subTest(tab=label):
                self.assertIn(f"**TrueNAS → {label}** (`{path}`)", guide)
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

    def test_kubernetes_locations_match_the_actual_tab_labels_and_paths(self):
        model = source("features/kubernetes/kubernetesPageModel.ts")
        tabs = dict(re.findall(r"label: '([^']+)', path: '(/kubernetes/[^']+)'", model))
        self.assertGreaterEqual(len(tabs), 7)
        self.assertIn("Overview", tabs)
        self.assertIn("Workloads", tabs)
        guide = doc_text("KUBERNETES.md")
        for label, path in tabs.items():
            with self.subTest(tab=label):
                self.assertIn(f"**{label}** (`{path}`)", guide)
        self.assertIn("**Kubernetes → Overview** (`/kubernetes/overview`)", guide)
        self.assertIn("hasKubernetesTabInventory", model)
        self.assertIn("Tabs outside **Overview** appear only when the agent reports inventory for them", guide)
        self.assertIn("**Settings → Security → API Access** (`/settings/security/api`)", guide)
        self.assertIn("SETTINGS_API_ACCESS_PATH = '/settings/security/api'", source("routing/resourceLinks.ts"))

    def test_unified_resources_matches_live_sources_navigation_and_query_params(self):
        guide = doc_text("UNIFIED_RESOURCES.md")
        types_go = (ROOT / "internal/unifiedresources/types.go").read_text(encoding="utf-8")
        resources_go = (ROOT / "internal/api/resourceapi/resources.go").read_text(encoding="utf-8")
        layout = source("AppLayout.tsx")

        for source_name in (
            "proxmox",
            "agent",
            "docker",
            "pbs",
            "pmg",
            "kubernetes",
            "truenas",
            "vmware",
            "availability",
        ):
            with self.subTest(data_source=source_name):
                self.assertIn(f'DataSource = "{source_name}"', types_go)
                self.assertIn(f"| `{source_name}` |", guide)

        self.assertIn("label: 'Availability'", layout)
        self.assertIn("| **Availability** | Website, service, device, and TLS certificate checks (`/availability`) |", guide)

        for param in (
            "type",
            "source",
            "excludeSource",
            "status",
            "parent",
            "cluster",
            "namespace",
            "q",
            "tags",
            "page",
            "limit",
            "sort",
            "order",
        ):
            with self.subTest(query_param=param):
                self.assertIn(f'r.URL.Query().Get("{param}")', resources_go)
                self.assertIn(f"| `{param}` |", guide)

    def test_migration_unified_nav_includes_availability_actions_and_shortcut(self):
        guide = doc_text("MIGRATION_UNIFIED_NAV.md")
        shortcuts = source("hooks/useKeyboardShortcuts.ts")
        self.assertIn("availability: { key: 'c', route: buildAvailabilityPath() }", shortcuts)
        self.assertIn("`g c` Availability", guide)
        self.assertIn("Availability, Alerts, Patrol, Actions, and Settings", guide)

    def test_relay_guide_uses_security_sidebar_path_for_pulse_mobile(self):
        guide = doc_text("RELAY.md")
        catalog = source("components/Settings/settingsNavCatalog.ts")
        security_group = catalog.split("id: 'security'", 1)[1]
        self.assertIn("id: 'system-relay'", security_group)
        self.assertIn("label: 'Pulse Mobile'", security_group)
        self.assertIn("**Settings → Security → Pulse Mobile** (`/settings/system-relay`)", guide)
        self.assertNotIn("**Settings → Pulse Mobile**", guide)

    def test_architecture_guide_matches_router_polling_routes_stores_and_rbac(self):
        arch = (ROOT / "ARCHITECTURE.md").read_text(encoding="utf-8")
        shipped = (ROOT / "frontend-modern/public/docs/ARCHITECTURE.md").read_text(encoding="utf-8")
        self.assertEqual(arch, shipped)

        for router_file in (
            "router_routes_auth_security.go",
            "router_routes_monitoring.go",
            "router_routes_ai_relay.go",
            "router_routes_licensing.go",
            "router_routes_registration.go",
            "router_routes_cloud.go",
            "router_routes_debug.go",
        ):
            with self.subTest(router_file=router_file):
                self.assertTrue((ROOT / "internal/api" / router_file).is_file())
                self.assertIn(f"`{router_file}`", arch)
        for stale_file in ("router_routes_org_license.go", "router_routes_hosted.go", "stores/metricsCollector.ts"):
            with self.subTest(stale_file=stale_file):
                self.assertNotIn(stale_file, arch)

        config_go = (ROOT / "internal/config/config.go").read_text(encoding="utf-8")
        self.assertIn("PVEPollingInterval:              10 * time.Second", config_go)
        self.assertIn("PBSPollingInterval:              60 * time.Second", config_go)
        self.assertIn("PMGPollingInterval:              60 * time.Second", config_go)
        self.assertIn("defaulting to `10 s` for PVE and `60 s` for PBS and PMG", arch)
        self.assertNotIn("default 2 s for PVE", arch)

        self.assertIn("| `/availability/*` | Availability |", arch)
        self.assertIn("`g c` → Availability", arch)
        self.assertIn("/api/recovery/postures", arch)
        self.assertIn("/api/agents/agent/report", arch)
        self.assertIn("31 March 2027", arch)

        perms_go = (ROOT / "pkg/auth/permissions.go").read_text(encoding="utf-8")
        for resource in ("settings", "audit_logs", "nodes", "users", "license", "ai", "discovery", "actions", "alerts"):
            with self.subTest(rbac_resource=resource):
                self.assertIn(f'= "{resource}"', perms_go)
                self.assertIn(f"`{resource}`", arch)

    def test_all_changed_guides_match_their_shipped_copies(self):
        for name in (
            "TROUBLESHOOTING.md",
            "TRUENAS.md",
            "PBS.md",
            "KUBERNETES.md",
            "UNIFIED_RESOURCES.md",
            "MIGRATION_UNIFIED_NAV.md",
            "RELAY.md",
        ):
            with self.subTest(guide=name):
                self.assertEqual(doc_text(name),
                                 (ROOT / "frontend-modern/public/docs" / name).read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
