import type { 
  SettingsResponse, 
  SettingsUpdateRequest
} from '@/types/settings';

// System settings type matching Go backend
export interface SystemSettingsUpdate {
  pollingInterval: number; // in seconds
  backendPort?: number;
  frontendPort?: number;
  allowedOrigins?: string;
  connectionTimeout?: number; // in seconds
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
    const response = await fetch(`${this.baseUrl}/settings`);
    
    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(errorText || 'Failed to fetch settings');
    }
    
    return response.json() as Promise<SettingsResponse>;
  }

  // Full settings update (legacy - avoid using)
  static async updateSettings(settings: SettingsUpdateRequest): Promise<ApiResponse> {
    const response = await fetch(`${this.baseUrl}/settings/update`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(settings),
    });
    
    if (!response.ok) {
      throw new Error('Failed to update settings');
    }
    
    return response.json() as Promise<ApiResponse>;
  }
  
  // System settings update (preferred)
  static async updateSystemSettings(settings: SystemSettingsUpdate): Promise<ApiResponse> {
    const response = await fetch(`${this.baseUrl}/config/system`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(settings),
    });
    
    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(errorText || 'Failed to update system settings');
    }
    
    return response.json() as Promise<ApiResponse>;
  }

  static async validateSettings(settings: SettingsUpdateRequest): Promise<ApiResponse> {
    const response = await fetch(`${this.baseUrl}/settings/validate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(settings),
    });
    
    if (!response.ok) {
      throw new Error('Failed to validate settings');
    }
    
    return response.json() as Promise<ApiResponse>;
  }
}