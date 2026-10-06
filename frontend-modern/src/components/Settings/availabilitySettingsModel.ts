import type { AvailabilityTarget, AvailabilityTargetKind } from '@/api/availabilityTargets';
import type { Resource } from '@/types/resource';
import { getStandaloneResourceStatusIndicator } from '@/features/standalone/standalonePageModel';
import {
  PROBE_AGENT_STALE_LABEL,
  getProbeSourceChipLabel,
  isProbeAgentStaleStatus,
  type ProbeAgentOption,
} from '@/utils/availabilityProbeAgents';

export const AVAILABILITY_SETTINGS_PATH = '/settings/monitoring/availability';
export const AVAILABILITY_ADD_QUERY_PARAM = 'add';
export const AVAILABILITY_ADD_TARGET_VALUE = 'target';
export const AVAILABILITY_TARGET_KIND_QUERY_PARAM = 'targetKind';

export function buildAvailabilitySettingsPath(): string {
  return AVAILABILITY_SETTINGS_PATH;
}

const AVAILABILITY_TARGET_KIND_VALUES: readonly AvailabilityTargetKind[] = [
  'machine',
  'service',
  'device',
];

export function normalizeAvailabilityTargetKind(
  value: string | null | undefined,
): AvailabilityTargetKind | undefined {
  const normalized = value?.trim().toLowerCase();
  return AVAILABILITY_TARGET_KIND_VALUES.find((kind) => kind === normalized);
}

export function buildAvailabilityTargetAddPath(targetKind?: AvailabilityTargetKind): string {
  const params = new URLSearchParams();
  params.set(AVAILABILITY_ADD_QUERY_PARAM, AVAILABILITY_ADD_TARGET_VALUE);
  if (targetKind) {
    params.set(AVAILABILITY_TARGET_KIND_QUERY_PARAM, targetKind);
  }
  return `${AVAILABILITY_SETTINGS_PATH}?${params.toString()}`;
}

export function shouldOpenAvailabilityTargetAddDialog(pathname: string, search: string): boolean {
  if (pathname !== AVAILABILITY_SETTINGS_PATH && pathname !== `${AVAILABILITY_SETTINGS_PATH}/`) {
    return false;
  }
  const params = new URLSearchParams(search);
  if (params.get(AVAILABILITY_ADD_QUERY_PARAM)?.trim() !== AVAILABILITY_ADD_TARGET_VALUE) {
    return false;
  }
  if (!params.has(AVAILABILITY_TARGET_KIND_QUERY_PARAM)) return true;
  return Boolean(normalizeAvailabilityTargetKind(params.get(AVAILABILITY_TARGET_KIND_QUERY_PARAM)));
}

export function getAvailabilityTargetAddKind(
  pathname: string,
  search: string,
): AvailabilityTargetKind | undefined {
  if (!shouldOpenAvailabilityTargetAddDialog(pathname, search)) return undefined;
  return normalizeAvailabilityTargetKind(
    new URLSearchParams(search).get(AVAILABILITY_TARGET_KIND_QUERY_PARAM),
  );
}

export function getAvailabilityTargetMethodLabel(target: AvailabilityTarget): string {
  switch (target.protocol) {
    case 'icmp':
      return 'ICMP ping';
    case 'tcp':
      return target.port ? `TCP ${target.port}` : 'TCP port';
    case 'udp':
      return target.port ? `UDP ${target.port}` : 'UDP port';
    case 'http':
      return 'HTTP check';
    default:
      return String(target.protocol).toUpperCase();
  }
}

export function getAvailabilityTargetKindLabel(target: AvailabilityTarget): string {
  switch (target.targetKind) {
    case 'machine':
      return 'Machine';
    case 'device':
      return 'Device';
    case 'service':
    case undefined:
      return 'Service';
    default:
      return 'Endpoint';
  }
}

export function getAvailabilityTargetAddressLabel(target: AvailabilityTarget): string {
  if (target.protocol === 'http') {
    const path = target.path?.trim();
    if (path && !target.address.endsWith(path)) {
      const normalizedAddress = target.address.replace(/\/+$/, '');
      const normalizedPath = path.startsWith('/') ? path : `/${path}`;
      return `${normalizedAddress}${normalizedPath}`;
    }
    return target.address;
  }
  if ((target.protocol === 'tcp' || target.protocol === 'udp') && target.port) {
    return `${target.address}:${target.port}`;
  }
  return target.address;
}

export function getAvailabilityTargetStatusLabel(target: AvailabilityTarget): string {
  if (!target.enabled) return 'Paused';
  const status = target.status;
  if (!status) return 'Not checked yet';
  if (status.aggregateState === 'degraded') return 'Observation paths disagree';
  if (status.aggregateState === 'unknown') {
    return `${status.reportingLocations ?? 0}/${status.expectedLocations ?? status.locations?.length ?? 0} locations reporting`;
  }
  if (status.aggregateState === 'unavailable') return 'Unavailable from all locations';
  if (status.aggregateState === 'healthy' && (status.expectedLocations ?? 0) > 1) {
    return `Available from all ${status.expectedLocations} locations`;
  }
  // A probe-assigned check whose agent stopped reporting derives to
  // indeterminate at read time. It shares the warning treatment with the UDP
  // open-or-filtered case but needs its own copy.
  if (isProbeAgentStaleStatus(status)) return PROBE_AGENT_STALE_LABEL;
  if (status.outcome === 'indeterminate') return 'Open or filtered';
  if (status.available) {
    return typeof status.latencyMillis === 'number'
      ? `Online · ${status.latencyMillis} ms`
      : 'Online';
  }
  return status.lastError?.trim() || 'Offline';
}

