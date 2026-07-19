import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';
import { formatRelativeTime } from '@/utils/format';

export interface AvailabilityProbePresentation {
  methodLabel: string;
  targetLabel: string | null;
  resultLabel: string;
  freshnessLabel: 'fresh' | 'stale' | 'freshness unknown';
  correlationLabel: string | null;
  netIoLabel: string;
  rowLabel: string;
  detailLabel: string;
  toneClassName: string;
}

type AvailabilityProbeResource = Pick<
  Resource,
  'type' | 'platformType' | 'status' | 'availability' | 'platformData'
>;

const AVAILABILITY_PROBE_SUCCESS_CLASS = 'text-emerald-600 dark:text-emerald-300';
const AVAILABILITY_PROBE_WARNING_CLASS = 'text-amber-600 dark:text-amber-300';
const AVAILABILITY_PROBE_ERROR_CLASS = 'text-red-600 dark:text-red-300';
const AVAILABILITY_PROBE_UNKNOWN_CLASS = 'text-muted';

const normalizeAvailabilityProtocol = (protocol?: string | null): string =>
  (protocol ?? '').trim().toLowerCase();

export const getAvailabilityProbeMethodLabel = (
  availability?: ResourceAvailabilityMeta | null,
): string => {
  const protocol = normalizeAvailabilityProtocol(availability?.protocol);
  if (protocol === 'icmp') return 'ICMP';
  if (protocol === 'tcp') {
    return availability?.port ? `TCP ${availability.port}` : 'TCP';
  }
  if (protocol === 'http' || protocol === 'https') {
    const path = (availability?.path ?? '').trim();
    return path ? `${protocol.toUpperCase()} ${path}` : protocol.toUpperCase();
  }
  return protocol ? protocol.toUpperCase() : 'Probe';
};

export const getAvailabilityProbeTargetLabel = (
  availability?: ResourceAvailabilityMeta | null,
): string | null => {
  const protocol = normalizeAvailabilityProtocol(availability?.protocol);
  if (protocol === 'tcp') {
    const port = availability?.port;
    return typeof port === 'number' && Number.isFinite(port) && port > 0 ? String(port) : null;
  }
  if (protocol === 'http' || protocol === 'https') {
    const path = (availability?.path ?? '').trim();
    return path || null;
  }
  return null;
};

export const getAvailabilityProbeEndpointLabel = (
  availability?: ResourceAvailabilityMeta | null,
): string => {
  const address = (availability?.address ?? '').trim();
  const protocol = normalizeAvailabilityProtocol(availability?.protocol);
  const port = availability?.port;
  const addressWithPort =
    address &&
    typeof port === 'number' &&
    Number.isFinite(port) &&
    port > 0 &&
    !address.endsWith(`:${port}`)
      ? `${address}:${port}`
      : address;
  if ((protocol === 'http' || protocol === 'https') && availability?.path) {
    const path = availability.path.trim();
    if (path && !addressWithPort.endsWith(path)) {
      return `${addressWithPort.replace(/\/+$/, '')}${path.startsWith('/') ? path : `/${path}`}`;
    }
  }
  return addressWithPort;
};

const getAvailabilityProbeFailureLabel = (availability: ResourceAvailabilityMeta): string => {
  const lastError = (availability.lastError ?? '').trim();
  const normalizedError = lastError.toLowerCase();
  if (normalizedError.includes('timed out') || normalizedError.includes('timeout')) {
    return 'timed out';
  }
  const httpStatus = lastError.match(/\b([45]\d{2})\b/);
  if (httpStatus) {
    return httpStatus[1];
  }
  return 'failed';
};

const getAvailabilityProbeResultLabel = (
  resource: Pick<Resource, 'status'>,
  availability: ResourceAvailabilityMeta,
): string => {
  const latency = availability.latencyMillis;
  const normalizedStatus = (resource.status ?? '').trim().toLowerCase();
  if (availability.available === false || ['offline', 'degraded'].includes(normalizedStatus)) {
    return getAvailabilityProbeFailureLabel(availability);
  }
  if (typeof latency === 'number' && Number.isFinite(latency) && latency >= 0) {
    return `${Math.round(latency)} ms`;
  }
  if (availability.available === true || normalizedStatus === 'online') {
    return 'reachable';
  }
  return 'not checked';
};

