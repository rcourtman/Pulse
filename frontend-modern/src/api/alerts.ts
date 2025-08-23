import type { Alert } from '@/types/api';
import type { AlertConfig } from '@/types/alerts';
import { apiFetchJSON } from '@/utils/apiClient';
// Error handling utilities available for future use
// import { handleError, createErrorBoundary } from '@/utils/errorHandler';

export class AlertsAPI {
  private static baseUrl = '/api/alerts';

  static async getActive(): Promise<Alert[]> {
    return apiFetchJSON(`${this.baseUrl}/active`);
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
    
    return apiFetchJSON(`${this.baseUrl}/history?${queryParams}`);
  }

  // Removed unused config methods - not implemented in backend

  static async acknowledge(alertId: string, user?: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/${alertId}/acknowledge`, {
      method: 'POST',
      body: JSON.stringify({ user }),
    });
  }

  // Alert configuration methods
  static async getConfig(): Promise<AlertConfig> {
    return apiFetchJSON(`${this.baseUrl}/config`);
  }

  static async updateConfig(config: AlertConfig): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/config`, {
      method: 'PUT',
      body: JSON.stringify(config),
    });
  }

  static async clearAlert(alertId: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/${alertId}/clear`, {
      method: 'POST',
    });
  }

  static async clearHistory(): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/history`, {
      method: 'DELETE',
    });
  }

  static async bulkAcknowledge(alertIds: string[], user?: string): Promise<{ results: Array<{ alertId: string; success: boolean; error?: string }> }> {
    return apiFetchJSON(`${this.baseUrl}/bulk/acknowledge`, {
      method: 'POST',
      body: JSON.stringify({ alertIds, user }),
    });
  }

  static async bulkClear(alertIds: string[]): Promise<{ results: Array<{ alertId: string; success: boolean; error?: string }> }> {
    return apiFetchJSON(`${this.baseUrl}/bulk/clear`, {
      method: 'POST',
      body: JSON.stringify({ alertIds }),
    });
  }
}