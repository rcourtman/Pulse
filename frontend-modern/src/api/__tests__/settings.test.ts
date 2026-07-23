import { describe, expect, it, vi, beforeEach } from 'vitest';
import { SettingsAPI, type SystemSettingsResponse, type TelemetryPingPreview } from '../settings';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

const mockTelemetryPreviewPayload = {
  schema_version: 3,
  sent_at: '2026-07-23T08:30:00Z',
  install_id: 'preview-install-id',
  version: '6.0.0',
  version_channel: 'stable',
  version_is_development: false,
  version_is_published_release: true,
  platform: 'docker',
  os: 'linux',
  arch: 'amd64',
  event: 'heartbeat',
  deployment_method: 'docker_compose',
  known_install_age_bucket: '1_7d',
  activation_stage: 'monitoring',
  time_to_first_monitored_resource_bucket: 'under_15m',
  estate_size_bucket: '1_10',
  auth_configured: true,
  configured_connections: 1,
  monitoring_active: true,
  outcome_observed_30d: false,
  pve_nodes: 1,
  pbs_instances: 0,
  pmg_instances: 0,
  vms: 2,
  containers: 3,
  agent_hosts: 1,
  docker_hosts: 0,
  docker_containers: 0,
  kubernetes_clusters: 0,
  kubernetes_nodes: 0,
  kubernetes_pods: 0,
  kubernetes_deployments: 0,
  storage_pools: 1,
  physical_disks: 2,
  ceph_clusters: 0,
  network_shares: 0,
  truenas_systems: 0,
  truenas_vms: 0,
  truenas_apps: 0,
  vmware_hosts: 0,
  vmware_vms: 0,
  vmware_datastores: 0,
  availability_targets: 1,
  ai_enabled: false,
  patrol_enabled: false,
  discovery_enabled: false,
  notifications_enabled: false,
  ai_actions_enabled: false,
  active_alerts: 0,
  relay_enabled: false,
  sso_enabled: false,
  multi_tenant: false,
  paid_license: false,
  has_api_tokens: true,
  update_attempts_30d: 0,
  update_successes_30d: 0,
  update_failures_30d: 0,
  update_last_failure_category: undefined,
  alerts_fired_30d: 0,
  alerts_acknowledged_30d: 0,
  alerts_resolved_30d: 0,
  notification_attempts_7d: 0,
  notification_deliveries_7d: 0,
  notification_failures_7d: 0,
  pulse_intelligence_loop_configured: false,
  pulse_intelligence_loop_active_30d: false,
  pulse_intelligence_complete_operations_loop_30d: false,
  pulse_intelligence_approved_execution_loop_30d: false,
  pulse_intelligence_resolved_operations_loop_30d: false,
  pulse_intelligence_patrol_control_completed_operations_loop_30d: false,
  pulse_intelligence_patrol_control_resolved_operations_loop_30d: false,
  pulse_intelligence_patrol_control_paid_completed_operations_loop_30d: false,
  pulse_intelligence_patrol_control_paid_resolved_operations_loop_30d: false,
  pulse_intelligence_pro_activation_completed_operations_loop_30d: false,
  pulse_intelligence_pro_activation_resolved_operations_loop_30d: false,
  pulse_intelligence_pro_activation_paid_completed_operations_loop_30d: false,
  pulse_intelligence_pro_activation_paid_resolved_operations_loop_30d: false,
  pulse_intelligence_governed_action_active_30d: false,
  pulse_intelligence_assistant_operations_loop_30d: false,
  pulse_intelligence_assistant_approved_execution_loop_30d: false,
  pulse_intelligence_assistant_approved_action_success_loop_30d: false,
  pulse_intelligence_assistant_resolved_operations_loop_30d: false,
  pulse_intelligence_external_agent_operations_loop_30d: false,
  pulse_intelligence_external_agent_approved_execution_loop_30d: false,
  pulse_intelligence_external_agent_approved_action_success_loop_30d: false,
  pulse_intelligence_external_agent_resolved_operations_loop_30d: false,
  pulse_intelligence_mcp_adapter_operations_loop_30d: false,
  pulse_intelligence_mcp_adapter_approved_execution_loop_30d: false,
  pulse_intelligence_mcp_adapter_approved_action_success_loop_30d: false,
  pulse_intelligence_mcp_adapter_resolved_operations_loop_30d: false,
  pulse_intelligence_operations_loop_starter_requests_30d: 0,
  pulse_intelligence_assistant_operations_loop_starter_requests_30d: 0,
  pulse_intelligence_patrol_operations_loop_starter_requests_30d: 0,
  pulse_intelligence_patrol_control_operations_loop_starter_requests_30d: 0,
  pulse_intelligence_pro_activation_operations_loop_starter_requests_30d: 0,
  pulse_intelligence_mcp_operations_loop_starter_requests_30d: 0,
  pulse_intelligence_assistant_ai_calls_30d: 0,
  pulse_intelligence_assistant_context_ai_calls_30d: 0,
  pulse_intelligence_assistant_tool_calls_30d: 0,
  pulse_intelligence_patrol_ai_calls_30d: 0,
  pulse_intelligence_patrol_runs_30d: 0,
  pulse_intelligence_patrol_new_findings_30d: 0,
  pulse_intelligence_patrol_investigations_30d: 0,
  pulse_intelligence_patrol_resolved_findings_30d: 0,
  pulse_intelligence_patrol_autofixes_30d: 0,
  pulse_intelligence_external_agent_enabled: false,
  pulse_intelligence_external_agent_used_30d: false,
  pulse_intelligence_mcp_adapter_used_30d: false,
  pulse_intelligence_external_agent_context_requests_30d: 0,
  pulse_intelligence_external_agent_event_stream_requests_30d: 0,
  pulse_intelligence_external_agent_provisioning_requests_30d: 0,
  pulse_intelligence_external_agent_operator_state_requests_30d: 0,
  pulse_intelligence_external_agent_finding_requests_30d: 0,
  pulse_intelligence_external_agent_action_requests_30d: 0,
  pulse_intelligence_action_plans_30d: 0,
  pulse_intelligence_approval_requests_30d: 0,
  pulse_intelligence_rejected_action_decisions_30d: 0,
  pulse_intelligence_approved_action_decisions_30d: 0,
  pulse_intelligence_approved_action_attempts_30d: 0,
  pulse_intelligence_approved_action_successes_30d: 0,
  pulse_intelligence_approved_action_failures_pre_dispatch_30d: 0,
  pulse_intelligence_approved_action_failures_execution_30d: 0,
  pulse_intelligence_approved_action_failures_unverified_30d: 0,
  pulse_intelligence_approved_action_stuck_executing_30d: 0,
  pulse_intelligence_approved_action_last_failure_reason_30d: undefined,
} satisfies TelemetryPingPreview;

