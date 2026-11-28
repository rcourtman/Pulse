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

    def test_help_index_and_storage_proposal_status_match_shipped_docs(self):
        readme = doc_text("README.md")
        self.assertEqual(
            readme,
            (ROOT / "frontend-modern/public/docs/README.md").read_text(encoding="utf-8"),
        )
        for target in (
            "(UNIFIED_RESOURCES.md)",
            "(PATROL_ARCHITECTURE.md)",
            "(ASSISTANT_ARCHITECTURE.md)",
            "(CLOUD.md)",
        ):
            with self.subTest(index_link=target):
                self.assertIn(target, readme)
        self.assertIn("[Relay and Pulse Mobile retirement](RELAY.md)", readme)
        self.assertIn("31 March 2027", readme)
        self.assertNotIn("Relay and Pulse Mobile handoff", readme)
        self.assertNotIn("Community, Relay, and Pro capabilities", readme)

        storage_proposal = doc_text("STORAGE_ARCHITECTURE.md")
        self.assertEqual(
            storage_proposal,
            (ROOT / "frontend-modern/public/docs/STORAGE_ARCHITECTURE.md").read_text(
                encoding="utf-8"
            ),
        )
        self.assertIn("historical pre-v6 design proposal", storage_proposal)
        for live_guide in (
            "(RECOVERY.md)",
            "(TRUENAS.md)",
            "(ZFS_MONITORING.md)",
            "(VM_DISK_MONITORING.md)",
            "(UNIFIED_RESOURCES.md)",
        ):
            with self.subTest(live_guide=live_guide):
                self.assertIn(live_guide, storage_proposal)

    def test_proxmox_93_persistent_identity_and_history_splicing_match_runtime(self):
        unified = doc_text("UNIFIED_RESOURCES.md")
        history = doc_text("METRICS_HISTORY.md")
        self.assertEqual(
            history,
            (ROOT / "frontend-modern/public/docs/METRICS_HISTORY.md").read_text(encoding="utf-8"),
        )
        unified_section = " ".join(
            unified.split("### Proxmox VE 9.3 persistent identity\n", 1)[1]
            .split("\n### ", 1)[0]
            .replace("**", "")
            .replace("`", "")
            .split()
        )
        for phrase in (
            "persistent guest identity (uid and rtime)",
            "cluster-id from GET /cluster/status",
            "pre-9.3 canonical ID when they receive their first uid",
            "live migrations between nodes in the same cluster stay pinned to that same canonical ID",
            "distinct UID-based canonical ID",
            "Differing cluster-id values prevent independent clusters or standalone nodes",
        ):
            with self.subTest(unified_phrase=phrase):
                self.assertIn(phrase, unified_section)

        history_section = " ".join(
            history.split("### Proxmox live migration and VMID reuse\n", 1)[1]
            .split("\n## ", 1)[0]
            .replace("**", "")
            .replace("`", "")
            .split()
        )
        for phrase in (
            "splices stored history chronologically across the cluster's sibling nodes",
            "Proxmox VE 9.3 and later",
            "persistent uid",
            "withholds sibling-node history from the previous guest",
            "keep their existing identity and history when they receive their first uid",
        ):
            with self.subTest(history_phrase=phrase):
                self.assertIn(phrase, history_section)

        ids_go = (ROOT / "internal/unifiedresources/ids.go").read_text(encoding="utf-8")
        metrics_go = (ROOT / "internal/monitoring/monitor_metrics.go").read_text(encoding="utf-8")
        cluster_go = (ROOT / "internal/monitoring/monitor_pve_cluster.go").read_text(encoding="utf-8")
        self.assertIn("func ProxmoxGuestUIDCanonicalID(", ids_go)
        self.assertIn("func (m *Monitor) proxmoxGuestMetricSiblingKeys(", metrics_go)
        self.assertIn("spliceGuestMetricHistory(", metrics_go)
        self.assertIn("clusterID := proxmox.ExtractClusterID(statuses)", cluster_go)
        self.assertIn("Proxmox cluster-id changed during membership read", cluster_go)

    def test_machines_stale_telemetry_and_resource_detail_api_match_runtime(self):
        unified = doc_text("UNIFIED_RESOURCES.md")
        vm_disk = doc_text("VM_DISK_MONITORING.md")
        self.assertEqual(
            unified,
            (ROOT / "frontend-modern/public/docs/UNIFIED_RESOURCES.md").read_text(encoding="utf-8"),
        )
        self.assertEqual(
            vm_disk,
            (ROOT / "frontend-modern/public/docs/VM_DISK_MONITORING.md").read_text(encoding="utf-8"),
        )

        # Keep preview-only standaloneVms out of stable user guides.
        for name, text in (("UNIFIED_RESOURCES.md", unified), ("VM_DISK_MONITORING.md", vm_disk)):
            with self.subTest(preview_guard=name):
                self.assertNotIn("/standalone/vms", text)
                self.assertNotIn("Machines VMs", text)

        unified_norm = " ".join(unified.split())
        for phrase in (
            "| `health` | Backend-owned canonical health envelope with a `verdict` (including `stale` and `critical`) and stable `reasons` (`code` such as `telemetry_stale` plus optional detail) |",
            "### Stale Pulse Agent telemetry on Machines",
            "**Machines** (`/standalone/machines`)",
            "`health.verdict` is `stale`, `telemetry_stale` is recorded, `sourceStatus.agent` is impaired, `agent.stale` is true, or `lastSeen` is older than 5 minutes",
            "leaves **Healthy**, joins **Degraded**, and shows an amber **Stale** status indicator",
            "Explicit **Offline** and awaiting-first-report states remain distinct.",
            "`Telemetry is stale for <host> (<age>). CPU, memory, disk, and sensor readings shown are last known values until their agents report again.`",
            "`Stale · <age>` or `Stale data` on desktop, and `Stale` on narrow screens",
            "`Agent has stopped reporting.`",
            "`Showing last known readings.`",
            "`Last error:`",
            "`RAID Arrays (last known)`",
            "| `sort` | Sort field: `name` (default), `status`, `type`, or `lastSeen` |",
            "`GET /api/resources/{id}`",
            "PMG domain statistics",
        ):
            with self.subTest(doc="UNIFIED_RESOURCES.md", phrase=phrase):
                self.assertIn(phrase, unified_norm)

        vm_disk_norm = " ".join(vm_disk.split())
        for phrase in (
            "**Machines** (`/standalone/machines`) moves that host out of **Healthy** into **Degraded** with an amber **Stale** status indicator",
            "`Telemetry is stale for ...`",
            "`Stale · <age>` or `Stale data` cue beneath the machine name (**Stale** on phones)",
            "`Agent has stopped reporting.` with `Showing last known readings.` and any recorded `Last error:`",
            "Explicit **Offline** and awaiting-first-report states remain distinct.",
        ):
            with self.subTest(doc="VM_DISK_MONITORING.md", phrase=phrase):
                self.assertIn(phrase, vm_disk_norm)

        staleness_ts = source("features/standalone/standaloneStaleness.ts")
        machines_tsx = source("features/standalone/AgentsMachinesTable.tsx")
        model_ts = source("features/standalone/agentMachineTableModel.ts")
        resources_go = (ROOT / "internal/api/resourceapi/resources.go").read_text(encoding="utf-8")

        for marker in (
            "const AGENT_REPORT_STALE_AFTER_MS = 5 * 60 * 1000;",
            "badge: age ? `Stale · ${age}` : 'Stale data',",
            "compactBadge: 'Stale',",
            "detail: `Agent has stopped reporting.${lastReportText} Showing last known readings.${errorText}`,",
            "{ variant: 'warning', label: 'Stale' }",
            "Telemetry is stale for ${list}. CPU, memory, disk, and sensor readings shown are last known values until their agents report again.",
        ):
            with self.subTest(source="standaloneStaleness.ts", marker=marker):
                self.assertIn(marker, staleness_ts)
        for marker in (
            "title={lastKnownTitle() ? 'RAID Arrays (last known)' : 'RAID Arrays'}",
            'data-testid="agent-machines-stale-banner"',
            'data-agent-machine-stale-cue="true"',
            'data-testid="agent-machine-stale-notice"',
        ):
            with self.subTest(source="AgentsMachinesTable.tsx", marker=marker):
                self.assertIn(marker, machines_tsx)
        self.assertIn("const staleNotice = getStandaloneMachineStaleness(machine, nowMs ?? 0);", model_ts)
        for marker in (
            "// HandleGetResource handles GET /api/resources/{id}.",
            "// PMG domain stats can be very large; keep summary-only in list.",
            'case "status":',
            'case "type":',
            'case "lastSeen":',
        ):
            with self.subTest(source="resources.go", marker=marker):
                self.assertIn(marker, resources_go)

    def test_drawer_history_zoom_and_custom_window_help_match_runtime(self):
        history = doc_text("METRICS_HISTORY.md")
        api_doc = doc_text("API.md")
        self.assertEqual(
            history,
            (ROOT / "frontend-modern/public/docs/METRICS_HISTORY.md").read_text(encoding="utf-8"),
        )
        self.assertEqual(
            api_doc,
            (ROOT / "frontend-modern/public/docs/API.md").read_text(encoding="utf-8"),
        )

        zoom_section = " ".join(
            history.split("### Zoom and custom time windows\n", 1)[1]
            .split("\n### ", 1)[0]
            .replace("**", "")
            .replace("`", "")
            .split()
        )
        for phrase in (
            "click and drag horizontally across a drawer chart",
            "synchronized time selection across every panel",
            "minimum span of 1 minute",
            "Press Escape while dragging to cancel without closing the drawer",
            "Select time window",
            "Start and End editor",
            "timezone-offset selector",
            "even when the current preset has no stored readings",
            "between 1 minute and 365 days and cannot end in the future",
            "Reset zoom",
            "Live 30-second background polling and live fallback readings pause",
            "Double-click a chart, click Reset zoom, or choose a preset in History range",
        ):
            with self.subTest(zoom_phrase=phrase):
                self.assertIn(phrase, zoom_section)

        history_prose = " ".join(history.replace("**", "").replace("`", "").split())
        api_prose = " ".join(api_doc.replace("**", "").replace("`", "").split())
        for label, text in (("METRICS_HISTORY.md", history_prose), ("API.md", api_prose)):
            for phrase in (
                "mutually exclusive with range",
                "Positive integer Unix-millisecond timestamps",
                "Both must be provided once when either is supplied",
                "end must be greater than start",
                "span must be between 1s and 365d",
                "end cannot be in the future",
                'range set to "custom"' if label == "METRICS_HISTORY.md" else 'set range to "custom"',
            ):
                with self.subTest(doc=label, phrase=phrase):
                    self.assertIn(phrase, text)

        drawer_tsx = source("components/Workloads/GuestDrawerHistory.tsx")
        query_ts = source("components/Workloads/guestDrawerHistoryQuery.ts")
        router_go = (ROOT / "internal/api/router.go").read_text(encoding="utf-8")

        for marker in (
            "const HISTORY_MIN_ZOOM_SPAN_MS = 60_000;",
            "Select time window",
            "Reset zoom",
            "Select a window of at least 1 minute.",
            "Window cannot exceed 365 days.",
            "End time cannot be in the future.",
            'label="Start timezone offset"',
            'label="End timezone offset"',
            "if (props.isZoomed) props.onResetZoom();",
        ):
            with self.subTest(source="GuestDrawerHistory.tsx", marker=marker):
                self.assertIn(marker, drawer_tsx)
        self.assertIn("source()?.customWindow == null", query_ts)
        for marker in (
            "func parseMetricsHistoryCustomWindow(",
            '"start and end cannot be combined with range"',
            '"start and end must both be provided once"',
            '"end must be greater than start"',
            '"custom window must be between 1s and 365d"',
            '"end cannot be in the future"',
            'timeRange = "custom"',
        ):
            with self.subTest(source="router.go", marker=marker):
                self.assertIn(marker, router_go)

    def test_upgrade_readiness_banner_and_modal_help_match_runtime(self):
        auto_update = doc_text("AUTO_UPDATE.md")
        self.assertEqual(
            auto_update,
            (ROOT / "frontend-modern/public/docs/AUTO_UPDATE.md").read_text(encoding="utf-8"),
        )
        section = " ".join(
            auto_update.split("## Using One-Click Updates\n", 1)[1]
            .split("\n### Update Process\n", 1)[0]
            .replace("**", "")
            .replace("`", "")
            .split()
        )
        for phrase in (
            "global update banner",
            "Confirm Update dialog",
            "Update blocked badge and a Review checks button",
            "1 upgrade warning",
            "N upgrade warnings",
            "Upgrade checks need review",
            "Upgrade checks panel ordered Blocked, Warning, then Pass",
            "Apply Update Automatically as Install blocked",
            "disables the acknowledgement checkbox and Start Update as Install blocked",
        ):
            with self.subTest(auto_update_phrase=phrase):
                self.assertIn(phrase, section)

        banner_tsx = source("components/UpdateBanner.tsx")
        modal_tsx = source("components/UpdateConfirmationModal.tsx")
        panel_tsx = source("components/UpdateReadinessChecksPanel.tsx")

        for marker in (
            "Update blocked",
            "Review checks",
            "Upgrade checks need review",
            "Install blocked",
            "Apply Update Automatically",
        ):
            with self.subTest(source="UpdateBanner.tsx", marker=marker):
                self.assertIn(marker, banner_tsx)
        for marker in (
            "Confirm Update",
            "Install blocked",
            "Start Update",
            "Resolve blocked upgrade checks before starting this update.",
        ):
            with self.subTest(source="UpdateConfirmationModal.tsx", marker=marker):
                self.assertIn(marker, modal_tsx)
        for marker in (
            "Upgrade checks",
            "label: 'Blocked'",
            "label: 'Warning'",
            "label: 'Pass'",
        ):
            with self.subTest(source="UpdateReadinessChecksPanel.tsx", marker=marker):
                self.assertIn(marker, panel_tsx)

    def test_all_changed_guides_match_their_shipped_copies(self):
        for name in (
            "README.md",
            "STORAGE_ARCHITECTURE.md",
            "TROUBLESHOOTING.md",
            "TRUENAS.md",
            "PBS.md",
            "MAIL_GATEWAY.md",
            "METRICS_HISTORY.md",
            "KUBERNETES.md",
            "UNIFIED_RESOURCES.md",
            "VM_DISK_MONITORING.md",
            "MIGRATION_UNIFIED_NAV.md",
            "RELAY.md",
            "API.md",
            "AUTO_UPDATE.md",
        ):
            with self.subTest(guide=name):
                self.assertEqual(doc_text(name),
                                 (ROOT / "frontend-modern/public/docs" / name).read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
