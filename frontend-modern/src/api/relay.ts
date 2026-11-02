import { apiFetchJSON, type APIContextOptions } from '@/utils/apiClient';

export interface RelayConfig {
  enabled: boolean;
  server_url: string;
  identity_fingerprint?: string;
  alert_minimum_severity?: 'all' | 'critical';
}

export interface RelayStatus {
  connected: boolean;
  instance_id?: string;
  active_channels: number;
  last_error?: string;
}

export class RelayAPI {
  private static baseUrl = '/api/settings/relay';

  static async getConfig(options?: APIContextOptions): Promise<RelayConfig> {
    return apiFetchJSON(this.baseUrl, options) as Promise<RelayConfig>;
  }

  static async updateConfig(
    update: Partial<Pick<RelayConfig, 'enabled' | 'server_url' | 'alert_minimum_severity'>>,
    options?: APIContextOptions,
  ): Promise<void> {
    await apiFetchJSON(this.baseUrl, {
      ...options,
      method: 'PUT',
      body: JSON.stringify(update),
    });
  }

  static async getStatus(): Promise<RelayStatus> {
    return apiFetchJSON(`${this.baseUrl}/status`) as Promise<RelayStatus>;
  }
}
