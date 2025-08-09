import type { Alert } from '@/types/api';
import type { AlertConfig } from '@/types/alerts';
// Error handling utilities available for future use
// import { handleError, createErrorBoundary } from '@/utils/errorHandler';

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

  // Alert configuration methods
  static async getConfig(): Promise<AlertConfig> {
    const response = await fetch(`${this.baseUrl}/config`);
    if (!response.ok) {
      throw new Error('Failed to fetch alert configuration');
    }
    return response.json();
  }

  static async updateConfig(config: AlertConfig): Promise<{ success: boolean }> {
    const response = await fetch(`${this.baseUrl}/config`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(config),
    });
    
    if (!response.ok) {
      throw new Error('Failed to update alert configuration');
    }
    
    return response.json();
  }

  static async clearAlert(alertId: string): Promise<{ success: boolean }> {
    const response = await fetch(`${this.baseUrl}/${alertId}/clear`, {
      method: 'POST',
    });
    
    if (!response.ok) {
      throw new Error('Failed to clear alert');
    }
    
    return response.json();
  }

  static async clearHistory(): Promise<{ success: boolean }> {
    const response = await fetch(`${this.baseUrl}/history`, {
      method: 'DELETE',
    });
    
    if (!response.ok) {
      throw new Error('Failed to clear alert history');
    }
    
    return response.json();
  }
}