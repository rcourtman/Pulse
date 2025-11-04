import type { SystemConfig } from '@/types/config';
import { apiFetchJSON } from '@/utils/apiClient';

export interface SystemSettingsResponse extends SystemConfig {
  envOverrides?: Record<string, boolean>;
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
}
