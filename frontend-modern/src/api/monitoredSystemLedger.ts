import { apiFetchJSON } from '@/utils/apiClient';
import {
  getMonitoredSystemExplanationFallbackSummary,
  getMonitoredSystemStatusFallbackSummary,
} from '@/utils/monitoredSystemPresentation';

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
  status_explanation: MonitoredSystemLedgerStatusExplanation;
  latest_included_signal: MonitoredSystemLedgerLatestSignal;
  source: string;
  explanation: MonitoredSystemLedgerExplanation;
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

type MonitoredSystemLedgerRawEntry = Omit<
  MonitoredSystemLedgerEntry,
  'status_explanation' | 'latest_included_signal' | 'explanation'
> & {
  status_explanation?: MonitoredSystemLedgerStatusExplanation;
  latest_included_signal?: MonitoredSystemLedgerLatestSignal;
  explanation?: MonitoredSystemLedgerExplanation;
};

type MonitoredSystemLedgerRawResponse = Omit<MonitoredSystemLedgerResponse, 'systems'> & {
  systems?: MonitoredSystemLedgerRawEntry[];
};

export class MonitoredSystemLedgerAPI {
  private static readonly baseUrl = '/api/license/monitored-system-ledger';

  static async getLedger(): Promise<MonitoredSystemLedgerResponse> {
    const response = await apiFetchJSON<MonitoredSystemLedgerRawResponse>(this.baseUrl);
    return {
      ...response,
      systems: (response.systems ?? []).map(normalizeMonitoredSystemLedgerEntry),
    };
  }
}

function normalizeMonitoredSystemLedgerEntry(
  entry: MonitoredSystemLedgerRawEntry,
): MonitoredSystemLedgerEntry {
  const explanation = entry.explanation;
  const status = normalizeMonitoredSystemLedgerStatus(entry.status);
  const latestIncludedSignal = normalizeMonitoredSystemLedgerLatestSignal(
    entry.latest_included_signal,
    entry,
  );
  return {
    name: entry.name,
    type: entry.type,
    status,
    source: entry.source,
    latest_included_signal: latestIncludedSignal,
    status_explanation: {
      summary: entry.status_explanation?.summary ?? getMonitoredSystemStatusFallbackSummary(status),
      reasons: (entry.status_explanation?.reasons ?? []).map(normalizeMonitoredSystemLedgerStatusReason),
    },
    explanation: {
      summary: explanation?.summary ?? getMonitoredSystemExplanationFallbackSummary(),
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
  entry: MonitoredSystemLedgerRawEntry,
): MonitoredSystemLedgerLatestSignal {
  return {
    name: signal?.name?.trim() || entry.name || 'Unnamed source',
    type: signal?.type?.trim() || entry.type || 'system',
    source: normalizeMonitoredSystemLedgerSource(
      signal?.source ?? (entry.source !== 'multiple' ? entry.source : ''),
    ),
    at: signal?.at?.trim() || '',
  };
}
