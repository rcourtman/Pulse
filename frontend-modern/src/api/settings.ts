import type { 
  SettingsResponse, 
  SettingsUpdateRequest
} from '@/types/settings';
import type { SystemConfig } from '@/types/config';
import { apiFetchJSON } from '@/utils/apiClient';

// Response types
export interface ApiResponse<T = unknown> {
  success?: boolean;
  status?: string;
  message?: string;
  data?: T;
}

export class SettingsAPI {
  private static baseUrl = '/api';

  static async getSettings(): Promise<SettingsResponse> {
    return apiFetchJSON(`${this.baseUrl}/settings`) as Promise<SettingsResponse>;
  }

  // Full settings update (legacy - avoid using)
  static async updateSettings(settings: SettingsUpdateRequest): Promise<ApiResponse> {
    return apiFetchJSON(`${this.baseUrl}/settings/update`, {
      method: 'POST',
      body: JSON.stringify(settings),
    }) as Promise<ApiResponse>;
  }
  
  // System settings update (preferred) - uses SystemConfig type from config.ts
  static async updateSystemSettings(settings: Partial<SystemConfig>): Promise<ApiResponse> {
    return apiFetchJSON(`${this.baseUrl}/system/settings/update`, {
      method: 'POST',
      body: JSON.stringify(settings),
    }) as Promise<ApiResponse>;
  }
  
  // Get system settings - returns SystemConfig
  static async getSystemSettings(): Promise<SystemConfig> {
    return apiFetchJSON(`${this.baseUrl}/system/settings`) as Promise<SystemConfig>;
  }

  static async validateSettings(settings: SettingsUpdateRequest): Promise<ApiResponse> {
    return apiFetchJSON(`${this.baseUrl}/settings/validate`, {
      method: 'POST',
      body: JSON.stringify(settings),
    }) as Promise<ApiResponse>;
  }
}