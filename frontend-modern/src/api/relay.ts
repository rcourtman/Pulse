import { apiFetchJSON } from '@/utils/apiClient';

export interface RelayConfig {
  enabled: boolean;
  server_url: string;
  identity_fingerprint?: string;
}

export interface RelayStatus {
  connected: boolean;
  instance_id?: string;
  active_channels: number;
  last_error?: string;
}

export class RelayAPI {
  private static baseUrl = '/api/settings/relay';

  static async getConfig(): Promise<RelayConfig> {
    return apiFetchJSON(this.baseUrl) as Promise<RelayConfig>;
  }

  static async updateConfig(update: Partial<Pick<RelayConfig, 'enabled' | 'server_url'>>): Promise<void> {
    await apiFetchJSON(this.baseUrl, {
      method: 'PUT',
      body: JSON.stringify(update),
    });
  }

  static async getStatus(): Promise<RelayStatus> {
    return apiFetchJSON(`${this.baseUrl}/status`) as Promise<RelayStatus>;
  }
}