const getAvailabilityProbeToneClassName = (
  resource: Pick<Resource, 'status'>,
  availability: ResourceAvailabilityMeta,
  freshnessLabel: AvailabilityProbePresentation['freshnessLabel'],
): string => {
  const normalizedStatus = (resource.status ?? '').trim().toLowerCase();
  if (availability.available === false || normalizedStatus === 'offline') {
    return AVAILABILITY_PROBE_ERROR_CLASS;
  }
  if (
    normalizedStatus === 'degraded' ||
    freshnessLabel === 'stale' ||
    availability.correlationState === 'ambiguous' ||
    availability.correlationState === 'unresolved'
  ) {
    return AVAILABILITY_PROBE_WARNING_CLASS;
  }
  if (availability.available === true || normalizedStatus === 'online') {
    return AVAILABILITY_PROBE_SUCCESS_CLASS;
  }
  return AVAILABILITY_PROBE_UNKNOWN_CLASS;
};

const getAvailabilityFreshnessLabel = (
  availability: ResourceAvailabilityMeta,
  now: Date,
): AvailabilityProbePresentation['freshnessLabel'] => {
  const explicitValidUntil = availability.evidence?.validUntil;
  const validUntilMillis = explicitValidUntil ? Date.parse(explicitValidUntil) : Number.NaN;
  if (Number.isFinite(validUntilMillis)) {
    return validUntilMillis >= now.getTime() ? 'fresh' : 'stale';
  }

  const checkedMillis = availability.lastChecked
    ? Date.parse(availability.lastChecked)
    : Number.NaN;
  const pollIntervalSeconds = availability.pollIntervalSeconds;
  if (
    Number.isFinite(checkedMillis) &&
    typeof pollIntervalSeconds === 'number' &&
    Number.isFinite(pollIntervalSeconds) &&
    pollIntervalSeconds > 0
  ) {
    return checkedMillis + pollIntervalSeconds * 2_000 >= now.getTime() ? 'fresh' : 'stale';
  }
  return 'freshness unknown';
};

const getAvailabilityCorrelationLabel = (availability: ResourceAvailabilityMeta): string | null => {
  switch (availability.correlationState) {
    case 'ambiguous':
      return availability.correlationCandidates && availability.correlationCandidates > 1
        ? `${availability.correlationCandidates} possible resource matches`
        : 'Resource match is ambiguous';
    case 'unresolved':
      return 'Resource link is unresolved';
    case 'standalone':
      return 'Standalone endpoint';
    default:
      return null;
  }
};

const getFailureCountLabel = (availability: ResourceAvailabilityMeta): string | null => {
  const failures = availability.consecutiveFailures;
  if (typeof failures !== 'number' || !Number.isFinite(failures) || failures <= 0) {
    return null;
  }
  const threshold = availability.failureThreshold;
  if (typeof threshold === 'number' && Number.isFinite(threshold) && threshold > 0) {
    return `${failures}/${threshold} failures`;
  }
  return failures === 1 ? '1 failure' : `${failures} failures`;
};

export const getAvailabilityProbePresentation = (
  resource: AvailabilityProbeResource,
  now = new Date(),
): AvailabilityProbePresentation | null => {
  const platformAvailability = resource.platformData?.availability as
    ResourceAvailabilityMeta | undefined;
  const availability = resource.availability ?? platformAvailability;
  if (!availability) {
    return null;
  }

  const methodLabel = getAvailabilityProbeMethodLabel(availability);
  const targetLabel = getAvailabilityProbeTargetLabel(availability);
  const resultLabel = getAvailabilityProbeResultLabel(resource, availability);
  const freshnessLabel = getAvailabilityFreshnessLabel(availability, now);
  const correlationLabel = getAvailabilityCorrelationLabel(availability);
  const netIoLabel = targetLabel ? `${targetLabel}: ${resultLabel}` : resultLabel;
  const checked = formatRelativeTime(availability.lastChecked);
  const failures = getFailureCountLabel(availability);
  const detailParts = [`${methodLabel} - ${resultLabel}`, freshnessLabel];
  if (checked) detailParts.push(`checked ${checked}`);
  if (correlationLabel) detailParts.push(correlationLabel);
  if (failures) detailParts.push(failures);
  if (availability.lastSuccess && availability.available === false) {
    const lastSuccess = formatRelativeTime(availability.lastSuccess);
    if (lastSuccess) detailParts.push(`last success ${lastSuccess}`);
  }
  if (availability.lastError) detailParts.push(availability.lastError);

  const rowLabel = checked ? `${netIoLabel} - checked ${checked}` : netIoLabel;

  return {
    methodLabel,
    targetLabel,
    resultLabel,
    freshnessLabel,
    correlationLabel,
    netIoLabel,
    rowLabel,
    detailLabel: detailParts.join(' - '),
    toneClassName: getAvailabilityProbeToneClassName(resource, availability, freshnessLabel),
  };
};
