import type { StorageRecord } from './models';
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

export function getCompactStoragePoolProtectionLabel(record: StorageRecord): string {
  const label = (record.protectionLabel || '').trim();
  if (record.rebuildInProgress || record.protectionReduced) {
    return label || '—';
  }
  if (label && label.toLowerCase() !== 'healthy') {
    return label;
  }
  return '—';
}

export function getStoragePoolStateLabel(record: StorageRecord): string {
  const arrayState = getRecordStringDetail(record, 'arrayState');
  if (arrayState) {
    return titleize(arrayState);
  }
  const pool = getStorageRecordZfsPool(record);
  if (pool?.state) {
    return pool.state === 'ONLINE' ? 'Online' : pool.state;
  }
  const status = getStorageRecordStatus(record);
  return status ? titleize(status) : '—';
}

export function getStoragePoolStateTitle(record: StorageRecord): string {
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
  return label;
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
