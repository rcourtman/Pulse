import type { StorageRecord } from './models';
import { formatRelativeTime } from '@/utils/format';
import {
  getStorageRecordIssueSummary,
  getStorageRecordStatus,
  getStorageRecordZfsPool,
} from './recordPresentation';

const titleize = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');

const getRecordDetails = (record: StorageRecord): Record<string, unknown> =>
  (record.details || {}) as Record<string, unknown>;

const getRecordStringDetail = (record: StorageRecord, key: string): string => {
  const value = getRecordDetails(record)[key];
  return typeof value === 'string' ? value.trim() : '';
};

export function getStoragePoolProtectionTextClass(record: StorageRecord): string {
  const label = getCompactStoragePoolProtectionLabel(record).trim().toLowerCase();
  if (record.rebuildInProgress) {
    return 'text-blue-700 dark:text-blue-300';
  }
  if (label === 'no parity') {
    return 'text-base-content';
  }
  if (record.protectionReduced || record.incidentCategory === 'recoverability') {
    return 'text-red-700 dark:text-red-300';
  }
  return 'text-base-content';
}

export function getStoragePoolIssueTextClass(record: StorageRecord): string {
  const severity = (record.incidentSeverity || record.health || '').trim().toLowerCase();
  if (severity === 'critical' || severity === 'offline') {
    return 'text-red-700 dark:text-red-300';
  }
  if (severity === 'warning') {
    return 'text-amber-700 dark:text-amber-300';
  }
  return 'text-base-content';
}

export function getStoragePoolStateTextClass(record: StorageRecord): string {
  if (record.freshness === 'stale') {
    return 'text-amber-700 dark:text-amber-300';
  }
  const normalized = getStoragePoolStateLabel(record).trim().toLowerCase();
  if (
    normalized === 'critical' ||
    normalized === 'faulted' ||
    normalized === 'failed' ||
    normalized === 'offline' ||
    normalized === 'unavailable'
  ) {
    return 'text-red-700 dark:text-red-300';
  }
  if (normalized === 'warning' || normalized === 'warn' || normalized === 'degraded') {
    return 'text-amber-700 dark:text-amber-300';
  }
  return 'text-base-content';
}

// The rebuild summary the backend promotes into the protection label is a
// sentence ("ZFS pool tank is resilvering (45.2%)"). The protection column is
// a badge, so a ZFS scan reads as the activity word, derived from the pool's
// own scan state (the same source the backend keys its risk reason on); the
// sentence stays available as the cell title. Non-ZFS rebuilds keep the label
// their platform supplies.
export function getStoragePoolRebuildLabel(record: StorageRecord): string {
  const pool = getStorageRecordZfsPool(record);
  if (!pool) return '';
  const scan = `${pool.scanDetails?.function || ''} ${pool.scan || ''}`.toLowerCase();
  const percentage = pool.scanDetails?.percentage;
  const progress =
    typeof percentage === 'number' && Number.isFinite(percentage) && percentage > 0
      ? ` ${Math.round(percentage)}%`
      : '';
  if (scan.includes('resilver')) return `Resilvering${progress}`;
  if (scan.includes('scrub')) return `Scrubbing${progress}`;
  return '';
}

export function getCompactStoragePoolProtectionLabel(record: StorageRecord): string {
  const label = (record.protectionLabel || '').trim();
  if (record.rebuildInProgress) {
    return getStoragePoolRebuildLabel(record) || label || '—';
  }
  if (record.protectionReduced) {
    return label || '—';
  }
  if (label && label.toLowerCase() !== 'healthy') {
    return label;
  }
  return '—';
}

