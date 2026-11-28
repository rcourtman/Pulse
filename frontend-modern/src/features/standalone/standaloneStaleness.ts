import type { Resource } from '@/types/resource';
import { getResourceStaleness } from '@/features/platformPage/resourceStaleness';
import { getAgentFirstReportStatusIndicator } from '@/utils/agentResources';
import { formatRelativeTime } from '@/utils/format';
import { getResourceSourceStatus, hasImpairedResourceSource } from '@/utils/resourceSourceHealth';
import { getSimpleStatusIndicator, type StatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { getStandaloneResourceStatusIndicator } from './standalonePageModel';

const AGENT_REPORT_STALE_AFTER_MS = 5 * 60 * 1000;
const HEALTHY_AGENT_STATUSES = new Set(['online', 'running', 'healthy', 'connected', 'ok']);

export interface StandaloneStaleNotice {
  badge: string;
  compactBadge: string;
  age: string | null;
  detail: string;
  cueTitle: string;
  error?: string;
}

const relativeAge = (value: unknown, nowMs: number): string => {
  const input =
    typeof value === 'number'
      ? Number.isFinite(value) && value > 0
        ? value
        : undefined
      : asTrimmedString(value);
  return input === undefined
    ? ''
    : formatRelativeTime(input, { compact: true, emptyText: '', now: nowMs });
};

const isAgentSourceImpaired = (resource: Resource, ignorePendingAndOffline = true): boolean => {
  if (hasImpairedResourceSource(resource, 'agent')) return true;
  const status = (asTrimmedString(resource.sourceStatus?.agent?.status) ?? '').toLowerCase();
  if (!status || HEALTHY_AGENT_STATUSES.has(status)) return false;
  return !ignorePendingAndOffline || (status !== 'pending' && status !== 'offline');
};

export const hasExplicitBackendAgentStaleness = (machine: Resource | undefined | null): boolean =>
  Boolean(
    machine &&
    machine.type === 'agent' &&
    !getAgentFirstReportStatusIndicator(machine) &&
    (machine.agent?.stale === true ||
      getResourceStaleness(machine) ||
      isAgentSourceImpaired(machine)),
  );

export const getStandaloneMachineStaleness = (
  machine: Resource | undefined | null,
  nowMs = Date.now(),
  options?: { includeClockStale?: boolean },
): StandaloneStaleNotice | null => {
  if (!machine || machine.type !== 'agent' || getAgentFirstReportStatusIndicator(machine)) {
    return null;
  }
  if (getSimpleStatusIndicator(machine.status).variant === 'danger') return null;

  const healthStale = getResourceStaleness(machine);
  const lastSeenStale =
    options?.includeClockStale !== false &&
    Number.isFinite(machine.lastSeen) &&
    machine.lastSeen > 0 &&
    nowMs - machine.lastSeen > AGENT_REPORT_STALE_AFTER_MS;

  if (
    !healthStale &&
    !isAgentSourceImpaired(machine) &&
    machine.agent?.stale !== true &&
    !lastSeenStale
  ) {
    return null;
  }

  const agentSource = getResourceSourceStatus(machine, 'agent') ?? machine.sourceStatus?.agent;
  const rel =
    relativeAge(machine.agent?.lastReportAt, nowMs) ||
    relativeAge(agentSource?.lastSeen, nowMs) ||
    relativeAge(machine.lastSeen, nowMs);
  const age =
    healthStale?.age ??
    (rel && rel !== 'just now' && rel !== '—' ? rel.replace(/\s+ago$/i, '').trim() || null : null);
  const error = asTrimmedString(agentSource?.error);
  const lastReportText = rel ? ` Last report ${rel}.` : age ? ` Last report ${age} ago.` : '';
  const errorText = error ? ` Last error: ${error}.` : '';
  const cueBase = age
    ? `No report for ${age} — showing last known readings.`
    : 'Showing last known readings — no fresh update received recently.';

  return {
    badge: age ? `Stale · ${age}` : 'Stale data',
    compactBadge: 'Stale',
    age,
    detail: `Agent has stopped reporting.${lastReportText} Showing last known readings.${errorText}`,
    cueTitle: `${cueBase}${errorText}`,
    ...(error ? { error } : {}),
  };
};

export const getStandaloneMachineStatusIndicator = (
  machine: Resource,
  nowMs = Date.now(),
): StatusIndicator => {
  const base = getStandaloneResourceStatusIndicator(machine, nowMs);
  if (base.variant === 'danger' || getAgentFirstReportStatusIndicator(machine)) return base;
  return getStandaloneMachineStaleness(machine, nowMs)
    ? { variant: 'warning', label: 'Stale' }
    : base;
};

export const getStandaloneVMStaleness = (
  vm: Resource,
  parentHost?: Resource,
  nowMs = Date.now(),
): StandaloneStaleNotice | null => {
  const healthStale = getResourceStaleness(vm);
  const parentStale = parentHost
    ? getStandaloneMachineStaleness(parentHost, nowMs, { includeClockStale: false })
    : null;
  if (!healthStale && !isAgentSourceImpaired(vm, false) && !parentStale) return null;

  const agentSource = getResourceSourceStatus(vm, 'agent') ?? vm.sourceStatus?.agent;
  const age = healthStale?.age ?? parentStale?.age ?? null;
  const error = asTrimmedString(agentSource?.error) || parentStale?.error;
  const detail =
    !age && !error
      ? 'Host agent has not reported recently'
      : `Host agent has not reported recently${age ? ` (${age} ago)` : ''}. Showing last known VM state and metrics.${error ? ` Last error: ${error}.` : ''}`;

  return {
    badge: age ? `stale · ${age}` : 'stale',
    compactBadge: 'stale',
    age,
    detail,
    cueTitle: detail,
    ...(error ? { error } : {}),
  };
};

const formatStaleSourceList = (
  entries: readonly { name: string; age: string | null }[],
): string => {
  const seen = new Set<string>();
  const labels: string[] = [];
  for (const entry of entries) {
    const key = entry.name.trim().toLowerCase();
    if (!key || seen.has(key)) continue;
    seen.add(key);
    labels.push(entry.age ? `${entry.name} (${entry.age})` : entry.name);
  }
  if (labels.length <= 1) return labels[0] ?? '';
  if (labels.length === 2) return `${labels[0]} and ${labels[1]}`;
  if (labels.length === 3) return `${labels[0]}, ${labels[1]}, and ${labels[2]}`;
  return `${labels[0]}, ${labels[1]}, ${labels[2]}, +${labels.length - 3} more`;
};

export const describeStaleStandaloneMachines = (
  entries: readonly { name: string; age: string | null }[],
): string | null => {
  const list = formatStaleSourceList(entries);
  return list
    ? `Telemetry is stale for ${list}. CPU, memory, disk, and sensor readings shown are last known values until their agents report again.`
    : null;
};

export const describeStaleStandaloneVMs = (
  entries: readonly { name: string; age: string | null }[],
): string | null => {
  const list = formatStaleSourceList(entries);
  return list
    ? `Host agent telemetry is stale for ${list}. VM states and metrics shown for affected guests are last known values.`
    : null;
};
