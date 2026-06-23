import type { SystemConfig } from '@/types/config';
import { apiFetchJSON } from '@/utils/apiClient';

export interface SystemSettingsResponse extends SystemConfig {
  envOverrides?: Record<string, boolean>;
}

export interface TelemetryPingPreview {
  install_id: string;
  version: string;
  version_raw?: string;
  version_channel: string;
  version_build?: string;
  version_is_development: boolean;
  version_is_published_release: boolean;
  platform: string;
  os: string;
  arch: string;
  event: string;
  pve_nodes: number;
  pbs_instances: number;
  pmg_instances: number;
  vms: number;
  containers: number;
  agent_hosts: number;
  docker_hosts: number;
  docker_containers: number;
  kubernetes_clusters: number;
  kubernetes_nodes: number;
  kubernetes_pods: number;
  kubernetes_deployments: number;
  storage_pools: number;
  physical_disks: number;
  ceph_clusters: number;
  network_shares: number;
  truenas_systems: number;
  truenas_vms: number;
  truenas_apps: number;
  vmware_hosts: number;
  vmware_vms: number;
  vmware_datastores: number;
  availability_targets: number;
  ai_enabled: boolean;
  patrol_enabled: boolean;
  discovery_enabled: boolean;
  notifications_enabled: boolean;
  ai_actions_enabled: boolean;
  active_alerts: number;
  relay_enabled: boolean;
  sso_enabled: boolean;
  multi_tenant: boolean;
  paid_license: boolean;
  has_api_tokens: boolean;
  pulse_intelligence_loop_configured: boolean;
  pulse_intelligence_loop_active_30d: boolean;
  pulse_intelligence_complete_operations_loop_30d: boolean;
  pulse_intelligence_approved_execution_loop_30d: boolean;
  pulse_intelligence_resolved_operations_loop_30d: boolean;
  pulse_intelligence_patrol_control_completed_operations_loop_30d: boolean;
  pulse_intelligence_patrol_control_resolved_operations_loop_30d: boolean;
  pulse_intelligence_patrol_control_paid_completed_operations_loop_30d: boolean;
  pulse_intelligence_patrol_control_paid_resolved_operations_loop_30d: boolean;
  pulse_intelligence_pro_activation_completed_operations_loop_30d: boolean;
  pulse_intelligence_pro_activation_resolved_operations_loop_30d: boolean;
  pulse_intelligence_pro_activation_paid_completed_operations_loop_30d: boolean;
  pulse_intelligence_pro_activation_paid_resolved_operations_loop_30d: boolean;
  pulse_intelligence_governed_action_active_30d: boolean;
  pulse_intelligence_assistant_operations_loop_30d: boolean;
  pulse_intelligence_assistant_approved_execution_loop_30d: boolean;
  pulse_intelligence_assistant_approved_action_success_loop_30d: boolean;
  pulse_intelligence_assistant_resolved_operations_loop_30d: boolean;
  pulse_intelligence_external_agent_operations_loop_30d: boolean;
  pulse_intelligence_external_agent_approved_execution_loop_30d: boolean;
  pulse_intelligence_external_agent_approved_action_success_loop_30d: boolean;
  pulse_intelligence_external_agent_resolved_operations_loop_30d: boolean;
  pulse_intelligence_mcp_adapter_operations_loop_30d: boolean;
  pulse_intelligence_mcp_adapter_approved_execution_loop_30d: boolean;
  pulse_intelligence_mcp_adapter_approved_action_success_loop_30d: boolean;
  pulse_intelligence_mcp_adapter_resolved_operations_loop_30d: boolean;
  pulse_intelligence_operations_loop_starter_requests_30d: number;
  pulse_intelligence_assistant_operations_loop_starter_requests_30d: number;
  pulse_intelligence_patrol_operations_loop_starter_requests_30d: number;
  pulse_intelligence_patrol_control_operations_loop_starter_requests_30d: number;
  pulse_intelligence_pro_activation_operations_loop_starter_requests_30d: number;
  pulse_intelligence_mcp_operations_loop_starter_requests_30d: number;
  pulse_intelligence_assistant_ai_calls_30d: number;
  pulse_intelligence_assistant_context_ai_calls_30d: number;
  pulse_intelligence_assistant_tool_calls_30d: number;
  pulse_intelligence_patrol_ai_calls_30d: number;
  pulse_intelligence_patrol_runs_30d: number;
  pulse_intelligence_patrol_new_findings_30d: number;
  pulse_intelligence_patrol_investigations_30d: number;
  pulse_intelligence_patrol_resolved_findings_30d: number;
  pulse_intelligence_patrol_autofixes_30d: number;
  pulse_intelligence_external_agent_enabled: boolean;
  pulse_intelligence_external_agent_used_30d: boolean;
  pulse_intelligence_mcp_adapter_used_30d: boolean;
  pulse_intelligence_external_agent_context_requests_30d: number;
  pulse_intelligence_external_agent_event_stream_requests_30d: number;
  pulse_intelligence_external_agent_provisioning_requests_30d: number;
  pulse_intelligence_external_agent_operator_state_requests_30d: number;
  pulse_intelligence_external_agent_finding_requests_30d: number;
  pulse_intelligence_external_agent_action_requests_30d: number;
  pulse_intelligence_action_plans_30d: number;
  pulse_intelligence_approval_requests_30d: number;
  pulse_intelligence_rejected_action_decisions_30d: number;
  pulse_intelligence_approved_action_decisions_30d: number;
  pulse_intelligence_approved_action_attempts_30d: number;
  pulse_intelligence_approved_action_successes_30d: number;
}

export interface TelemetryPreviewResponse {
  enabled: boolean;
  payload: TelemetryPingPreview;
}

export class SettingsAPI {
  private static baseUrl = '/api';

  // System settings update (preferred) - uses SystemConfig type from config.ts
  static async updateSystemSettings(settings: Partial<SystemConfig>): Promise<void> {
    await apiFetchJSON(`${this.baseUrl}/system/settings/update`, {
      method: 'POST',
      body: JSON.stringify(settings),
    });
  }

  // Get system settings - returns SystemConfig
  static async getSystemSettings(): Promise<SystemSettingsResponse> {
    return apiFetchJSON(`${this.baseUrl}/system/settings`) as Promise<SystemSettingsResponse>;
  }

  static async getTelemetryPreview(): Promise<TelemetryPreviewResponse> {
    return apiFetchJSON(
      `${this.baseUrl}/system/settings/telemetry-preview`,
    ) as Promise<TelemetryPreviewResponse>;
  }

  static async resetTelemetryInstallID(): Promise<TelemetryPreviewResponse> {
    return apiFetchJSON(`${this.baseUrl}/system/settings/telemetry-reset-id`, {
      method: 'POST',
    }) as Promise<TelemetryPreviewResponse>;
  }
}
