#!/usr/bin/env python3
"""Bind Proxmox, vSphere, Kubernetes, Docker stale-reading, Availability nav, and agent opt-out Help to live source.

Documentation consistency checks only, not live Proxmox, vSphere, Kubernetes, Docker, or agent runtime tests.
"""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]


def doc_text(rel: str) -> str:
    return (ROOT / rel).read_text(encoding="utf-8")


def norm(text: str) -> str:
    return " ".join(text.split())


class StaleReadingAndAgentHelpDocsTest(unittest.TestCase):
    def test_faq_documents_top_level_availability_navigation(self):
        faq = doc_text("docs/FAQ.md")
        section = norm(
            faq.split("### How is navigation organised in Pulse v6?\n", 1)[1].split(
                "\n### ", 1
            )[0]
        )
        for phrase in (
            "**Availability** (`/availability`)",
            "website, service, device, and TLS certificate checks",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, section)
        self.assertNotIn("Push checks", section)

        layout = doc_text("frontend-modern/src/AppLayout.tsx")
        for marker in (
            "id: 'availability'",
            "label: 'Availability'",
            "route: ROOT_AVAILABILITY_PATH",
            "tooltip: 'Website, service, device, and TLS certificate checks'",
        ):
            self.assertIn(marker, layout)

    def test_faq_documents_proxmox_node_reading_status_badges(self):
        faq = doc_text("docs/FAQ.md")
        section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**Proxmox nodes**",
            "beside the node name on both desktop and phones",
            "withholding live metrics and uptime when readings are not current",
            "**Offline** (a failed Proxmox read does not prove the host is powered off)",
            "**Stale · API error**",
            "**Stale · Degraded**",
            "**Stale** (Pulse has no recent Proxmox readings)",
            "**Degraded** (a node warning with current Proxmox readings)",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, section)

        table = doc_text("frontend-modern/src/features/proxmox/ProxmoxNodesTable.tsx")
        for marker in (
            "Offline in Pulse. A failed Proxmox read does not prove the host is powered off.",
            "label: 'Stale · API error'",
            "label: 'Stale · Degraded'",
            "Pulse has no recent Proxmox readings.",
            "label: 'Degraded'",
            "const showReadings = () => readingState().variant === 'success';",
        ):
            self.assertIn(marker, table)

    def test_faq_documents_proxmox_ceph_stale_readings(self):
        faq = doc_text("docs/FAQ.md")
        ceph_section = norm(
            faq.split("### Does Pulse monitor Ceph?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**Proxmox → Ceph**",
            "**Stale data** beneath the cluster name on both desktop and phones",
            "switches stale **Healthy** or **Unknown** rows to amber **Stale**",
            "out of **Healthy** into **Warning**",
            "keeping worse **Warning** and **Critical** verdicts visible",
            "[No data showing?](#no-data-showing)",
        ):
            with self.subTest(section="Does Pulse monitor Ceph?", phrase=phrase):
                self.assertIn(phrase, ceph_section)

        no_data_section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**Proxmox Ceph clusters** (`/proxmox/ceph`)",
            "**Stale data** beneath the cluster name on desktop and phones",
            "moving stale **Healthy** or **Unknown** clusters out of **Healthy** into **Warning** with an amber **Stale** indicator",
            "keeping worse **Warning** and **Critical** verdicts ahead of staleness",
            "`Proxmox collection error`",
            "`Proxmox has not updated this cluster recently`",
            "**Readings** under **Worth checking** (`Shown figures are last reported values.`)",
            "withholds stale snapshot values as live chart readings until fresh Proxmox updates arrive",
        ):
            with self.subTest(section="No data showing?", phrase=phrase):
                self.assertIn(phrase, no_data_section)

        model = doc_text(
            "frontend-modern/src/features/proxmox/proxmoxCephClusterDrawerModel.ts"
        )
        for marker in (
            "badge: 'Stale data'",
            "'Proxmox collection error'",
            "'Proxmox has not updated this cluster recently'",
            "makeDetailRow('Readings', `${staleNotice.detail}. Shown figures are last reported values.`",
        ):
            self.assertIn(marker, model)

        table = doc_text("frontend-modern/src/features/proxmox/ProxmoxCephTable.tsx")
        for marker in (
            "if (reported === 'critical') return 'critical';",
            "return getCephClusterStaleNotice(resource) ? 'warning' : reported;",
            "label: 'Stale'",
            "if (staleNotice && (reported === 'healthy' || reported === 'unknown')) {",
            'data-proxmox-ceph-stale="cluster"',
            "healthMessageCell(cluster, staleNotice())",
        ):
            self.assertIn(marker, table)

        drawer = doc_text(
            "frontend-modern/src/features/proxmox/ProxmoxCephClusterDrawer.tsx"
        )
        self.assertIn("liveReadingsAvailable={!staleNotice()}", drawer)

    def test_faq_documents_vsphere_datastore_stale_readings_and_unknown_capacity(self):
        faq = doc_text("docs/FAQ.md")
        section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**vSphere datastores**",
            "**Last reported inaccessible**",
            "**Stale data** beside the name",
            "`vCenter has not updated this datastore recently.`",
            "Missing or unreadable capacity displays as a dash (`—`), not `0%`",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, section)

        table = doc_text("frontend-modern/src/features/vmware/VsphereDatastoresTable.tsx")
        model = doc_text("frontend-modern/src/features/vmware/vmwarePageModel.ts")
        history = doc_text(
            "frontend-modern/src/components/Infrastructure/resourceDetailDrawerMetricsHistoryModel.ts"
        )
        self.assertIn("Stale data", table)
        self.assertIn("PlatformTableMetricFallback", table)
        self.assertIn("'Last reported inaccessible'", model)
        self.assertIn(
            "`vCenter has not updated this ${isHost ? 'host' : 'datastore'} recently.`",
            history,
        )

    def test_faq_documents_vsphere_host_and_network_stale_readings(self):
        faq = doc_text("docs/FAQ.md")
        section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**vSphere ESXi hosts and networks**",
            "**Stale data** beneath stale names",
            "**Last reported offline**",
            "**Worth checking** (**Readings**)",
            "`vCenter has not updated this host recently.`",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, section)

        hosts_table = doc_text("frontend-modern/src/features/vmware/VsphereHostsTable.tsx")
        networks_table = doc_text("frontend-modern/src/features/vmware/VsphereNetworksTable.tsx")
        model = doc_text("frontend-modern/src/features/vmware/vmwarePageModel.ts")
        drawer_model = doc_text(
            "frontend-modern/src/components/Infrastructure/resourceDetailDrawerVmwareModel.ts"
        )
        shared_notice = doc_text("frontend-modern/src/utils/truenasDatasetState.ts")
        self.assertIn("getVmwareStaleNotice(host)", hosts_table)
        self.assertIn("Stale data", networks_table)
        self.assertIn("'Last reported offline'", model)
        self.assertIn("wrapRow('Readings', staleNotice.detail, { tone: 'warning' })", drawer_model)
        self.assertIn("badge: 'Stale data'", shared_notice)

    def test_faq_and_kubernetes_guide_document_kubernetes_stale_readings(self):
        faq = doc_text("docs/FAQ.md")
        faq_section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**Kubernetes clusters, nodes, pods, Deployments and controllers**",
            "**Stale data**",
            "**Offline**, **NotReady**, **Unschedulable**, **Failed**, **CrashLoopBackOff**",
            "**Healthy** into **Degraded** and **Nodes needing attention**",
            "`Showing the last Kubernetes report — no fresh update received recently.`",
            "[stale Kubernetes readings](KUBERNETES.md#stale-kubernetes-readings)",
        ):
            with self.subTest(doc="FAQ.md", phrase=phrase):
                self.assertIn(phrase, faq_section)

        k8s_doc = doc_text("docs/KUBERNETES.md")
        k8s_section = norm(k8s_doc.split("### Stale Kubernetes readings\n", 1)[1])
        for phrase in (
            "**Stale data**",
            "**Offline**, **NotReady**, **Unschedulable**, **Failed**, **CrashLoopBackOff**",
            "**Healthy** and appear under **Degraded**",
            "**Nodes needing attention** on **Kubernetes → Overview**",
            "`Showing the last Kubernetes report — no fresh update received recently.`",
            "`Last error:`",
        ):
            with self.subTest(doc="KUBERNETES.md", phrase=phrase):
                self.assertIn(phrase, k8s_section)

        k8s_model = doc_text("frontend-modern/src/features/kubernetes/kubernetesPageModel.ts")
        k8s_detail = doc_text("frontend-modern/src/features/kubernetes/KubernetesRecordDetail.tsx")
        k8s_surface = doc_text("frontend-modern/src/features/kubernetes/KubernetesPageSurface.tsx")
        for marker in (
            "export const KUBERNETES_STALE_READINGS_LABEL = 'Stale data';",
            "'Showing the last Kubernetes report — no fresh update received recently.'",
            "Last error: ${sourceError}",
            "if (hasStaleKubernetesReadings(resource) || hasImpairedKubernetesSource(resource)) {",
            "return 'degraded';",
        ):
            self.assertIn(marker, k8s_model)
        self.assertIn("KubernetesStaleDataCue", k8s_detail)
        self.assertIn('data-testid="kubernetes-stale-readings-notice"', k8s_detail)
        self.assertIn('title="Nodes needing attention"', k8s_surface)

    def test_help_documents_kubernetes_inventory_record_drawers_and_events_staleness(
        self,
    ):
        k8s_doc = doc_text("docs/KUBERNETES.md")
        k8s_section = norm(k8s_doc.split("### Stale Kubernetes readings\n", 1)[1])
        for phrase in (
            "**Autoscaling**, **Services**, **Networking**, **Storage**, **Configuration**, **Policy**, and **Events**",
            "**Active Alerts** on **Kubernetes → Overview**",
            "**Events**, rows keep their **Warning** and **Normal** severity dots",
            "Inventory table search matches the **Stale data** label and recorded Kubernetes source error text",
            "inline record drawer (ConfigMaps, Secrets, ServiceAccounts, RBAC, Policy and Events)",
            "`No Kubernetes report for <age> — showing the last values received.`",
            "`This alert reflects the last report received and may no longer be current.`",
        ):
            with self.subTest(doc="KUBERNETES.md", phrase=phrase):
                self.assertIn(phrase, k8s_section)

        faq = doc_text("docs/FAQ.md")
        faq_section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**Autoscaling**, **Services**, **Networking**, **Storage**, **Configuration**, **Policy** and **Events**",
            "**Events** rows keep their **Warning** and **Normal** severity dots alongside the **Stale data** cue",
            "inventory search matches **Stale data** and recorded source errors",
            "expanded object and record drawers (including ConfigMaps, Secrets, ServiceAccounts, RBAC, Policy and Events)",
        ):
            with self.subTest(doc="FAQ.md", phrase=phrase):
                self.assertIn(phrase, faq_section)

        k8s_model = doc_text("frontend-modern/src/features/kubernetes/kubernetesPageModel.ts")
        k8s_detail = doc_text("frontend-modern/src/features/kubernetes/KubernetesRecordDetail.tsx")
        k8s_events = doc_text("frontend-modern/src/features/kubernetes/KubernetesEventsTable.tsx")
        for marker in (
            "export function mapKubernetesInventoryStatus(resource: Resource): StatusIndicator",
            "hasStaleKubernetesReadings(resource) ? KUBERNETES_STALE_READINGS_LABEL : undefined",
            "hasStaleKubernetesReadings(resource) ? getKubernetesSourceError(resource) : undefined",
        ):
            self.assertIn(marker, k8s_model)
        self.assertIn("<KubernetesStaleReadingsNotice resource={props.resource} />", k8s_detail)
        self.assertIn('staleScope="event"', k8s_events)
        for rel in (
            "frontend-modern/src/features/kubernetes/KubernetesAutoscalingTable.tsx",
            "frontend-modern/src/features/kubernetes/KubernetesServicesTable.tsx",
            "frontend-modern/src/features/kubernetes/KubernetesNetworkingTable.tsx",
            "frontend-modern/src/features/kubernetes/KubernetesStorageTable.tsx",
            "frontend-modern/src/features/kubernetes/KubernetesConfigTable.tsx",
            "frontend-modern/src/features/kubernetes/KubernetesPolicyTable.tsx",
        ):
            with self.subTest(source=rel):
                self.assertIn("mapKubernetesInventoryStatus", doc_text(rel))

    def test_help_documents_platform_alerts_and_vsphere_activity_staleness(self):
        faq = doc_text("docs/FAQ.md")
        faq_section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**vSphere Activity** (`/vmware/activity`) and **platform alert tables**",
            "**Health Signals** on **VMware → Overview** and **VMware → Health**",
            "**Health Alerts** on **TrueNAS → Overview**",
            "**Active Alerts** on **Docker → Overview** and **Kubernetes → Overview**",
            "`This alert reflects the last report received and may no longer be current.`",
            "`Activity records reflect the last report received and newer vCenter tasks or events may be missing.`",
            "search matches stale labels and collection errors on vSphere, TrueNAS and Docker alert/activity tables",
        ):
            with self.subTest(doc="FAQ.md", phrase=phrase):
                self.assertIn(phrase, faq_section)

        truenas = norm(doc_text("docs/TRUENAS.md"))
        for phrase in (
            "**Health Alerts** on **TrueNAS → Overview** (`/truenas/overview`)",
            "**Critical**, **Warning**, and **Info** severity indicators",
            "show an amber **Stale data** cue beneath the affected resource name",
            "match stale labels and collection-error text in search",
            "`This alert reflects the last report received and may no longer be current.`",
        ):
            with self.subTest(doc="TRUENAS.md", phrase=phrase):
                self.assertIn(phrase, truenas)

        alerts_table = doc_text("frontend-modern/src/features/platformPage/PlatformAlertsTable.tsx")
        staleness = doc_text("frontend-modern/src/features/platformPage/resourceStaleness.ts")
        activity = doc_text("frontend-modern/src/features/vmware/VsphereActivityTable.tsx")
        for marker in (
            "getPlatformResourceStaleNotice(props.platform, incident.resource)",
            "data-platform-alert-stale={props.attrPrefix}",
            "This alert",
            "reflects the last report received and may no longer be",
            "current.",
        ):
            self.assertIn(marker, alerts_table)
        self.assertIn("export const getPlatformResourceStaleNotice =", staleness)
        for marker in (
            'data-vsphere-activity-stale="true"',
            'data-testid="vsphere-activity-stale-readings-notice"',
            "Activity records reflect the last report",
            "received and newer vCenter tasks or events may be missing.",
        ):
            self.assertIn(marker, activity)
        for rel in (
            "frontend-modern/src/features/docker/dockerPageModel.ts",
            "frontend-modern/src/features/truenas/truenasPageModel.ts",
            "frontend-modern/src/features/vmware/vmwarePageModel.ts",
        ):
            with self.subTest(source=rel):
                src = doc_text(rel)
                self.assertIn("staleNotice?.label", src)
                self.assertIn("staleNotice?.detail", src)

    def test_faq_documents_docker_compose_and_swarm_stale_readings(self):
        faq = doc_text("docs/FAQ.md")
        faq_section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**Docker Compose applications and Swarm**",
            "`/docker/applications` and `/docker/swarm`",
            "Services, Tasks, Nodes, Configs and Secrets",
            "**Stale data**",
            "**Failed**, **Rejected**, **Orphaned**, **Manager unreachable**, **Down**, **Drained**, **Update paused**, **Rolling back**, **Rollback paused**, and `0 / N running`",
            "Stale Compose applications move out of **Healthy** into **Attention**",
            "stale Swarm resources move out of **Healthy** into **Degraded**",
            "`Showing the last Docker report — no fresh update received recently.`",
            "`No Docker report for <age> — showing the last values received.`",
            "`Docker collection error — showing the last report received.`",
            "`Last error:`",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, faq_section)

        docker_model = doc_text("frontend-modern/src/features/docker/dockerPageModel.ts")
        docker_apps = doc_text("frontend-modern/src/features/docker/DockerApplications.tsx")
        docker_shared = doc_text("frontend-modern/src/features/docker/DockerNativeTableShared.tsx")
        for marker in (
            "export const DOCKER_STALE_READINGS_LABEL = 'Stale data';",
            "'Docker collection error — showing the last report received.'",
            "`No Docker report for ${age} — showing the last values received.`",
            "'Showing the last Docker report — no fresh update received recently.'",
            "Last error: ${sourceError}",
            "if ((triad === 'online' || triad === 'unknown') && getDockerStaleNotice(resource)) {",
            "return 'degraded';",
        ):
            self.assertIn(marker, docker_model)
        self.assertIn("label: 'Attention'", docker_apps)
        self.assertIn('data-testid="docker-application-stale-notice"', docker_apps)
        self.assertIn('data-testid="docker-stale-readings-notice"', docker_shared)

    def test_unified_agent_aligns_external_probes_nav_and_proxmox_opt_out_preservation(
        self,
    ):
        agent_doc = doc_text("docs/UNIFIED_AGENT.md")
        probes = norm(
            agent_doc.split("## External Probes (Pro)\n", 1)[1].split("\n## ", 1)[0]
        )
        for phrase in (
            "Open **Availability** (`/availability`",
            "**Observation locations**",
            "**This Pulse server**",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, probes)

        auto_update = norm(
            agent_doc.split("## Auto-Update\n", 1)[1].split("\n## ", 1)[0]
        )
        for phrase in (
            "Current installers preserve the selected agent's installed Proxmox setup choice during `--update`",
            "`--enable-proxmox=false`",
            "Older installers (including v6.8.0 and v6.9.0)",
            "passing `--disable-proxmox` alongside `--update` remains safe across versions",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, auto_update)

        installer = doc_text("scripts/install.sh")
        for marker in (
            "EXEC_ARG_ITEMS+=(--enable-proxmox=false)",
            "recover_installed_proxmox_setup()",
            'ENABLE_PROXMOX="${ENABLE_PROXMOX:-false}"',
        ):
            self.assertIn(marker, installer)

    def test_faq_truenas_and_recovery_document_truenas_protection_stale_readings(self):
        faq = doc_text("docs/FAQ.md")
        truenas_faq = norm(
            faq.split("### Does Pulse monitor TrueNAS?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**TrueNAS → Protection** (`/truenas/protection`)",
            "**Stale data** (**Stale** on phones for stale **Healthy** rows)",
            "excludes stale **Healthy** and **Running** events from those filters",
            "dynamic amber **Stale** filter chip",
            "**Status** and **Last reported**",
            "[Read TrueNAS protection evidence](TRUENAS.md#read-truenas-protection-evidence)",
        ):
            with self.subTest(section="Does Pulse monitor TrueNAS?", phrase=phrase):
                self.assertIn(phrase, truenas_faq)

        no_data_section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**TrueNAS protection** (`/truenas/protection`)",
            "by connection ID or provider scope first, then unambiguous hostname",
            "Stale **Healthy** and **Running** events leave those filters, join **Attention** and the dynamic amber **Stale** filter chip",
            "Stale **Healthy** rows show an amber **Stale data** badge (**Stale** on phones)",
            "muted non-pulsing **Running**, and **Unknown** rows keep their outcome badge with an amber **Stale data** cue beneath the dataset name",
            "`Readings from <system> may be out of date.`",
            "`Showing the last reported protection event. Newer snapshots, replication runs, or failures may be missing.`",
            "**Status** (`Stale data`) and **Last reported**",
        ):
            with self.subTest(section="No data showing?", phrase=phrase):
                self.assertIn(phrase, no_data_section)

        recovery = doc_text("docs/RECOVERY.md")
        where_to_look = norm(
            recovery.split("## Where to look\n", 1)[1].split("\n## ", 1)[0]
        )
        for phrase in (
            "**TrueNAS → Protection** (`/truenas/protection`)",
            "**Stale data** indicators (**Stale** on phones for stale **Healthy** rows)",
            "move out of **Healthy** and **Running** into **Attention** and the dynamic **Stale** filter chip",
            "[Read TrueNAS protection evidence](TRUENAS.md#read-truenas-protection-evidence)",
        ):
            with self.subTest(doc="RECOVERY.md", phrase=phrase):
                self.assertIn(phrase, where_to_look)

        surface = doc_text("frontend-modern/src/features/truenas/TrueNASPageSurface.tsx")
        table = doc_text("frontend-modern/src/features/truenas/TrueNASProtectionTable.tsx")
        model = doc_text("frontend-modern/src/features/truenas/truenasPageModel.ts")
        self.assertIn("systems={model().systems}", surface)
        for marker in (
            "const TRUENAS_PROTECTION_STALE_OPTION",
            "label: 'Stale'",
            'compact="Stale" full="Stale data"',
            "detailRow('Status', staleNotice.badge, { tone: 'warning' })",
            "detailRow('Last reported', outcomeLabel, { tone: outcomeTone(point) })",
            "pulse={!staleNotice() && statusBucket() === 'running'}",
            "Readings from {notice().systemName} may be out of date.",
            "Showing the last reported protection event.",
            "Newer snapshots, replication runs, or failures may be missing.",
        ):
            self.assertIn(marker, table)
        for marker in (
            "export function buildTrueNASProtectionStaleNoticeMap(",
            "if (status === 'stale' && !isStale) return false;",
            "if (status === 'success' && (pointStatus !== 'success' || isStale)) return false;",
            "if (status === 'running' && (pointStatus !== 'running' || isStale)) return false;",
            "if (staleNotices?.has(point.id)) return 2;",
        ):
            self.assertIn(marker, model)

    def test_faq_documents_docker_images_storage_and_networks_stale_readings(self):
        faq = doc_text("docs/FAQ.md")
        faq_section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**Docker images, storage and networks**",
            "`/docker/images`, `/docker/storage` covering **Volumes** and **Engine Storage Usage**, and `/docker/networks`",
            "amber **Stale data** status indicator and inline **Stale data** cue beside the row name",
            "On `/docker/images`, stale **Current** update badges switch from green to muted",
            "On `/docker/networks`, stale networks move out of **In use** and **Unused** into **Attention**",
            "keep last-reported container counts without painting **State** green",
            "apply source freshness to attached containers while keeping worse states (such as **Offline** or **Unhealthy**) ahead of staleness",
            "same inline Docker stale-readings notice and freshness-aware status indicator",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, faq_section)

        images = doc_text("frontend-modern/src/features/docker/DockerImagesTable.tsx")
        volumes = doc_text("frontend-modern/src/features/docker/DockerVolumesTable.tsx")
        storage = doc_text("frontend-modern/src/features/docker/DockerStorageUsageTable.tsx")
        networks = doc_text("frontend-modern/src/features/docker/DockerNetworksTable.tsx")
        shared = doc_text("frontend-modern/src/features/docker/DockerNativeTableShared.tsx")
        model = doc_text("frontend-modern/src/features/docker/dockerPageModel.ts")

        self.assertIn("return staleNotice() && tone === 'success' ? 'muted' : tone;", images)
        self.assertIn('<DockerStaleDataCue notice={staleNotice()} scope="image" />', images)
        self.assertIn('<DockerStaleReadingsNotice notice={staleNotice()} class="mb-3" />', images)
        self.assertIn('<DockerStaleDataCue notice={staleNotice()} scope="volume" />', volumes)
        self.assertIn('<DockerStaleReadingsNotice notice={staleNotice()} class="mb-3" />', volumes)
        self.assertIn('<DockerStaleDataCue notice={staleNotice()} scope="storage-usage" />', storage)
        self.assertIn("<DockerStaleReadingsNotice notice={staleNotice()} />", storage)
        for marker in (
            '<DockerStaleDataCue notice={staleNotice()} scope="network" />',
            "variant: attention.variant === 'success' ? 'muted' : attention.variant,",
            "if (filter === 'attention' && !hasAttention) return false;",
            "if (filter === 'in-use' && (hasAttention || attachments.length === 0)) return false;",
            "if (filter === 'unused' && (hasAttention || attachments.length > 0)) return false;",
        ):
            self.assertIn(marker, networks)
        self.assertIn('data-docker-stale={props.scope}', shared)
        self.assertIn('<DockerStaleReadingsNotice notice={staleNotice()} />', shared)
        self.assertIn(
            "const status = applyDockerSourceFreshness(container, mapDockerContainerStatus(container));",
            model,
        )

    def test_agent_help_documents_runner_free_apt_inspection_and_windows_staging_flush(self):
        unified = norm(doc_text("docs/UNIFIED_AGENT.md"))
        for phrase in (
            "Read-only Linux APT pending-update inspection runs directly in the collector without requiring an action runner or package-remediation authority.",
            "`apt-get -s -o Debug::NoLocking=1 upgrade`",
            "clean environment (`LC_ALL=C`) with a 20-second deadline and process-group cancellation",
            "failing closed on truncated or cancelled output rather than reporting zero pending updates",
            "Current Windows agents flush the downloaded binary through its original writable handle before closing rather than reopening it for write after identity and self-tests, while still refusing replacement if flushing fails.",
        ):
            with self.subTest(doc="UNIFIED_AGENT.md", phrase=phrase):
                self.assertIn(phrase, unified)

        security = norm(doc_text("docs/AGENT_SECURITY.md"))
        for phrase in (
            "Package commands stay with the runner.",
            "Read-only Linux APT pending-update inspection (`apt-get -s -o Debug::NoLocking=1 upgrade`) is a local monitoring read that runs in the collector without an action runner or privilege gain. Package installation and upgrade mutations remain runner-only and are refused on the collector command client.",
        ):
            with self.subTest(doc="AGENT_SECURITY.md", phrase=phrase):
                self.assertIn(phrase, security)

        pkg_linux = doc_text("internal/hostagent/package_inspection_linux.go")
        pkg_common = doc_text("internal/hostagent/package_inspection.go")
        updater = doc_text("internal/agentupdate/update.go")
        for marker in (
            "const packageInspectionTimeout = 20 * time.Second",
            'exec.CommandContext(ctx, path, "-s", "-o", "Debug::NoLocking=1", "upgrade")',
            '"LC_ALL=C"',
            "configureCommandProcessGroup(cmd)",
            "package inspection output exceeded its limit",
        ):
            self.assertIn(marker, pkg_linux)
        self.assertIn(
            'only the fixed APT report simulation is allowed',
            pkg_common,
        )
        for marker in (
            "if runtimeGOOS == goOSWindows {",
            "if err := syncOpenFileFn(tmpFile); err != nil {",
            'syncErr = fmt.Errorf("failed to sync staged binary: %w", err)',
            "if runtimeGOOS != goOSWindows {",
        ):
            self.assertIn(marker, updater)

    def test_agent_help_documents_monitoring_only_ceilings_signed_config_and_docker_report_shedding(
        self,
    ):
        security = norm(doc_text("docs/AGENT_SECURITY.md"))
        for phrase in (
            "That ceiling also governs the Docker agent module in both foreground and Windows service runtimes, rejecting legacy `update_container` and `update_all` commands in server report responses regardless of server settings.",
            "On Windows services, the local `CommandAuthorityProfile`, `DisableUpdateChecks`, and `DisableRegistryCredentials` settings are forwarded to match foreground behavior.",
            "When signed remote configuration is required (`PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED=true`), an unsigned `commandsEnabled` flag in a host report acknowledgement can still turn command execution off, but cannot enable it.",
        ):
            with self.subTest(doc="AGENT_SECURITY.md", phrase=phrase):
                self.assertIn(phrase, security)

        unified = norm(doc_text("docs/UNIFIED_AGENT.md"))
        for phrase in (
            "a `monitoring-only` install cannot be promoted remotely, including in the Docker module and Windows service runtimes",
            "With signature verification required, unsigned `commandsEnabled` overrides carried in report acknowledgements can disable command execution, but cannot enable it.",
            "shared report size limit of **2 MiB** encoded HTTP body or **10 MiB** decoded JSON on Docker and Podman reports",
            "objects over **32 KiB**, then **4 KiB**, then all free-form metadata",
            "containers with more than **64** attachments, then all ports and mounts",
            "HTTP **413** (`report exceeds the size the destination accepts`)",
            "drops that report without buffering or replaying it",
        ):
            with self.subTest(doc="UNIFIED_AGENT.md", phrase=phrase):
                self.assertIn(phrase, unified)

        win_svc = doc_text("cmd/pulse-agent/service_windows.go")
        for marker in (
            "CommandAuthorityProfile: ws.cfg.CommandAuthorityProfile,",
            "MonitoringOnly:     ws.cfg.CommandAuthorityProfile == hostagent.CommandAuthorityMonitoringOnly,",
            "DisableUpdateChecks:        ws.cfg.DisableDockerUpdateChecks,",
            "DisableRegistryCredentials: ws.cfg.DisableRegistryCredentials,",
        ):
            self.assertIn(marker, win_svc)

        host_agent = doc_text("internal/hostagent/agent.go")
        self.assertIn("if commandsEnabled && remoteconfig.SignatureRequired() {", host_agent)
        remote_cfg = doc_text("internal/remoteconfig/client.go")
        self.assertIn('utils.GetenvTrim("PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED")', remote_cfg)

        docker_agent = doc_text("internal/dockeragent/agent.go")
        for marker in (
            'var errReportTooLarge = errors.New("report exceeds the size the destination accepts")',
            "var reportMetadataShedSteps = []int{32 * 1024, 4 * 1024, 0}",
            "for _, limit := range []int{64, 0} {",
            "if errors.Is(err, errReportTooLarge) {",
        ):
            self.assertIn(marker, docker_agent)

        report_limits = doc_text("pkg/agents/docker/report_limits.go")
        self.assertIn("ReportEncodedBodyLimitBytes int64 = 2 * 1024 * 1024", report_limits)
        self.assertIn("ReportDecodedBodyLimitBytes int64 = 10 * 1024 * 1024", report_limits)

    def test_help_documents_physical_disk_stale_readings_across_storage_and_drawers(self):
        faq = doc_text("docs/FAQ.md")
        faq_section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**Physical disks**",
            "**Physical Disks & SMART** cards in guest, machine, and host drawers",
            "switch from **Healthy** or **Online** to amber **Stale**",
            "leave the **Healthy** filter, join **Attention** and **Warning**, and sort ahead of fresh healthy disks",
            "Worse verdicts (**Replace Now** and **Needs Attention**) stay visible with an inline amber **Stale data** cue beneath the disk name",
            "Stale temperatures and SSD **Life remaining** percentages render in muted dotted-underline last-known styling and never trigger a live **Running Hot** verdict",
            "Physical disk search matches `stale`, `stale data`, refresh age, and recorded source error text",
            "`Stale data. Retained last-known disk readings.`",
            "`Health, temperature, and SMART readings reflect the last report received and may no longer be current.`",
            "labels the health card **Last known temperature**, and defers temperature and SMART history charts as last-known values until live reporting recovers",
        ):
            with self.subTest(doc="FAQ.md", phrase=phrase):
                self.assertIn(phrase, faq_section)

        truenas = doc_text("docs/TRUENAS.md")
        truenas_disk = norm(
            truenas.split("## Disk temperature and health\n", 1)[1].split(
                "## Multiple TrueNAS Systems", 1
            )[0]
        )
        for phrase in (
            "switch from **Healthy** to amber **Stale** (leaving the **Healthy** filter and joining **Attention** and **Warning**)",
            "**Replace Now** and **Needs Attention** verdicts stay visible with an inline amber **Stale data** cue beneath the disk name",
            "Stale temperatures and SSD **Life remaining** values render as muted last-known readings without triggering **Running Hot**",
            "`Stale data. Retained last-known disk readings.`",
            "deferring temperature and SMART history charts as last-known values until live polling recovers",
        ):
            with self.subTest(doc="TRUENAS.md", phrase=phrase):
                self.assertIn(phrase, truenas_disk)

        unified = doc_text("docs/UNIFIED_AGENT.md")
        smart_section = norm(
            unified.split("## S.M.A.R.T. Disk Health\n", 1)[1].split(
                "\n## Auto-Update", 1
            )[0]
        )
        for phrase in (
            "**Physical Disks & SMART** cards inside guest, machine, and host drawers",
            "switch clear disks from **Healthy** or **Online** to amber **Stale**",
            "keep **Replace Now** and **Needs Attention** verdicts visible with an inline amber **Stale data** cue",
            "style stale temperatures and SSD **Life remaining** as muted last-known values so they never trigger **Running Hot**",
            "`Stale data. Retained last-known disk readings.`",
            "deferring temperature and SMART history charts until fresh agent reports arrive",
        ):
            with self.subTest(doc="UNIFIED_AGENT.md", phrase=phrase):
                self.assertIn(phrase, smart_section)

        disk_pres = doc_text("frontend-modern/src/features/storageBackups/diskPresentation.ts")
        disk_temp = doc_text(
            "frontend-modern/src/features/storageBackups/diskTemperaturePresentation.ts"
        )
        disk_drawer = doc_text(
            "frontend-modern/src/features/storageBackups/diskDrawerPresentation.ts"
        )
        disk_list = doc_text("frontend-modern/src/components/Storage/DiskList.tsx")
        disk_overview = doc_text("frontend-modern/src/components/Storage/DiskDrawerOverview.tsx")
        guest_disks = doc_text("frontend-modern/src/components/Workloads/GuestPhysicalDisks.tsx")
        pool_detail = doc_text(
            "frontend-modern/src/features/storageBackups/storagePoolDetailPresentation.ts"
        )

        for marker in (
            "export function extractPhysicalDiskFreshness(resource: Resource)",
            "return `Retained last-known disk readings.${age}${error}`.trim();",
            "disk.freshness === 'stale' ? 'stale stale data' : ''",
            "label: 'Stale'",
            "if (verdict.kind === 'attention' || disk.freshness === 'stale') return 'warning';",
            "return disk.freshness === 'stale' ? 100 : 0;",
        ):
            self.assertIn(marker, disk_pres)
        for marker in (
            "if (freshness === 'stale') return false;",
            "if (!isPhysicalDiskTemperatureCurrent(disk.collection, disk.freshness)) return false;",
        ):
            self.assertIn(marker, disk_temp)
        for marker in (
            "if (disk.freshness === 'stale') return {};",
            "if (disk.freshness === 'stale') {",
            "temperature.current ? 'Temperature' : 'Last known temperature'",
        ):
            self.assertIn(marker, disk_drawer)
        self.assertIn("data-disk-stale-cue", disk_list)
        self.assertIn('data-life-reading={isStale() ? \'last-known\' : \'current\'}', disk_list)
        for marker in (
            'data-testid="disk-drawer-stale-notice"',
            '<span class="font-medium">Stale data.</span>',
            "Health, temperature, and",
            "SMART readings reflect the last report received and may no longer be current.",
        ):
            self.assertIn(marker, disk_overview)
        self.assertIn("data-disk-stale-cue", guest_disks)
        self.assertIn("extractPhysicalDiskFreshness(disk)", pool_detail)

    def test_help_documents_proxmox_backups_stale_telemetry(self):
        recovery = doc_text("docs/RECOVERY.md")
        where_to_look = norm(
            recovery.split("## Where to look\n", 1)[1].split("\n## ", 1)[0]
        )
        self.assertIn("(#stale-proxmox-backup-telemetry)", where_to_look)

        stale_section = norm(
            recovery.split("## Stale Proxmox backup telemetry\n", 1)[1].split(
                "\n## ", 1
            )[0]
        )
        for phrase in (
            "**Proxmox → Backups** (`/proxmox/backups`)",
            "optional stale host agent on a Proxmox host does not mark backup telemetry stale",
            "`Backup telemetry is stale for <sources>. Protection status, job counts, and restore points show the last values received before polling stopped.`",
            "**Backup health** switches a green protected verdict indicator to amber and adds `· Stale data`",
            "**Backup jobs** adds `· Stale data` in its header while keeping **Last 24h** pills and 14-day bars intact",
            "`Stale · <age>` or `Stale data`, or `Stale` on compact phone layouts",
            "Unaffected fresh siblings stay untouched",
            "`Readings are stale.` (`protection status and restore points show the last values received.`)",
            "`Source telemetry is stale.` (`newer backups or prunes may not appear yet.`)",
            "Search in both **Coverage** and **By date** matches stale labels, stale details, and recorded source error text",
            "[Stale PBS readings and unread datastore capacity](PBS.md#stale-pbs-readings-and-unread-datastore-capacity)",
            "VM_DISK_MONITORING.md#backup-safety",
        ):
            with self.subTest(doc="RECOVERY.md", phrase=phrase):
                self.assertIn(phrase, stale_section)

        pbs = doc_text("docs/PBS.md")
        pbs_section = norm(
            pbs.split("### Stale PBS readings and unread datastore capacity\n", 1)[1].split(
                "\n### ", 1
            )[0]
        )
        for phrase in (
            "**Page-wide Proxmox Backups cues:**",
            "shares its PBS poll-staleness check with the rest of **Proxmox → Backups** (`/proxmox/backups`)",
            "`· Stale data` header cues on **Backup health** and **Backup jobs**",
            "[Stale Proxmox backup telemetry](RECOVERY.md#stale-proxmox-backup-telemetry)",
        ):
            with self.subTest(doc="PBS.md", phrase=phrase):
                self.assertIn(phrase, pbs_section)

        faq = doc_text("docs/FAQ.md")
        faq_section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**Proxmox Backups** (`/proxmox/backups`)",
            "`Backup telemetry is stale for <sources>. Protection status, job counts, and restore points show the last values received before polling stopped.`",
            "**Backup health** downgrades a green protected dot to amber with a `· Stale data` header cue",
            "**Backup jobs** adds `· Stale data` while preserving **Last 24h** pills and 14-day bars",
            "`Readings are stale.` in **Coverage** and `Source telemetry is stale.` in **By date**",
            "[Stale Proxmox backup telemetry](RECOVERY.md#stale-proxmox-backup-telemetry)",
            "[Stale PBS readings](PBS.md#stale-pbs-readings-and-unread-datastore-capacity)",
        ):
            with self.subTest(doc="FAQ.md", phrase=phrase):
                self.assertIn(phrase, faq_section)

        staleness = doc_text(
            "frontend-modern/src/features/proxmox/proxmoxBackupStaleness.ts"
        )
        backups_table = doc_text(
            "frontend-modern/src/features/proxmox/ProxmoxBackupsTable.tsx"
        )
        strip = doc_text(
            "frontend-modern/src/features/proxmox/ProxmoxBackupsCoverageStrip.tsx"
        )
        jobs_card = doc_text(
            "frontend-modern/src/features/proxmox/ProxmoxBackupJobsCard.tsx"
        )
        coverage = doc_text(
            "frontend-modern/src/features/proxmox/ProxmoxCoverageTable.tsx"
        )
        recoverable = doc_text(
            "frontend-modern/src/features/proxmox/ProxmoxRecoverableTable.tsx"
        )
        shared = doc_text(
            "frontend-modern/src/features/proxmox/proxmoxBackupsTableShared.tsx"
        )

        for marker in (
            "Backup telemetry is stale for ${list}. Protection status, job counts, and restore points show the last values received before polling stopped.",
            "badge: age ? `Stale · ${age}` : 'Stale data'",
            "compactBadge: 'Stale'",
            "'protection status and restore points show the last values received.'",
            "'newer backups or prunes may not appear yet.'",
            "export const isPBSServerPollStale =",
            "export const hasStaleTextMatch =",
        ):
            self.assertIn(marker, staleness)
        self.assertIn("data-stale-backup-sources", backups_table)
        self.assertIn("hasStaleTextMatch(coverageRowStaleNotice(row), term)", backups_table)
        self.assertIn("hasStaleTextMatch(artifactStaleNotice(artifact), term)", backups_table)
        self.assertIn("props.staleNotice && props.verdict.tone === 'protected' ? 'attention' : props.verdict.tone", strip)
        self.assertIn('<BackupStaleHeaderCue notice={props.staleNotice} attr="verdict" />', strip)
        self.assertIn('<BackupStaleHeaderCue notice={props.staleNotice} attr="jobs" />', jobs_card)
        self.assertIn('lead="Readings are stale."', coverage)
        self.assertIn('lead="Source telemetry is stale."', recoverable)
        self.assertIn("· Stale data", shared)

    def test_help_documents_workload_stale_telemetry_across_rows_filters_and_drawers(self):
        faq = doc_text("docs/FAQ.md")
        faq_section = norm(
            faq.split("### No data showing?\n", 1)[1].split("\n### ", 1)[0]
        )
        for phrase in (
            "**Workloads (VMs and containers)** across Proxmox, VMware vSphere, Docker, and Kubernetes",
            "without falsely flagging fresh hypervisor readings when only an optional linked Pulse agent goes quiet",
            "Stale running and stopped workloads leave **Running** and join **Attention**",
            "explicit **Offline**, **Error**, and **Failed** states stay ahead of staleness",
            "amber **Stale data** badge in the node group header (`Node telemetry is stale. Workload readings below reflect the last poll received.`)",
            "without dimming guests as **Node offline**",
            "inline amber cue (`Stale · <age>` or `Stale data`)",
            "workload search matches stale labels, details, and recorded source errors",
            "inline **Readings are stale** notice that clears automatically when live telemetry recovers",
        ):
            with self.subTest(doc="FAQ.md", phrase=phrase):
                self.assertIn(phrase, faq_section)

        unified = doc_text("docs/UNIFIED_RESOURCES.md")
        stale_section = norm(
            unified.split("### Stale workload telemetry\n", 1)[1].split("\n## ", 1)[0]
        )
        for phrase in (
            "primary hypervisor or container provider (`Proxmox`, `vCenter`, `Docker`, or `Kubernetes`)",
            "fresh hypervisor readings stay unflagged when only the linked agent goes silent",
            "Stale running and stopped workloads leave **Running** and route into **Attention**",
            "explicit `offline`, `error`, and `failed` resource states stay ahead of staleness",
            "`Running, not responding to probe · <stale label>`",
            "Only a confirmed offline parent node marks its guests **Node offline** and dims them",
            "`Node telemetry is stale. Workload readings below reflect the last poll received.`",
            "`Stale · <age>` when report age is available, or `Stale data`",
            "Workload search matches stale labels, stale detail text (`Showing the last values received.`), and recorded source error text",
            "inline **Readings are stale** warning notice with the provider stale detail",
        ):
            with self.subTest(doc="UNIFIED_RESOURCES.md", phrase=phrase):
                self.assertIn(phrase, stale_section)

        use_workloads = doc_text("frontend-modern/src/hooks/useWorkloads.ts")
        selectors = doc_text("frontend-modern/src/components/Workloads/workloadSelectors.ts")
        panel = doc_text("frontend-modern/src/components/Workloads/WorkloadPanel.tsx")
        node_header = doc_text("frontend-modern/src/components/shared/NodeGroupHeader.tsx")
        guest_row = doc_text("frontend-modern/src/components/Workloads/GuestRow.tsx")
        guest_drawer = doc_text("frontend-modern/src/components/Workloads/GuestDrawer.tsx")
        status_util = doc_text("frontend-modern/src/utils/status.ts")

        for marker in (
            "case 'proxmox-pve':",
            "label: 'Proxmox'",
            "case 'vmware-vsphere':",
            "label: 'vCenter'",
            "case 'docker':",
            "label: 'Docker'",
            "case 'kubernetes':",
            "label: 'Kubernetes'",
            "if (primarySource && CONNECTED_WORKLOAD_SOURCE_STATUS.test(primaryStatus)) {",
            "Showing the last values received.",
            "telemetryStale: resolveWorkloadTelemetryStale(resource, platformType)",
        ):
            self.assertIn(marker, use_workloads)
        for marker in (
            "guest.telemetryStale &&",
            "resourceStatus !== 'offline' &&",
            "resourceStatus !== 'error' &&",
            "resourceStatus !== 'failed'",
            "return 'degraded';",
            "guest.telemetryStale?.label,",
            "guest.telemetryStale?.detail,",
            "guest.telemetryStale?.error,",
        ):
            self.assertIn(marker, selectors)
        self.assertIn(
            "return pn ? getNodeStatusIndicator(pn).variant !== 'danger' : true;",
            panel,
        )
        for marker in (
            "data-node-group-stale",
            'title="Node telemetry is stale. Workload readings below reflect the last poll received."',
            "Stale data",
        ):
            self.assertIn(marker, node_header)
        for marker in (
            "data-workload-stale-cue",
            "{stale().age ? `Stale · ${stale().age}` : stale().label}",
        ):
            self.assertIn(marker, guest_row)
        for marker in (
            "getGuestHealthIndicator(props.guest, props.parentNodeOnline !== false)",
            'data-testid="guest-telemetry-stale-notice"',
            '<p class="font-medium">Readings are stale</p>',
        ):
            self.assertIn(marker, guest_drawer)
        for marker in (
            "`Running, not responding to probe · ${stale.label}`",
            "if (stale) {",
            "return { variant: 'warning', label: stale.detail || stale.label };",
        ):
            self.assertIn(marker, status_util)

    def test_recovery_series_window_and_rollup_scope_isolation_match_source(self):
        recovery = norm(doc_text("docs/RECOVERY.md"))
        types_go = (ROOT / "internal/recovery/model/types.go").read_text(encoding="utf-8")
        series_go = (ROOT / "internal/recovery/store/store_series.go").read_text(encoding="utf-8")
        handlers_go = (ROOT / "internal/api/recovery_handlers.go").read_text(encoding="utf-8")
        keys_go = (ROOT / "internal/recovery/keys.go").read_text(encoding="utf-8")

        self.assertIn("const MaxPointsSeriesDays = 3660", types_go)
        self.assertIn("1-recovery.MaxPointsSeriesDays", series_go)
        self.assertIn("1-recovery.MaxPointsSeriesDays", handlers_go)
        self.assertIn("func externalSubjectScope(p RecoveryPoint) string", keys_go)
        self.assertIn("case ProviderKubernetes, ProviderTrueNAS:", keys_go)

        for phrase in (
            "`/api/recovery/series` returns dense daily buckets bounded to at most **3,660** UTC days ending at `to`",
            "Unlinked Kubernetes PVC and TrueNAS dataset rollups include their reporting cluster or connection scope",
            "stay separate instead of sharing one rollup",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, recovery)

    def test_shipped_public_docs_mirrors_match_byte_for_byte(self):
        for name in (
            "AGENT_SECURITY.md",
            "FAQ.md",
            "KUBERNETES.md",
            "PBS.md",
            "RECOVERY.md",
            "TRUENAS.md",
            "UNIFIED_AGENT.md",
            "UNIFIED_RESOURCES.md",
        ):
            with self.subTest(doc=name):
                self.assertEqual(
                    (ROOT / "docs" / name).read_bytes(),
                    (ROOT / "frontend-modern/public/docs" / name).read_bytes(),
                )


if __name__ == "__main__":
    unittest.main()
