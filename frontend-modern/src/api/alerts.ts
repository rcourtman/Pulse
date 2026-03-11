import type { Alert, Incident } from '@/types/api';
import type { AlertConfig } from '@/types/alerts';
import { apiFetchJSON } from '@/utils/apiClient';

export class AlertsAPI {
  private static baseUrl = '/api/alerts';

  private static normalizeAlertResult(result: {
    alertIdentifier?: string;
    alertId?: string;
    success: boolean;
    error?: string;
  }): {
    alertIdentifier?: string;
    alertId?: string;
    success: boolean;
    error?: string;
  } {
    const alertIdentifier = result.alertIdentifier ?? result.alertId;
    return {
      ...result,
      ...(alertIdentifier ? { alertIdentifier } : {}),
      ...(result.alertId ?? alertIdentifier ? { alertId: result.alertId ?? alertIdentifier } : {}),
    };
  }

  private static normalizeIncident(incident: Incident | null): Incident | null {
    if (!incident) {
      return null;
    }
    const alertIdentifier = incident.alertIdentifier ?? incident.alertId;
    return {
      ...incident,
      ...(alertIdentifier ? { alertIdentifier } : {}),
      ...(incident.alertId ?? alertIdentifier ? { alertId: incident.alertId ?? alertIdentifier } : {}),
    };
  }

  private static normalizeIncidents(incidents: Incident[]): Incident[] {
    return incidents.map((incident) => this.normalizeIncident(incident) as Incident);
  }

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
    const query = new URLSearchParams({ alert_identifier: alertId });
    if (startedAt) {
      query.set('started_at', startedAt);
    }
    const incident = (await apiFetchJSON(
      `${this.baseUrl}/incidents?${query.toString()}`,
    )) as Incident | null;
    return this.normalizeIncident(incident);
  }

  static async getIncidentsForResource(resourceId: string, limit?: number): Promise<Incident[]> {
    const query = new URLSearchParams({ resource_id: resourceId });
    if (limit) query.set('limit', String(limit));
    const incidents = (await apiFetchJSON(
      `${this.baseUrl}/incidents?${query.toString()}`,
    )) as Incident[];
    return this.normalizeIncidents(incidents || []);
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
        alertIdentifier: params.alertId,
        incident_id: params.incidentId,
        note: params.note,
        user: params.user,
      }),
    }) as Promise<{ success: boolean }>;
  }

  static async acknowledge(alertId: string, user?: string): Promise<{ success: boolean }> {
    // Use body-based endpoint to avoid URL encoding issues with reverse proxies
    return apiFetchJSON(`${this.baseUrl}/acknowledge`, {
      method: 'POST',
      body: JSON.stringify({ alertIdentifier: alertId, user }),
    });
  }

  static async unacknowledge(alertId: string): Promise<{ success: boolean }> {
    // Use body-based endpoint to avoid URL encoding issues with reverse proxies
    return apiFetchJSON(`${this.baseUrl}/unacknowledge`, {
      method: 'POST',
      body: JSON.stringify({ alertIdentifier: alertId }),
    });
  }

  // Alert configuration methods
  static async getConfig(): Promise<AlertConfig> {
    return apiFetchJSON(`${this.baseUrl}/config`) as Promise<AlertConfig>;
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
  ): Promise<{
    results: Array<{ alertIdentifier?: string; alertId?: string; success: boolean; error?: string }>;
  }> {
    const response = (await apiFetchJSON(`${this.baseUrl}/bulk/acknowledge`, {
      method: 'POST',
      body: JSON.stringify({ alertIdentifiers: alertIds, user }),
    })) as {
      results?: Array<{
        alertIdentifier?: string;
        alertId?: string;
        success: boolean;
        error?: string;
      }>;
    };
    return {
      ...response,
      results: (response.results || []).map((result) => this.normalizeAlertResult(result)),
    };
  }
}