// One classification for a check, shared with the Machines availability tab.
// When the check's unified resource is loaded, its Machines status indicator
// decides (so stale evidence and unresolved identity count the same way on
// both pages). Without it, the target's own status is read the way
// availability_poller.go derives the resource status: a failing probe is
// offline only once its consecutive failures reach the failure threshold
// (default 2) and needs attention before that.
export type AvailabilityTargetHealth = 'paused' | 'pending' | 'healthy' | 'attention' | 'offline';

const DEFAULT_AVAILABILITY_FAILURE_THRESHOLD = 2;

const availabilityFailureThreshold = (target: AvailabilityTarget): number => {
  const configured = target.status?.failureThreshold ?? target.failureThreshold;
  return typeof configured === 'number' && configured > 0
    ? configured
    : DEFAULT_AVAILABILITY_FAILURE_THRESHOLD;
};

// `nowMs` is the caller's shared relative-time clock: a loaded check turns
// stale by time alone, so the classification must not freeze at render.
export function getAvailabilityTargetHealth(
  target: AvailabilityTarget,
  resource?: Resource,
  nowMs: number = Date.now(),
): AvailabilityTargetHealth {
  if (!target.enabled) return 'paused';
  if (resource) {
    const variant = getStandaloneResourceStatusIndicator(resource, nowMs).variant;
    if (variant === 'success') return 'healthy';
    if (variant === 'warning') return 'attention';
    if (variant === 'danger') return 'offline';
    return 'pending';
  }
  const status = target.status;
  if (!status) return 'pending';
  if (isProbeAgentStaleStatus(status)) return 'attention';
  const failures = status.consecutiveFailures ?? 0;
  const thresholdReached = failures >= availabilityFailureThreshold(target);
  switch (status.aggregateState) {
    case 'healthy':
      return 'healthy';
    case 'degraded':
    case 'unknown':
      return 'attention';
    case 'unavailable':
      return thresholdReached ? 'offline' : 'attention';
  }
  if (!status.lastChecked) return 'pending';
  if (status.available) return 'healthy';
  return thresholdReached ? 'offline' : 'attention';
}

/** Hover text for a check that is failing but not yet offline. */
export function getAvailabilityTargetStatusTitle(
  target: AvailabilityTarget,
  resource?: Resource,
  nowMs: number = Date.now(),
): string | undefined {
  if (getAvailabilityTargetHealth(target, resource, nowMs) !== 'attention') return undefined;
  const status = target.status;
  const failures = status?.consecutiveFailures;
  if (status?.available !== false || typeof failures !== 'number' || failures <= 0) {
    return undefined;
  }
  const threshold = availabilityFailureThreshold(target);
  return `${failures} failed ${failures === 1 ? 'check' : 'checks'} in a row. It counts as offline after ${threshold}.`;
}

const AVAILABILITY_HEALTH_CLASS: Record<AvailabilityTargetHealth, string> = {
  paused: 'bg-surface-alt text-muted',
  pending: 'bg-sky-100 text-sky-700 dark:bg-sky-900/25 dark:text-sky-300',
  healthy: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300',
  attention: 'bg-amber-100 text-amber-700 dark:bg-amber-900/25 dark:text-amber-300',
  offline: 'bg-rose-100 text-rose-700 dark:bg-rose-900/25 dark:text-rose-300',
};

export function getAvailabilityTargetStatusClass(
  target: AvailabilityTarget,
  resource?: Resource,
  nowMs: number = Date.now(),
): string {
  return AVAILABILITY_HEALTH_CLASS[getAvailabilityTargetHealth(target, resource, nowMs)];
}

/**
 * Source attribution for the latest observation. Returns null when the check
 * ran locally, so the chip only appears for probe-reported results.
 */
export function getAvailabilityTargetProbeSourceLabel(
  target: AvailabilityTarget,
  probeAgentOptions: readonly ProbeAgentOption[],
): string | null {
  const locationCount =
    target.status?.locations?.length ?? target.observationLocationIds?.length ?? 0;
  if (locationCount > 1) return `${locationCount} observation locations`;
  return getProbeSourceChipLabel(probeAgentOptions, target.status?.probeAgentId);
}

// The same words and buckets as the Machines availability summary: every
// unhealthy check needs attention, and offline ones are also named, so the two
// pages never report different counts for the same checks.
export function getAvailabilityTargetsSummary(
  targets: readonly AvailabilityTarget[],
  resourceFor: (target: AvailabilityTarget) => Resource | undefined = () => undefined,
  nowMs: number = Date.now(),
): string {
  if (targets.length === 0) return 'No availability checks configured';
  const enabled = targets.filter((target) => target.enabled).length;
  const health = targets.map((target) =>
    getAvailabilityTargetHealth(target, resourceFor(target), nowMs),
  );
  const offline = health.filter((state) => state === 'offline').length;
  const attention = health.filter((state) => state === 'attention' || state === 'offline').length;
  if (attention === 0) return `${enabled} enabled · ${targets.length} total`;
  const parts = [`${attention} ${attention === 1 ? 'needs' : 'need'} attention`];
  if (offline > 0) parts.push(`${offline} offline`);
  return `${parts.join(' · ')} · ${enabled} enabled`;
}
