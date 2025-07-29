import type { Alert } from '@/types/api';

export class AlertsAPI {
  private static baseUrl = '/api/alerts';

  static async getActive(): Promise<Alert[]> {
    const response = await fetch(`${this.baseUrl}/active`);
    if (!response.ok) {
      throw new Error('Failed to fetch active alerts');
    }
    return response.json();
  }

  static async getHistory(params?: {
    limit?: number;
    offset?: number;
    startTime?: string;
    endTime?: string;
    severity?: 'warning' | 'critical' | 'all';
    resourceId?: string;
  }): Promise<Alert[]> {
    const queryParams = new URLSearchParams();
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined) {
          queryParams.append(key, value.toString());
        }
      });
    }
    
    const response = await fetch(`${this.baseUrl}/history?${queryParams}`);
    if (!response.ok) {
      throw new Error('Failed to fetch alert history');
    }
    return response.json();
  }

  // Removed unused config methods - not implemented in backend

  static async acknowledge(alertId: string, user?: string): Promise<{ success: boolean }> {
    const response = await fetch(`${this.baseUrl}/${alertId}/acknowledge`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ user }),
    });
    
    if (!response.ok) {
      throw new Error('Failed to acknowledge alert');
    }
    
    return response.json();
  }

  // Removed unused notification test methods - not implemented in backend
}