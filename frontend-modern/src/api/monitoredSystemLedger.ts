import { apiFetchJSON } from '@/utils/apiClient';

export type MonitoredSystemLedgerStatus = 'online' | 'warning' | 'offline' | 'unknown';

export interface MonitoredSystemLedgerExplanationReason {
  kind: string;
  signal: string;
  summary: string;
}

export interface MonitoredSystemLedgerExplanationSurface {
  name: string;
  type: string;
  source: string;
}

export interface MonitoredSystemLedgerExplanation {
  summary: string;
  reasons: MonitoredSystemLedgerExplanationReason[];
  surfaces: MonitoredSystemLedgerExplanationSurface[];
}

export interface MonitoredSystemLedgerEntry {
  name: string;
  type: string;
  status: MonitoredSystemLedgerStatus;
  last_seen: string; // RFC3339 or empty
  source: string;
  explanation?: MonitoredSystemLedgerExplanation;
}

export interface MonitoredSystemLedgerResponse {
  systems: MonitoredSystemLedgerEntry[];
  total: number;
  limit: number; // 0 = unlimited
}

export class MonitoredSystemLedgerAPI {
  private static readonly baseUrl = '/api/license/monitored-system-ledger';

  static async getLedger(): Promise<MonitoredSystemLedgerResponse> {
    const response = await apiFetchJSON<MonitoredSystemLedgerResponse>(this.baseUrl);
    return {
      ...response,
      systems: (response.systems ?? []).map(normalizeMonitoredSystemLedgerEntry),
    };
  }
}

function normalizeMonitoredSystemLedgerEntry(
  entry: MonitoredSystemLedgerEntry,
): MonitoredSystemLedgerEntry {
  const explanation = entry.explanation;
  return {
    ...entry,
    status: normalizeMonitoredSystemLedgerStatus(entry.status),
    explanation: {
      summary:
        explanation?.summary ??
        'Pulse counts this top-level collection path as one monitored system.',
      reasons: explanation?.reasons ?? [],
      surfaces: explanation?.surfaces ?? [],
    },
  };
}

function normalizeMonitoredSystemLedgerStatus(
  status: MonitoredSystemLedgerStatus | string | null | undefined,
): MonitoredSystemLedgerStatus {
  switch ((status ?? '').trim().toLowerCase()) {
    case 'online':
    case 'warning':
    case 'offline':
    case 'unknown':
      return status.trim().toLowerCase() as MonitoredSystemLedgerStatus;
    default:
      return 'unknown';
  }
}
