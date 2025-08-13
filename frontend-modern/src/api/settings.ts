import type { 
  SettingsResponse, 
  SettingsUpdateRequest
} from '@/types/settings';
import { apiFetchJSON } from '@/utils/apiClient';

// System settings type matching Go backend
export interface SystemSettingsUpdate {
  pollingInterval: number; // in seconds
  backendPort?: number;
  frontendPort?: number;
  allowedOrigins?: string;
  connectionTimeout?: number; // in seconds
  updateChannel?: string;
  autoUpdateEnabled?: boolean;
  autoUpdateCheckInterval?: number; // in hours
  autoUpdateTime?: string; // HH:MM format
}

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
  
  // System settings update (preferred)
  static async updateSystemSettings(settings: SystemSettingsUpdate): Promise<ApiResponse> {
    return apiFetchJSON(`${this.baseUrl}/config/system`, {
      method: 'PUT',
      body: JSON.stringify(settings),
    }) as Promise<ApiResponse>;
  }

  static async validateSettings(settings: SettingsUpdateRequest): Promise<ApiResponse> {
    return apiFetchJSON(`${this.baseUrl}/settings/validate`, {
      method: 'POST',
      body: JSON.stringify(settings),
    }) as Promise<ApiResponse>;
  }
}