export function getStoragePoolStateLabel(record: StorageRecord): string {
  if (record.freshness === 'stale') {
    return 'Stale';
  }
  const arrayState = getRecordStringDetail(record, 'arrayState');
  if (arrayState) {
    return titleize(arrayState);
  }
  const pool = getStorageRecordZfsPool(record);
  if (pool?.state) {
    // zpool reports upper-case states; present them like every other state
    // path so a DEGRADED pool reads the same as a degraded array.
    return titleize(pool.state);
  }
  const status = getStorageRecordStatus(record);
  return status ? titleize(status) : '—';
}

export function getStoragePoolStateTitle(record: StorageRecord): string {
  if (record.freshness === 'stale') {
    const observed = formatRelativeTime(record.observedAt);
    const age = observed ? ` Last successful refresh ${observed}.` : '';
    const error = record.freshnessError ? ` ${record.freshnessError}` : '';
    return `Retained last-known storage values.${age}${error}`.trim();
  }
  const label = getStoragePoolStateLabel(record);
  const summary =
    getCompactStoragePoolIssueSummary(record).trim() || getStorageRecordIssueSummary(record).trim();
  if (summary && summary.toLowerCase() !== 'healthy' && label.toLowerCase() !== 'started') {
    return summary;
  }
  return label === '—' ? '' : label;
}

export function getCompactStoragePoolProtectionTitle(record: StorageRecord): string {
  const label = getCompactStoragePoolProtectionLabel(record);
  if (label === '—') return '';
  const protectionSummary = (record.protectionSummary || '').trim();
  if (protectionSummary && protectionSummary.toLowerCase() !== label.toLowerCase()) {
    return protectionSummary;
  }
  if (record.protectionReduced || record.rebuildInProgress) {
    const issueSummary = getStorageRecordIssueSummary(record).trim();
    if (issueSummary && issueSummary.toLowerCase() !== 'healthy') {
      return issueSummary;
    }
  }
  // When the badge shows a derived word, the platform's full label is the title.
  const fullLabel = (record.protectionLabel || '').trim();
  return fullLabel && fullLabel.toLowerCase() !== label.toLowerCase() ? fullLabel : label;
}

export function getCompactStoragePoolImpactLabel(record: StorageRecord): string {
  if (
    (record.consumerCount || 0) > 0 ||
    (record.protectedWorkloadCount || 0) > 0 ||
    (record.affectedDatastoreCount || 0) > 0
  ) {
    return (record.impactSummary || '').trim() || '—';
  }
  return '—';
}

export function getCompactStoragePoolIssueLabel(record: StorageRecord): string {
  const label = (record.issueLabel || '').trim();
  const protection = getCompactStoragePoolProtectionLabel(record).trim();
  if (label && label.toLowerCase() !== 'healthy') {
    if (protection && protection !== '—' && protection.toLowerCase() === label.toLowerCase()) {
      return '—';
    }
    return label;
  }
  const pool = getStorageRecordZfsPool(record);
  if (pool?.state && pool.state !== 'ONLINE') {
    return pool.state;
  }
  const normalizedStatus = (record.statusLabel || '').trim().toLowerCase();
  if (
    normalizedStatus &&
    !['online', 'available', 'running', 'healthy'].includes(normalizedStatus)
  ) {
    return record.statusLabel || 'Issue';
  }
  return '—';
}

export function getCompactStoragePoolIssueSummary(record: StorageRecord): string {
  if (getCompactStoragePoolIssueLabel(record) === '—') return '';
  const summary = getStorageRecordIssueSummary(record).trim();
  if (summary && summary.toLowerCase() !== 'healthy') {
    return summary;
  }
  const pool = getStorageRecordZfsPool(record);
  if (!pool) return '';
  const errorParts: string[] = [];
  if ((pool.readErrors || 0) > 0) errorParts.push(`${pool.readErrors} read`);
  if ((pool.writeErrors || 0) > 0) errorParts.push(`${pool.writeErrors} write`);
  if ((pool.checksumErrors || 0) > 0) errorParts.push(`${pool.checksumErrors} checksum`);
  return errorParts.length > 0 ? `${errorParts.join(', ')} errors` : '';
}
