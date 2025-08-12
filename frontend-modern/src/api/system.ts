// System API for managing system settings including API tokens

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
    const response = await fetch('/api/system/api-token');
    if (!response.ok) {
      throw new Error('Failed to get API token status');
    }
    return response.json();
  }
  
  static async generateAPIToken(): Promise<APITokenStatus> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    
    // Include existing token if we have one
    const existingToken = localStorage.getItem('apiToken');
    if (existingToken) {
      headers['X-API-Token'] = existingToken;
    }
    
    const response = await fetch('/api/system/api-token/generate', {
      method: 'POST',
      headers,
    });
    
    if (!response.ok) {
      throw new Error('Failed to generate API token');
    }
    
    const result = await response.json();
    
    // Store the new token locally
    if (result.token) {
      localStorage.setItem('apiToken', result.token);
    }
    
    return result;
  }
  
  static async deleteAPIToken(): Promise<void> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    
    // Include existing token for auth
    const existingToken = localStorage.getItem('apiToken');
    if (existingToken) {
      headers['X-API-Token'] = existingToken;
    }
    
    const response = await fetch('/api/system/api-token/delete', {
      method: 'DELETE',
      headers,
    });
    
    if (!response.ok) {
      throw new Error('Failed to delete API token');
    }
    
    // Clear local storage
    localStorage.removeItem('apiToken');
  }
  
  // System Settings
  static async getSystemSettings(): Promise<SystemSettings> {
    const response = await fetch('/api/system/settings');
    if (!response.ok) {
      throw new Error('Failed to get system settings');
    }
    return response.json();
  }
  
  static async updateSystemSettings(settings: Partial<SystemSettings>): Promise<void> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    
    // Include API token if configured
    const apiToken = localStorage.getItem('apiToken');
    if (apiToken) {
      headers['X-API-Token'] = apiToken;
    }
    
    const response = await fetch('/api/system/settings/update', {
      method: 'POST',
      headers,
      body: JSON.stringify(settings),
    });
    
    if (!response.ok) {
      throw new Error('Failed to update system settings');
    }
  }
}