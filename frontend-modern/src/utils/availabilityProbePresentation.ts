import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';
import { formatRelativeTime } from '@/utils/format';

export interface AvailabilityProbePresentation {
  methodLabel: string;
  targetLabel: string | null;
  resultLabel: string;
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
): string => {
  const normalizedStatus = (resource.status ?? '').trim().toLowerCase();
  if (availability.available === false || normalizedStatus === 'offline') {
    return AVAILABILITY_PROBE_ERROR_CLASS;
  }
  if (normalizedStatus === 'degraded') {
    return AVAILABILITY_PROBE_WARNING_CLASS;
  }
  if (availability.available === true || normalizedStatus === 'online') {
    return AVAILABILITY_PROBE_SUCCESS_CLASS;
  }
  return AVAILABILITY_PROBE_UNKNOWN_CLASS;
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
): AvailabilityProbePresentation | null => {
  const platformAvailability = resource.platformData?.availability as
    | ResourceAvailabilityMeta
    | undefined;
  const availability = resource.availability ?? platformAvailability;
  if (
    !availability ||
    (resource.type !== 'network-endpoint' && resource.platformType !== 'availability')
  ) {
    return null;
  }

  const methodLabel = getAvailabilityProbeMethodLabel(availability);
  const targetLabel = getAvailabilityProbeTargetLabel(availability);
  const resultLabel = getAvailabilityProbeResultLabel(resource, availability);
  const netIoLabel = targetLabel ? `${targetLabel}: ${resultLabel}` : resultLabel;
  const checked = formatRelativeTime(availability.lastChecked);
  const failures = getFailureCountLabel(availability);
  const detailParts = [`${methodLabel} - ${resultLabel}`];
  if (checked) detailParts.push(`checked ${checked}`);
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
    netIoLabel,
    rowLabel,
    detailLabel: detailParts.join(' - '),
    toneClassName: getAvailabilityProbeToneClassName(resource, availability),
  };
};
