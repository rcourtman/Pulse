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

export type MonitoredSystemLedgerStatusReasonStatus = 'online' | 'stale' | 'offline' | 'unknown';

export interface MonitoredSystemLedgerStatusReason {
  kind: string;
  name: string;
  type: string;
  source: string;
  status: MonitoredSystemLedgerStatusReasonStatus;
  reported_at: string;
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

export interface MonitoredSystemLedgerExplainRequest {
  candidate?: MonitoredSystemLedgerPreviewCandidate | null;
  replacement?: MonitoredSystemLedgerPreviewReplacement | null;
}

export interface MonitoredSystemLedgerPreviewCandidate {
  source: string;
  type?: string;
  name?: string;
  hostname?: string;
  host_url?: string;
  agent_id?: string;
  machine_id?: string;
  resource_id?: string;
}

export interface MonitoredSystemLedgerPreviewReplacement {
  source?: string;
  name?: string;
  hostname?: string;
  host_url?: string;
  agent_id?: string;
  machine_id?: string;
  resource_id?: string;
}

export interface MonitoredSystemLedgerPreviewRequest {
  candidate: MonitoredSystemLedgerPreviewCandidate;
  replacement?: MonitoredSystemLedgerPreviewReplacement | null;
}

export type MonitoredSystemLedgerPreviewEffect =
  | 'creates_new'
  | 'attaches_existing'
  | 'replaces_existing'
  | 'splits_existing';

export interface MonitoredSystemLedgerPreviewResponse {
  current_count: number;
  projected_count: number;
  additional_count: number;
  limit: number;
  would_exceed_limit: boolean;
  effect: MonitoredSystemLedgerPreviewEffect | string;
  current_systems: MonitoredSystemLedgerEntry[];
  projected_systems: MonitoredSystemLedgerEntry[];
  current_system: MonitoredSystemLedgerEntry | null;
  projected_system: MonitoredSystemLedgerEntry | null;
}

export interface MonitoredSystemLedgerExplainResponse {
  ledger: MonitoredSystemLedgerResponse;
  preview: MonitoredSystemLedgerPreviewResponse | null;
}

type MonitoredSystemLedgerRawEntry = Omit<
  MonitoredSystemLedgerEntry,
  'status_explanation' | 'latest_included_signal' | 'explanation'
> & {
  status_explanation?: MonitoredSystemLedgerRawStatusExplanation;
  latest_included_signal?: MonitoredSystemLedgerLatestSignal;
  explanation?: MonitoredSystemLedgerExplanation;
};

type MonitoredSystemLedgerRawResponse = Omit<MonitoredSystemLedgerResponse, 'systems'> & {
  systems?: MonitoredSystemLedgerRawEntry[];
};

type MonitoredSystemLedgerRawExplainResponse = {
  ledger?: MonitoredSystemLedgerRawResponse;
  preview?: MonitoredSystemLedgerRawPreviewResponse | null;
};

export type MonitoredSystemLedgerRawPreviewResponse = Omit<
  MonitoredSystemLedgerPreviewResponse,
  'current_systems' | 'projected_systems' | 'current_system' | 'projected_system'
> & {
  current_systems?: MonitoredSystemLedgerRawEntry[];
  projected_systems?: MonitoredSystemLedgerRawEntry[];
  current_system?: MonitoredSystemLedgerRawEntry | null;
  projected_system?: MonitoredSystemLedgerRawEntry | null;
};

interface MonitoredSystemLedgerRawStatusExplanation extends Omit<
  MonitoredSystemLedgerStatusExplanation,
  'reasons'
> {
  reasons?: MonitoredSystemLedgerRawStatusReason[];
}

interface MonitoredSystemLedgerRawStatusReason extends Omit<
  MonitoredSystemLedgerStatusReason,
  'reported_at'
> {
  reported_at?: string;
}

export class MonitoredSystemLedgerAPI {
  private static readonly baseUrl = '/api/license/monitored-system-ledger';

  static async getLedger(): Promise<MonitoredSystemLedgerResponse> {
    const response = await apiFetchJSON<MonitoredSystemLedgerRawResponse>(this.baseUrl);
    return normalizeMonitoredSystemLedgerResponse(response);
  }

  static async preview(
    request: MonitoredSystemLedgerPreviewRequest,
  ): Promise<MonitoredSystemLedgerPreviewResponse> {
    const response = await apiFetchJSON<MonitoredSystemLedgerRawPreviewResponse>(
      `${this.baseUrl}/preview`,
      {
        method: 'POST',
        body: JSON.stringify(request),
      },
    );
    return normalizeMonitoredSystemLedgerPreviewResponse(response);
  }

  static async explain(
    request: MonitoredSystemLedgerExplainRequest = {},
  ): Promise<MonitoredSystemLedgerExplainResponse> {
    const response = await apiFetchJSON<MonitoredSystemLedgerRawExplainResponse>(
      `${this.baseUrl}/explain`,
      {
        method: 'POST',
        body: JSON.stringify(request),
      },
    );
    return normalizeMonitoredSystemLedgerExplainResponse(response);
  }
}

export function normalizeMonitoredSystemLedgerResponse(
  response: MonitoredSystemLedgerRawResponse,
): MonitoredSystemLedgerResponse {
  return {
    ...response,
    systems: (response.systems ?? []).map(normalizeMonitoredSystemLedgerEntry),
  };
}

export function normalizeMonitoredSystemLedgerPreviewResponse(
  response: MonitoredSystemLedgerRawPreviewResponse,
): MonitoredSystemLedgerPreviewResponse {
  const currentSystems = (response.current_systems ?? []).map(normalizeMonitoredSystemLedgerEntry);
  const projectedSystems = (response.projected_systems ?? []).map(
    normalizeMonitoredSystemLedgerEntry,
  );
  return {
    ...response,
    current_systems: currentSystems,
    projected_systems: projectedSystems,
    current_system:
      response.current_system != null
        ? normalizeMonitoredSystemLedgerEntry(response.current_system)
        : currentSystems.length === 1
          ? currentSystems[0]
          : null,
    projected_system:
      response.projected_system != null
        ? normalizeMonitoredSystemLedgerEntry(response.projected_system)
        : projectedSystems.length === 1
          ? projectedSystems[0]
          : null,
  };
}

export function normalizeMonitoredSystemLedgerExplainResponse(
  response: MonitoredSystemLedgerRawExplainResponse,
): MonitoredSystemLedgerExplainResponse {
  const rawLedger: MonitoredSystemLedgerRawResponse = response.ledger ?? {
    systems: [],
    total: 0,
    limit: 0,
  };
  return {
    ledger: normalizeMonitoredSystemLedgerResponse(rawLedger),
    preview:
      response.preview != null
        ? normalizeMonitoredSystemLedgerPreviewResponse(response.preview)
        : null,
  };
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
      reasons: (entry.status_explanation?.reasons ?? []).map(
        normalizeMonitoredSystemLedgerStatusReason,
      ),
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
  reason: MonitoredSystemLedgerRawStatusReason,
): MonitoredSystemLedgerStatusReason {
  return {
    kind: reason.kind,
    name: reason.name,
    type: reason.type,
    source: reason.source,
    status: normalizeMonitoredSystemLedgerStatusReasonStatus(reason.status),
    reported_at: reason.reported_at ?? '',
    summary: reason.summary,
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
    case 'vmware':
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
