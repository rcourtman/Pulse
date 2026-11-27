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

    def test_shipped_public_docs_mirrors_match_byte_for_byte(self):
        for name in ("FAQ.md", "KUBERNETES.md", "UNIFIED_AGENT.md"):
            with self.subTest(doc=name):
                self.assertEqual(
                    (ROOT / "docs" / name).read_bytes(),
                    (ROOT / "frontend-modern/public/docs" / name).read_bytes(),
                )


if __name__ == "__main__":
    unittest.main()
