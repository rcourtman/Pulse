#!/usr/bin/env python3
"""Verify the shipped Alerting outage-detection Help matches live runtime signals.

Binds in-app ALERTING.md to the actual Prometheus metrics, agent report routes,
History source labels, and shipped Help headings without touching a live server.
"""

from __future__ import annotations

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/ALERTING.md"
SHIPPED = ROOT / "frontend-modern/public/docs/ALERTING.md"


def outage_section() -> str:
    text = DOC.read_text(encoding="utf-8")
    marker = "## Detect an outage of Pulse itself\n"
    if marker not in text:
        raise AssertionError("missing outage-detection section in docs/ALERTING.md")
    return text.split(marker, 1)[1]


def heading_slug(title: str) -> str:
    slug = title.strip().lower()
    slug = re.sub(r"[^\w\s-]", "", slug)
    return re.sub(r"\s+", "-", slug).strip("-")


class AlertingOutageHelpDocsTest(unittest.TestCase):
    def test_shipped_copy_matches_source(self):
        self.assertEqual(DOC.read_bytes(), SHIPPED.read_bytes())

    def test_distinguishes_device_history_machines_and_docker_reports(self):
        section = " ".join(outage_section().split())
        history_model = (
            ROOT / "frontend-modern/src/components/shared/historyChartModel.ts"
        ).read_text(encoding="utf-8")
        machine_model = (
            ROOT / "frontend-modern/src/features/standalone/agentMachineTableModel.ts"
        ).read_text(encoding="utf-8")

        for label in ("Latest from device", "Latest recorded"):
            self.assertIn(label, history_model)
            self.assertIn(f"**{label}**", section)
        self.assertIn("label: 'Last seen'", machine_model)
        self.assertIn("**Last seen**", section)
        self.assertIn("a new chart point alone does not prove Pulse polled again", section)
        self.assertIn("Check **Docker** hosts separately", section)
        self.assertIn("machine reports keep arriving from the same agent", section)

    def test_prometheus_scheduler_and_agent_signals_match_runtime_source(self):
        section = " ".join(outage_section().split())
        monitoring_metrics = (ROOT / "internal/monitoring/metrics.go").read_text(encoding="utf-8")
        queue_metrics = (ROOT / "internal/monitoring/poll_queue_metrics.go").read_text(encoding="utf-8")
        dead_letter_metrics = (ROOT / "internal/monitoring/poll_dead_letter_metrics.go").read_text(encoding="utf-8")
        work_progress = (ROOT / "internal/monitoring/poll_work_progress.go").read_text(encoding="utf-8")
        http_metrics = (ROOT / "internal/api/http_metrics.go").read_text(encoding="utf-8")
        routes = (ROOT / "internal/api/router_routes_registration.go").read_text(encoding="utf-8")

        for metric, source in (
            ("pulse_monitor_poll_total", monitoring_metrics),
            ("pulse_monitor_poll_last_success_timestamp", monitoring_metrics),
            ("pulse_scheduler_breaker_retry_seconds", monitoring_metrics),
            ("pulse_scheduler_queued_poll_overdue_seconds", queue_metrics),
            ("pulse_scheduler_claimed_poll_overdue_seconds", work_progress),
            ("pulse_scheduler_dead_letter_depth", dead_letter_metrics),
            ("pulse_http_requests_total", http_metrics),
        ):
            with self.subTest(metric=metric):
                self.assertIn(metric, section)
                needle = (
                    metric.removeprefix("pulse_monitor_")
                    if metric.startswith("pulse_monitor_")
                    else metric.removeprefix("pulse_http_")
                    if metric.startswith("pulse_http_")
                    else metric
                )
                self.assertIn(f'"{needle}"', source)

        for stage in ("waiting_for_slot", "running"):
            self.assertIn(f'"{stage}"', work_progress)
            self.assertIn(f'`stage="{stage}"`', section)

        for route in (
            "/api/agents/agent/report",
            "/api/agents/host/report",
            "/api/agents/docker/report",
        ):
            self.assertIn(f'"{route}"', routes)
            self.assertIn(f"`{route}`", section)

        for boundary in (
            "an absent series on an older build is not zero",
            "not that a poll ran",
            "Zero does not prove polling recovered",
            "Do not add the routes together",
            "without restarting Pulse or sending test traffic",
            "Neither alert above checks whether Pulse evaluated its alert rules",
        ):
            self.assertIn(boundary, section)

    def test_in_app_links_target_real_shipped_docs_and_headings(self):
        section = outage_section()
        links = re.findall(r"\[[^\]]+\]\(([^)]+)\)", section)
        self.assertGreaterEqual(len(links), 5)
        for target in links:
            self.assertFalse(target.startswith("http"), target)
            if target.startswith("#"):
                doc_path = SHIPPED
                fragment = target[1:]
            else:
                file_part, _, fragment = target.partition("#")
                doc_path = ROOT / "frontend-modern/public/docs" / file_part
            self.assertTrue(doc_path.is_file(), f"missing shipped doc for link {target}")
            if fragment:
                headings = re.findall(r"^#{1,6}\s+(.+)$", doc_path.read_text(encoding="utf-8"), re.M)
                slugs = {heading_slug(h) for h in headings}
                self.assertIn(fragment, slugs, f"missing anchor #{fragment} in {doc_path.name}")

    def test_recovery_severity_continuity_help_matches_dispatch_receipts(self):
        text = DOC.read_text(encoding="utf-8")
        section = " ".join(
            text.split("## Confirm recovery separately\n", 1)[1]
            .split("\n## ", 1)[0]
            .split()
        )
        dispatch = (
            ROOT / "internal/notifications/notification_dispatch.go"
        ).read_text(encoding="utf-8")
        self.assertIn("filterResolvedJobsByDeliveryReceipt(", dispatch)
        self.assertIn("notifiedLevel", dispatch)
        self.assertIn("cloned.Level = notifiedLevel", dispatch)
        for phrase in (
            "keeps the severity level from the firing notification that destination actually received",
            "stepped down from critical to warning before clearing",
            "match on the alert `ID` rather than relying on severity alone",
        ):
            self.assertIn(phrase, section)

    def test_active_alert_reevaluation_help_matches_runtime_contract(self):
        alerting_section = " ".join(outage_section().split())
        api_doc = (ROOT / "docs/API.md").read_text(encoding="utf-8")
        shipped_api_doc = (ROOT / "frontend-modern/public/docs/API.md").read_text(encoding="utf-8")
        self.assertEqual(api_doc, shipped_api_doc)

        marker = "### Active alerts\n"
        self.assertIn(marker, api_doc)
        active_alerts_section = " ".join(
            api_doc.split(marker, 1)[1].split("\n---", 1)[0].split()
        )

        alerts_handler = (ROOT / "internal/api/alerting/alerts.go").read_text(encoding="utf-8")
        alert_model = (ROOT / "internal/alerts/model.go").read_text(encoding="utf-8")
        models_go = (ROOT / "internal/models/models.go").read_text(encoding="utf-8")
        persistence = (ROOT / "internal/alerts/active_persistence.go").read_text(encoding="utf-8")

        self.assertIn('case path == "active" && r.Method == http.MethodGet:', alerts_handler)
        self.assertIn("config.ScopeMonitoringRead", alerts_handler)
        self.assertIn("h.GetActiveAlerts(w, r)", alerts_handler)
        self.assertIn("clone.MetricStatus = nil", persistence)

        for field, source in (
            ("id", alert_model),
            ("startTime", alert_model),
            ("lastSeen", alert_model),
            ("metricStatus", alert_model),
            ("phase", models_go),
            ("value", models_go),
            ("observedAt", models_go),
            ("trigger", models_go),
            ("recovery", models_go),
            ("lastBreachAt", models_go),
        ):
            with self.subTest(field=field):
                self.assertRegex(
                    source, rf'json:"{re.escape(field)}(?:,(?:omitempty|omitzero))?"'
                )
                self.assertIn(field, active_alerts_section)

        for phase in ("breaching", "latched", "recovering"):
            with self.subTest(phase=phase):
                self.assertIn(f'"{phase}"', models_go)
                self.assertIn(f"`{phase}`", active_alerts_section)
                self.assertIn(f"`{phase}`", alerting_section)

        for phrase in (
            "GET /api/alerts/active",
            "monitoring:read",
            "both `id` and `startTime`",
            "metricStatus.observedAt",
            "metricStatus.lastBreachAt",
            "omitted from durable checkpoints",
            "do not lower thresholds or force a test alert",
        ):
            with self.subTest(alerting_phrase=phrase):
                self.assertIn(phrase, alerting_section)

        for phrase in (
            "GET /api/alerts/active",
            "monitoring:read",
            "stripped from durable alert snapshots",
            "never in a command line or URL",
            "ALERTING.md#check-whether-an-open-threshold-alert-was-reevaluated",
        ):
            with self.subTest(api_phrase=phrase):
                self.assertIn(phrase, active_alerts_section)

    def test_builtin_ceph_health_alert_help_matches_runtime_contract(self):
        text = DOC.read_text(encoding="utf-8")
        marker = "### Built-in provider health alerts\n"
        self.assertIn(marker, text)
        section = " ".join(
            text.split(marker, 1)[1].split("\n## ", 1)[0].split()
        )

        adapters = (
            ROOT / "internal/unifiedresources/adapters.go"
        ).read_text(encoding="utf-8")
        ceph_monitor = (
            ROOT / "internal/monitoring/ceph.go"
        ).read_text(encoding="utf-8")
        unified_incidents = (
            ROOT / "internal/alerts/unified_incidents.go"
        ).read_text(encoding="utf-8")

        self.assertIn("func cephHealthIncident(", adapters)
        self.assertIn('Code:                          "ceph_cluster_health"', adapters)
        self.assertIn("ConfirmationsRequired:         2", adapters)
        self.assertIn("RecoveryConfirmationsRequired: 2", adapters)
        self.assertIn('fmt.Sprintf("%s: %s", checkName, msg)', ceph_monitor)
        self.assertIn("case unifiedresources.ResourceTypeCeph", unified_incidents)

        for phrase in (
            "Alerts → Thresholds",
            "`HEALTH_WARN` opens a warning alert",
            "`HEALTH_ERR` opens a critical alert",
            "two consecutive degraded observations",
            "two consecutive `HEALTH_OK` observations",
            "active Ceph health check codes and summaries",
            "carry no resource tags",
            "Only Ceph pool capacity thresholds are configured under **Alerts → Thresholds**",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, section)

    def test_prometheus_freshness_and_stopped_scheduler_exclusion_match_source(self):
        section = " ".join(outage_section().split())
        monitoring_metrics = (ROOT / "internal/monitoring/metrics.go").read_text(encoding="utf-8")
        queue_metrics = (ROOT / "internal/monitoring/poll_queue_metrics.go").read_text(
            encoding="utf-8"
        )

        for token in ("func rememberPollSuccess[", "func emitPollFreshness(", "completedAt.After("):
            self.assertIn(token, monitoring_metrics)
        for token in ("func (r pollQueueRuntime) active() bool", "if !runtime.active() {"):
            self.assertIn(token, queue_metrics)

        for phrase in (
            "from one snapshot of the latest successful completion",
            "delayed older result still counts in `pulse_monitor_poll_total` without moving that completion backwards",
            "A stopped scheduler drops out of `pulse_scheduler_queued_poll_overdue_seconds` even if a blocked worker has not returned yet",
            "keep checking `pulse_scheduler_claimed_poll_overdue_seconds` for that worker",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, section)

    def test_non_retryable_tls_and_skip_verify_boundary_match_source(self):
        text = " ".join(DOC.read_text(encoding="utf-8").split())
        failure_class = (ROOT / "internal/notifications/failure_class.go").read_text(
            encoding="utf-8"
        )
        notifications_api = (ROOT / "internal/api/alerting/notifications.go").read_text(
            encoding="utf-8"
        )

        retryable_body = failure_class.split(
            "func (class NotificationFailureClass) Retryable() bool", 1
        )[1]
        self.assertIn("NotificationFailureTLS:", retryable_body)
        self.assertIn("return false", retryable_body)
        self.assertIn(
            'summary = "TLS certificate error: check the server certificate and TLS settings"',
            notifications_api,
        )

        for phrase in (
            "TLS certificate trust, expiry, hostname, and HTTP-on-HTTPS errors stop immediately as terminal failures across email, webhook, and Apprise destinations",
            "instead of retrying across the delivery queue or SMTP retry budget",
            "Fix the server certificate or TLS settings rather than disabling verification",
            "Email and webhook destinations do not provide a Skip TLS Verify toggle",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)

    def test_email_branding_help_matches_runtime_and_ui_source(self):
        text = " ".join(DOC.read_text(encoding="utf-8").split())
        email_template = (
            ROOT / "internal/notifications/email_template.go"
        ).read_text(encoding="utf-8")
        continuity = (
            ROOT / "internal/notifications/delivery_continuity.go"
        ).read_text(encoding="utf-8")
        presentation = (
            ROOT / "frontend-modern/src/utils/alertEmailPresentation.ts"
        ).read_text(encoding="utf-8")

        for token in (
            'DefaultEmailSubjectPrefix = "[Pulse Alert]"',
            'DefaultEmailBrandName = "Pulse Monitoring"',
            "MaxEmailSubjectPrefixRunes = 64",
            "MaxEmailBrandNameRunes     = 80",
            "MaxEmailFooterNoteRunes    = 240",
            "func NormalizeEmailBranding(",
            "html.EscapeString(branding.FooterNote)",
        ):
            self.assertIn(token, email_template)
        for token in ('c.SubjectPrefix = ""', 'c.BrandName = ""', 'c.FooterNote = ""'):
            self.assertIn(token, continuity)
        for token in (
            "Show advanced options",
            "Email branding & subject",
            "Subject prefix",
            "Brand name",
            "Footer note",
            "Send test email",
        ):
            self.assertIn(token, presentation)

        for phrase in (
            "expand **Show advanced options** in the email settings form to open **Email branding & subject**",
            "**Subject prefix** (up to 64 characters, default `[Pulse Alert]`)",
            "**Brand name** (up to 80 characters, default `Pulse Monitoring`)",
            "plain-text **Footer note** (up to 240 characters)",
            "HTML-escapes those fields across single, grouped, resolved, and **Send test email** messages",
            "Raw HTML and custom email template files are not supported",
            "does not change the email destination continuity fingerprint",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)

    def test_saved_notification_credentials_and_auto_replay_help_match_source(self):
        text = " ".join(DOC.read_text(encoding="utf-8").split())
        saved_dest = (
            ROOT / "internal/notifications/saved_credential_destination.go"
        ).read_text(encoding="utf-8")
        notifications_api = (
            ROOT / "internal/api/alerting/notifications.go"
        ).read_text(encoding="utf-8")
        email_enhanced = (
            ROOT / "internal/notifications/email_enhanced.go"
        ).read_text(encoding="utf-8")
        queue_go = (ROOT / "internal/notifications/queue.go").read_text(encoding="utf-8")

        for token in ("func EmailConfigKeepsSMTPServer(", "func AppriseConfigKeepsServer("):
            self.assertIn(token, saved_dest)
        for token in (
            "smtpServerChangeNeedsPasswordMessage",
            "appriseServerChangeNeedsAPIKeyMessage",
        ):
            self.assertIn(token, notifications_api)
        self.assertIn("configure an SMTP server in email notification settings", email_enhanced)
        for token in (
            "autoReplayDLQWindow                           = 24 * time.Hour",
            "autoReplayDLQBatchLimit                       = 25",
            "autoReplayDLQCooldown                         = 5 * time.Minute",
        ):
            self.assertIn(token, queue_go)

        for phrase in (
            "Saved SMTP passwords and Apprise API keys are write-only and stay preserved only while editing the same destination",
            "Changing the SMTP server host or port, or turning off TLS or STARTTLS encryption on an encrypted SMTP configuration, requires entering the SMTP password again",
            "Changing the Apprise server origin or turning off its certificate check requires entering the Apprise API key again",
            "blank SMTP server host is refused as a configuration error instead of dialing loopback",
            "automatically re-queues up to 25 recent (24-hour) retryable retained failures for that destination once, subject to a five-minute cooldown per destination",
            "Authentication, configuration, rejected, and TLS failures, edited destinations, rehearsals, and deliveries older than 24 hours are not automatically re-queued, and a successful **Test** send does not trigger this recovery",
        ):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)

    def test_platform_alert_table_staleness_and_metrics_data_flow_match_source(self):
        section = " ".join(outage_section().split())
        platform_table = (
            ROOT / "frontend-modern/src/features/platformPage/PlatformAlertsTable.tsx"
        ).read_text(encoding="utf-8")
        staleness = (
            ROOT / "frontend-modern/src/features/platformPage/resourceStaleness.ts"
        ).read_text(encoding="utf-8")
        docker_alerts = (
            ROOT / "frontend-modern/src/features/docker/DockerAlertsTable.tsx"
        ).read_text(encoding="utf-8")
        k8s_alerts = (
            ROOT / "frontend-modern/src/features/kubernetes/KubernetesAlertsTable.tsx"
        ).read_text(encoding="utf-8")
        truenas_alerts = (
            ROOT / "frontend-modern/src/features/truenas/TrueNASAlertsTable.tsx"
        ).read_text(encoding="utf-8")
        vsphere_alerts = (
            ROOT / "frontend-modern/src/features/vmware/VsphereAlertsTable.tsx"
        ).read_text(encoding="utf-8")

        self.assertIn("getPlatformResourceStaleNotice(", platform_table)
        self.assertIn("reflects the last report received and may no longer be", platform_table)
        self.assertIn("label: 'Stale data'", staleness)
        self.assertIn("Last error:", staleness)
        self.assertIn('shellTitle="Active Alerts"', docker_alerts)
        self.assertIn('shellTitle="Active Alerts"', k8s_alerts)
        self.assertIn('shellTitle="Health Alerts"', truenas_alerts)
        self.assertIn('shellTitle="Health Signals"', vsphere_alerts)

        for phrase in (
            "**Platform alert tables:**",
            "Docker (**Active Alerts**)",
            "Kubernetes (**Active Alerts**)",
            "TrueNAS (**Health Alerts**)",
            "VMware (**Health Signals**)",
            "amber **Stale data** cue beneath the resource name",
            "keeping their **Critical**, **Warning**, or **Info** severity indicator visible",
            "reflects the last report received and may no longer be current",
            "clears automatically once live reporting resumes",
            "acknowledging an alert does not restore collection",
        ):
            with self.subTest(alerting_phrase=phrase):
                self.assertIn(phrase, section)

        flow_doc = " ".join(
            (ROOT / "docs/monitoring/METRICS_DATA_FLOW.md")
            .read_text(encoding="utf-8")
            .split()
        )
        for phrase in (
            "**Bars** and live status rows across platform tables",
            "Proxmox (including Ceph and replication)",
            "Docker (including images, storage, networks, Compose, Swarm, and Active Alerts)",
            "TrueNAS (including protection and Health Alerts)",
            "Kubernetes (including inventory, record drawers, and Active Alerts)",
            "VMware (including Health Signals and Activity)",
            "Physical Disks (Storage tables, **Physical Disks & SMART** cards, linked pool and guest physical disks, and disk drawers)",
            "amber **Stale data** cue (or amber **Stale** verdict for clear stale disks) and expanded-drawer notice while preserving worse Offline, Degraded, Replace Now, Needs Attention, or alert severity states",
            "Stale physical disk temperatures and SSD life stay muted as last known without triggering live **Running Hot**, and temperature or SMART history charts defer until live reporting recovers.",
        ):
            with self.subTest(flow_phrase=phrase):
                self.assertIn(phrase, flow_doc)

    def test_vm_disk_and_zfs_help_align_with_physical_disk_stale_reading_cues(
        self,
    ) -> None:
        guest_disks_src = (
            ROOT / "frontend-modern/src/components/Workloads/GuestPhysicalDisks.tsx"
        ).read_text(encoding="utf-8")
        pool_detail_src = (
            ROOT
            / "frontend-modern/src/features/storageBackups/storagePoolDetailPresentation.ts"
        ).read_text(encoding="utf-8")
        disk_drawer_src = (
            ROOT / "frontend-modern/src/components/Storage/DiskDrawerOverview.tsx"
        ).read_text(encoding="utf-8")
        disk_pres_src = (
            ROOT / "frontend-modern/src/features/storageBackups/diskPresentation.ts"
        ).read_text(encoding="utf-8")

        self.assertIn("data-disk-stale-cue", guest_disks_src)
        self.assertIn("getPhysicalDiskStaleDetail", guest_disks_src)
        self.assertIn("extractPhysicalDiskFreshness", pool_detail_src)
        self.assertIn("getPhysicalDiskStaleDetail", pool_detail_src)
        self.assertIn('data-testid="disk-drawer-stale-notice"', disk_drawer_src)
        self.assertIn("label: 'Stale'", disk_pres_src)
        self.assertIn("Replace Now", disk_pres_src)
        self.assertIn("Needs Attention", disk_pres_src)

        vm_doc = (ROOT / "docs/VM_DISK_MONITORING.md").read_text(encoding="utf-8")
        vm_mirror = (
            ROOT / "frontend-modern/public/docs/VM_DISK_MONITORING.md"
        ).read_text(encoding="utf-8")
        self.assertEqual(vm_doc, vm_mirror)
        vm_norm = " ".join(vm_doc.split())
        for phrase in (
            "Linked physical disks shown in a guest drawer follow the same freshness rules when their host agent or Proxmox source stops updating.",
            "Clear stale disks switch from **Healthy** to an amber **Stale** verdict, while **Replace Now** and **Needs Attention** disks keep their fault verdict beside an inline amber **Stale data** cue.",
            "Retained temperatures stay muted as last-known values without triggering live **Running Hot**, and opening the disk drawer displays an inline stale-readings warning and defers temperature and SMART history charts until live reporting recovers.",
        ):
            with self.subTest(doc="VM_DISK_MONITORING.md", phrase=phrase):
                self.assertIn(phrase, vm_norm)

        zfs_doc = (ROOT / "docs/ZFS_MONITORING.md").read_text(encoding="utf-8")
        zfs_mirror = (
            ROOT / "frontend-modern/public/docs/ZFS_MONITORING.md"
        ).read_text(encoding="utf-8")
        self.assertEqual(zfs_doc, zfs_mirror)
        zfs_norm = " ".join(zfs_doc.split())
        for phrase in (
            "When a linked physical disk's host agent or Proxmox source goes stale, pool member details keep its retained temperature muted as a last-known reading without triggering live **Running Hot**, and clear the stale cue automatically once live reporting resumes.",
        ):
            with self.subTest(doc="ZFS_MONITORING.md", phrase=phrase):
                self.assertIn(phrase, zfs_norm)


if __name__ == "__main__":
    unittest.main()