describe('SettingsAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('updateSystemSettings', () => {
    it('updates system settings', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await SettingsAPI.updateSystemSettings({ fullWidthMode: true });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/system/settings/update',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ fullWidthMode: true }),
        }),
      );
    });
  });

  describe('getSystemSettings', () => {
    it('fetches system settings', async () => {
      const mockSettings: SystemSettingsResponse = {
        autoUpdateEnabled: true,
        fullWidthMode: true,
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockSettings);

      const result = await SettingsAPI.getSystemSettings();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/system/settings');
      expect(result).toEqual(mockSettings);
    });
  });

  describe('getTelemetryPreview', () => {
    it('fetches the telemetry preview payload', async () => {
      const mockPreview = {
        enabled: true,
        payload: mockTelemetryPreviewPayload,
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockPreview);

      const result = await SettingsAPI.getTelemetryPreview();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/system/settings/telemetry-preview');
      expect(result).toEqual(mockPreview);
    });
  });

  describe('resetTelemetryInstallID', () => {
    it('posts the telemetry install-id reset action', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({
        enabled: true,
        payload: {
          install_id: 'rotated-install-id',
        },
      });

      await SettingsAPI.resetTelemetryInstallID();

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/system/settings/telemetry-reset-id',
        expect.objectContaining({
          method: 'POST',
        }),
      );
    });
  });
});
