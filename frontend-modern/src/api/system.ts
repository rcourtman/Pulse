// System API for managing system settings including API tokens
import { apiFetchJSON } from '@/utils/apiClient';

export interface APITokenStatus {
  hasToken: boolean;
  token?: string;
}

export interface SystemSettings {
  pollingInterval: number;
  updateChannel?: string;
  autoUpdateEnabled: boolean;
  autoUpdateCheckInterval?: number;
  autoUpdateTime?: string;
  apiToken?: string;
}

export class SystemAPI {
  // API Token Management
  static async getAPITokenStatus(): Promise<APITokenStatus> {
    return apiFetchJSON('/api/system/api-token');
  }
  
  static async getAPIToken(reveal: boolean = false): Promise<APITokenStatus> {
    const url = reveal ? '/api/system/api-token?reveal=true' : '/api/system/api-token';
    return apiFetchJSON(url);
  }
  
  static async generateAPIToken(): Promise<APITokenStatus> {
    const result = await apiFetchJSON('/api/system/api-token/generate', {
      method: 'POST',
    });
    
    // Store the new token locally
    if (result.token) {
      localStorage.setItem('apiToken', result.token);
    }
    
    return result;
  }
  
  static async deleteAPIToken(): Promise<void> {
    await apiFetchJSON('/api/system/api-token/delete', {
      method: 'DELETE',
    });
    
    // Clear local storage
    localStorage.removeItem('apiToken');
  }
  
  // System Settings
  static async getSystemSettings(): Promise<SystemSettings> {
    return apiFetchJSON('/api/system/settings');
  }
  
  static async updateSystemSettings(settings: Partial<SystemSettings>): Promise<void> {
    await apiFetchJSON('/api/system/settings/update', {
      method: 'POST',
      body: JSON.stringify(settings),
    });
  }
}