import type { Alert, AlertDeliveryDiagnosis, AlertEvent, Incident } from '@/types/api';
import type { AlertConfig, DeadManStatus } from '@/types/alerts';
import { apiFetchJSON } from '@/utils/apiClient';
import { arrayOrEmpty } from './responseUtils';

// A completed HTTP response does not confirm an alert-state mutation. Keep
// unconfirmed bodies on the existing error/rollback path, without echoing
// response diagnostics or issuing another write.
function confirmedAlertAction(value: unknown): Record<string, unknown> {
  if (
    !value ||
    typeof value !== 'object' ||
    Array.isArray(value) ||
    (value as Record<string, unknown>).success !== true ||
    (value as Record<string, unknown>).error !== undefined
  ) {
    throw new Error('Alert action could not be confirmed. Refresh alerts before trying again.');
  }
  return value as Record<string, unknown>;
}

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

  static async getEvents(
    params?: {
      alertIdentifier?: string;
      types?: string[];
      since?: string;
      limit?: number;
    },
    signal?: AbortSignal,
  ): Promise<AlertEvent[]> {
    const query = new URLSearchParams();
    if (params?.alertIdentifier) query.set('alertIdentifier', params.alertIdentifier);
    if (params?.types?.length) query.set('type', params.types.join(','));
    if (params?.since) query.set('since', params.since);
    if (params?.limit) query.set('limit', String(params.limit));
    const suffix = query.size > 0 ? `?${query.toString()}` : '';
    const events = (await apiFetchJSON(
      `${this.baseUrl}/events${suffix}`,
      signal ? { signal } : undefined,
    )) as AlertEvent[];
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
    const result = await apiFetchJSON<unknown>(`${this.baseUrl}/acknowledge`, {
      method: 'POST',
      body: JSON.stringify({ alertIdentifier, user }),
    });
    confirmedAlertAction(result);
    return { success: true };
  }

  static async unacknowledge(alertIdentifier: string): Promise<{ success: boolean }> {
    // Use body-based endpoint to avoid URL encoding issues with reverse proxies
    const result = await apiFetchJSON<unknown>(`${this.baseUrl}/unacknowledge`, {
      method: 'POST',
      body: JSON.stringify({ alertIdentifier }),
    });
    confirmedAlertAction(result);
    return { success: true };
  }

  static async snooze(
    alertIdentifier: string,
    until: string,
  ): Promise<{ success: boolean; snoozedUntil: string }> {
    const result = confirmedAlertAction(
      await apiFetchJSON<unknown>(`${this.baseUrl}/snooze`, {
        method: 'POST',
        body: JSON.stringify({ alertIdentifier, until }),
      }),
    );
    // The producer echoes the requested instant in UTC, not necessarily the
    // same string form. Another expiry cannot confirm this local hold.
    if (
      typeof result.snoozedUntil !== 'string' ||
      !Number.isFinite(Date.parse(until)) ||
      Date.parse(result.snoozedUntil) !== Date.parse(until)
    ) {
      throw new Error('Alert snooze could not be confirmed. Refresh alerts before trying again.');
    }
    return { success: true, snoozedUntil: result.snoozedUntil };
  }

  static async unsnooze(alertIdentifier: string): Promise<{ success: boolean }> {
    const result = await apiFetchJSON<unknown>(`${this.baseUrl}/unsnooze`, {
      method: 'POST',
      body: JSON.stringify({ alertIdentifier }),
    });
    confirmedAlertAction(result);
    return { success: true };
  }

  // Alert configuration methods
  static async getConfig(signal?: AbortSignal): Promise<AlertConfig> {
    return apiFetchJSON(
      `${this.baseUrl}/config`,
      signal ? { signal } : undefined,
    ) as Promise<AlertConfig>;
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

  static async getDeadManConfig(
    signal?: AbortSignal,
  ): Promise<{ pingUrl: string; configured: boolean }> {
    return apiFetchJSON(
      `${this.baseUrl}/deadman/config`,
      signal ? { signal } : undefined,
    ) as Promise<{
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
