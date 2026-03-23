import { apiFetchJSON } from '@/utils/apiClient';

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
  status: string; // "online" | "offline" | "unknown"
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
    explanation: {
      summary:
        explanation?.summary ??
        'Pulse counts this top-level collection path as one monitored system.',
      reasons: explanation?.reasons ?? [],
      surfaces: explanation?.surfaces ?? [],
    },
  };
}
