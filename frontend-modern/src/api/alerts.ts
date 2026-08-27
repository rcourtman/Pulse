import type { Alert, AlertDeliveryDiagnosis, AlertEvent, Incident } from '@/types/api';
import type { AlertConfig, DeadManStatus } from '@/types/alerts';
import { apiFetchJSON } from '@/utils/apiClient';
import { arrayOrEmpty } from './responseUtils';

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
    severity?: 'info' | 'warning' | 'critical' | 'all';
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

  static async getDeliveryDiagnoses(): Promise<AlertDeliveryDiagnosis[]> {
    const diagnoses = (await apiFetchJSON(
      `${this.baseUrl}/delivery-diagnosis`,
    )) as AlertDeliveryDiagnosis[];
    return arrayOrEmpty<AlertDeliveryDiagnosis>(diagnoses);
  }

  static async getEvents(params?: {
    alertIdentifier?: string;
    types?: string[];
    since?: string;
    limit?: number;
  }): Promise<AlertEvent[]> {
    const query = new URLSearchParams();
    if (params?.alertIdentifier) query.set('alertIdentifier', params.alertIdentifier);
    if (params?.types?.length) query.set('type', params.types.join(','));
    if (params?.since) query.set('since', params.since);
    if (params?.limit) query.set('limit', String(params.limit));
    const suffix = query.size > 0 ? `?${query.toString()}` : '';
    const events = (await apiFetchJSON(`${this.baseUrl}/events${suffix}`)) as AlertEvent[];
    return arrayOrEmpty<AlertEvent>(events);
  }

  static async getIncidentTimeline(
    alertIdentifier: string,
    startedAt?: string,
  ): Promise<Incident | null> {
    const query = new URLSearchParams({ alertIdentifier });
    if (startedAt) {
      query.set('started_at', startedAt);
    }
    const incident = (await apiFetchJSON(
      `${this.baseUrl}/incidents?${query.toString()}`,
    )) as Incident | null;
    return incident;
  }

  static async getIncidentsForResource(resourceId: string, limit?: number): Promise<Incident[]> {
    const query = new URLSearchParams({ resource_id: resourceId });
    if (limit) query.set('limit', String(limit));
    const incidents = (await apiFetchJSON(
      `${this.baseUrl}/incidents?${query.toString()}`,
    )) as Incident[];
    return arrayOrEmpty<Incident>(incidents);
  }

  static async addIncidentNote(params: {
    alertIdentifier?: string;
    incidentId?: string;
    note: string;
    user?: string;
  }): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/incidents/note`, {
      method: 'POST',
      body: JSON.stringify({
        alertIdentifier: params.alertIdentifier,
        incident_id: params.incidentId,
        note: params.note,
        user: params.user,
      }),
    }) as Promise<{ success: boolean }>;
  }

  static async acknowledge(alertIdentifier: string, user?: string): Promise<{ success: boolean }> {
    // Use body-based endpoint to avoid URL encoding issues with reverse proxies
    return apiFetchJSON(`${this.baseUrl}/acknowledge`, {
      method: 'POST',
      body: JSON.stringify({ alertIdentifier, user }),
    });
  }

  static async unacknowledge(alertIdentifier: string): Promise<{ success: boolean }> {
    // Use body-based endpoint to avoid URL encoding issues with reverse proxies
    return apiFetchJSON(`${this.baseUrl}/unacknowledge`, {
      method: 'POST',
      body: JSON.stringify({ alertIdentifier }),
    });
  }

  static async snooze(
    alertIdentifier: string,
    until: string,
  ): Promise<{ success: boolean; snoozedUntil: string }> {
    return apiFetchJSON(`${this.baseUrl}/snooze`, {
      method: 'POST',
      body: JSON.stringify({ alertIdentifier, until }),
    });
  }

  static async unsnooze(alertIdentifier: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/unsnooze`, {
      method: 'POST',
      body: JSON.stringify({ alertIdentifier }),
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

  static async getDeadManStatus(): Promise<DeadManStatus> {
    return apiFetchJSON(`${this.baseUrl}/deadman/status`) as Promise<DeadManStatus>;
  }

  static async getDeadManConfig(): Promise<{ pingUrl: string; configured: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/deadman/config`) as Promise<{
      pingUrl: string;
      configured: boolean;
    }>;
  }

  static async updateDeadManConfig(pingUrl: string): Promise<{
    success: boolean;
    configured: boolean;
  }> {
    return apiFetchJSON(`${this.baseUrl}/deadman/config`, {
      method: 'PUT',
      body: JSON.stringify({ pingUrl }),
    }) as Promise<{ success: boolean; configured: boolean }>;
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
    alertIdentifiers: string[],
    user?: string,
  ): Promise<{
    results: Array<{ alertIdentifier: string; success: boolean; error?: string }>;
  }> {
    const response = (await apiFetchJSON(`${this.baseUrl}/bulk/acknowledge`, {
      method: 'POST',
      body: JSON.stringify({ alertIdentifiers, user }),
    })) as {
      results?: Array<{
        alertIdentifier: string;
        success: boolean;
        error?: string;
      }>;
    };
    return {
      ...response,
      results: arrayOrEmpty<{
        alertIdentifier: string;
        success: boolean;
        error?: string;
      }>(response.results),
    };
  }
}
