// Simple types - no complex validation needed

export class SettingsAPI {
  private static baseUrl = '/api';

  static async getSettings() {
    const response = await fetch(`${this.baseUrl}/settings`);
    
    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(errorText || 'Failed to fetch settings');
    }
    
    return response.json();
  }

  static async updateSettings(settings: any) {
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
    
    return response.json();
  }

  static async validateSettings(settings: any) {
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
    
    return response.json();
  }
}