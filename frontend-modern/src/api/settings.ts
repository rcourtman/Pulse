import type { SystemConfig } from '@/types/config';
import { apiFetchJSON } from '@/utils/apiClient';

export interface SystemSettingsResponse extends SystemConfig {
  envOverrides?: Record<string, boolean>;
}

export interface TelemetryPingPreview {
  install_id: string;
  version: string;
  platform: string;
  os: string;
  arch: string;
  event: string;
  pve_nodes: number;
  pbs_instances: number;
  pmg_instances: number;
  vms: number;
  containers: number;
  docker_hosts: number;
  kubernetes_clusters: number;
  ai_enabled: boolean;
  active_alerts: number;
  relay_enabled: boolean;
  sso_enabled: boolean;
  multi_tenant: boolean;
  paid_license: boolean;
  has_api_tokens: boolean;
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
