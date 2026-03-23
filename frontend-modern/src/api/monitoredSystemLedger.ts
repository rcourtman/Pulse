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

export interface MonitoredSystemLedgerStatusExplanation {
  summary: string;
  reasons: MonitoredSystemLedgerStatusReason[];
}

export type MonitoredSystemLedgerStatusReasonStatus =
  | 'online'
  | 'stale'
  | 'offline'
  | 'unknown';

export interface MonitoredSystemLedgerStatusReason {
  kind: string;
  name: string;
  type: string;
  source: string;
  status: MonitoredSystemLedgerStatusReasonStatus;
  last_seen: string;
  summary: string;
}

export interface MonitoredSystemLedgerEntry {
  name: string;
  type: string;
  status: MonitoredSystemLedgerStatus;
  status_explanation?: MonitoredSystemLedgerStatusExplanation;
  latest_included_signal?: MonitoredSystemLedgerLatestSignal;
  latest_included_signal_at: string; // freshest included observation, RFC3339 or empty
  latest_included_signal_source?: string;
  last_seen?: string; // deprecated compatibility alias
  source: string;
  explanation?: MonitoredSystemLedgerExplanation;
}

export interface MonitoredSystemLedgerLatestSignal {
  name: string;
  type: string;
  source?: string;
  at: string;
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
  const status = normalizeMonitoredSystemLedgerStatus(entry.status);
  const latestIncludedSignal = normalizeMonitoredSystemLedgerLatestSignal(
    entry.latest_included_signal,
    entry,
  );
  return {
    ...entry,
    status,
    latest_included_signal: latestIncludedSignal,
    latest_included_signal_at: latestIncludedSignal.at,
    latest_included_signal_source: normalizeMonitoredSystemLedgerSource(
      latestIncludedSignal.source,
    ),
    status_explanation: {
      summary: entry.status_explanation?.summary ?? defaultMonitoredSystemStatusExplanation(status),
      reasons: (entry.status_explanation?.reasons ?? []).map(normalizeMonitoredSystemLedgerStatusReason),
    },
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

function defaultMonitoredSystemStatusExplanation(status: MonitoredSystemLedgerStatus): string {
  switch (status) {
    case 'online':
      return 'All included top-level collection paths currently report online status.';
    case 'warning':
      return 'At least one included top-level collection path is degraded, so Pulse marks this monitored system as warning.';
    case 'offline':
      return 'At least one included source is offline or disconnected, so Pulse marks this monitored system as offline.';
    default:
      return 'Pulse cannot determine a canonical runtime status for this monitored system yet.';
  }
}

function normalizeMonitoredSystemLedgerStatusReason(
  reason: MonitoredSystemLedgerStatusReason,
): MonitoredSystemLedgerStatusReason {
  return {
    ...reason,
    status: normalizeMonitoredSystemLedgerStatusReasonStatus(reason.status),
    last_seen: reason.last_seen ?? '',
  };
}

function normalizeMonitoredSystemLedgerStatusReasonStatus(
  status: MonitoredSystemLedgerStatusReasonStatus | string | null | undefined,
): MonitoredSystemLedgerStatusReasonStatus {
  switch ((status ?? '').trim().toLowerCase()) {
    case 'online':
    case 'stale':
    case 'offline':
    case 'unknown':
      return status.trim().toLowerCase() as MonitoredSystemLedgerStatusReasonStatus;
    default:
      return 'unknown';
  }
}

function normalizeMonitoredSystemLedgerSource(
  source: string | null | undefined,
): string | undefined {
  switch ((source ?? '').trim().toLowerCase()) {
    case 'agent':
    case 'docker':
    case 'kubernetes':
    case 'pbs':
    case 'pmg':
    case 'proxmox':
    case 'truenas':
      return source?.trim().toLowerCase();
    default:
      return undefined;
  }
}

function normalizeMonitoredSystemLedgerLatestSignal(
  signal: MonitoredSystemLedgerLatestSignal | undefined,
  entry: MonitoredSystemLedgerEntry,
): MonitoredSystemLedgerLatestSignal {
  return {
    name: signal?.name?.trim() || entry.name || 'Unnamed source',
    type: signal?.type?.trim() || entry.type || 'system',
    source: normalizeMonitoredSystemLedgerSource(
      signal?.source ?? entry.latest_included_signal_source ?? (entry.source !== 'multiple' ? entry.source : ''),
    ),
    at: signal?.at?.trim() || entry.latest_included_signal_at?.trim() || entry.last_seen?.trim() || '',
  };
}
