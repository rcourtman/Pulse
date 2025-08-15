// System API for managing system settings
import { apiFetchJSON } from '@/utils/apiClient';

export interface SystemSettings {
  pollingInterval: number;
  updateChannel?: string;
  autoUpdateEnabled: boolean;
  autoUpdateCheckInterval?: number;
  autoUpdateTime?: string;
  // apiToken removed - now handled via security API
}

export class SystemAPI {
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