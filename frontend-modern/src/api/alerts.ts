import type { Alert, Incident } from '@/types/api';
import type { AlertConfig } from '@/types/alerts';
import { apiFetchJSON } from '@/utils/apiClient';

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

  static async getIncidentTimeline(alertId: string, startedAt?: string): Promise<Incident | null> {
    const query = new URLSearchParams({ alert_id: alertId });
    if (startedAt) {
      query.set('started_at', startedAt);
    }
    return apiFetchJSON(`${this.baseUrl}/incidents?${query.toString()}`) as Promise<Incident | null>;
  }

  static async getIncidentsForResource(resourceId: string, limit?: number): Promise<Incident[]> {
    const query = new URLSearchParams({ resource_id: resourceId });
    if (limit) query.set('limit', String(limit));
    return apiFetchJSON(`${this.baseUrl}/incidents?${query.toString()}`) as Promise<Incident[]>;
  }

  static async addIncidentNote(params: {
    alertId?: string;
    incidentId?: string;
    note: string;
    user?: string;
  }): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/incidents/note`, {
      method: 'POST',
      body: JSON.stringify({
        alert_id: params.alertId,
        incident_id: params.incidentId,
        note: params.note,
        user: params.user,
      }),
    }) as Promise<{ success: boolean }>;
  }

  static async acknowledge(alertId: string, user?: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(alertId)}/acknowledge`, {
      method: 'POST',
      body: JSON.stringify({ user }),
    });
  }

  static async unacknowledge(alertId: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/${encodeURIComponent(alertId)}/unacknowledge`, {
      method: 'POST',
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

  static async activate(): Promise<{ success: boolean; state: string; activationTime?: string }> {
    return apiFetchJSON(`${this.baseUrl}/activate`, {
      method: 'POST',
    });
  }

  static async clearHistory(): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/history`, {
      method: 'DELETE',
    });
  }

  static async bulkAcknowledge(
    alertIds: string[],
    user?: string,
  ): Promise<{ results: Array<{ alertId: string; success: boolean; error?: string }> }> {
    return apiFetchJSON(`${this.baseUrl}/bulk/acknowledge`, {
      method: 'POST',
      body: JSON.stringify({ alertIds, user }),
    });
  }

}
